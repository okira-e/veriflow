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
	BeforeRun   []string    `json:"beforeRun"`
	Flows       []*app.Flow `json:"flows"`
	AfterRun    []string    `json:"afterRun"`
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

	return nil
}

func (self *Cfg) Save() error {
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

	self.buildFlowsIndex()

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

/*
project file-io
base http://localhost:3000/api

flow init # Define the first flow.

step add-storage
  POST /storages
  body:
    name = seaweed-local
    driver.type = s3
    driver.options.bucket = play
    driver.options.endpoint = http://localhost:8333
    driver.options.forcePathStyle = true
    driver.options.region = global
    driver.options.credentials.accessKeyId = accessMe
    driver.options.credentials.secretAccessKey = tellmeyoursecret

  expect status 201
  expect json $.data exists
  expect json $.data.name = seaweed-local

  export storage_id = $.data.id


step set-default
  POST /settings
  body:
    key = defaultStorage
    value = {{storage_id}}

  expect status 201


step add-storage-tier
  POST /storage-tiers
  body:
    name = hot
    threshold = 0.8
    storageId = {{storage_id}}

  expect status 201


flow errors

step file-not-found
  GET /files/00000000-0000-0000-0000-000000000000

  expect status 404
  expect json $.code = METADATA_NOT_FOUND
  expect json $.message exists


step file-source-not-found
  GET /files/00000000-0000-0000-0000-000000000000/source

  expect status 404
  expect json $.code = METADATA_NOT_FOUND


step storage-not-found
  GET /storages/00000000-0000-0000-0000-000000000000

  expect status 404
  expect json $.code = STORAGE_NOT_FOUND


step move-file-storage-not-found
  POST /files/move
  body:
    fileId = 00000000-0000-0000-0000-000000000001
    destStorageId = 00000000-0000-0000-0000-000000000002

  expect status 404
  expect json $.code = METADATA_NOT_FOUND


step invalid-input-missing-fields
  POST /storages
  body:
    name = incomplete-storage

  expect status 400
  expect json $.code = INVALID_INPUT
*/
