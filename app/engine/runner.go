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
	"strings"
	"time"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/oops"
	. "github.com/okira-e/veriflow/app/opt"
	"github.com/okira-e/veriflow/app/utils"
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
	for i, step := range flow.Steps {
		err := self.ExecuteStep(step, i)
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
// Returns a Go error on anything unexpected.
//
// Returns an ExecutionError on an assertion failure.
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return oops.Err(oops.StepResponseReadFailed, "failed to read response body", err)
	}

	// Validate assertion

	err = step.Assert.Validate(resp.StatusCode, bodyBytes)
	if err != nil {
		return &AssertionFailure{
			Err:      oops.Err(oops.StepRequestAssertionFailed, "step request assertion failed", err),
			Response: bodyBytes,
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

	fmt.Printf("Ran %d/%d tests in %s.\n\n", self.stepsRan, self.settings.Cfg.GetTotalSteps())

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
