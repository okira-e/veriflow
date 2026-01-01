package utils

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
	"unicode"
)

func NewId() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789" // base36

	id := make([]byte, 10)
	for i := range id {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			panic("crypto/rand failed to generate a random ID: " + err.Error())
		}
		id[i] = alphabet[num.Int64()]
	}
	return string(id)
}

// PrettyJson pretty indents your json string
func PrettyJson(s []byte) (string, error) {
	var v any
	if err := json.Unmarshal(s, &v); err != nil {
		return "", err
	}

	out, _ := json.MarshalIndent(v, "", "  ")
	return string(out), nil
}

func PascalToScreamingSnake(s string) string {
	if s == "" {
		return ""
	}

	var out []rune
	runes := []rune(s)

	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}

			// Insert underscore on:
			// - lower/digit -> upper (FooBar -> FOO_BAR)
			// - acronym end (JSONData -> JSON_DATA)
			if (unicode.IsLower(prev) || unicode.IsDigit(prev)) && unicode.IsUpper(r) ||
				unicode.IsUpper(prev) && unicode.IsUpper(r) && next != 0 && unicode.IsLower(next) {
				out = append(out, '_')
			}
		}

		out = append(out, unicode.ToUpper(r))
	}

	return string(out)
}

func FormatDuration(duration time.Duration) string {
	if duration.Hours() >= 1 {
		return duration.Truncate(time.Minute).String()
	}

	if duration.Minutes() >= 1 {
		return duration.Truncate(time.Second).String()
	}

	return duration.Truncate(time.Millisecond).String()
}

func Stringify(val any) (string, error) {
	b, err := json.Marshal(val)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func Parse[T any](s string) (T, error) {
	var v T
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}

func ToDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil // caller decides default
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}

	return d, nil
}
