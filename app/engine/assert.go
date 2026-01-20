package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/oliveagle/jsonpath"
)

func validateAssertClause(assert *app.Assert, statusCode int, body []byte, contentType string) error {
	// Validate status
	err := validateStatus(assert, statusCode)
	if err != nil {
		return err
	}

	// Validate the response body (if exists)
	if assert.All.IsSome() {
		assertions := assert.All.Unwrap()
		isXML := strings.Contains(strings.ToLower(contentType), "xml")

		for _, assertion := range assertions {
			if isXML {
				err = validateXMLAssertion(assertion, body)
			} else {
				err = validateJSONAssertion(assertion, body)
			}

			if err != nil {
				return oops.Err(oops.StepRequestResponseAssertionFailed, "validation failed", err)
			}
		}
	}

	return nil
}

func validateJSONAssertion(assertion *app.Assertion, body []byte) error {
	if len(body) == 0 {
		return oops.Err(oops.StepResponseEmpty, "step's response body is empty", nil)
	}

	var response any
	err := json.Unmarshal(body, &response)
	if err != nil {
		return oops.Err(oops.StepRequestResponseParsingFailure, "failed to parse JSON response for step", err)
	}

	path := assertion.JsonPath
	if path == "" {
		path = assertion.XPath // fallback if user provided xpath for json
	}

	value, err := jsonpath.JsonPathLookup(response, path) // err means it wasn't found
	return validateAssertionValue(path, value, err, assertion, "jsonpath")
}

func validateXMLAssertion(assertion *app.Assertion, body []byte) error {
	if len(body) == 0 {
		return oops.Err(oops.StepResponseEmpty, "step's response body is empty", nil)
	}

	doc, err := xmlquery.Parse(bytes.NewReader(body))
	if err != nil {
		return oops.Err(oops.StepRequestResponseParsingFailure, "failed to parse XML response for step", err)
	}

	path := assertion.XPath
	if path == "" {
		path = assertion.JsonPath // fallback if user provided jsonpath for xml
	}

	node := xmlquery.FindOne(doc, path)
	var value any
	var lookupErr error
	if node != nil {
		value = node.InnerText()
	} else {
		lookupErr = fmt.Errorf("xpath not found")
	}

	return validateAssertionValue(path, value, lookupErr, assertion, "xpath")
}

func validateAssertionValue(path string, value any, lookupErr error, assertion *app.Assertion, pathType string) error {
	if assertion.Exists.IsSome() {
		exists := assertion.Exists.Unwrap()
		if exists && lookupErr != nil {
			message := fmt.Sprintf("%s '%s' not found in response", pathType, path)
			return oops.Err(oops.StepRequestResponseKeyNotFound, message, lookupErr)
		}

		if !exists && lookupErr == nil {
			message := fmt.Sprintf("%s '%s' found in response but it shouldn't exist", pathType, path)
			return oops.Err(oops.StepRequestResponseKeyForbidden, message, nil)
		}
	}

	if assertion.IsNot.IsSome() {
		expectedNotToBe := assertion.IsNot.Unwrap()
		actual := fmt.Sprintf("%v", value)
		if actual == "<nil>" {
			actual = "null"
		}
		if actual == expectedNotToBe {
			message := fmt.Sprintf("%s '%s' expected to never equal '%s'", pathType, path, expectedNotToBe)
			return oops.Err(oops.StepRequestResponseValueMismatch, message, nil)
		}
	}

	if assertion.Equals.IsSome() {
		expected := assertion.Equals.Unwrap()
		actual := fmt.Sprintf("%v", value)
		if actual == "<nil>" {
			actual = "null"
		}
		if actual != expected {
			message := fmt.Sprintf("%s '%s' expected to equal '%s' but got '%s'", pathType, path, expected, actual)
			return oops.Err(oops.StepRequestResponseValueMismatch, message, nil)
		}
	}

	if assertion.Contains.IsSome() {
		expectedSubstring := assertion.Contains.Unwrap()
		actual := fmt.Sprintf("%v", value)
		if !strings.Contains(actual, expectedSubstring) {
			message := fmt.Sprintf("%s '%s' expected to contain '%s' but got '%s'", pathType, path, expectedSubstring, actual)
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
