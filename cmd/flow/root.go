package flow

import (
	"github.com/spf13/cobra"
)

var flowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Perform an action on a flow",
	Long:  `Perform an action on a flow.`,
}

func SetupFlowCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(flowCmd)

	flowCmd.AddCommand(newCreateCmd())
	flowCmd.AddCommand(newDeleteCmd())
}
