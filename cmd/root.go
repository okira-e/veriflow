package cmd

import (
	"log"
	"os"

	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/cmd/flow"
	"github.com/okira-e/veriflow/cmd/run"
	"github.com/okira-e/veriflow/cmd/step"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "veriflow",
	Short: "Veriflow is a CLI tool to define and run end-to-end API test flows using a simple JSON config.",
	Long: `
Veriflow is a lightweight, language-agnostic testing tool for backend APIs. Instead of writing test suites in a full framework, you declare flows (like “user-onboarding”) and their ordered steps (like register → login → fetchProfile) in a veriflow.json file at your project root. Each step describes its request, expected response, and any exported values (like IDs or tokens) that can be reused in later steps.

With Veriflow, you can:
  • Run complete integration scenarios from the command line.
  • Share test definitions as a single JSON file alongside your service.
  • Easily feed data between steps (e.g., JWTs, user IDs).
  • Catch regressions without writing boilerplate code or maintaining heavy test frameworks.

It’s designed for speed, clarity, and portability—perfect for developers, CI pipelines, and even bots that need structured output.

	`,
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	// Persistent flags are inherited by ALL subcommands.
	rootCmd.PersistentFlags().BoolVar(&cliopts.JSON, "json", false, "output machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&cliopts.NoColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&cliopts.NonInteractive, "non-interactive", false, "disable interactive prompts")

	// Respect NO_COLOR env automatically.
	cobra.OnInitialize(func() {
		if os.Getenv("NO_COLOR") != "" {
			cliopts.NoColor = true
		}
	})
}

func Execute() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)

	flow.SetupFlowCommands(rootCmd)
	step.SetupStepCommands(rootCmd)

	run.SetupRunCommands(rootCmd)

	err := rootCmd.Execute()
	if err != nil {
		log.Fatalf("Error executing root command. %s", err.Error())
	}
}
