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

func SetupRunCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(newRootCmd())
}

type runRootFlags struct {
	BaseUrlOverride  string
	ShowFullResponse bool
	Skips            []string
	KeepGoing        bool
	ShowHooks        bool
	SkipHooks        bool
}

type RunAssertionError struct{}

func (self *RunAssertionError) Error() string {
	return ""
}

func newRootCmd() *cobra.Command {
	rootFlags := &runRootFlags{}

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
			assertionFailuresHappened, err := rootCmd(rootFlags, args)
			cli.HandleCliError(err)

			if assertionFailuresHappened {
				return &RunAssertionError{} // Signal that we should exit with 1.
			}

			return nil
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&rootFlags.BaseUrlOverride, "base-url", "", "Override the base URL in the config")
	flags.BoolVar(&rootFlags.ShowFullResponse, "show-full-response", false, "Display entire server response payload on error")
	flags.StringArrayVar(&rootFlags.Skips, "skip", []string{}, "Flows/steps to skip for this run")
	flags.BoolVar(&rootFlags.KeepGoing, "keep-going", false, "Continue running tests even if some fail")
	flags.BoolVar(&rootFlags.ShowHooks, "show-hooks", false, "Print stdout and stderr from runBefore and runAfter shell commands")
	flags.BoolVar(&rootFlags.SkipHooks, "skip-hooks", false, "Skip executing runBefore and runAfter shell commands")

	return cmd
}

// rootCmd starts the `run` command and returns if an assertion failure has happened as well as
// an error
func rootCmd(rootFlags *runRootFlags, args []string) (bool, error) {
	cfg, err := config.LoadConfig(cliopts.ConfigFile)
	if err != nil {
		return false, err
	}

	///
	// Run and print the pre/post run hooks if not skipped
	//

	if !rootFlags.SkipHooks {
		var printer logging.Printer
		if cliopts.JSONOutput {
			printer = logging.NullPrinter{}
		} else {
			printer = logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())
		}

		beforeRunHooksExist := len(cfg.BeforeRun) > 0
		afterRunHooksExist := len(cfg.AfterRun) > 0

		if beforeRunHooksExist {
			printer.Styled(logging.Info, logging.Normal, "Running beforeRun hook commands", true)
			if err := beforeRunCommands(cfg, rootFlags.ShowHooks); err != nil {
				return false, err
			}
		}
		if afterRunHooksExist {
			defer afterRunCommands(cfg, rootFlags.ShowHooks)
			defer printer.Styled(logging.Info, logging.Normal, "Running afterRun hook commands", true)
		}
	}

	targets := []Target{}

	//
	// Conditionally choose which flows/steps to run
	//

	if len(args) > 0 {
		for _, arg := range args {
			if arg[0] == '/' {
				msg := fmt.Sprintf("invalid target \"%s\"", arg)
				return false, oops.Err(oops.InvalidTarget, msg, nil)
			}
			if arg[len(arg)-1] == '/' {
				arg = arg[:len(arg)-1]
			}
			if strings.Contains(arg, "/") {
				flowName := strings.Split(arg, "/")[0]
				stepName := strings.Split(arg, "/")[1]

				flow, ok := cfg.GetFlow(flowName)
				if !ok {
					msg := fmt.Sprintf("flow \"%s\" doesn't exist", flowName)
					return false, oops.Err(oops.FlowNotFound, msg, nil)
				}

				step, ok := flow.GetStep(stepName)
				if !ok {
					msg := fmt.Sprintf("step \"%s\" doesn't exist on flow \"%s\"", stepName, flowName)
					return false, oops.Err(oops.StepNotFound, msg, nil)
				}

				targets = append(targets, Target{Flow: flow, Step: step})
			} else {
				flow, ok := cfg.GetFlow(arg)
				if !ok {
					msg := fmt.Sprintf("flow \"%s\" doesn't exist", arg)
					return false, oops.Err(oops.FlowNotFound, msg, nil)
				}

				targets = append(targets, Target{Flow: flow, Step: nil})
			}
		}
	} else {
		for _, flow := range cfg.Flows {
			targets = append(targets, Target{Flow: flow, Step: nil})
		}
	}

	//
	// Collect steps to be skipped.
	//

	targetsToSkip := []Target{}
	for _, skip := range rootFlags.Skips {
		if strings.Contains(skip, "/") {
			flowName := strings.Split(skip, "/")[0]
			stepName := strings.Split(skip, "/")[1]

			flow, ok := cfg.GetFlow(flowName)
			if !ok {
				msg := fmt.Sprintf("flow \"%s\" doesn't exist", flowName)
				return false, oops.Err(oops.FlowNotFound, msg, nil)
			}

			step, ok := flow.GetStep(stepName)
			if !ok {
				msg := fmt.Sprintf("step \"%s\" doesn't exist on flow \"%s\"", stepName, flowName)
				return false, oops.Err(oops.StepNotFound, msg, nil)
			}

			targetsToSkip = append(targetsToSkip, Target{Flow: flow, Step: step})
		} else { // Skipping an entire flow
			flowName := skip

			flow, ok := cfg.GetFlow(flowName)
			if !ok {
				msg := fmt.Sprintf("skipped flow \"%s\" doesn't exist", flowName)
				return false, oops.Err(oops.FlowNotFound, msg, nil)
			}

			for _, step := range flow.Steps {
				targetsToSkip = append(targetsToSkip, Target{Flow: flow, Step: step})
			}
		}
	}

	//
	// Run the steps
	//

	runnerConfig := engine.RunnerSettings{
		Cfg:             cfg,
		BaseUrlOverride: rootFlags.BaseUrlOverride,
	}
	runner := engine.NewRunner(runnerConfig)

	start := time.Now()
	failures, err := runTargets(
		runner,
		targets,
		targetsToSkip,
		!cliopts.JSONOutput,
		!rootFlags.ShowFullResponse,
		rootFlags.KeepGoing,
	)
	if err != nil {
		return false, err // This is a program error and not an assertion failure.
	}
	elapsed := time.Since(start)

	if cliopts.JSONOutput {
		reportJSON(
			failures == 0,
			elapsed,
			runner.StepsRan(),
			runner.TotalSteps(),
			!rootFlags.SkipHooks,
		)
	} else {
		report(
			failures == 0,
			elapsed,
			runner.StepsRan(),
			runner.TotalSteps(),
		)
	}

	return failures != 0, nil
}

func runTargets(
	runner *engine.Runner,
	targets []Target,
	targetsToSkip []Target,
	logText bool,
	trimResponse bool,
	keepGoing bool,
) (int, error) {
	failures := 0

	// Conditionally assign a NullPritner so that printing doesn't happen with if-else hell.
	var printer logging.Printer
	if logText {
		printer = logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())
	} else {
		printer = logging.NullPrinter{}
	}

	for _, target := range targets {
		// Run a single step.
		if target.Step != nil {
			// @Dupe
			step := target.Step
			if includesFullTarget(targetsToSkip, target) {
				continue
			}
			printer.Styled(logging.Info, logging.Grey, "Running ", false)
			msg := fmt.Sprintf("%s...", step.Name)
			printer.Print(logging.Info, msg)
			err := runner.Execute(step)
			if err != nil {
				isAssertionFailure := false
				// Count this as one of the failures if it is an assertion failure.
				var assertionFailure *engine.AssertionFailure
				if errors.As(err, &assertionFailure) {
					failures += 1
					isAssertionFailure = true
				}

				printer.Styled(logging.Info, logging.Red, "FAILED", true)

				if !keepGoing {
					// Report the error through the engine if it's an assertion failure.
					if isAssertionFailure {
						reportAssertionFailure(assertionFailure, trimResponse)
						return failures, nil
					} else {
						return failures, err // Return any other error that's not an assertion failure
					}
				}
			} else {
				printer.Styled(logging.Info, logging.Green, "OK", true)
			}
		} else {
			// @Dupe
			// Run a complete flow
			for _, step := range target.Flow.Steps {
				if includesFullTarget(targetsToSkip, Target{Flow: target.Flow, Step: step}) {
					continue
				}
				printer.Styled(logging.Info, logging.Grey, "Running ", false)
				msg := fmt.Sprintf("%s/%s...", target.Flow.Name, step.Name)
				printer.Print(logging.Info, msg)
				err := runner.Execute(step)
				if err != nil {
					isAssertionFailure := false
					// Count this as one of the failures if it is an assertion failure.
					var assertionFailure *engine.AssertionFailure
					if errors.As(err, &assertionFailure) {
						failures += 1
						isAssertionFailure = true
					}

					printer.Styled(logging.Info, logging.Red, "FAILED", true)

					if !keepGoing {
						// Report the error through the engine if it's an assertion failure.
						if isAssertionFailure {
							assertionFailure.Flow = target.Flow // Flag the step that failed if the error is an ExecutionError.
							reportAssertionFailure(assertionFailure, trimResponse)
							return failures, nil
						} else {
							return failures, err // Return any other error that's not an assertion failure
						}
					}
				} else {
					printer.Styled(logging.Info, logging.Green, "OK", true)
				}
			}
		}
	}

	return failures, nil
}

func reportAssertionFailure(assertionFailure *engine.AssertionFailure, trimResponse bool) {
	if cliopts.JSONOutput {
		return
	}

	printer := logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())

	printer.Styled(logging.Info, logging.Grey, "Step: ", false)

	flowName := "---"
	if assertionFailure.Flow != nil {
		flowName = assertionFailure.Flow.Name
	}
	stepMsg := fmt.Sprintf("%s/%s", flowName, assertionFailure.Step.Name)
	printer.Print(logging.Info, stepMsg)

	printer.Styled(logging.Info, logging.Normal, " FAILED.\n", false)

	printer.Styled(logging.Info, logging.Grey, "Cause: ", false)
	causeMsg := fmt.Sprintf("%s\n", assertionFailure.Err.RootCause().Error())
	printer.Print(logging.Info, causeMsg)

	if len(assertionFailure.Response) != 0 {
		if pretty, err := utils.PrettyJson(assertionFailure.Response); err == nil {
			lines := strings.Split(pretty, "\n")
			if trimResponse && len(lines) > 15 {
				lines = lines[:15]
			}

			printer.Styled(logging.Info, logging.Grey, "Server Response:\n", false)
			printer.Println(logging.Info, strings.Join(lines, "\n"))
		}
	}
}

// @TODO: Figure out how to report individual step error when not --keep-going with reportJSON happening.
func reportAssertionFailureJSON(assertionFailure *engine.AssertionFailure) {
	printer := logging.NewJSONPrinter(cliopts.Silent)

	flowName := "---"
	if assertionFailure.Flow != nil {
		flowName = assertionFailure.Flow.Name
	}

	result := map[string]any{
		"flow":  flowName,
		"step":  assertionFailure.Step.Name,
		"error": assertionFailure.Err.RootCause().Error(),
		"code":  oops.StepRequestAssertionFailed.String(),
	}

	printer.PrintStructured(result)
}

func report(success bool, elapsed time.Duration, stepsRan int, totalSteps int) {
	printer := logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())

	timeTookMsg := fmt.Sprintf("\nTook: %s", utils.FormatDuration(elapsed))
	printer.Println(logging.Info, timeTookMsg)
	ranMsg := fmt.Sprintf("Ran %d/%d tests.", stepsRan, totalSteps)
	printer.Println(logging.Info, ranMsg)

	if success {
		printer.Styled(logging.Info, logging.Green, "All tests passed.", true)
	} else {
		printer.Styled(logging.Info, logging.Red, "Some tests have failed", true)
	}
}

func reportJSON(success bool, elapsed time.Duration, stepsRan int, totalSteps int, ranHooks bool) {
	printer := logging.NewJSONPrinter(cliopts.Silent)

	result := map[string]any{
		"took":     utils.FormatDuration(elapsed),
		"success":  success,
		"ran":      stepsRan,
		"ranHooks": ranHooks,
		"total":    totalSteps,
	}

	printer.PrintStructured(result)
}

type Target struct {
	Flow *app.Flow
	Step *app.Step
}

// includesFullTarget assumes that both flow and step are not nil
func includesFullTarget(targets []Target, target Target) bool {
	for _, t := range targets {
		if target.Flow.Name == t.Flow.Name && target.Step.Name == t.Step.Name {
			return true
		}
	}

	return false
}

func execute(cmd string, silent bool) error {
	shell := "sh"
	args := []string{"-c", cmd}

	if runtime.GOOS == "windows" {
		shell = "cmd"
		args = []string{"/C", cmd}
	}

	c := exec.Command(shell, args...)
	if !silent {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}
	c.Env = os.Environ()

	return c.Run()
}

func beforeRunCommands(cfg *config.Cfg, showOutput bool) error {
	for _, cmd := range cfg.BeforeRun {
		if err := execute(cmd, !showOutput); err != nil {
			return oops.Err(oops.BeforeRunFailed, "beforeRun command failed", err)
		}
	}
	return nil
}

func afterRunCommands(cfg *config.Cfg, showOutput bool) error {
	for _, cmd := range cfg.AfterRun {
		if err := execute(cmd, !showOutput); err != nil {
			return oops.Err(oops.AfterRunFailed, "afterRun command failed", err)
		}
	}
	return nil
}
