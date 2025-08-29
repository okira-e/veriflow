package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okira-e/veriflow/internal"
)

type Cfg struct {
	ProjectName string           `json:"projectName"`
	BaseUrl     string           `json:"baseUrl"`
	Flows       []*internal.Flow `json:"flows"`
	flowsIdx    map[string]int
}

func NewDefaultConfig(baseUrl string) (Cfg, error) {
	defaultConfigJson, err := os.ReadFile("assets/configs/default-config.json")
	if err != nil {
		return Cfg{}, fmt.Errorf("Failed to read default config file. %s", err)
	}

	var defaultConfig Cfg
	err = json.Unmarshal(defaultConfigJson, &defaultConfig)
	if err != nil {
		return Cfg{}, fmt.Errorf("Failed to unmarshal default config. %s", err)
	}

	// Set the project name
	cwd, err := os.Getwd()
	if err != nil {
		return Cfg{}, fmt.Errorf("Failed to get current working directory. %s", err)
	}

	defaultConfig.ProjectName = filepath.Base(cwd)

	defaultConfig.BaseUrl = baseUrl

	return defaultConfig, nil
}

// GetFlow returns a pointer to the named flow with a boolean 'ok'
// if it's found.
func (cfg *Cfg) GetFlow(name string) (*internal.Flow, bool) {
	if i, ok := cfg.flowsIdx[name]; ok {
		return cfg.Flows[i], true
	}

	return nil, false
}

func (cfg *Cfg) AddFlow(flow *internal.Flow) error {
	if _, ok := cfg.GetFlow(flow.Name); ok {
		return fmt.Errorf("Flow with name '%s' already exists.", flow.Name)
	}

	cfg.Flows = append(cfg.Flows, flow)

	cfg.buildFlowsIndex()

	err := saveConfig(*cfg)
	if err != nil {
		return fmt.Errorf("Error saving config after creating flow: %w.", err)
	}

	return nil
}

func (cfg *Cfg) RemoveFlow(flowName string) error {
	if _, ok := cfg.GetFlow(flowName); !ok {
		return fmt.Errorf("Flow with name '%s' doesn't exists.", flowName)
	}

	for i, flow := range cfg.Flows {
		if flow.Name == flowName {
			cfg.Flows = append(cfg.Flows[:i], cfg.Flows[i+1:]...)
			break
		}
	}

	err := saveConfig(*cfg)
	if err != nil {
		return fmt.Errorf("Error saving config after creating flow: %w.", err)
	}

	return nil
}

// buildFlowsIndex creates an index for quick access to flows by name.
// Since the flows are array-based, this index allows O(1) access time
// instead of O(n) for searching through the array.
func (cfg *Cfg) buildFlowsIndex() {
	cfg.flowsIdx = make(map[string]int, len(cfg.Flows))
	for i, f := range cfg.Flows {
		cfg.flowsIdx[f.Name] = i
	}
}
