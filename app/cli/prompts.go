package cli

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/okira-e/veriflow/app/oops"
)

var cliTheme = huh.ThemeCatppuccin()

// Adapter functions to convert our oops.AppError validators to standard error validators
// that the huh library expects
func validateUrlAdapter(s string) error {
	if err := ValidateUrl(s); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func validateEmptyStringAdapter(s string) error {
	if err := ValidateEmptyString(s); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func validateJsonAdapter(s string, allowEmpty bool) error {
	if err := ValidateJson(s, allowEmpty); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func PromptForUrl(promptMsg string, placeHolder string, withTrailingSlash bool) (string, error) {
	var ret string
	err := huh.NewInput().
		Title(promptMsg).
		Value(&ret).
		Placeholder(placeHolder).
		Validate(validateUrlAdapter).
		WithTheme(cliTheme).
		Run()

	if err != nil {
		switch {
		case isAborted(err):
			return "", oops.Err(oops.UserAborted, "user aborted URL prompt", err)
		default:
			return "", oops.Err(oops.PromptError, "URL prompt failed", err)
		}
	}

	if withTrailingSlash && len(ret) > 0 && ret[len(ret)-1] != '/' {
		ret += "/"
	}

	if !withTrailingSlash && len(ret) > 0 && ret[len(ret)-1] == '/' {
		ret = ret[:len(ret)-1]
	}

	return ret, nil
}

func PromptForString(promptMsg string, placeHolder string, required bool) (string, error) {
	var ret string

	input := huh.NewInput().
		Title(promptMsg).
		Value(&ret).
		Placeholder(placeHolder)

	if required {
		input.Validate(validateEmptyStringAdapter)
	}

	err := input.WithTheme(cliTheme).Run()
	if err != nil {
		switch {
		case isAborted(err):
			return "", oops.Err(oops.UserAborted, "user aborted string prompt", err)
		default:
			return "", oops.Err(oops.PromptError, "string prompt failed", err)
		}
	}

	return ret, nil
}

func PromptForJson(promptMsg string, placeholder string, required bool) (string, error) {
	var ret string

	input := huh.NewText().
		Title(promptMsg).
		Value(&ret).
		Placeholder(placeholder)

	input.Validate(func(s string) error {
		return validateJsonAdapter(s, !required)
	})

	err := input.WithTheme(cliTheme).WithHeight(20).Run()
	if err != nil {
		switch {
		case isAborted(err):
			return "", oops.Err(oops.UserAborted, "user aborted JSON prompt", err)
		default:
			return "", oops.Err(oops.PromptError, "JSON prompt failed", err)
		}
	}

	return ret, nil
}

func PromptForInt(promptMsg string, placeHolder string, required bool) (int, error) {
	var ret int

	var inputVal string
	input := huh.NewInput().
		Title(promptMsg).
		Value(&inputVal).
		Placeholder(placeHolder)

	input.Validate(func(s string) error {
		if required {
			err := ValidateEmptyString(s)
			if err != nil {
				return err
			}
		}

		intVal, err := strconv.Atoi(s)
		if err != nil {
			// Convert to standard error for huh validation
			return fmt.Errorf("Value should be a number: %s", err.Error())
		}

		ret = intVal

		return nil
	})

	err := input.WithTheme(cliTheme).Run()
	if err != nil {
		switch {
		case isAborted(err):
			return 0, oops.Err(oops.UserAborted, "user aborted integer prompt", err)
		default:
			return 0, oops.Err(oops.PromptError, "integer prompt failed", err)
		}
	}

	return ret, nil
}

func PromptForBool(promptMsg string) (bool, error) {
	ret := false

	confirmInput := huh.NewConfirm().
		Title(promptMsg).
		Value(&ret)

	err := confirmInput.WithTheme(cliTheme).Run()
	if err != nil {
		switch {
		case isAborted(err):
			return false, oops.Err(oops.UserAborted, "user aborted confirmation prompt", err)
		default:
			return false, oops.Err(oops.PromptError, "confirmation prompt failed", err)
		}
	}

	return ret, nil
}

func PromptForOptions[T comparable](promptMsg string, options []huh.Option[T], min int, limit int) ([]T, error) {
	if min == 0 {
		min = 1
	}

	if limit == 0 {
		limit = 1
	}

	ret := make([]T, limit)
	err := huh.NewMultiSelect[T]().
		Title(promptMsg).
		Options(options...).
		Limit(limit).
		Value(&ret).
		Validate(func(values []T) error {
			if len(values) < min {
				return oops.Err(oops.ValidationError, fmt.Sprintf("please select at least %d options", min), nil)
			}

			return nil
		}).
		WithTheme(cliTheme).
		Run()

	if err != nil {
		switch {
		case isAborted(err):
			return nil, oops.Err(oops.UserAborted, "user aborted multi-select prompt", err)
		default:
			return nil, oops.Err(oops.PromptError, "multi-select prompt failed", err)
		}
	}

	return ret, nil
}

func PromptForOption[T comparable](promptMsg string, options []huh.Option[T], required bool) (T, error) {
	var ret T
	input := huh.NewSelect[T]().
		Title(promptMsg).
		Options(options...).
		Value(&ret)

	if required {
		input.Validate(func(val T) error {
			var zero T

			if val == zero {
				return oops.Err(oops.ValidationError, "selection is required", nil)
			}

			return nil
		})
	}

	if err := input.WithTheme(cliTheme).Run(); err != nil {
		var zero T

		switch {
		case isAborted(err):
			return zero, oops.Err(oops.UserAborted, "user aborted option prompt", err)
		default:
			// library/runtime prompt failure or our validation error
			return zero, oops.Err(oops.PromptError, "option prompt failed", err)
		}
	}

	return ret, nil
}
