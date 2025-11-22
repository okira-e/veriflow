package engine

import (
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/oops"
)

type ExecutionError struct {
	Err      *oops.AppError
	Response []byte
	Flow     *app.Flow
	Step     *app.Step
}

func (self *ExecutionError) Error() string {
	return self.Flow.Name + "/" + self.Step.Name + ": " + self.Err.Error()
}

func (self *ExecutionError) Unwrap() error {
	return self.Err
}
