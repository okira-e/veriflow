package internal

import "github.com/okira-e/veriflow/internal/opt"

type Expect struct {
	Status int                        `json:"status"`
	All    opt.Option[[]ExpectResult] `json:"all"`
}

type ExpectResult struct {
	JsonPath opt.Option[string] `json:"jsonpath"`
	Exists   bool               `json:"exists"`
	Secret   bool               `json:"secret"`
}
