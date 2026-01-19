package engine

import (
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/oops"
)

type AssertionFailure struct {
	Err      *oops.AppError
	Response []byte
	Flow     *app.Flow
	Step     *app.Step
}

func (self *AssertionFailure) Error() string {
	flowName := "---"
	if self.Flow != nil {
		flowName = self.Flow.Name
	}
	return flowName + "/" + self.Step.Name + ": " + self.Err.Error()
}

func (self *AssertionFailure) Unwrap() error {
	return self.Err
}
