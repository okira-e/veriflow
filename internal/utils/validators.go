package utils

import (
	"fmt"
	"net/url"
)

func ValidateUrl(s string) error {
	if s == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsed, err := url.ParseRequestURI(s)
	if err != nil {
		return fmt.Errorf("invalid URL format: %s", err.Error())
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL must start with http:// or https://")
	}

	if parsed.Host == "" {
		return fmt.Errorf("URL must contain a valid host")
	}

	return nil
}

func ValidateEmptyString(s string) error {
	if s == "" {
		return fmt.Errorf("field cannot be empty")
	}

	return nil
}
