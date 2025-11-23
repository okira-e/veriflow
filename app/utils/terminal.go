package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/oops"
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
		return false
	}
	term := os.Getenv("TERM")
	return term != "" && term != "dumb"
}

// HandleCliError will log and exit the program appropriately based on
// the error. It will skip if the error is nil.
// It takes a verbose flag for logging the entire error chain or just the root cause.
func HandleCliError(err error, verbose bool) {
	if err == nil {
		return
	}

	err = error(err)

	var appErr *oops.AppError
	isAppErr := errors.As(err, &appErr)

	if !verbose {
		appErr = appErr.RootCause().(*oops.AppError)
	}

	// Check if the error is a user one based on the root cause at
	// the bottom of the chain.
	isUserErr := false
	if isAppErr {
		if rootCause, ok := appErr.RootCause().(*oops.AppError); ok {
			isUserErr = rootCause.Code.IsUserError()
		}
	}

	// Handle JSON output format
	if cliopts.JSONOutput {
		if isAppErr {
			if cliopts.Verbose {
				// Use the Trail method with JSON format instead of encoding separately
				fmt.Fprintln(os.Stderr, appErr.TrailJSON(true))
			} else {
				_ = json.NewEncoder(os.Stderr).Encode(map[string]string{
					"error": appErr.Error(),
					"code":  appErr.Code.String(),
				})
			}
		} else {
			_ = json.NewEncoder(os.Stderr).Encode(map[string]string{
				"error": err.Error(),
			})
		}
	} else {
		// Handle text output format
		if isUserErr && IsColorEnabled() {
			fmt.Fprint(os.Stderr, "\033[31m") // red
		}

		if isAppErr {
			var errorMsg string
			if cliopts.Verbose {
				errorMsg = appErr.Trail()
			} else {
				errorMsg = appErr.Error()
			}
			fmt.Fprintf(os.Stderr, "%s\n", errorMsg)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}

		if isUserErr && IsColorEnabled() {
			fmt.Fprint(os.Stderr, "\033[0m") // reset
		}
	}

	// Determine exit code
	if isUserErr {
		os.Exit(2)
	} else {
		os.Exit(1)
	}
}
