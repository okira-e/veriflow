package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

func drawStepsExplorer(model StepsExplorerModel, rect Rect, focused bool) string {
	titleBackgroundColor := lipgloss.Color("#8e74ac")
	if focused {
		titleBackgroundColor = lipgloss.Color(unfocusedTabBg)
	}

	title := "Step View"
	titleUI := lipgloss.NewStyle().
		Background(titleBackgroundColor).
		Render(centerIt(title, rect.W))

	content := ""
	if model.selectedFlow.IsNone() {
		content = "\n" + centerIt("Select a step", rect.W)
	} else {
		if len(model.List.Items()) == 0 {
			content = "Flow is empty"
		} else {
			model.List.SetWidth(rect.W)
			model.List.SetHeight(rect.H - 1 /* title height */)
			content = model.List.View()
		}
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

type StepItem struct {
	title       string
	description string
}

func (i StepItem) Title() string       { return i.title }
func (i StepItem) Description() string { return i.description }
func (i StepItem) FilterValue() string { return i.title }

func newStepsListItemDelagate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	return d
}

type StepListItemDelegate struct{}

func (self StepListItemDelegate) Height() int  { return 1 }
func (self StepListItemDelegate) Spacing() int { return 0 }
func (self StepListItemDelegate) Render(writer io.Writer, model list.Model, index int, listItem list.Item) {
	item, ok := listItem.(StepItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("• %s", item)

	itemStyle := lipgloss.NewStyle().Width(model.Width())
	selectedItemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#74c7ec")).
		Width(model.Width())

	renderItemFn := itemStyle.Render
	if model.Index() == index {
		renderItemFn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(writer, renderItemFn(str))
}
