package utils

import (
	"os"

	"github.com/okira-e/veriflow/app/cliopts"
)

// IsColorEnabled returns true only when color isn't disabled and we're on a TTY-ish terminal.
// (Simple heuristic: respect NO_COLOR and require a TERM that usually supports ANSI.)
func IsColorEnabled() bool {
	if cliopts.NoColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	return term != "" && term != "dumb"
}
