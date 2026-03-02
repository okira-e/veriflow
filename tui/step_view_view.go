package tui

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/okira-e/veriflow/app"
)

var (
	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#cba6f7"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6adc8"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cdd6f4"))

	methodStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#a6e3a1"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#585b70"))
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

	var content string
	if model.selectedStep.IsNone() {
		content = "\n" + centerIt("Select a step", rect.W)
	} else {
		step := model.selectedStep.Unwrap()
		lines := buildStepViewLines(step, rect.W)

		offset := model.scrollOffset
		if offset >= len(lines) {
			offset = saturatingSub(len(lines), 1)
		}
		visibleLines := lines[offset:]
		maxVisible := rect.H - 1
		if len(visibleLines) > maxVisible {
			visibleLines = visibleLines[:maxVisible]
		}

		content = strings.Join(visibleLines, "\n")
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

func buildStepViewLines(step *app.Step, w int) []string {
	var lines []string

	padding := "  "

	// Name
	lines = append(lines, "")
	lines = append(lines, padding+sectionTitleStyle.Render("Name"))
	lines = append(lines, padding+valueStyle.Render(step.Name))

	// Request
	lines = append(lines, "")
	lines = append(lines, padding+sectionTitleStyle.Render("Request"))
	lines = append(lines,
		padding+labelStyle.Render("Method: ")+methodStyle.Render(step.Request.Method))
	lines = append(lines,
		padding+labelStyle.Render("Path:   ")+valueStyle.Render(step.Request.Path))

	if step.Request.Headers.IsSome() {
		lines = append(lines, padding+labelStyle.Render("Headers:"))
		for k, v := range step.Request.Headers.Unwrap() {
			lines = append(lines, padding+"  "+dimStyle.Render(k+": ")+valueStyle.Render(v))
		}
	}

	if step.Request.Json.IsSome() {
		lines = append(lines, padding+labelStyle.Render("JSON Body:"))
		jsonBytes, err := json.MarshalIndent(step.Request.Json.Unwrap(), "", "  ")
		if err != nil {
			lines = append(lines, padding+"  "+valueStyle.Render(fmt.Sprintf("%v", step.Request.Json.Unwrap())))
		} else {
			for line := range strings.SplitSeq(string(jsonBytes), "\n") {
				lines = append(lines, padding+"  "+valueStyle.Render(line))
			}
		}
	}

	if step.Request.Xml.IsSome() {
		lines = append(lines, padding+labelStyle.Render("XML Body:"))
		rawXml := step.Request.Xml.Unwrap()
		if formatted, err := indentXml(rawXml); err == nil {
			for _, line := range strings.Split(formatted, "\n") {
				lines = append(lines, padding+"  "+valueStyle.Render(line))
			}
		} else {
			for _, line := range wrapText(rawXml, w-4) {
				lines = append(lines, padding+"  "+valueStyle.Render(line))
			}
		}
	}

	if step.Request.Files.IsSome() {
		lines = append(lines, padding+labelStyle.Render("Files:"))
		for field, path := range step.Request.Files.Unwrap() {
			lines = append(lines, padding+"  "+dimStyle.Render(field+": ")+valueStyle.Render(path))
		}
	}

	if step.Request.DisableHeaders {
		lines = append(lines, padding+labelStyle.Render("Disable Headers: ")+valueStyle.Render("true"))
	}

	// Assert
	lines = append(lines, "")
	lines = append(lines, padding+sectionTitleStyle.Render("Assert"))
	lines = append(lines,
		padding+labelStyle.Render("Status: ")+valueStyle.Render(fmt.Sprintf("%d", step.Assert.Status)))

	if step.Assert.All.IsSome() {
		assertions := step.Assert.All.Unwrap()
		lines = append(lines, padding+labelStyle.Render(fmt.Sprintf("Assertions (%d):", len(assertions))))
		for i, a := range assertions {
			lines = append(lines, padding+"  "+dimStyle.Render(fmt.Sprintf("#%d", i+1)))

			if a.JsonPath != "" {
				lines = append(lines, padding+"    "+labelStyle.Render("JsonPath: ")+valueStyle.Render(a.JsonPath))
			}
			if a.XPath != "" {
				lines = append(lines, padding+"    "+labelStyle.Render("XPath:    ")+valueStyle.Render(a.XPath))
			}
			if a.Exists.IsSome() {
				lines = append(lines, padding+"    "+labelStyle.Render("Exists:   ")+valueStyle.Render(fmt.Sprintf("%v", a.Exists.Unwrap())))
			}
			if a.Contains.IsSome() {
				lines = append(lines, padding+"    "+labelStyle.Render("Contains: ")+valueStyle.Render(a.Contains.Unwrap()))
			}
			if a.Equals.IsSome() {
				lines = append(lines, padding+"    "+labelStyle.Render("Equals:   ")+valueStyle.Render(a.Equals.Unwrap()))
			}
			if a.IsNot.IsSome() {
				lines = append(lines, padding+"    "+labelStyle.Render("IsNot:    ")+valueStyle.Render(a.IsNot.Unwrap()))
			}
			if a.Len.IsSome() {
				lines = append(lines, padding+"    "+labelStyle.Render("Length:   ")+valueStyle.Render(fmt.Sprintf("%d", a.Len.Unwrap())))
			}
		}
	}

	// Exports
	if len(step.Exports) > 0 {
		lines = append(lines, "")
		lines = append(lines, padding+sectionTitleStyle.Render("Exports"))
		for varName, jsonPath := range step.Exports {
			lines = append(lines, padding+dimStyle.Render(varName+" -> ")+valueStyle.Render(jsonPath))
		}
	}

	// Options
	if step.Options.Timeout.IsSome() {
		lines = append(lines, "")
		lines = append(lines, padding+sectionTitleStyle.Render("Options"))
		lines = append(lines, padding+labelStyle.Render("Timeout: ")+valueStyle.Render(step.Options.Timeout.Unwrap()))
	}

	return lines
}

func indentXml(raw string) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(raw))
	var tokens []xml.Token
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		tokens = append(tokens, xml.CopyToken(tok))
	}
	if len(tokens) == 0 {
		return "", fmt.Errorf("no XML tokens found")
	}

	var buf strings.Builder
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")
	for _, tok := range tokens {
		if err := encoder.EncodeToken(tok); err != nil {
			return "", err
		}
	}
	encoder.Flush()
	return buf.String(), nil
}

func wrapText(s string, maxWidth int) []string {
	if maxWidth <= 0 {
		maxWidth = 1
	}
	var result []string
	for _, line := range strings.Split(s, "\n") {
		for len(line) > maxWidth {
			result = append(result, line[:maxWidth])
			line = line[maxWidth:]
		}
		result = append(result, line)
	}
	return result
}
