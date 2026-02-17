package engine

import (
	"fmt"
	"regexp"

	"github.com/okira-e/veriflow/app"
)

// ValidateConfig validates the config logically at runtime where some features might not work with
// others, or if some bindings are being used before assignment for example.
// It returns a "is valid" boolean and a string error message on failure.
func (self *Runner) ValidateConfig() error {
	exportedBindings := map[string]bool{}
	for _, flow := range self.settings.Cfg.Flows {
		for _, step := range flow.Steps {
			err := ValidateStep(step, exportedBindings)
			if err != nil {
				return err
			}

			for exportedKey, _ := range step.Exports {
				exportedBindings[exportedKey] = true
			}
		}
	}

	return nil
}

func ValidateStep(step *app.Step, exportedBindings map[string]bool) error {
	//
	// Validate assertion rules.
	//

	// Length + XML
	if step.Assert.All.IsSome() {
		assertions := step.Assert.All.Unwrap()
		for _, assertion := range assertions {
			if assertion.XPath != "" && assertion.Len.IsSome() {
				return fmt.Errorf("The \"length\" property is not supported on XPaths. Asserting the length of a tag container in XML is not currnetly supported.")
			}
		}
	}

	// Multiple payload options
	count := 0
	if step.Request.Files.IsSome() {
		count += 1
	}
	if step.Request.Json.IsSome() {
		count += 1
	}
	if step.Request.Xml.IsSome() {
		count += 1
	}

	if count > 1 {
		return fmt.Errorf("Found more than one payload type attached to step: \"%s\". To avert confusion, please provide only one for each step.", step.Name)
	}

	//
	// Check undefined bindings
	//

	re := regexp.MustCompile(`\{\{bind:([a-zA-Z0-9_]+)\}\}`)

	// URL Paths
	if step.Request.Path != "" {
		matches := re.FindAllStringSubmatch(step.Request.Path, -1)
		for _, match := range matches {
			if len(match) > 1 {
				binding := match[1]

				if _, ok := exportedBindings[binding]; !ok {
					return fmt.Errorf("Undefined binding '%s' used in step '%s' path. Make sure the binding is exported by any of the previous steps.", binding, step.Name)
				}
			}
		}
	}

	// Request bodies
	if step.Request.Json.IsSome() {
		jsonBody := step.Request.Json.Unwrap()
		_, err := walkJSON(jsonBody, func(s string) (any, error) {
			matches := re.FindAllStringSubmatch(s, -1)
			for _, match := range matches {
				if len(match) > 1 {
					binding := match[1]

					if _, ok := exportedBindings[binding]; !ok {
						return "", fmt.Errorf("Undefined binding '%s' used in step '%s' JSON body. Make sure the binding is exported by any of the previous steps.", binding, step.Name)
					}
				}
			}

			return s, nil
		})

		if err != nil {
			return err
		}
	} else if step.Request.Xml.IsSome() {
		xmlBody := step.Request.Xml.Unwrap()

		matches := re.FindAllStringSubmatch(xmlBody, -1)
		for _, match := range matches {
			if len(match) > 1 {
				binding := match[1]

				if _, ok := exportedBindings[binding]; !ok {
					return fmt.Errorf("Undefined binding '%s' used in step '%s' XML body. Make sure the binding is exported by any of the previous steps.", binding, step.Name)
				}
			}
		}
	}

	// Headers
	if step.Request.Headers.IsSome() {
		headers := step.Request.Headers.Unwrap()
		for key, val := range headers {
			matches := re.FindAllStringSubmatch(val, -1)
			for _, match := range matches {
				if len(match) > 1 {
					binding := match[1]

					if _, ok := exportedBindings[binding]; !ok {
						return fmt.Errorf("Undefined binding '%s' used in step '%s' header \"%s\". Make sure the binding is exported by any of the previous steps.", binding, step.Name, key)
					}
				}
			}
		}
	}

	// Assertions
	if step.Assert.All.IsSome() {
		assertions := step.Assert.All.Unwrap()
		for _, assertion := range assertions {
			// contains
			if assertion.Contains.IsSome() {
				matches := re.FindAllStringSubmatch(assertion.Contains.Unwrap(), -1)
				for _, match := range matches {
					if len(match) > 1 {
						binding := match[1]

						if _, ok := exportedBindings[binding]; !ok {
							return fmt.Errorf("Undefined binding '%s' used in step '%s' assertion value. Make sure the binding is exported by any of the previous steps.", binding, step.Name)
						}
					}
				}
			}
			// contains
			if assertion.Equals.IsSome() {
				matches := re.FindAllStringSubmatch(assertion.Equals.Unwrap(), -1)
				for _, match := range matches {
					if len(match) > 1 {
						binding := match[1]

						if _, ok := exportedBindings[binding]; !ok {
							return fmt.Errorf("Undefined binding '%s' used in step '%s' equality assertion value. Make sure the binding is exported by any of the previous steps.", binding, step.Name)
						}
					}
				}
			}
		}
	}

	return nil
}

// walkJSON is a generic function that traverses every value in a json valid object and
// performs an action
func walkJSON(v any, fn func(string) (any, error)) (any, error) {
	switch val := v.(type) {
	case string:
		return fn(val)

	case map[string]string:
		// I could cast to map[string]any and recall this function to not duplicate logic in the next case
		result := make(map[string]any, len(val))
		for k, v := range val {
			processed, err := walkJSON(v, fn)
			if err != nil {
				return "", err
			}
			result[k] = processed
		}
		return result, nil

	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			processed, err := walkJSON(v, fn)
			if err != nil {
				return "", err
			}
			result[k] = processed
		}
		return result, nil

	case []any:
		result := make([]any, len(val))
		for i, elem := range val {
			processed, err := walkJSON(elem, fn)
			if err != nil {
				return "", err
			}
			result[i] = processed
		}
		return result, nil

	default:
		return val, nil
	}
}
