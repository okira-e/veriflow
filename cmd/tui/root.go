// Copyright 2026 Omar Khaleel
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package tui

import (
	"fmt"
	"os"

	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	tuipkg "github.com/okira-e/veriflow/tui"
	"github.com/spf13/cobra"
)

func SetupTuiCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(newRootCmd())
}

func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive TUI explorer",
		Long:  `Opens an interactive terminal UI for browsing flows, steps, and their details.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig(cliopts.ConfigFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load config at path \"%s\": %s\n", cliopts.ConfigFile, err)
				os.Exit(1)
			}

			tuipkg.Run(cfg)
		},
	}
}
