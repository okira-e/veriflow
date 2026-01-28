package cmd

import (
	"fmt"

	"github.com/okira-e/veriflow/app/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays the version of Veriflow",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(
			"veriflow %s (commit=%s, built=%s)\n",
			version.Version,
			version.Commit,
			version.Built,
		)
	},
}
