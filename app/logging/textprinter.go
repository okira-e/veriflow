package logging

import (
	"fmt"
	"io"
	"os"
)

type Style uint8

const (
	Normal Style = iota
	Emphasis
	Green
	Red
	Grey
)

type Kind uint8

const (
	Info Kind = iota
	Success
	Warn
	Error
	Debug
)

type TextPrinter struct {
	Out    io.Writer
	Err    io.Writer
	Silent bool
	Color  bool
}

func NewPrinter(silent bool, color bool) *TextPrinter {
	return &TextPrinter{
		Out:    os.Stdout,
		Err:    os.Stderr,
		Silent: silent,
		Color:  color,
	}
}

func (self *TextPrinter) Print(kind Kind, msg string) {
	self.print(kind, msg, false)
}

func (self *TextPrinter) Println(kind Kind, msg string) {
	self.print(kind, msg, true)
}

func (self *TextPrinter) print(kind Kind, msg string, newline bool) {
	// Silence policy: errors always print
	if self.Silent && kind != Error {
		return
	}

	writer := self.writerFor(kind)

	if self.Color {
		fmt.Fprint(writer, self.colorFor(kind))
	}

	fmt.Fprint(writer, msg)

	if self.Color {
		fmt.Fprint(writer, ansiReset)
	}

	if newline {
		fmt.Fprint(writer, "\n")
	}
}

func (self *TextPrinter) Styled(kind Kind, style Style, msg string, newline bool) {
	if self.Silent && kind != Error {
		return
	}

	writer := self.writerFor(kind)

	if self.Color {
		fmt.Fprint(writer, self.styleColor(style))
	}

	fmt.Fprint(writer, msg)

	if self.Color {
		fmt.Fprint(writer, ansiReset)
	}

	if newline {
		fmt.Fprint(writer, "\n")
	}
}

func (self *TextPrinter) styleColor(style Style) string {
	switch style {
	case Green:
		return ansiGreen
	case Red:
		return ansiRed
	case Emphasis:
		return ansiYellow
	case Grey:
		return ansiGrey
	default:
		return ""
	}
}

func (self *TextPrinter) writerFor(kind Kind) io.Writer {
	if kind == Error {
		return self.Err
	}
	return self.Out
}

func (self *TextPrinter) colorFor(kind Kind) string {
	switch kind {
	case Success:
		return ansiGreen
	case Warn:
		return ansiYellow
	case Error:
		return ansiRed
	case Debug:
		return ansiGrey
	case Info:
		fallthrough
	default:
		return ""
	}
}

const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiGrey   = "\033[90m"
)

type Printer interface {
	Print(Kind, string)
	Println(Kind, string)
	Styled(Kind, Style, string, bool)
}

type NullPrinter struct{}

func (NullPrinter) Print(Kind, string)               {}
func (NullPrinter) Println(Kind, string)             {}
func (NullPrinter) Styled(Kind, Style, string, bool) {}
