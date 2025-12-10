package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/okira-e/veriflow/defaults"
)

type Cfg struct {
	ProjectName string      `json:"projectName"`
	BaseUrl     string      `json:"baseUrl"`
	Flows       []*app.Flow `json:"flows"`
	flowsIdx    map[string]int
}

// @TODO: Add versioning.
func NewDefaultConfig(baseUrl string) (Cfg, error) {
	var defaultConfig Cfg
	err := json.Unmarshal(defaults.DefaultConfig, &defaultConfig)
	if err != nil {
		return Cfg{}, oops.Err(oops.ConfigUnmarshalError, "Failed to unmarshal default config", err)
	}

	// Set the project name
	cwd, err := os.Getwd()
	if err != nil {
		return Cfg{}, oops.Err(oops.Internal, "Failed to get current working directory", err)
	}

	defaultConfig.ProjectName = filepath.Base(cwd)

	defaultConfig.BaseUrl = baseUrl

	return defaultConfig, nil
}

// GetFlow returns a pointer to the named flow with a boolean 'ok'
// if it's found.
func (self *Cfg) GetFlow(name string) (*app.Flow, bool) {
	if i, ok := self.flowsIdx[name]; ok {
		return self.Flows[i], true
	}

	return nil, false
}

func (self *Cfg) AddFlow(flow *app.Flow) error {
	if _, ok := self.GetFlow(flow.Name); ok {
		return oops.Err(oops.FlowAlreadyExists, fmt.Sprintf("Flow with name '%s' already exists", flow.Name), nil)
	}

	self.Flows = append(self.Flows, flow)

	self.buildFlowsIndex()

	err := saveConfig(*self)
	if err != nil {
		return oops.Err(oops.ConfigFileNotFound, "Error saving config after creating flow", err)
	}

	return nil
}

func (self *Cfg) RemoveFlow(flowName string) error {
	if _, ok := self.GetFlow(flowName); !ok {
		return oops.Err(oops.FlowNotFound, fmt.Sprintf("Flow with name '%s' doesn't exist", flowName), nil)
	}

	for i, flow := range self.Flows {
		if flow.Name == flowName {
			self.Flows = append(self.Flows[:i], self.Flows[i+1:]...)
			break
		}
	}

	err := saveConfig(*self)
	if err != nil {
		return oops.Err(oops.FlowRemovalError, "Error saving config after removing flow", err)
	}

	return nil
}

func (self *Cfg) UpdateFlow(flow *app.Flow) error {
	if _, ok := self.GetFlow(flow.Name); !ok {
		return oops.Err(oops.FlowNotFound, fmt.Sprintf("Flow with name '%s' doesn't exist", flow.Name), nil)
	}

	for i, f := range self.Flows {
		if f.Name == flow.Name {
			self.Flows[i] = flow
			break
		}
	}

	err := saveConfig(*self)
	if err != nil {
		return oops.Err(oops.FlowUpdateError, "Error saving config after updating flow", err)
	}

	return nil
}

func (self *Cfg) GetTotalSteps() int {
	var totalSteps int
	for _, flow := range self.Flows {
		totalSteps += len(flow.Steps)
	}
	return totalSteps
}

// buildFlowsIndex creates an index for quick access to flows by name.
// Since the flows are array-based, this index allows O(1) access time
// instead of O(n) for searching through the array.
func (self *Cfg) buildFlowsIndex() {
	self.flowsIdx = make(map[string]int, len(self.Flows))
	for i, f := range self.Flows {
		self.flowsIdx[f.Name] = i
	}
}
