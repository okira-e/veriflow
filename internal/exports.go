package internal

import "github.com/okira-e/veriflow/internal/opt"

type Exports = map[string]ExportExpression

type ExportExpression struct {
	JsonPath opt.Option[string] `json:"jsonpath"`
}
