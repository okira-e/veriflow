package app

import "github.com/okira-e/veriflow/app/opt"

type Step struct {
	Name    string  `json:"name"`
	Request Request `json:"request"`
	Assert  Assert  `json:"assert"`
	Exports Exports `json:"exports"`
}

func NewStep(name string, request Request, assert Assert, exports Exports) *Step {
	return &Step{
		Name:    name,
		Request: request,
		Assert:  assert,
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

type Assert struct {
	Status int                     `json:"status"`
	All    opt.Option[[]Assertion] `json:"all"`
}

func NewAssert(status int, all opt.Option[[]Assertion]) Assert {
	return Assert{
		Status: status,
		All:    all,
	}
}

type Assertion struct {
	JsonPath string `json:"jsonpath"`
	Value    string `json:"value"`
	Exists   bool   `json:"exists"`
	Contains bool   `json:"contains"`
	Equals   bool   `json:"equals"`
	Secret   bool   `json:"secret"`
}

func NewAssertion(jsonPath string, exists bool, contains bool, equals bool, secret bool, value string) Assertion {
	return Assertion{
		JsonPath: jsonPath,
		Value:    value,
		Exists:   exists,
		Contains: contains,
		Equals:   equals,
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
