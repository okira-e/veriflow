package run

import (
	"errors"

	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
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
}

func newRootCmd() *cobra.Command {
	rootFlags := &runRootFlags{}

	cmd := &cobra.Command{
		Use:   "run [OPTIONS] [TARGET...]",
		Short: "Runs the testing engine against the configuration file",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := rootCmd(rootFlags, args)
			utils.HandleCliError(err, cliopts.Verbose)

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
	err = runner.Execute()
	if err != nil {
		// Report the error through the engine if it's an execution error ie an assertion
		// failure.
		var execErr *engine.ExecutionError
		if errors.As(err, &execErr) {
			runner.ReportFailure(execErr)
		} else {
			return err
		}
	}

	runner.ReportSuccess()

	return nil
}
