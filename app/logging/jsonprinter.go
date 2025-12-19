package logging

import (
	"encoding/json"
	"io"
	"os"
)

type JSONPrinter struct {
	Out    io.Writer
	Silent bool
}

func NewJSONPrinter(silent bool) *JSONPrinter {
	return &JSONPrinter{
		Out:    os.Stdout,
		Silent: silent,
	}
}

type jsonLog struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func (self *JSONPrinter) Print(kind Kind, msg string) {
	self.print(kind, msg)
}

func (self *JSONPrinter) Println(kind Kind, msg string) {
	self.print(kind, msg)
}

func (self *JSONPrinter) print(kind Kind, msg string) {
	// Silence policy: errors always print
	if self.Silent && kind != Error {
		return
	}

	entry := jsonLog{
		Kind:    kindToString(kind),
		Message: msg,
	}

	enc := json.NewEncoder(self.Out)
	_ = enc.Encode(entry) // logging must never panic
}

// PrintStructured outputs arbitrary structured data as formatted JSON.
// This is used for outputting test results and other structured data,
// not log messages. Respects the Silent flag.
func (self *JSONPrinter) PrintStructured(data any) {
	if self.Silent {
		return
	}

	enc := json.NewEncoder(self.Out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data) // logging must never panic
}

func kindToString(kind Kind) string {
	switch kind {
	case Info:
		return "info"
	case Success:
		return "success"
	case Warn:
		return "warn"
	case Error:
		return "error"
	case Debug:
		return "debug"
	default:
		return "unknown"
	}
}
