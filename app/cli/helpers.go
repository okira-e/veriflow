package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/oops"
)

func isAborted(err error) bool {
	if err == nil {
		return false
	}

	if err.Error() == "user aborted" {
		return true
	}

	return false
}

func ValidateUrl(s string) error {
	if s == "" {
		return oops.Err(oops.ValidationError, "URL cannot be empty", nil)
	}

	parsed, err := url.ParseRequestURI(s)
	if err != nil {
		return oops.Err(oops.ValidationError, fmt.Sprintf("invalid URL format: %s", err.Error()), err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return oops.Err(oops.ValidationError, "URL must start with http:// or https://", nil)
	}

	if parsed.Host == "" {
		return oops.Err(oops.ValidationError, "URL must contain a valid host", nil)
	}

	return nil
}

func ValidateEmptyString(s string) error {
	if s == "" {
		return oops.Err(oops.ValidationError, "field cannot be empty", nil)
	}

	return nil
}

// ValidateJson validates if unmarshaling the text into JSON
// causes en error. It also takes a param to allow for empty text
// to pass, making it optional.
func ValidateJson(s string, allowEmpty bool) error {
	if allowEmpty {
		if s == "" {
			return nil
		}
	}

	var js json.RawMessage
	if err := json.Unmarshal([]byte(s), &js); err != nil {
		return oops.Err(oops.JSONParseError, "Value is not valid JSON", err)
	}

	return nil
}

var validate = validator.New()

// ValidateStruct validates a struct using the validator package
func ValidateStruct(s any) error {
	if err := validate.Struct(s); err != nil {
		return oops.Err(
			oops.ValidationError,
			"failed to validate struct",
			err,
		)
	}

	return nil
}

// ValidateVar validates a single variable using the validator package
func ValidateVar(field any, tag string) error {
	if err := validate.Var(field, tag); err != nil {
		return oops.Err(
			oops.ValidationError,
			"failed to validate variable",
			err,
		)
	}

	return nil
}

// Target represents a user-specified target: either a whole flow or a specific step.
type Target struct {
	Flow *app.Flow
	Step *app.Step // nil means "run entire flow"
}

// ParseTargets converts CLI args into Target structs.
// Empty args means "run all flows".
func ParseTargets(cfg *config.Cfg, args []string) ([]Target, error) {
	if len(args) == 0 {
		targets := make([]Target, len(cfg.Flows))
		for i, flow := range cfg.Flows {
			targets[i] = Target{Flow: flow, Step: nil}
		}
		return targets, nil
	}

	targets := make([]Target, 0, len(args))
	for _, arg := range args {
		target, err := ParseTarget(cfg, arg)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// ParseTarget parses a single target string like "flow" or "flow/step".
func ParseTarget(cfg *config.Cfg, arg string) (Target, error) {
	if len(arg) == 0 {
		return Target{}, oops.Err(oops.InvalidTarget, "empty target", nil)
	}
	if arg[0] == '/' {
		return Target{}, oops.Err(oops.InvalidTarget, fmt.Sprintf("invalid target %q", arg), nil)
	}

	arg = strings.TrimSuffix(arg, "/")

	if !strings.Contains(arg, "/") {
		// Whole flow
		flow, ok := cfg.GetFlow(arg)
		if !ok {
			return Target{}, oops.Err(oops.FlowNotFound, fmt.Sprintf("flow %q doesn't exist", arg), nil)
		}
		return Target{Flow: flow, Step: nil}, nil
	}

	// Specific step
	parts := strings.SplitN(arg, "/", 2)
	flowName, stepName := parts[0], parts[1]

	flow, ok := cfg.GetFlow(flowName)
	if !ok {
		return Target{}, oops.Err(oops.FlowNotFound, fmt.Sprintf("flow %q doesn't exist", flowName), nil)
	}

	step, ok := flow.GetStep(stepName)
	if !ok {
		return Target{}, oops.Err(oops.StepNotFound, fmt.Sprintf("step %q doesn't exist on flow %q", stepName, flowName), nil)
	}

	return Target{Flow: flow, Step: step}, nil
}

// flattenTargets expands targets into individual targets, excluding skipped ones.
func FlattenTargets(targets []Target, skips map[string]bool) []Target {
	var runs []Target

	for _, t := range targets {
		if t.Step != nil {
			// Single step target
			if !skips[stepKey(t.Flow.Name, t.Step.Name)] {
				runs = append(runs, Target{Flow: t.Flow, Step: t.Step})
			}
		} else {
			// Entire flow
			for _, step := range t.Flow.Steps {
				if !skips[stepKey(t.Flow.Name, step.Name)] {
					runs = append(runs, Target{Flow: t.Flow, Step: step})
				}
			}
		}
	}

	return runs
}

func stepKey(flowName string, stepName string) string {
	return flowName + "/" + stepName
}
