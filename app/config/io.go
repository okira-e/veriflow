package config

import (
	"encoding/json"
	"fmt"
	"os"
)

var (
	configFilename = "veriflow.json"
)

func LoadConfig() (Cfg, error) {
	configBytes, err := readConfigBytes()
	if err != nil {
		return Cfg{}, fmt.Errorf("Failed to read config file as bytes. %s", err)
	}

	var cfg Cfg
	err = json.Unmarshal(configBytes, &cfg)
	if err != nil {
		return Cfg{}, fmt.Errorf("Failed to unmarshal config JSON. %s", err)
	}

	cfg.buildFlowsIndex()
	
	for _, flow := range cfg.Flows {
		flow.BuildStepsIndex()
	}

	return cfg, nil
}

func saveConfig(config Cfg) error {
	configJson, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("Failed to marshal config to JSON. %s", err)
	}

	err = os.WriteFile(configFilename, configJson, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write config to file. %s", err)
	}

	return nil
}

func readConfigBytes() ([]byte, error) {
	if _, err := os.Stat(configFilename); os.IsNotExist(err) {
		return nil, fmt.Errorf("Config file %s does not exist. Please run 'veriflow init' to generate it.", configFilename)
	}

	data, err := os.ReadFile(configFilename)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", configFilename, err)
	}

	return data, nil
}
