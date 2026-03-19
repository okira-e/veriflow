// Copyright 2026 Omar Khaleel
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package run

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
	"github.com/okira-e/veriflow/app/logging"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/spf13/cobra"
)

const maxResponseLines = 15

// RunCmdOptions holds all runtime configuration for executing targets.
type RunCmdOptions struct {
	TrimErrorResponse   bool
	KeepGoing           bool
	ShowServerResponses bool
	Printer             logging.Printer
}

// RunAssertionError signals that assertion failures occurred (for exit code 1).
type RunAssertionError struct{}

func (self *RunAssertionError) Error() string { return "" }

func SetupRunCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(newRootCmd())
}

type runFlags struct {
	BaseUrlOverride       string
	ShowFullErrorResponse bool
	Skips                 []string
	KeepGoing             bool
	ShowHooks             bool
	SkipHooks             bool
	ShowServerResponses   bool
}

func newRootCmd() *cobra.Command {
	flags := &runFlags{}

	cmd := &cobra.Command{
		Use:   "run [OPTIONS] [TARGET...]",
		Short: "Runs the testing engine against the configuration file",
		Long: `Execute test flows defined in veriflow.json.

Targets can be flow names or flow/step pairs:
  veriflow run                          Run all flows
  veriflow run user-onboarding          Run a specific flow
  veriflow run user-onboarding/login    Run a specific step
  veriflow run flow1 flow2/step1        Run multiple targets

Hooks (beforeRun/afterRun) execute automatically unless --skip-hooks is set.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			hadFailures, err := runCommand(flags, args)
			cli.HandleCliError(err)

			if hadFailures {
				return &RunAssertionError{}
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&flags.BaseUrlOverride, "base-url", "", "Override the base URL in the config")
	f.BoolVar(&flags.ShowFullErrorResponse, "show-full-error-response", false, "Display entire server response payload on error")
	f.StringArrayVar(&flags.Skips, "skip", []string{}, "Flows/steps to skip for this run")
	f.BoolVar(&flags.KeepGoing, "keep-going", false, "Continue running tests even if some fail")
	f.BoolVar(&flags.ShowHooks, "show-hooks", false, "Print stdout and stderr from runBefore and runAfter shell commands")
	f.BoolVar(&flags.SkipHooks, "skip-hooks", false, "Skip executing runBefore and runAfter shell commands")
	f.BoolVar(&flags.ShowServerResponses, "show-server-responses", false, "View responses sent from the server on every request")

	return cmd
}

// runCommand is the entry point for the run command.
// Returns (hadFailures, error) where hadFailures indicates assertion failures occurred.
func runCommand(flags *runFlags, args []string) (bool, error) {
	cfg, err := config.LoadConfig(cliopts.ConfigFile)
	if err != nil {
		return false, err
	}

	targets, err := cli.ParseTargets(cfg, args)
	if err != nil {
		return false, err
	}

	skips, err := parseSkips(cfg, flags.Skips)
	if err != nil {
		return false, err
	}

	stepsToRun := cli.FlattenTargets(targets, skips)

	runner := engine.NewRunner(engine.RunnerSettings{
		Cfg:             cfg,
		BaseUrlOverride: flags.BaseUrlOverride,
	})

	// Validate before running anything
	err = runner.ValidateConfig()
	if err != nil {
		return false, oops.Err(oops.ConfigInvalid, "Config validation failed: "+err.Error(), nil)
	}

	// Run hooks
	if !flags.SkipHooks {
		if len(cfg.BeforeRun) > 0 {
			if err := runBeforeHooks(cfg, flags.ShowHooks); err != nil {
				return false, err
			}
		}
		if len(cfg.AfterRun) > 0 {
			defer runAfterHooks(cfg, flags.ShowHooks)
		}
	}

	// Execute
	runCmdOptions := RunCmdOptions{
		TrimErrorResponse:   !flags.ShowFullErrorResponse,
		KeepGoing:           flags.KeepGoing,
		ShowServerResponses: flags.ShowServerResponses,
		Printer:             makePrinter(),
	}

	runCmdOptions.Printer.Println(logging.Info, fmt.Sprintf("Run ID: %s", runner.RunId))

	start := time.Now()
	failureCount, af, err := executeSteps(runner, stepsToRun, runCmdOptions)
	elapsed := time.Since(start)
	if err != nil {
		return false, err
	}

	// Report results
	if cliopts.JSONOutput {
		reportJSON(failureCount == 0, elapsed, runner.StepsRan(), runner.TotalSteps(), !flags.SkipHooks, af)
	} else {
		report(failureCount == 0, elapsed, runner.StepsRan(), runner.TotalSteps())
	}

	return failureCount > 0, nil
}

// parseSkips converts skip args into a set of cli.Target for fast lookup.
func parseSkips(cfg *config.Cfg, skipArgs []string) (map[string]bool, error) {
	skips := make(map[string]bool)

	for _, skip := range skipArgs {
		target, err := cli.ParseTarget(cfg, skip)
		if err != nil {
			// Adjust error message for skips
			if strings.Contains(err.Error(), "doesn't exist") {
				return nil, oops.Err(oops.FlowNotFound, fmt.Sprintf("skipped %s", err.Error()), nil)
			}
			return nil, err
		}

		if target.Step != nil {
			// Skip specific step
			skips[stepKey(target.Flow.Name, target.Step.Name)] = true
		} else {
			// Skip entire flow
			for _, step := range target.Flow.Steps {
				skips[stepKey(target.Flow.Name, step.Name)] = true
			}
		}
	}

	return skips, nil
}

func stepKey(flowName string, stepName string) string {
	return flowName + "/" + stepName
}

// executeSteps runs all steps, knows to print them, and returns the failure count
// along with the last assertion failure (if any).
func executeSteps(runner *engine.Runner, steps []cli.Target, opts RunCmdOptions) (int, *engine.AssertionFailure, error) {
	failures := 0
	var lastFailure *engine.AssertionFailure

	for _, target := range steps {
		opts.Printer.Styled(logging.Info, logging.Grey, "Running ", false)
		opts.Printer.Print(logging.Info, fmt.Sprintf("%s/%s...", target.Flow.Name, target.Step.Name))

		resp, err := runner.Execute(target.Step)
		if err != nil {
			var af *engine.AssertionFailure
			isAssertionFailure := errors.As(err, &af)
			if isAssertionFailure {
				failures += 1
				af.Flow = target.Flow
				lastFailure = af
			}

			opts.Printer.Styled(logging.Info, logging.Red, "FAILED", true)

			if !opts.KeepGoing {
				if isAssertionFailure {
					reportAssertionFailure(af, opts.TrimErrorResponse)
					return failures, lastFailure, nil
				}
				return failures, lastFailure, err
			}
		} else {
			if opts.ShowServerResponses {
				printServerResponse(resp, opts.Printer)
			}
			opts.Printer.Styled(logging.Info, logging.Green, "OK", true)
		}
	}

	return failures, lastFailure, nil
}

func runBeforeHooks(cfg *config.Cfg, showOutput bool) error {
	printer := makePrinter()
	printer.Styled(logging.Info, logging.Normal, "Running beforeRun hook commands", true)
	for _, cmd := range cfg.BeforeRun {
		if err := shellExec(cmd, !showOutput); err != nil {
			return oops.Err(oops.BeforeRunFailed, "beforeRun command failed", err)
		}
	}
	return nil
}

func runAfterHooks(cfg *config.Cfg, showOutput bool) {
	printer := makePrinter()
	printer.Styled(logging.Info, logging.Normal, "Running afterRun hook commands", true)
	for _, cmd := range cfg.AfterRun {
		_ = shellExec(cmd, !showOutput) // Best effort
	}
}

func shellExec(cmd string, silent bool) error {
	shell, args := "sh", []string{"-c", cmd}
	if runtime.GOOS == "windows" {
		shell, args = "cmd", []string{"/C", cmd}
	}

	c := exec.Command(shell, args...)
	if !silent {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}
	c.Env = os.Environ()

	return c.Run()
}

func makePrinter() logging.Printer {
	if cliopts.JSONOutput {
		return logging.NullPrinter{}
	}
	return logging.NewPrinter()
}

func report(success bool, elapsed time.Duration, stepsRan int, totalSteps int) {
	printer := logging.NewPrinter()

	printer.Println(logging.Info, fmt.Sprintf("\nTook: %s", utils.FormatDuration(elapsed)))
	printer.Println(logging.Info, fmt.Sprintf("Ran %d/%d tests.", stepsRan, totalSteps))

	if success {
		printer.Styled(logging.Info, logging.Green, "All tests passed.", true)
	} else {
		printer.Styled(logging.Info, logging.Red, "Some tests have failed", true)
	}
}

func reportJSON(success bool, elapsed time.Duration, stepsRan int, totalSteps int, ranHooks bool, af *engine.AssertionFailure) {
	printer := logging.NewJSONPrinter(cliopts.Silent)
	result := map[string]any{
		"took":     utils.FormatDuration(elapsed),
		"success":  success,
		"ran":      stepsRan,
		"ranHooks": ranHooks,
		"total":    totalSteps,
		"code":     "",
		"message":  "",
	}
	if af != nil {
		if appErr, ok := af.Err.RootCause().(*oops.AppError); ok {
			result["code"] = appErr.Code.String()
			result["message"] = appErr.Msg
		}
	}
	printer.PrintStructured(result)
}

func reportAssertionFailure(af *engine.AssertionFailure, trimResponse bool) {
	if cliopts.JSONOutput {
		return
	}

	printer := logging.NewPrinter()

	flowName := "---"
	if af.Flow != nil {
		flowName = af.Flow.Name
	}

	printer.Styled(logging.Info, logging.Grey, "Step: ", false)
	printer.Print(logging.Info, fmt.Sprintf("%s/%s", flowName, af.Step.Name))
	printer.Styled(logging.Info, logging.Normal, " FAILED.\n", false)

	printer.Styled(logging.Info, logging.Grey, "Cause: ", false)
	printer.Print(logging.Info, af.Err.RootCause().Error()+"\n")

	if len(af.Response) > 0 {
		if pretty, err := utils.PrettyJson(af.Response); err == nil {
			lines := strings.Split(pretty, "\n")
			if trimResponse && len(lines) > maxResponseLines {
				lines = lines[:maxResponseLines]
			}
			printer.Styled(logging.Info, logging.Grey, "Server Response:\n", false)
			printer.Println(logging.Info, strings.Join(lines, "\n"))
		}
	}
}

func printServerResponse(body []byte, printer logging.Printer) {
	if pretty, err := utils.PrettyJson(body); err == nil {
		printer.Styled(logging.Info, logging.Grey, "\nResponse: ", false)
		printer.Print(logging.Info, pretty+"\n")
	}
}
