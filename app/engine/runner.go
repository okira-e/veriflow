package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/oops"
	. "github.com/okira-e/veriflow/app/opt"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/oliveagle/jsonpath"
)

type RunnerSettings struct {
	// The settings file for this project.
	Cfg             *config.Cfg
	BaseUrlOverride string
}

func (self *RunnerSettings) getBaseUrl() string {
	if self.BaseUrlOverride != "" {
		return self.BaseUrlOverride
	}

	return self.Cfg.BaseUrl
}

type Runner struct {
	settings RunnerSettings
	RunId    string
	stepsRan int
	// cookieJar is a per-run cookie jar to maintain stateful cookies across steps.
	//
	// it implicitly stores and sends cookies for each request.
	cookieJar *cookiejar.Jar
	symtable  map[string]any
}

func NewRunner(settings RunnerSettings) *Runner {
	runId := utils.NewId()

	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create cookie jar: %v", err))
	}

	return &Runner{
		settings:  settings,
		RunId:     runId,
		stepsRan:  0,
		cookieJar: cookieJar,
		symtable:  map[string]any{},
	}
}

// Execute a step.
//
// Returns an AssertionFailure on an error caused from assertion failure which is not an actual error.
func (self *Runner) Execute(step *app.Step) error {
	baseCtx := context.Background()
	timeout := 30 * time.Second // have a default
	if step.Options.Timeout.IsSome() {
		var err error
		timeout, err = utils.ToDuration(step.Options.Timeout.Unwrap())
		if err != nil {
			return oops.Err(oops.StepRequestProcessingFailed, "failed to parse timeout duration for step", err)
		}
	}

	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	// Setup request body

	self.processBindingsForStep(step)

	var body io.Reader
	if step.Request.Json.IsSome() {
		payload := step.Request.Json.Unwrap()
		b, err := json.Marshal(payload)
		if err != nil {
			return oops.Err(oops.StepRequestBuildFailed, "failed to marshal JSON body", err)
		}

		body = bytes.NewReader(b)
	}

	url := fmt.Sprintf("%s%s", self.settings.getBaseUrl(), step.Request.Path)
	method := strings.ToUpper(step.Request.Method)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return oops.Err(oops.Internal, "failed to initialize the request for the step", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request

	client := http.DefaultClient
	if !step.Request.DisableHeaders {
		client = &http.Client{
			Jar: self.cookieJar,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return &AssertionFailure{
				Err:      oops.Err(oops.StepRequestDeadlineExceeded, "request was cancelled by context deadline", err),
				Response: nil,
				Step:     step,
			}
		} else {
			return oops.Err(oops.StepRequestFailed, "failed to execute the request for the step", err)
		}
	}
	defer resp.Body.Close()

	self.stepsRan += 1

	responseBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return oops.Err(oops.StepResponseReadFailed, "failed to read response body", err)
	}

	// Validate assertion

	err = validateAssertClause(&step.Assert, resp.StatusCode, responseBodyBytes)
	if err != nil {
		return &AssertionFailure{
			Err:      oops.Err(oops.StepRequestAssertionFailed, "step request assertion failed", err),
			Response: responseBodyBytes,
			Step:     step,
		}
	}

	// Set exports if they exist

	if len(step.Exports) != 0 {
		for ident, jspath := range step.Exports {
			var body map[string]any
			err := json.Unmarshal(responseBodyBytes, &body)
			if err != nil {
				return oops.Err(oops.StepResponseReadFailed, "failed to unmarshal response body for exports", err)
			}

			value, err := jsonpath.JsonPathLookup(body, jspath)
			if err != nil {
				return oops.Err(oops.StepExportFailed, fmt.Sprintf("failed to lookup jsonpath %s for export %s", jspath, ident), err)
			}

			self.symtable[ident] = value
		}
	}

	return nil
}

func (self *Runner) StepsRan() int {
	return self.stepsRan
}

func (self *Runner) TotalSteps() int {
	return self.settings.Cfg.GetTotalSteps()
}

func (self *Runner) processBindingsForStep(step *app.Step) error {
	// process url path since that might be injected for a dynamic route
	step.Request.Path = self.resolveBindingFromString(step.Request.Path)

	// Process body
	if step.Request.Json.IsSome() {
		requestBody := step.Request.Json.Unwrap()
		processedRequestBody, err := self.processRequestBody(requestBody)
		if err != nil {
			return oops.Err(oops.StepRequestProcessingFailed, "failed to process request body", err)
		}
		step.Request.Json = Some(processedRequestBody)
	}

	// Process assertions
	if step.Assert.All.IsSome() {
		assertions := step.Assert.All.Unwrap()
		for _, assertion := range assertions {
			if assertion.Contains.IsSome() {
				contains := assertion.Contains.Unwrap()
				assertion.Contains = Some(self.resolveBindingFromString(contains))
			}
			if assertion.Equals.IsSome() {
				equals := assertion.Equals.Unwrap()
				assertion.Equals = Some(self.resolveBindingFromString(equals))
			}
		}
	}

	return nil
}

// processRequestBody takes the request body and processes them by replacing any injectable variable (like {{RUN_ID}}) with its value.
func (self *Runner) processRequestBody(body map[string]any) (map[string]any, error) {
	var err error

	for k, v := range body {
		switch val := v.(type) {
		case string:
			body[k] = self.resolveBindingFromString(val)
		case map[string]any:
			body[k], err = self.processRequestBody(val)
			if err != nil {
				return nil, oops.Err(oops.StepRequestProcessingFailed, "failed to process/modify request body", err)
			}
		// optionally handle []any if you expect arrays
		case []any:
			for i, elem := range val {
				if s, ok := elem.(string); ok {
					val[i] = self.resolveBindingFromString(s)
				}
			}
			body[k] = val
		}
	}

	return body, nil
}

// resolveBindingFromString takes a string and replaces any injectable bindings built-in or defined
//
// Example 1: "test-{{RUN_ID}}" -> "test-12345"
//
// Example 2: "test-{{RUN_ID}}" -> "test-12345"
func (self *Runner) resolveBindingFromString(s string) string {
	reBinding := regexp.MustCompile(`\{\{([a-zA-Z0-9_:-]+)\}\}`)

	return reBinding.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-2] // strip {{ }}

		// built-ins
		resolved, ok := self.resolveBuiltinBinding(inner)
		if ok {
			return resolved
		}

		// user bindings: bind:key
		if strings.HasPrefix(inner, "bind:") {
			key := inner[5:]
			if v, ok := self.symtable[key]; ok {
				return fmt.Sprintf("%v", v)
			}
			return m // unresolved stays as-is
		}

		return m
	})
}

func (self *Runner) resolveBuiltinBinding(str string) (string, bool) {
	switch str {
	case "RUN_ID":
		return self.RunId, true
	case "RAND_DIGIT":
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			panic("rng unavailable: veriflow requires OS entropy")
		}
		return fmt.Sprintf("%d", n.Int64()), true
	}

	return "", false
}
