package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirmModal is an overlay gating a mutation. When expected is set the user
// must type it verbatim (guarded environments, archives); otherwise y/n.
type confirmModal struct {
	prompt   string
	expected string
	input    textinput.Model
	// onConfirm runs when the user confirms.
	onConfirm tea.Cmd
}

func newConfirmModal(prompt, expected string, onConfirm tea.Cmd) *confirmModal {
	m := &confirmModal{prompt: prompt, expected: expected, onConfirm: onConfirm}
	if expected != "" {
		ti := textinput.New()
		ti.Placeholder = expected
		ti.Focus()
		ti.CharLimit = 128
		ti.Width = max(20, len(expected)+8)
		m.input = ti
	}
	return m
}

// update returns (done, cmd): done=true means the modal should close.
func (m *confirmModal) update(msg tea.Msg) (bool, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return false, nil
	}

	if m.expected == "" {
		switch strings.ToLower(keyMsg.String()) {
		case "y":
			return true, m.onConfirm
		case "n", "esc":
			return true, nil
		}
		return false, nil
	}

	switch keyMsg.String() {
	case "esc":
		return true, nil
	case "enter":
		if m.input.Value() == m.expected {
			return true, m.onConfirm
		}
		return false, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return false, cmd
}

func (m *confirmModal) view() string {
	var b strings.Builder
	b.WriteString(m.prompt + "\n\n")
	if m.expected == "" {
		b.WriteString(styleDim.Render("y to confirm · n/esc to cancel"))
	} else {
		b.WriteString(styleWarn.Render("guarded — type "+m.expected+" to confirm") + "\n")
		b.WriteString(m.input.View() + "\n")
		b.WriteString(styleDim.Render("enter to confirm · esc to cancel"))
	}
	return styleModal.Render(b.String())
}

// overlayCenter centers content over a w×h area.
func overlayCenter(w, h int, content string) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
