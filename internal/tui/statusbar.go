package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderStatusBar draws the bottom bar: project ▸ env on the left, transient
// status or error on the right.
func renderStatusBar(width int, project, env, status string, err error) string {
	left := "vex"
	if project != "" {
		left = project
		if env != "" {
			left += " ▸ " + env
		}
	}
	left = styleBar.Render(left)

	right := ""
	switch {
	case err != nil:
		right = styleError.Render(err.Error())
	case status != "":
		right = styleStatus.Render(status)
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		// Truncate the right side rather than wrapping the bar.
		maxRight := max(0, width-lipgloss.Width(left)-1)
		right = lipgloss.NewStyle().MaxWidth(maxRight).Render(right)
		gap = max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	}
	return left + strings.Repeat(" ", gap) + right
}
