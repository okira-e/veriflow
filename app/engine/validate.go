package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/oliveagle/jsonpath"
)

func validateAssertClause(assert *app.Assert, statusCode int, body []byte) error {
	// Validate status
	err := validateStatus(assert, statusCode)
	if err != nil {
		return err
	}

	// Validate the response body (if exists)
	if assert.All.IsSome() {
		assertions := assert.All.Unwrap()

		for _, assertion := range assertions {
			err = validateAssertion(assertion, body)
			if err != nil {
				return oops.Err(oops.StepRequestResponseAssertionFailed, "validation failed", err)
			}
		}
	}

	return nil
}


func validateAssertion(assertion *app.Assertion, body []byte) error {
	if len(body) == 0 {
		return oops.Err(oops.StepResponseEmpty, "step's response body is empty", nil)
	}

	var response any
	err := json.Unmarshal(body, &response)
	if err != nil {
		return oops.Err(oops.StepRequestResponseParsingFailure, "failed to parse response for step", err)
	}

	value, err := jsonpath.JsonPathLookup(response, assertion.JsonPath)
	if assertion.Exists.IsSome() {
		exists := assertion.Exists.Unwrap()
		if exists && err != nil {
			message := fmt.Sprintf("jsonpath '%s' not found in response", assertion.JsonPath)
			return oops.Err(oops.StepRequestResponseKeyNotFound, message, err)
		}

		if !exists && err == nil {
			// It shouldn't exist, yet it does.
			message := fmt.Sprintf("jsonpath '%s' found in response but it shouldn't exist", assertion.JsonPath)
			return oops.Err(oops.StepRequestResponseKeyForbidden, message, nil)
		}
	}

	if assertion.Equals.IsSome() {
		expected := assertion.Equals.Unwrap()
		actual := fmt.Sprintf("%v", value)
		if actual == "<nil>" {
			actual = "null"
		}
		if actual != expected {
			message := fmt.Sprintf("jsonpath '%s' expected to equal '%s' but got '%s'", assertion.JsonPath, expected, actual)
			return oops.Err(oops.StepRequestResponseValueMismatch, message, nil)
		}
	}

	if assertion.Contains.IsSome() {
		expectedSubstring := assertion.Contains.Unwrap()
		actual := fmt.Sprintf("%v", value)
		if !strings.Contains(actual, expectedSubstring) {
			message := fmt.Sprintf("jsonpath '%s' expected to contain '%s' but got '%s'", assertion.JsonPath, expectedSubstring, actual)
			return oops.Err(oops.StepRequestResponseValueMismatch, message, nil)
		}
	}

	return nil
}

func validateStatus(assert *app.Assert, status int) error {
	if status != assert.Status {
		message := fmt.Sprintf("expected %d but got %d (status code)", assert.Status, status)
		return oops.Err(oops.StepRequestStatusMismatch, message, nil)
	}

	return nil
}
