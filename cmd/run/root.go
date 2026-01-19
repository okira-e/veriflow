package run

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/okira-e/veriflow/app"
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

// Target represents a user-specified target: either a whole flow or a specific step.
type Target struct {
	Flow *app.Flow
	Step *app.Step // nil means "run entire flow"
}

// StepRun is a flattened, executable unit: one step with its flow context.
type StepRun Target

// RunOptions holds all runtime configuration for executing targets.
type RunOptions struct {
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

	targets, err := parseTargets(cfg, args)
	if err != nil {
		return false, err
	}

	skips, err := parseSkips(cfg, flags.Skips)
	if err != nil {
		return false, err
	}

	stepsToRun := flattenTargets(targets, skips)

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
	runner := engine.NewRunner(engine.RunnerSettings{
		Cfg:             cfg,
		BaseUrlOverride: flags.BaseUrlOverride,
	})

	opts := RunOptions{
		TrimErrorResponse:   !flags.ShowFullErrorResponse,
		KeepGoing:           flags.KeepGoing,
		ShowServerResponses: flags.ShowServerResponses,
		Printer:             makePrinter(),
	}

	start := time.Now()
	failures, err := executeSteps(runner, stepsToRun, opts)
	elapsed := time.Since(start)

	if err != nil {
		return false, err
	}

	// Report results
	if cliopts.JSONOutput {
		reportJSON(failures == 0, elapsed, runner.StepsRan(), runner.TotalSteps(), !flags.SkipHooks)
	} else {
		report(failures == 0, elapsed, runner.StepsRan(), runner.TotalSteps())
	}

	return failures > 0, nil
}

// parseTargets converts CLI args into Target structs.
// Empty args means "run all flows".
func parseTargets(cfg *config.Cfg, args []string) ([]Target, error) {
	if len(args) == 0 {
		targets := make([]Target, len(cfg.Flows))
		for i, flow := range cfg.Flows {
			targets[i] = Target{Flow: flow, Step: nil}
		}
		return targets, nil
	}

	targets := make([]Target, 0, len(args))
	for _, arg := range args {
		target, err := parseTarget(cfg, arg)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// parseTarget parses a single target string like "flow" or "flow/step".
func parseTarget(cfg *config.Cfg, arg string) (Target, error) {
	if len(arg) == 0 {
		return Target{}, oops.Err(oops.InvalidTarget, "empty target", nil)
	}
	if arg[0] == '/' {
		return Target{}, oops.Err(oops.InvalidTarget, fmt.Sprintf("invalid target %q", arg), nil)
	}

	arg = strings.TrimSuffix(arg, "/")

	if !strings.Contains(arg, "/") {
		// Whole flow
		flow, ok := cfg.GetFlow(arg)
		if !ok {
			return Target{}, oops.Err(oops.FlowNotFound, fmt.Sprintf("flow %q doesn't exist", arg), nil)
		}
		return Target{Flow: flow, Step: nil}, nil
	}

	// Specific step
	parts := strings.SplitN(arg, "/", 2)
	flowName, stepName := parts[0], parts[1]

	flow, ok := cfg.GetFlow(flowName)
	if !ok {
		return Target{}, oops.Err(oops.FlowNotFound, fmt.Sprintf("flow %q doesn't exist", flowName), nil)
	}

	step, ok := flow.GetStep(stepName)
	if !ok {
		return Target{}, oops.Err(oops.StepNotFound, fmt.Sprintf("step %q doesn't exist on flow %q", stepName, flowName), nil)
	}

	return Target{Flow: flow, Step: step}, nil
}

// parseSkips converts skip args into a set of StepRun for fast lookup.
func parseSkips(cfg *config.Cfg, skipArgs []string) (map[string]bool, error) {
	skips := make(map[string]bool)

	for _, skip := range skipArgs {
		target, err := parseTarget(cfg, skip)
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

// flattenTargets expands targets into individual StepRuns, excluding skipped ones.
func flattenTargets(targets []Target, skips map[string]bool) []StepRun {
	var runs []StepRun

	for _, t := range targets {
		if t.Step != nil {
			// Single step target
			if !skips[stepKey(t.Flow.Name, t.Step.Name)] {
				runs = append(runs, StepRun{Flow: t.Flow, Step: t.Step})
			}
		} else {
			// Entire flow
			for _, step := range t.Flow.Steps {
				if !skips[stepKey(t.Flow.Name, step.Name)] {
					runs = append(runs, StepRun{Flow: t.Flow, Step: step})
				}
			}
		}
	}

	return runs
}

func stepKey(flowName, stepName string) string {
	return flowName + "/" + stepName
}

// executeSteps runs all steps, knows to print them, and returns the failure count.
func executeSteps(runner *engine.Runner, steps []StepRun, opts RunOptions) (int, error) {
	failures := 0

	for _, sr := range steps {
		opts.Printer.Styled(logging.Info, logging.Grey, "Running ", false)
		opts.Printer.Print(logging.Info, fmt.Sprintf("%s/%s...", sr.Flow.Name, sr.Step.Name))

		resp, err := runner.Execute(sr.Step)

		if err != nil {
			var af *engine.AssertionFailure
			isAssertionFailure := errors.As(err, &af)

			if isAssertionFailure {
				failures++
				af.Flow = sr.Flow
			}

			opts.Printer.Styled(logging.Info, logging.Red, "FAILED", true)

			if !opts.KeepGoing {
				if isAssertionFailure {
					reportAssertionFailure(af, opts.TrimErrorResponse)
					return failures, nil
				}
				return failures, err
			}
		} else {
			if opts.ShowServerResponses {
				printServerResponse(resp, opts.Printer)
			}
			opts.Printer.Styled(logging.Info, logging.Green, "OK", true)
		}
	}

	return failures, nil
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
	return logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())
}

func report(success bool, elapsed time.Duration, stepsRan, totalSteps int) {
	printer := logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())

	printer.Println(logging.Info, fmt.Sprintf("\nTook: %s", utils.FormatDuration(elapsed)))
	printer.Println(logging.Info, fmt.Sprintf("Ran %d/%d tests.", stepsRan, totalSteps))

	if success {
		printer.Styled(logging.Info, logging.Green, "All tests passed.", true)
	} else {
		printer.Styled(logging.Info, logging.Red, "Some tests have failed", true)
	}
}

func reportJSON(success bool, elapsed time.Duration, stepsRan, totalSteps int, ranHooks bool) {
	printer := logging.NewJSONPrinter(cliopts.Silent)
	printer.PrintStructured(map[string]any{
		"took":     utils.FormatDuration(elapsed),
		"success":  success,
		"ran":      stepsRan,
		"ranHooks": ranHooks,
		"total":    totalSteps,
	})
}

func reportAssertionFailure(af *engine.AssertionFailure, trimResponse bool) {
	if cliopts.JSONOutput {
		return
	}

	printer := logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())

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
