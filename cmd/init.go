package cmd

import (
	"encoding/json"
	"os"

	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/logging"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the base configuration for Veriflow",
	Long: `
	Initialize the base configuration for Veriflow.
	`,
	Run: func(cmd *cobra.Command, args []string) {
		baseUrl, err := cli.PromptForUrl("What's the base API endpoint?", "http://localhost:8080/api", false)
		if err != nil {
			cli.HandleCliError(err, cliopts.Verbose)
		}

		defaultConfig, err := config.NewDefaultConfig(baseUrl)
		if err != nil {
			cli.HandleCliError(err, cliopts.Verbose)
		}

		defaultConfigJson, jsonErr := json.MarshalIndent(defaultConfig, "", "    ")
		if jsonErr != nil {
			appErr := oops.Err(oops.ConfigMarshalError, "Failed to marshal config to JSON", jsonErr)
			cli.HandleCliError(appErr, cliopts.Verbose)
		}

		writeErr := os.WriteFile("veriflow.json", defaultConfigJson, 0644)
		if writeErr != nil {
			appErr := oops.Err(oops.FileWriteError, "Failed to write the default config to file", writeErr)
			cli.HandleCliError(appErr, cliopts.Verbose)
		}

		printer := logging.NewPrinter(cliopts.Silent, utils.IsColorEnabled())
		printer.Println(logging.Success, "Default config file created successfully at veriflow.json")
	},
}
