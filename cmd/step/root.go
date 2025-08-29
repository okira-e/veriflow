package step

import (
	"github.com/spf13/cobra"
)

var stepCmd = &cobra.Command{
	Use:   "step",
	Short: "Perform an action on a step",
	Long:  `Perform an action on a step.`,
}

func SetupStepCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(stepCmd)

	stepCmd.AddCommand(newCreateCmd())
}
