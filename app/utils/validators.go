package utils

import (
	"encoding/json"
	"fmt"
	"net/url"
	
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
