package app

import "github.com/okira-e/veriflow/app/opt"

type Step struct {
	Name    string  `json:"name"`
	Request Request `json:"request"`
	Expect  Expect  `json:"expect"`
	Exports Exports `json:"exports"`
}

func NewStep(name string, request Request, expect Expect, exports Exports) *Step {
	return &Step{
		Name:    name,
		Request: request,
		Expect:  expect,
		Exports: exports,
	}
}

type Request struct {
	Method string                     `json:"method"`
	Path   string                     `json:"path"`
	Json   opt.Option[map[string]any] `json:"json"`
}

func NewRequest(
	method string,
	path string,
	json map[string]any,
) Request {
	var optionalJson = opt.None[map[string]any]()
	if json != nil {
		optionalJson = opt.Some(json)
	}

	return Request{
		Method: method,
		Path:   path,
		Json:   optionalJson,
	}
}

type Expect struct {
	Status int                        `json:"status"`
	All    opt.Option[[]ExpectResult] `json:"all"`
}

func NewExpect(status int) Expect {
	return Expect{
		Status: status,
		// All:    all,
	}
}

type ExpectResult struct {
	JsonPath opt.Option[string] `json:"jsonpath"`
	Exists   bool               `json:"exists"`
	Secret   bool               `json:"secret"`
}

func NewExpectResult(jsonPath opt.Option[string], exists bool, secret bool) ExpectResult {
	return ExpectResult{
		JsonPath: jsonPath,
		Exists:   exists,
		Secret:   secret,
	}
}

type Exports = map[string]ExportExpression

type ExportExpression struct {
	JsonPath opt.Option[string] `json:"jsonpath"`
}

func NewExportExpression(jsonPath opt.Option[string]) ExportExpression {
	return ExportExpression{
		JsonPath: jsonPath,
	}
}
