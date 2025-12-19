package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/okira-e/veriflow/app/oops"
	. "github.com/okira-e/veriflow/app/opt"
	"github.com/oliveagle/jsonpath"
)

type Step struct {
	Name    string      `json:"name"`
	Request Request     `json:"request"`
	Assert  Assert      `json:"assert"`
	Exports Exports     `json:"exports"`
	Options StepOptions `json:"options"`
}

func NewStep(name string, request Request, assert Assert, exports Exports) *Step {
	return &Step{
		Name:    name,
		Request: request,
		Assert:  assert,
		Exports: exports,
	}
}

type StepOptions struct {
	Timeout time.Duration `json:"timeout"`
}

type Request struct {
	Method string                 `json:"method"`
	Path   string                 `json:"path"`
	Json   Option[map[string]any] `json:"json"` // @TODO: I put xml as the key instead of json and no unmarshal error happened.
}

func NewRequest(method string, path string, json map[string]any) Request {
	var optionalJson = None[map[string]any]()
	if json != nil {
		optionalJson = Some(json)
	}

	return Request{
		Method: method,
		Path:   path,
		Json:   optionalJson,
	}
}

type Assert struct {
	Status int                  `json:"status"`
	All    Option[[]*Assertion] `json:"all"`
}

func NewAssert(status int, all Option[[]*Assertion]) Assert {
	return Assert{
		Status: status,
		All:    all,
	}
}

func (self *Assert) Validate(statusCode int, body []byte) error {
	// Validate status
	err := self.validateStatus(statusCode)
	if err != nil {
		return err
	}

	// Validate the response body (if exists)
	if self.All.IsSome() {
		assertions := self.All.Unwrap()

		for _, assertion := range assertions {
			err = assertion.Validate(body)
			if err != nil {
				return oops.Err(oops.StepRequestResponseAssertionFailed, "validation failed", err)
			}
		}
	}

	return nil
}

func (self *Assert) validateStatus(status int) error {
	if status != self.Status {
		message := fmt.Sprintf("expected %d but got %d (status code)", self.Status, status)
		return oops.Err(oops.StepRequestStatusMismatch, message, nil)
	}

	return nil
}

type Assertion struct {
	JsonPath string         `json:"jsonpath"`
	Exists   Option[bool]   `json:"exists"`
	Contains Option[string] `json:"contains"`
	Equals   Option[string] `json:"equals"`
	Secret   bool           `json:"secret"`
}

func (self *Assertion) Validate(body []byte) error {
	if len(body) == 0 {
		return oops.Err(oops.StepResponseEmpty, "step's response body is empty", nil)
	}

	var response any
	err := json.Unmarshal(body, &response)
	if err != nil {
		return oops.Err(oops.StepRequestResponseParsingFailure, "failed to parse response for step", err)
	}

	value, err := jsonpath.JsonPathLookup(response, self.JsonPath)
	if self.Exists.IsSome() {
		exists := self.Exists.Unwrap()
		if exists && err != nil {
			message := fmt.Sprintf("jsonpath '%s' not found in response", self.JsonPath)
			return oops.Err(oops.StepRequestResponseKeyNotFound, message, err)
		}

		if !exists && err == nil {
			// It shouldn't exist, yet it does.
			message := fmt.Sprintf("jsonpath '%s' found in response but it shouldn't exist", self.JsonPath)
			return oops.Err(oops.StepRequestResponseKeyForbidden, message, nil)
		}
	}

	if self.Equals.IsSome() {
		expected := self.Equals.Unwrap()
		actual := fmt.Sprintf("%v", value)
		if actual == "<nil>" {
			actual = "null"
		}
		if actual != expected {
			message := fmt.Sprintf("jsonpath '%s' expected to equal '%s' but got '%s'", self.JsonPath, expected, actual)
			return oops.Err(oops.StepRequestResponseValueMismatch, message, nil)
		}
	}

	if self.Contains.IsSome() {
		expectedSubstring := self.Contains.Unwrap()
		actual := fmt.Sprintf("%v", value)
		if !strings.Contains(actual, expectedSubstring) {
			message := fmt.Sprintf("jsonpath '%s' expected to contain '%s' but got '%s'", self.JsonPath, expectedSubstring, actual)
			return oops.Err(oops.StepRequestResponseValueMismatch, message, nil)
		}
	}

	return nil
}

type Exports = map[string]string // var_name->jsonpath

func NewExportExpression(jsonPath Option[string]) Exports {
	return map[string]string{}
}
