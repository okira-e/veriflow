package utils

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/okira-e/veriflow/app/cliopts"
)

func PrintInColor(color string, text string) {
	colors := map[string]string{
		"red":    "\033[31m",
		"green":  "\033[32m",
		"yellow": "\033[33m",
		"blue":   "\033[34m",
		"reset":  "\033[0m",
	}

	if code, ok := colors[color]; ok {
		fmt.Println(code + text + colors["reset"])
	} else {
		fmt.Println(text) // fallback no color
	}
}

// isColorEnabled returns true only when color isn't disabled and we're on a TTY-ish terminal.
// (Simple heuristic: respect NO_COLOR and require a TERM that usually supports ANSI.)
func isColorEnabled() bool {
	if cliopts.NoColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	return term != "" && term != "dumb"
}

func ErrorOut(code, msg string) {
	if cliopts.JSON {
		_ = json.NewEncoder(os.Stderr).Encode(map[string]string{
			"error": msg,
			"code":  code,
		})
	} else {
		if isColorEnabled() {
			fmt.Fprint(os.Stderr, "\033[31m") // red
		}
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		if isColorEnabled() {
			fmt.Fprint(os.Stderr, "\033[0m") // reset
		}
	}
}
