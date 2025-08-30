package cmd

import (
	"encoding/json"
	"log"
	"os"

	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the base configuration for Veriflow.",
	Long: `
	Initialize the base configuration for Veriflow.
	`,
	Run: func(cmd *cobra.Command, args []string) {
		baseUrl, err := cli.PromptForUrl("What's the base API endpoint?", "http://localhost:8080/api", false)
		if err != nil {
			log.Fatalf("Failed to get the base URL. %s", err)
		}

		defaultConfig, err := config.NewDefaultConfig(baseUrl)
		if err != nil {
			log.Fatalf("Failed to generate default config. %s", err)
		}

		defaultConfigJson, err := json.MarshalIndent(defaultConfig, "", "    ")

		err = os.WriteFile("veriflow.json", defaultConfigJson, 0644)
		if err != nil {
			log.Fatalf("Failed to write the default config to file. %s", err)
		}

		utils.PrintInColor("green", "Default config file created successfully at veriflow.json")
	},
}
