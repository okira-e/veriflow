package tui

import "strings"

func centerIt(s string, w int) string {
	return strings.Repeat(" ", saturatingSub(w, len(s))/2) +
		s +
		strings.Repeat(" ", saturatingSub(w, len(s))/2)
}
