// Copyright 2026 Omar Khaleel
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package app

import (
	"fmt"

	"github.com/okira-e/veriflow/app/oops"
)

type Flow struct {
	Name     string  `json:"name"`
	Steps    []*Step `json:"steps"`
	stepsIdx map[string]int
}

func NewFlow(name string) *Flow {
	flow := Flow{
		Name:  name,
		Steps: []*Step{},
	}

	flow.BuildStepsIndex()

	return &flow
}

// GetStep returns a pointer to the named step with a boolean 'ok'
// if it's found.
func (flow *Flow) GetStep(name string) (*Step, bool) {
	if i, ok := flow.stepsIdx[name]; ok {
		return flow.Steps[i], true
	}

	return nil, false
}

func (flow *Flow) AddStep(step *Step) error {
	if _, ok := flow.GetStep(step.Name); ok {
		errMsg := fmt.Sprintf("Cannot add step to flow '%s': Step with name '%s' already exists", flow.Name, step.Name)
		return oops.Err(
			oops.StepAlreadyExists,
			errMsg,
			nil,
		)
	}

	flow.Steps = append(flow.Steps, step)

	flow.BuildStepsIndex()

	return nil
}

// BuildStepsIndex creates an index for quick access to steps by name.
// Since the steps are array-based, this index allows O(1) access time
// instead of O(n) for searching through the array.
func (flow *Flow) BuildStepsIndex() {
	flow.stepsIdx = make(map[string]int, len(flow.Steps))
	for i, f := range flow.Steps {
		flow.stepsIdx[f.Name] = i
	}
}
