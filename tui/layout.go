package tui

type Layout struct {
	FlowsExplorer Rect
	StepsExplorer Rect
	StepView      Rect
}

func layout(fullWidth int, fullHeight int) Layout {
	flowsExplorerWidth := fourthOf(fullWidth)
	return Layout{
		FlowsExplorer: Rect{
			W: flowsExplorerWidth,
			H: fullHeight - 1,
		},
		StepsExplorer: Rect{
			X: flowsExplorerWidth,
			W: fourthOf(fullWidth),
			H: fullHeight - 1,
		},
		StepView: Rect{
			W: fullWidth / 2,
			H: fullHeight - 2,
		},
	}
}

type Rect struct {
	X, Y, W, H int
}

func thirdOf(x int) int {
	return x / 3
}

func fourthOf(x int) int {
	return x / 4
}

func saturatingSub[T int | ~uint](a, b T) T {
	if a < b {
		return 0
	}
	return a - b
}
