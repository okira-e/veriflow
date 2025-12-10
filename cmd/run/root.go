package run

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
	"github.com/okira-e/veriflow/app/oops"
	. "github.com/okira-e/veriflow/app/opt"
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

	runner := engine.NewRunner(runnerConfig)
	start := time.Now()

	// Conditionally choose which flows/steps to run
	if len(args) > 0 {
		for _, arg := range args {
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

				err = runner.ExecuteStep(step, map[string]any{})
				if err != nil {
					break
				}
			} else {
				flow, ok := cfg.GetFlow(arg)
				if !ok {
					msg := fmt.Sprintf("flow \"%s\" doesn't exist", arg)
					return oops.Err(oops.FlowNotFound, msg, nil)
				}

				err = runner.ExecuteFlow(flow)
				if err != nil {
					break
				}
			}
		}
	} else {
		err = runner.ExecuteAll()
	}

	if err != nil {
		// Report the error through the engine if it's an execution error ie an assertion
		// failure.
		var assertionError *engine.AssertionFailure
		if errors.As(err, &assertionError) {
			runner.ReportFailure(assertionError, Some(time.Since(start)))
			return nil
		} else {
			return err
		}
	}

	runner.ReportSuccess(Some(time.Since(start)))

	return nil
}
