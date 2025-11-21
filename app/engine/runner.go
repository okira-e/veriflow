package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/okira-e/veriflow/utils"
)

var (
	injectableVariables = []string{
		"RUN_ID",
	}
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
}

func NewRunner(settings RunnerSettings) *Runner {
	runId := utils.NewId()

	return &Runner{
		settings: settings,
		RunId:    runId,
	}
}

func (self *Runner) Execute() error {
	flows := self.settings.Cfg.Flows

	for _, flow := range flows {
		err := self.ExecuteFlow(flow)
		if err != nil {
			return oops.Err(oops.FlowExecutionFailed, fmt.Sprintf("failed to execute flow \"%s\"", flow.Name), err)
		}
	}

	return nil
}

func (self *Runner) ExecuteFlow(flow *app.Flow) error {
	for i, step := range flow.Steps {
		err := self.ExecuteStep(step, i)
		if err != nil {
			return oops.Err(oops.StepExecutionFailed, fmt.Sprintf("failed to execute step \"%s\"", step.Name), err)
		}
	}

	return nil
}

func (self *Runner) ExecuteStep(step *app.Step, i int) error {
	baseCtx := context.Background()
	timeout := step.Options.Timeout
	if step.Options.Timeout == 0 {
		timeout = 30 * time.Second // have a default
	}

	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	// Setup request body

	var body io.Reader
	if step.Request.Json.IsSome() {
		requestBody := step.Request.Json.Unwrap()
		processedRequestBody, err := self.processRequestBody(requestBody)
		if err != nil {
			return oops.Err(oops.StepRequestProcessingFailed, "failed to process request body", err)
		}

		b, err := json.Marshal(processedRequestBody)
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return oops.Err(oops.StepResponseReadFailed, "failed to read response body", err)
	}

	// Validate assertion

	err = step.Assert.Validate(resp.StatusCode, bodyBytes)
	if err != nil {
		return oops.Err(oops.StepRequestAssertionFailed, "step request assertion failed", err)
	}

	// bodyStr := string(bodyBytes)
	// fmt.Println("GOT RESPONSE: ", bodyStr)

	return nil
}

func (self *Runner) GetMaxConcurrent() int {
	if self.settings.RunInParallel == false {
		return 1
	}

	return self.settings.MaxConcurrent
}

// processRequestBody takes the request body and processes them by replacing any injectable variable (like {{RUN_ID}}) with its value.
func (self *Runner) processRequestBody(body map[string]any) (map[string]any, error) {
	var err error

	for k, v := range body {
		switch val := v.(type) {
		case string:
			body[k] = self.resolveInjectedVariables(val)
		case map[string]any:
			body[k], err = self.processRequestBody(val)
			if err != nil {
				return nil, oops.Err(oops.StepRequestProcessingFailed, "failed to process/modify request body", err)
			}
		// optionally handle []any if you expect arrays
		case []any:
			for i, elem := range val {
				if s, ok := elem.(string); ok {
					val[i] = self.resolveInjectedVariables(s)
				}
			}
			body[k] = val
		}
	}

	return body, nil
}

func (self *Runner) resolveInjectedVariables(str string) string {
	out := str
	for _, variable := range injectableVariables {
		placeHolder := fmt.Sprintf("{{%s}}", variable)
		out = strings.ReplaceAll(out, placeHolder, self.resolveInjectedVariable(variable))
	}

	return out
}

func (self *Runner) resolveInjectedVariable(str string) string {
	if str == "RUN_ID" {
		return self.RunId
	}
	return ""
}
