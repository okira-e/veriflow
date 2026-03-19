// Copyright 2026 Omar Khaleel
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/oops"
)

func LoadConfig(path string) (*Cfg, error) {
	configBytes, err := readConfigBytes(path)
	if err != nil {
		return nil, err
	}

	var cfg Cfg
	jsonErr := json.Unmarshal(configBytes, &cfg)
	if jsonErr != nil {
		return nil, oops.Err(oops.ConfigUnmarshalError, "Failed to unmarshal config JSON", jsonErr)
	}

	cfg.buildFlowsIndex()

	for _, flow := range cfg.Flows {
		flow.BuildStepsIndex()
	}

	return &cfg, nil
}

func saveConfig(config Cfg) error {
	configJson, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return oops.Err(oops.ConfigMarshalError, "Failed to marshal config to JSON", err)
	}

	err = os.WriteFile(cliopts.ConfigFile, configJson, 0644)
	if err != nil {
		return oops.Err(oops.FileWriteError, "Failed to write config to file", err)
	}

	return nil
}

func readConfigBytes(configPath string) ([]byte, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, oops.Err(
			oops.ConfigFileNotFound,
			"Config file does not exist. Please run 'veriflow init' to generate it.",
			err,
		)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, oops.Err(
			oops.FileReadError,
			fmt.Sprintf("could not read %s", configPath),
			err,
		)
	}

	return data, nil
}
