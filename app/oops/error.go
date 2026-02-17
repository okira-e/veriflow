package oops

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// AppError is an app defined and processed error object
type AppError struct {
	Code Code   `json:"code"` // programmatic category
	Msg  string `json:"msg"`  // safe, user-presentable summary
	Err  error  `json:"err"`  // wrapped cause (can be nil)
}

func Err(code Code, msg string, cause error) *AppError {
	// normalize unknowns
	if code == OK {
		code = Internal
	}

	return &AppError{
		Code: code,
		Msg:  msg,
		Err:  cause,
	}
}

func (self *AppError) Error() string {
	errMsg := fmt.Errorf("%s", self.Msg)
	if self.Err != nil {
		errMsg = fmt.Errorf("%s. %w", self.Msg, self.Err)
	}

	return fmt.Sprintf("[%s] %s", self.Code.String(), errMsg)
}

func (self *AppError) Unwrap() error {
	return self.Err
}

// Trail returns the full causal chain in a string format.
func (self *AppError) Trail() string {
	var parts []string

	current := error(self)
	for current != nil {
		if e, ok := current.(*AppError); ok {
			parts = append(parts, fmt.Sprintf("[%s] %s", e.Code.String(), e.Msg))
			current = e.Err
		} else {
			parts = append(parts, current.Error())
			current = errors.Unwrap(current)
		}
	}

	return strings.Join(parts, ": ")
}

// TrailJSON returns the full causal chain as a string format.
func (self *AppError) TrailJSON(pretty bool) string {
	var buildErrorChain func(err error) error
	buildErrorChain = func(err error) error {
		if err == nil {
			return &AppError{}
		}

		if appErr, ok := err.(*AppError); ok {
			node := AppError{
				Code: appErr.Code,
				Msg:  appErr.Msg,
			}

			if appErr.Err != nil {
				node.Err = buildErrorChain(appErr.Err)
			}

			return &node
		} else {
			// Regular Go error
			node := AppError{
				Code: Internal,
				Msg:  err.Error(),
			}

			if wrapped := errors.Unwrap(err); wrapped != nil {
				node.Err = buildErrorChain(wrapped)
			}

			return &node
		}
	}

	result := buildErrorChain(self)
	var err error
	var jsonBytes []byte
	if pretty {
		jsonBytes, err = json.MarshalIndent(result, "", "    ")
	} else {
		jsonBytes, err = json.Marshal(result)
	}
	if err != nil {
		return fmt.Sprintf(`{"code":"unknown","msg":"Failed to marshal error chain: %s"}`, err.Error())
	}

	return string(jsonBytes)
}

// RootCause returns the deepest underlying error in the chain.
func (self *AppError) RootCause() error {
	current := error(self)
	for {
		unwrapped := errors.Unwrap(current)
		var appErr *AppError
		isAppErr := errors.As(unwrapped, &appErr)

		if !isAppErr || unwrapped == nil {
			return current
		}
		current = unwrapped
	}
}
