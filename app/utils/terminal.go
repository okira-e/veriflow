package utils

import (
	"fmt"
	"os"

	"github.com/okira-e/veriflow/app/cliopts"
)

func PrintInColor(color string, text string, newLine bool) {
	colors := map[string]string{
		"red":     "\033[31m",
		"green":   "\033[32m",
		"yellow":  "\033[33m",
		"blue":    "\033[34m",
		"grey":    "\033[90m",
		"reset":   "\033[0m",
		"nothing": "",
	}

	isColorEnabled := IsColorEnabled()
	if !isColorEnabled {
		color = "nothing"
	}

	if code, ok := colors[color]; ok {
		fmt.Print(code + text + colors["reset"])
	} else {
		fmt.Print(text) // fallback no color
	}

	if newLine {
		fmt.Printf("\n")
	}
}

// IsColorEnabled returns true only when color isn't disabled and we're on a TTY-ish terminal.
// (Simple heuristic: respect NO_COLOR and require a TERM that usually supports ANSI.)
func IsColorEnabled() bool {
	if cliopts.NoColor || os.Getenv("NO_COLOR") != "" {
		fmt.Println("TRUE? ", cliopts.NoColor)
		return false
	}
	term := os.Getenv("TERM")
	return term != "" && term != "dumb"
}
