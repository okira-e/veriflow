package run

import (
	"fmt"

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
		Short: "",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			return rootCmd(opts, args)
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&opts.Parallel, "parallel", false, "Run all flows and their steps.")
	flags.IntVar(&opts.Concurrency, "concurrency", 1, "Number of concurrent flows to run.")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Validate and print all steps without executing.")

	return cmd
}

func rootCmd(opts *runRootOpts, args []string) error {
	fmt.Println("ARGS: ", args)

	return nil
}
