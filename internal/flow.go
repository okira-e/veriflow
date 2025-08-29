package internal

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

	flow.buildStepsIndex()

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

// buildStepsIndex creates an index for quick access to steps by name.
// Since the steps are array-based, this index allows O(1) access time
// instead of O(n) for searching through the array.
func (flow *Flow) buildStepsIndex() {
	flow.stepsIdx = make(map[string]int, len(flow.Steps))
	for i, f := range flow.Steps {
		flow.stepsIdx[f.Name] = i
	}
}
