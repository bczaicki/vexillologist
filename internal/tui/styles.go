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
	styleOnDot    = lipgloss.NewStyle().Foreground(colorOn)
	styleOffDot   = lipgloss.NewStyle().Foreground(colorOff)
	styleError    = lipgloss.NewStyle().Foreground(colorErr)
	styleStatus   = lipgloss.NewStyle().Foreground(colorOn)
	styleDim      = lipgloss.NewStyle().Foreground(colorDim)
	styleWarn     = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleBar      = lipgloss.NewStyle().Foreground(colorBarFg).Background(colorBarBg).Padding(0, 1)
	styleDetail   = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorDim).Padding(0, 1)
	styleModal    = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorModalBrd).Padding(1, 2)
	styleHelpLine = lipgloss.NewStyle().Foreground(colorDim)
)
