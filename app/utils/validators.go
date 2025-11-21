package utils

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/go-playground/validator/v10"
	"github.com/okira-e/veriflow/app/oops"
)

func ValidateUrl(s string) error {
	if s == "" {
		return oops.Err(oops.ValidationError, "URL cannot be empty", nil)
	}

	parsed, err := url.ParseRequestURI(s)
	if err != nil {
		return oops.Err(oops.ValidationError, fmt.Sprintf("invalid URL format: %s", err.Error()), err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return oops.Err(oops.ValidationError, "URL must start with http:// or https://", nil)
	}

	if parsed.Host == "" {
		return oops.Err(oops.ValidationError, "URL must contain a valid host", nil)
	}

	return nil
}

func ValidateEmptyString(s string) error {
	if s == "" {
		return oops.Err(oops.ValidationError, "field cannot be empty", nil)
	}

	return nil
}

// ValidateJson validates if unmarshaling the text into JSON
// causes en error. It also takes a param to allow for empty text
// to pass, making it optional.
func ValidateJson(s string, allowEmpty bool) error {
	if allowEmpty {
		if s == "" {
			return nil
		}
	}

	var js json.RawMessage
	if err := json.Unmarshal([]byte(s), &js); err != nil {
		return oops.Err(oops.JSONParseError, "Value is not valid JSON", err)
	}

	return nil
}

var validate = validator.New()

// ValidateStruct validates a struct using the validator package
func ValidateStruct(s any) error {
	if err := validate.Struct(s); err != nil {
		return oops.Err(
			oops.ValidationError,
			"failed to validate struct",
			err,
		)
	}

	return nil
}

// ValidateVar validates a single variable using the validator package
func ValidateVar(field any, tag string) error {
	if err := validate.Var(field, tag); err != nil {
		return oops.Err(
			oops.ValidationError,
			"failed to validate variable",
			err,
		)
	}

	return nil
}
