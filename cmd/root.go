package cmd

import (
	"errors"
	"os"

	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/cmd/flow"
	"github.com/okira-e/veriflow/cmd/run"
	"github.com/okira-e/veriflow/cmd/step"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "veriflow",
	Short:         "Veriflow is a CLI tool to define and run end-to-end API test flows using a simple JSON config.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Veriflow is a CLI tool for defining and running end-to-end API test flows.

It uses a JSON configuration file (veriflow.json) to define flows, steps,
assertions, and exports. Tests run in order, passing data between steps.`,
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	// Persistent flags are inherited by ALL subcommands.
	rootCmd.PersistentFlags().StringVar(&cliopts.ConfigFile, "config", "veriflow.json", "specify a config file")
	rootCmd.PersistentFlags().BoolVar(&cliopts.JSONOutput, "json-output", false, "output machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&cliopts.NoColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&cliopts.NonInteractive, "non-interactive", false, "disable interactive prompts")
	rootCmd.PersistentFlags().BoolVarP(&cliopts.Verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&cliopts.Silent, "silent", false, "suppress all output except errors")

	// Respect NO_COLOR env
	cobra.OnInitialize(func() {
		if os.Getenv("NO_COLOR") != "" {
			cliopts.NoColor = true
		}
	})
}

func Execute() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(newInitCmd())

	flow.SetupFlowCommands(rootCmd)
	step.SetupStepCommands(rootCmd)

	run.SetupRunCommands(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		if errors.Is(err, &run.RunAssertionError{}) { // An assertion failure happened from the server not a veriflow error.
			os.Exit(1)
		}

		cli.HandleCliError(err)
	}
}
