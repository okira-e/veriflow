package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func drawStepView(model StepViewModel, rect Rect, focused bool) string {
	titleBackgroundColor := lipgloss.Color(focusedTabBg)
	if focused {
		titleBackgroundColor = lipgloss.Color(unfocusedTabBg)
	}

	title := "Step View"
	titleUI := lipgloss.NewStyle().
		Background(titleBackgroundColor).
		Render(centerIt(title, rect.W))

	content := ""

	if model.selectedStep.IsNone() {
		content = "\n" + centerIt("Select a step", rect.W)
	} else {
		// Incomplete
	}

	return lipgloss.
		NewStyle().
		MarginTop(rect.Y).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				titleUI,
				content,
			),
		)
}
