package cli

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/okira-e/veriflow/app/utils"
)

var cliTheme = huh.ThemeCatppuccin()

func PromptForUrl(promptMsg string, placeHolder string, withTrailingSlash bool) (string, error) {
	var ret string
	err := huh.NewInput().
		Title(promptMsg).
		Value(&ret).
		Placeholder(placeHolder).
		Validate(utils.ValidateUrl).
		WithTheme(cliTheme).
		Run()

	if err != nil {
		return "", err
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
		input.Validate(utils.ValidateEmptyString)
	}

	err := input.WithTheme(cliTheme).Run()
	if err != nil {
		return "", err
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
		return utils.ValidateJson(s, !required)
	})

	err := input.WithTheme(cliTheme).WithHeight(20).Run()
	if err != nil {
		return "", err
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
			err := utils.ValidateEmptyString(s)
			if err != nil {
				return err
			}
		}

		intVal, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("Value should be a number.\n")
		}

		ret = intVal

		return nil
	})

	err := input.WithTheme(cliTheme).Run()
	if err != nil {
		return 0, err
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
		return false, err
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
				return fmt.Errorf("please select at least %d options", min)
			}

			return nil
		}).
		WithTheme(cliTheme).
		Run()

	if err != nil {
		return nil, err
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
			var defaultVal T

			if val == defaultVal {
				return fmt.Errorf("selection is required")
			}

			return nil
		})
	}
	err := input.WithTheme(cliTheme).Run()

	if err != nil {
		return ret, err
	}

	return ret, nil
}
