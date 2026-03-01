package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	. "github.com/okira-e/veriflow/app/opt"
)

type View uint8

const (
	ViewFlowsExplorer View = iota
	ViewStepsExplorer
	ViewStepView
)

// Model represents the application state
type Model struct {
	width              int
	height             int
	cfg                *config.Cfg
	layout             Layout
	flowsExplorerModel FlowsExplorerModel
	stepsExplorerModel StepsExplorerModel
	stepViewModel      StepViewModel
	focusedView        View
}

func NewModel(cfg *config.Cfg) Model {
	items := make([]list.Item, len(cfg.Flows))
	for i, f := range cfg.Flows {
		items[i] = FlowItem(f.Name)
	}

	model := Model{
		cfg:         cfg,
		focusedView: ViewFlowsExplorer,
	}

	flowsExplorerList := list.New(items, FlowListItemDelegate{}, 0, 0)
	flowsExplorerList.SetShowTitle(false)
	flowsExplorerList.SetShowStatusBar(true)
	flowsExplorerList.SetFilteringEnabled(true)
	flowsExplorerList.SetShowHelp(false)
	model.flowsExplorerModel.List = flowsExplorerList

	stepsExplorerList := list.New([]list.Item{}, newStepsListItemDelagate(), 0, 0)
	stepsExplorerList.SetShowTitle(false)
	stepsExplorerList.SetShowStatusBar(true)
	stepsExplorerList.SetFilteringEnabled(true)
	stepsExplorerList.SetShowHelp(false)
	model.stepsExplorerModel.List = stepsExplorerList

	return model
}

// Init initializes the model
func (model Model) Init() tea.Cmd {
	return nil
}

type FlowsExplorerModel struct {
	List list.Model
}

type StepsExplorerModel struct {
	List         list.Model
	selectedFlow Option[*app.Flow]
}

type StepViewModel struct {
	selectedStep Option[*app.Step]
}
