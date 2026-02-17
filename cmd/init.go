// Copyright 2026 Omar Khaleel
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package cmd

import (
	"encoding/json"
	"os"

	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/logging"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/spf13/cobra"
)

type initCmdFlags struct {
	BaseUrl string
}

func newInitCmd() *cobra.Command {
	cmdFlags := &initCmdFlags{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the base configuration for Veriflow",
		Long: `
Initialize the base configuration for Veriflow.
		`,
		Run: func(cmd *cobra.Command, args []string) {
			err := runInitCmd(cmdFlags)
			cli.HandleCliError(err)
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&cmdFlags.BaseUrl, "base-url", "", "The base URL for the backend API")

	return cmd
}

func runInitCmd(flags *initCmdFlags) error {
	// Check if the config already exists
	_, err := config.LoadConfig("veriflow.json")
	if err == nil {
		return oops.Err(oops.ConfigFileExistsError, "A veriflow.json config file already exists in the current directory", nil)
	}

	baseUrl := flags.BaseUrl

	if baseUrl == "" && !cliopts.NonInteractive {
		var err error
		baseUrl, err = cli.PromptForUrl("What's the base API endpoint?", "http://localhost:8080/api", false)
		if err != nil {
			return err
		}
	} else if baseUrl == "" && cliopts.NonInteractive {
		return oops.Err(oops.MissingRequiredFlag, "Base URL is required in non-interactive mode. Please provide it using the --base-url flag.", nil)
	}

	defaultConfig, err := config.NewDefaultConfig(baseUrl)
	if err != nil {
		return err
	}

	defaultConfigJson, jsonErr := json.MarshalIndent(defaultConfig, "", "    ")
	if jsonErr != nil {
		return oops.Err(oops.ConfigMarshalError, "Failed to marshal config to JSON", jsonErr)
	}

	configFile := "veriflow.json"
	if cliopts.ConfigFile != "" {
		configFile = cliopts.ConfigFile
	}

	writeErr := os.WriteFile(configFile, defaultConfigJson, 0644)
	if writeErr != nil {
		return oops.Err(oops.FileWriteError, "Failed to write the default config to file", writeErr)
	}

	printer := logging.NewPrinter()
	printer.Println(logging.Success, "Default config file created successfully at veriflow.json")

	return nil
}
