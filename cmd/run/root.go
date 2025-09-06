package run

import (
	"github.com/okira-e/veriflow/app/utils"
	"github.com/spf13/cobra"
)

func SetupRunCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(newRootCmd())
}

type runRootOpts struct {
	Parallel    bool
	Concurrency int
	DryRun      bool
}

func newRootCmd() *cobra.Command {
	opts := &runRootOpts{}

	cmd := &cobra.Command{
		Use:   "run [OPTIONS] [TARGET...]",
		Short: "Runs the testing engine against the configuration file",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := rootCmd(opts, args)
			utils.HandleCliError(err)
			return err
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&opts.Parallel, "parallel", false, "Run all flows and their steps.")
	flags.IntVar(&opts.Concurrency, "concurrency", 1, "Number of concurrent flows to run.")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Validate and print all steps without executing.")

	return cmd
}

func rootCmd(opts *runRootOpts, args []string) error {
	// Example of how to return a proper error if needed
	// return oops.Err(oops.OperationFailed, "Failed to run test", err)

	return nil
}
