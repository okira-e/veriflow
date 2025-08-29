package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "DEV"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays the version of Veriflow.",
	Long:  `Displays the version of Veriflow.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("veriflow version: %s\n", version)
	},
}
