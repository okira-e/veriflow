package run

import (
	"errors"
	"fmt"
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
	Parallel        bool
	Concurrency     int
	DryRun          bool
	BaseUrlOverride string
	TrimResponse    bool
	Skips           []string
}

func newRootCmd() *cobra.Command {
	rootFlags := &runRootFlags{}

	cmd := &cobra.Command{
		Use:   "run [OPTIONS] [TARGET...]",
		Short: "Runs the testing engine against the configuration file",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := rootCmd(rootFlags, args)
			cli.HandleCliError(err, cliopts.Verbose)

			return nil
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&rootFlags.Parallel, "parallel", false, "Run all flows and their steps")
	flags.IntVar(&rootFlags.Concurrency, "concurrency", 1, "Number of concurrent flows to run")
	flags.BoolVar(&rootFlags.DryRun, "dry-run", false, "Validate and print all steps without executing")
	flags.StringVar(&rootFlags.BaseUrlOverride, "base-url", "", "Override the base URL in the config")
	flags.BoolVar(&rootFlags.TrimResponse, "trim-response", true, "Trim response from the server")
	flags.StringArrayVar(&rootFlags.Skips, "skip", []string{}, "Flows/steps to skip for this run")

	return cmd
}

func rootCmd(rootFlags *runRootFlags, args []string) error {
	cfg, err := config.LoadConfig(cliopts.ConfigFile)
	if err != nil {
		return err
	}

	runnerConfig := engine.RunnerSettings{
		Cfg:             cfg,
		RunInParallel:   rootFlags.Parallel,
		MaxConcurrent:   rootFlags.Concurrency,
		DryRun:          rootFlags.DryRun,
		BaseUrlOverride: rootFlags.BaseUrlOverride,
	}

	targets := []Target{}

	// Conditionally choose which flows/steps to run
	if len(args) > 0 {
		for _, arg := range args {
			if arg[0] == '/' {
				msg := fmt.Sprintf("invalid target \"%s\"", arg)
				return oops.Err(oops.InvalidTarget, msg, nil)
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
					return oops.Err(oops.FlowNotFound, msg, nil)
				}

				step, ok := flow.GetStep(stepName)
				if !ok {
					msg := fmt.Sprintf("step \"%s\" doesn't exist on flow \"%s\"", stepName, flowName)
					return oops.Err(oops.StepNotFound, msg, nil)
				}

				targets = append(targets, Target{Flow: flow, Step: step})
			} else {
				flow, ok := cfg.GetFlow(arg)
				if !ok {
					msg := fmt.Sprintf("flow \"%s\" doesn't exist", arg)
					return oops.Err(oops.FlowNotFound, msg, nil)
				}

				targets = append(targets, Target{Flow: flow, Step: nil})
			}
		}
	} else {
		for _, flow := range cfg.Flows {
			targets = append(targets, Target{Flow: flow, Step: nil})
		}
	}

	// Collect steps to be skipped.
	targetsToSkip := []Target{}
	for _, skip := range rootFlags.Skips {
		if strings.Contains(skip, "/") {
			flowName := strings.Split(skip, "/")[0]
			stepName := strings.Split(skip, "/")[1]

			flow, ok := cfg.GetFlow(flowName)
			if !ok {
				msg := fmt.Sprintf("flow \"%s\" doesn't exist", flowName)
				return oops.Err(oops.FlowNotFound, msg, nil)
			}

			step, ok := flow.GetStep(stepName)
			if !ok {
				msg := fmt.Sprintf("step \"%s\" doesn't exist on flow \"%s\"", stepName, flowName)
				return oops.Err(oops.StepNotFound, msg, nil)
			}

			targetsToSkip = append(targetsToSkip, Target{Flow: flow, Step: step})
		} else { // Skipping an entire flow
			flowName := skip

			flow, ok := cfg.GetFlow(flowName)
			if !ok {
				msg := fmt.Sprintf("skipped flow \"%s\" doesn't exist", flowName)
				return oops.Err(oops.FlowNotFound, msg, nil)
			}

			for _, step := range flow.Steps {
				targetsToSkip = append(targetsToSkip, Target{Flow: flow, Step: step})
			}
		}
	}

	runner := engine.NewRunner(runnerConfig)
	start := time.Now()
	err = runTargets(runner, targets, targetsToSkip, !cliopts.JSONOutput)
	elapsed := time.Since(start)
	if err != nil {
		// Report the error through the engine if it's an execution error ie an assertion
		// failure.
		var assertionFailure *engine.AssertionFailure
		if errors.As(err, &assertionFailure) {
			reportFailure(runner, assertionFailure, rootFlags.TrimResponse, elapsed)
			return nil
		} else {
			return err
		}
	}

	reportSuccess(runner, elapsed)

	return nil
}

func runTargets(runner *engine.Runner, targets []Target, targetsToSkip []Target, logText bool) error {
	// Conditionally assign a NullPritner so that printing doesn't happen with if-else hell.
	var printer logging.Printer
	if logText {
		printer = logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())
	} else {
		printer = logging.NullPrinter{}
	}

	var err error
	for _, target := range targets {
		// Run a single flow.
		if target.Step != nil {
			step := target.Step
			if includesFullTarget(targetsToSkip, target) {
				continue
			}
			msg := fmt.Sprintf("Running %s...", step.Name)
			printer.Print(logging.Info, msg)
			err = runner.Execute(step, map[string]any{})
			if err != nil {
				printer.Styled(logging.Info, logging.Red, "FAILED", true)
				break
			}
			printer.Styled(logging.Info, logging.Green, "OK", true)
		} else {
			// Run a complete flow
			symtable := map[string]any{}
			for _, step := range target.Flow.Steps {
				if includesFullTarget(targetsToSkip, Target{Flow: target.Flow, Step: step}) {
					continue
				}
				msg := fmt.Sprintf("Running %s/%s...", target.Flow.Name, step.Name)
				printer.Print(logging.Info, msg)
				err := runner.Execute(step, symtable)
				if err != nil {
					printer.Styled(logging.Info, logging.Red, "FAILED", true)
					// Flag the step that failed if the error is an ExecutionError.
					var assertionFailure *engine.AssertionFailure
					if errors.As(err, &assertionFailure) {
						assertionFailure.Flow = target.Flow
						return assertionFailure
					} else {
						return err
					}
				}
				printer.Styled(logging.Info, logging.Green, "OK", true)
			}
		}
	}

	return err
}

func reportFailure(runner *engine.Runner, assertionFailure *engine.AssertionFailure, trimResponse bool, elapsed time.Duration) {
	if cliopts.JSONOutput {
		reportFailureJSON(runner, assertionFailure, elapsed)
		return
	}

	printer := logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())

	timeTookMsg := fmt.Sprintf("\nTook: %s", utils.FormatDuration(elapsed))
	printer.Println(logging.Info, timeTookMsg)

	ranMsg := fmt.Sprintf("Ran %d/%d tests.", runner.StepsRan(), runner.TotalSteps())
	printer.Println(logging.Info, ranMsg)

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
			if trimResponse && len(lines) > 10 {
				lines = lines[:10]
			}

			printer.Styled(logging.Info, logging.Grey, "Server Response:\n", false)
			printer.Println(logging.Info, strings.Join(lines, "\n"))
		}
	}

	printer.Print(logging.Info, "\n")
	printer.Styled(logging.Info, logging.Red, "Some tests failed.", true)
}

func reportFailureJSON(runner *engine.Runner, assertionFailure *engine.AssertionFailure, elapsed time.Duration) {
	printer := logging.NewJSONPrinter(cliopts.Silent)

	flowName := "---"
	if assertionFailure.Flow != nil {
		flowName = assertionFailure.Flow.Name
	}

	result := map[string]any{
		"took":    utils.FormatDuration(elapsed),
		"success": false,
		"ran":     runner.StepsRan(),
		"total":   runner.TotalSteps(),
		"flow":    flowName,
		"step":    assertionFailure.Step.Name,
		"error":   assertionFailure.Err.RootCause().Error(),
		"code":    oops.StepRequestAssertionFailed.String(),
	}

	printer.PrintStructured(result)
}

func reportSuccess(runner *engine.Runner, elapsed time.Duration) {
	if cliopts.JSONOutput {
		reportSuccessJSON(runner, elapsed)
		return
	}

	printer := logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())

	// Print timing
	timeTookMsg := fmt.Sprintf("Took: %s", utils.FormatDuration(elapsed))
	printer.Println(logging.Info, timeTookMsg)

	// Print test count
	ranMsg := fmt.Sprintf("Ran %d/%d tests.", runner.StepsRan(), runner.TotalSteps())
	printer.Println(logging.Info, ranMsg)

	printer.Styled(logging.Info, logging.Green, "All tests passed.", true)
}

func reportSuccessJSON(runner *engine.Runner, elapsed time.Duration) {
	printer := logging.NewJSONPrinter(cliopts.Silent)

	result := map[string]any{
		"took":    utils.FormatDuration(elapsed),
		"success": true,
		"ran":     runner.StepsRan(),
		"total":   runner.TotalSteps(),
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
