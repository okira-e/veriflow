package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func drawFlowsExplorer(model FlowsExplorerModel, rect Rect, focused bool) string {
	titleBackgroundColor := lipgloss.Color(focusedTabBg)
	if focused {
		titleBackgroundColor = lipgloss.Color(unfocusedTabBg)
	}

	title := "Flows"
	titleUI := lipgloss.NewStyle().
		Background(titleBackgroundColor).
		Render(centerIt(title, rect.W))

	content := ""
	if len(model.List.Items()) == 0 {
		content = "\n" + centerIt("No flows defined", rect.W)
	} else {
		content = model.List.View()
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

type FlowItem string

func (i FlowItem) FilterValue() string { return string(i) }

type FlowListItemDelegate struct{}

func (self FlowListItemDelegate) Height() int                             { return 1 }
func (self FlowListItemDelegate) Spacing() int                            { return 0 }
func (self FlowListItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (self FlowListItemDelegate) Render(writer io.Writer, model list.Model, index int, listItem list.Item) {
	item, ok := listItem.(FlowItem)
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
