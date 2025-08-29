package internal

import "github.com/okira-e/veriflow/internal/opt"

type Request struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Json   opt.Option[any] `json:"json"`
}
