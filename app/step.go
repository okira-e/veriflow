package app

import (
	. "github.com/okira-e/veriflow/app/opt"
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
	Timeout Option[string] `json:"timeout"`
}

type Request struct {
	Method         string                 `json:"method"`
	Path           string                 `json:"path"`
	Json           Option[map[string]any] `json:"json"`
	Xml            Option[string]         `json:"xml"`
	DisableHeaders bool                   `json:"disableHeaders"`
}

func NewRequest(method string, path string, json map[string]any) Request {
	optionalJson := None[map[string]any]()
	if json != nil {
		optionalJson = Some(json)
	}

	return Request{
		Method:         method,
		Path:           path,
		Json:           optionalJson,
		DisableHeaders: false, // By default we implicitly set and include headers
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

type Assertion struct {
	JsonPath string         `json:"jsonpath"`
	XPath    string         `json:"xpath"`
	Exists   Option[bool]   `json:"exists"`
	Contains Option[string] `json:"contains"`
	Equals   Option[string] `json:"equals"`
	IsNot    Option[string] `json:"isNot"`
}

type Exports = map[string]string // var_name->jsonpath

func NewExportExpression(jsonPath Option[string]) Exports {
	return map[string]string{}
}
