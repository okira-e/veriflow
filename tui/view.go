package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (model Model) View() string {
	const statuslineHeight int = 1

	if len(model.cfg.Flows) == 0 {
		return "no flows mate"
	}

	flowsExplorerUI := drawFlowsExplorer(
		model.flowsExplorerModel,
		model.layout.FlowsExplorer,
		model.focusedView == ViewFlowsExplorer,
	)
	stepsExplorerUI := drawStepsExplorer(
		model.stepsExplorerModel,
		model.layout.StepsExplorer,
		model.focusedView == ViewStepsExplorer,
	)
	stepViewUI := drawStepView(
		model.stepViewModel,
		model.layout.StepView,
		model.focusedView == ViewStepView,
	)

	return lipgloss.JoinHorizontal(0, flowsExplorerUI, stepsExplorerUI, stepViewUI)
}
