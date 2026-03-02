package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/okira-e/veriflow/app/opt"
)

func (model Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch model.focusedView {
		case ViewFlowsExplorer:
			model, cmd = handleFlowsExplorerEvent(model, cmd, msg)
		case ViewStepsExplorer:
			model, cmd = handleStepsExplorerEvent(model, cmd, msg)
		case ViewStepView:
			model, cmd = handleStepViewEvent(model, cmd, msg)
		}

		switch msg.String() {
		case "ctrl+c":
			return model, tea.Quit
		}

	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height
		model.layout = layout(model.width, model.height)
		model.flowsExplorerModel.List.SetSize(model.layout.FlowsExplorer.W, model.layout.FlowsExplorer.H)

	case list.FilterMatchesMsg:
		switch model.focusedView {
		case ViewFlowsExplorer:
			model.flowsExplorerModel.List, cmd = model.flowsExplorerModel.List.Update(msg)
		case ViewStepsExplorer:
			model.stepsExplorerModel.List, cmd = model.stepsExplorerModel.List.Update(msg)
		}
	}

	return model, cmd
}

func handleFlowsExplorerEvent(model Model, cmd tea.Cmd, msg tea.KeyMsg) (Model, tea.Cmd) {
	if model.flowsExplorerModel.List.FilterState() == list.Filtering {
		model.flowsExplorerModel.List, cmd = model.flowsExplorerModel.List.Update(msg)
		return model, cmd
	}

	switch msg.String() {
	case tea.KeyTab.String():
		if len(model.stepsExplorerModel.List.Items()) > 0 {
			model.focusedView = ViewStepsExplorer
		}

	case tea.KeyShiftTab.String():
		if model.stepViewModel.selectedStep.IsSome() {
			model.focusedView = ViewStepView
		}

	case tea.KeyEnter.String():
		selectedItem := model.flowsExplorerModel.List.SelectedItem()
		flowName := fmt.Sprintf("%s", selectedItem)
		flow, ok := model.cfg.GetFlow(flowName)
		if !ok {
			panic("Flow wasn't found from the flows explorer list: " + flowName)
		}

		model.stepsExplorerModel.selectedFlow = Some(flow)
		model.focusedView = ViewStepsExplorer

		items := make([]list.Item, len(flow.Steps))
		for i, step := range flow.Steps {
			items[i] = StepItem{
				title:       step.Name,
				description: fmt.Sprintf("[%s] %d", step.Request.Method, step.Assert.Status),
			}
		}
		model.stepsExplorerModel.List.SetItems(items)
		model.stepsExplorerModel.List.Select(0)

	default:
		model.flowsExplorerModel.List, cmd = model.flowsExplorerModel.List.Update(msg)
	}

	return model, cmd
}

func handleStepsExplorerEvent(model Model, cmd tea.Cmd, msg tea.KeyMsg) (Model, tea.Cmd) {
	if model.stepsExplorerModel.List.FilterState() == list.Filtering {
		model.stepsExplorerModel.List, cmd = model.stepsExplorerModel.List.Update(msg)
		return model, cmd
	}

	switch msg.String() {
	case tea.KeyTab.String():
		if model.stepViewModel.selectedStep.IsSome() {
			model.focusedView = ViewStepView
		} else {
			model.focusedView = ViewFlowsExplorer
		}

	case tea.KeyShiftTab.String():
		model.focusedView = ViewFlowsExplorer

	case tea.KeyEnter.String():
		selectedItem := model.stepsExplorerModel.List.SelectedItem()
		if selectedItem == nil {
			break
		}
		stepItem, ok := selectedItem.(StepItem)
		if !ok {
			break
		}
		flow := model.stepsExplorerModel.selectedFlow.Unwrap()
		step, ok := flow.GetStep(stepItem.title)
		if !ok {
			panic("Step wasn't found from the steps explorer list: " + stepItem.title)
		}

		model.stepViewModel.selectedStep = Some(step)
		model.stepViewModel.scrollOffset = 0

	default:
		model.stepsExplorerModel.List, cmd = model.stepsExplorerModel.List.Update(msg)
	}

	return model, cmd
}

func handleStepViewEvent(model Model, cmd tea.Cmd, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case tea.KeyTab.String():
		model.focusedView = ViewFlowsExplorer

	case tea.KeyShiftTab.String():
		model.focusedView = ViewStepsExplorer

	case "j", "down":
		model.stepViewModel.scrollOffset++

	case "k", "up":
		if model.stepViewModel.scrollOffset > 0 {
			model.stepViewModel.scrollOffset--
		}
	}

	return model, cmd
}
