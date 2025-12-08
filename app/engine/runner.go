package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cliopts"
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
	RunInParallel   bool
	// MaxConcurrent is the max number of concurrent flows to run which works if RunInParallel is set to true.
	MaxConcurrent int
	// DryRun makes the runner validate and print all steps without sending the requests.
	DryRun bool
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
	flowsRan int
	stepsRan int
}

func NewRunner(settings RunnerSettings) *Runner {
	runId := utils.NewId()

	return &Runner{
		settings: settings,
		RunId:    runId,
	}
}

func (self *Runner) Execute() error {
	for _, flow := range self.settings.Cfg.Flows {
		err := self.ExecuteFlow(flow)
		self.flowsRan += 1
		if err != nil {
			// Flag the flow that failed if the error is an ExecutionError.
			var execFailure *AssertionFailure
			if errors.As(err, &execFailure) {
				execFailure.Flow = flow
				return execFailure
			} else {
				return err
			}
		}
	}

	return nil
}

func (self *Runner) ExecuteFlow(flow *app.Flow) error {
	symtable := map[string]any{}

	for i, step := range flow.Steps {
		err := self.ExecuteStep(step, i, symtable)
		self.stepsRan += 1
		if err != nil {
			// Flag the step that failed if the error is an ExecutionError.
			var assertionFailure *AssertionFailure
			if errors.As(err, &assertionFailure) {
				assertionFailure.Step = step
				return assertionFailure
			} else {
				return err
			}
		}
	}

	return nil
}

// Executes a step.
//
// Returns an AssertionFailure on an error caused from assertion failure which is not an actual error.
func (self *Runner) ExecuteStep(step *app.Step, i int, symtable map[string]any) error {
	baseCtx := context.Background()
	timeout := step.Options.Timeout
	if step.Options.Timeout == 0 {
		timeout = 30 * time.Second // have a default
	}

	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	// Setup request body

	self.processBindingsForStep(step, symtable)

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
	req, err := http.NewRequestWithContext(ctx, step.Request.Method, url, body)
	if err != nil {
		return oops.Err(oops.Internal, "failed to initialize the request for the step", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return oops.Err(oops.StepRequestDeadlineExceeded, "request was cancelled by context deadline", err)
		} else {
			return oops.Err(oops.StepRequestFailed, "failed to execute the request for the step", err)
		}
	}
	defer resp.Body.Close()

	responseBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return oops.Err(oops.StepResponseReadFailed, "failed to read response body", err)
	}

	// Validate assertion

	err = step.Assert.Validate(resp.StatusCode, responseBodyBytes)
	if err != nil {
		return &AssertionFailure{
			Err:      oops.Err(oops.StepRequestAssertionFailed, "step request assertion failed", err),
			Response: responseBodyBytes,
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

			symtable[ident] = value
		}
	}

	return nil
}

func (self *Runner) GetMaxConcurrent() int {
	if self.settings.RunInParallel == false {
		return 1
	}

	return self.settings.MaxConcurrent
}

func (self *Runner) ReportFailure(execErr *AssertionFailure, timeTook Option[time.Duration]) {
	if execErr == nil {
		return
	}

	if cliopts.JSONOutput {
		self.reportFailureJSON(execErr, timeTook)
		return
	}

	// existing human output
	if timeTook.IsSome() {
		fmt.Printf("Took: %s\n\n", utils.FormatDuration(timeTook.Unwrap()))
	}

	fmt.Printf("Ran %d tests in %d.\n\n", self.stepsRan, self.settings.Cfg.GetTotalSteps())

	utils.PrintInColor("grey", "Step: ", false)
	fmt.Printf("%s/%s", execErr.Flow.Name, execErr.Step.Name)
	utils.PrintInColor("grey", " FAILED.", true)

	utils.PrintInColor("grey", "Cause: ", false)
	fmt.Printf("%s\n", execErr.Err.RootCause().Error())

	if len(execErr.Response) != 0 {
		if pretty, err := utils.PrettyJson(execErr.Response); err == nil {
			utils.PrintInColor("grey", "Server Response: ", true)
			fmt.Println(pretty)
		}
	}

	fmt.Printf("\n")
	utils.PrintInColor("red", "Some tests failed.", true)
}

func (self *Runner) ReportSuccess(timeTook Option[time.Duration]) {
	if cliopts.JSONOutput {
		self.reportSuccessJSON(timeTook)
		return
	}

	// existing human output
	if timeTook.IsSome() {
		fmt.Printf("Took: %s\n\n", utils.FormatDuration(timeTook.Unwrap()))
	}

	fmt.Printf("Ran %d/%d tests.\n\n", self.stepsRan, self.settings.Cfg.GetTotalSteps())
	utils.PrintInColor("green", "All tests passed.", true)
}

func (self *Runner) processBindingsForStep(step *app.Step, symtable map[string]any) error {
	// Process body
	if step.Request.Json.IsSome() {
		requestBody := step.Request.Json.Unwrap()
		processedRequestBody, err := self.processRequestBody(requestBody, symtable)
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
				assertion.Contains = Some(self.resolveBinding(contains, symtable))
			}
			if assertion.Equals.IsSome() {
				equals := assertion.Equals.Unwrap()
				assertion.Equals = Some(self.resolveBinding(equals, symtable))
			}
		}
	}

	return nil
}

// processRequestBody takes the request body and processes them by replacing any injectable variable (like {{RUN_ID}}) with its value.
func (self *Runner) processRequestBody(body map[string]any, symtable map[string]any) (map[string]any, error) {
	var err error

	for k, v := range body {
		switch val := v.(type) {
		case string:
			body[k] = self.resolveBinding(val, symtable)
		case map[string]any:
			body[k], err = self.processRequestBody(val, symtable)
			if err != nil {
				return nil, oops.Err(oops.StepRequestProcessingFailed, "failed to process/modify request body", err)
			}
		// optionally handle []any if you expect arrays
		case []any:
			for i, elem := range val {
				if s, ok := elem.(string); ok {
					val[i] = self.resolveBinding(s, symtable)
				}
			}
			body[k] = val
		}
	}

	return body, nil
}

func (self *Runner) reportFailureJSON(execErr *AssertionFailure, timeTook Option[time.Duration]) {
	took := "N/A"
	if timeTook.IsSome() {
		took = utils.FormatDuration(timeTook.Unwrap())
	}
	out := map[string]any{
		"took":   took,
		"status": "failure",
		"ran":    self.stepsRan,
		"total":  self.settings.Cfg.GetTotalSteps(),
		"flow":   execErr.Flow.Name,
		"step":   execErr.Step.Name,
		"error":  execErr.Err.RootCause().Error(),
		"code":   oops.StepRequestAssertionFailed.String(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func (self *Runner) reportSuccessJSON(timeTook Option[time.Duration]) {
	took := "N/A"
	if timeTook.IsSome() {
		took = utils.FormatDuration(timeTook.Unwrap())
	}
	out := map[string]any{
		"took":   took,
		"status": "success",
		"ran":    self.stepsRan,
		"total":  self.settings.Cfg.GetTotalSteps(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// resolveBinding takes a string and replaces any injectable bindings built-in or defined
//
// Example 1: "test-{{RUN_ID}}" -> "test-12345"
//
// Example 2: "test-{{RUN_ID}}" -> "test-12345"
func (self *Runner) resolveBinding(s string, symtable map[string]any) string {
	reBinding := regexp.MustCompile(`\{\{([a-zA-Z0-9_:-]+)\}\}`)

	return reBinding.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-2] // strip {{ }}

		// built-ins
		resolved, ok := self.resolveBuiltinBinding(inner)
		if ok {
			return resolved
		}

		// user vars: var:key
		if strings.HasPrefix(inner, "var:") {
			key := inner[4:]
			if v, ok := symtable[key]; ok {
				return fmt.Sprintf("%v", v)
			}
			return m // unresolved stays as-is
		}

		return m
	})
}

func (self *Runner) resolveBuiltinBinding(str string) (string, bool) {
	if str == "RUN_ID" {
		return self.RunId, true
	}
	return "", false
}
