package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorOn       = lipgloss.AdaptiveColor{Light: "#2e7d32", Dark: "#4caf50"}
	colorOff      = lipgloss.AdaptiveColor{Light: "#9e9e9e", Dark: "#757575"}
	colorErr      = lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ef5350"}
	colorAccent   = lipgloss.AdaptiveColor{Light: "#3949ab", Dark: "#7986cb"}
	colorWarn     = lipgloss.AdaptiveColor{Light: "#e65100", Dark: "#ffb74d"}
	colorDim      = lipgloss.AdaptiveColor{Light: "#757575", Dark: "#9e9e9e"}
	colorBarFg    = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#ffffff"}
	colorBarBg    = lipgloss.AdaptiveColor{Light: "#3949ab", Dark: "#283593"}
	colorModalBrd = colorAccent
)

var (
	// styleOnDot / styleOffDot are a flag's state: the matrix's dots, and the
	// on/off words in the detail view. On is bold as well as green so a lit
	// column still stands out on a washed-out terminal.
	styleOnDot  = lipgloss.NewStyle().Foreground(colorOn).Bold(true)
	styleOffDot = lipgloss.NewStyle().Foreground(colorOff)
	styleError  = lipgloss.NewStyle().Foreground(colorErr)
	styleStatus = lipgloss.NewStyle().Foreground(colorOn)
	styleDim    = lipgloss.NewStyle().Foreground(colorDim)
	styleWarn   = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleBar    = lipgloss.NewStyle().Foreground(colorBarFg).Background(colorBarBg).Padding(0, 1)
	// styleRowKey / styleRowKeySelected are the flag-key column in the state
	// matrix; the selected row is the only one that takes the accent color.
	styleRowKey         = lipgloss.NewStyle()
	styleRowKeySelected = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	// styleDetail frames the detail view that replaces the list on enter.
	styleDetail   = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Padding(0, 1)
	styleModal    = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorModalBrd).Padding(1, 2)
	styleHelpLine = lipgloss.NewStyle().Foreground(colorDim)
)
