package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/okira-e/veriflow/app/utils"
)

// HandleCliError will log and exit the program appropriately based on
// the error. It will skip if the error is nil.
// It takes a verbose flag for logging the entire error chain or just the root cause.
func HandleCliError(err error) {
	if err == nil {
		return
	}

	err = error(err)

	var appErr *oops.AppError
	isAppErr := errors.As(err, &appErr)

	if !cliopts.Verbose && isAppErr {
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
					"error": appErr.Error(), // @TODO: This returns the code in the message. Remove it.
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
		if isUserErr && utils.IsColorEnabled() {
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

		if isUserErr && utils.IsColorEnabled() {
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
