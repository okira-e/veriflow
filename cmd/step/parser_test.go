package step

import (
	"testing"
)

// These tests are important because they validate user-provided CLI input parsing

func TestBuildAssertObjectFromExpressions(t *testing.T) {
	t.Run("parses all assertion types", func(t *testing.T) {
		exprs := []string{
			"exists $.data.id",
			"equals $.name John",
			`equals $.full_name "John Doe"`,
			"contains $.email @example",
		}

		result, err := BuildAssertObjectFromExpressions(exprs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != 4 {
			t.Fatalf("expected 4 assertions, got %d", len(result))
		}

		// exists
		if result[0].JsonPath != "$.data.id" || !result[0].Exists.IsSome() {
			t.Error("exists assertion not parsed correctly")
		}

		// equals unquoted
		if result[1].Equals.Unwrap() != "John" {
			t.Errorf("equals unquoted not parsed, got: %s", result[1].Equals.Unwrap())
		}

		// equals quoted
		if result[2].Equals.Unwrap() != "John Doe" {
			t.Errorf("equals quoted not parsed, got: %s", result[2].Equals.Unwrap())
		}

		// contains
		if result[3].Contains.Unwrap() != "@example" {
			t.Error("contains not parsed correctly")
		}
	})

	t.Run("rejects invalid syntax", func(t *testing.T) {
		invalid := []string{
			"",              // empty
			"unknown $.path", // invalid keyword
			"exists",        // missing jsonpath
			"equals $.path", // missing value
		}

		for _, expr := range invalid {
			_, err := BuildAssertObjectFromExpressions([]string{expr})
			if err == nil {
				t.Errorf("expected error for: %q", expr)
			}
		}
	})
}

func TestBuildExportsFromExpressions(t *testing.T) {
	t.Run("parses export expressions", func(t *testing.T) {
		exprs := []string{
			"user_id $.data.id",
			"token $.auth.token",
		}

		result, err := BuildExportsFromExpressions(exprs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result["user_id"] != "$.data.id" {
			t.Errorf("user_id not parsed, got: %s", result["user_id"])
		}
		if result["token"] != "$.auth.token" {
			t.Errorf("token not parsed, got: %s", result["token"])
		}
	})

	t.Run("rejects invalid format", func(t *testing.T) {
		invalid := []string{
			"",        // empty
			"user_id", // missing jsonpath
		}

		for _, expr := range invalid {
			_, err := BuildExportsFromExpressions([]string{expr})
			if err == nil {
				t.Errorf("expected error for: %q", expr)
			}
		}
	})
}
