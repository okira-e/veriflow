package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/okira-e/veriflow/app/oops"
)

var (
	filename = "veriflow.json"
)

func LoadConfig() (*Cfg, error) {
	configBytes, err := readConfigBytes()
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

	err = os.WriteFile(filename, configJson, 0644)
	if err != nil {
		return oops.Err(oops.FileWriteError, "Failed to write config to file", err)
	}

	return nil
}

func readConfigBytes() ([]byte, error) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, oops.Err(
			oops.ConfigFileNotFound,
			fmt.Sprintf("Config file %s does not exist. Please run 'veriflow init' to generate it.", filename),
			err,
		)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, oops.Err(
			oops.FileReadError,
			fmt.Sprintf("could not read %s", filename),
			err,
		)
	}

	return data, nil
}
