package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Toggle       key.Binding
	New          key.Binding
	Archive      key.Binding
	ShowArchived key.Binding
	Project      key.Binding
	Env          key.Binding
	Refresh      key.Binding
	Filter       key.Binding
	Detail       key.Binding
	Help         key.Binding
	Back         key.Binding
	Quit         key.Binding
}

var keys = keyMap{
	Toggle:       key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle on/off")),
	New:          key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new flag")),
	Archive:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
	ShowArchived: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "archived view")),
	Project:      key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "switch project")),
	Env:          key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "switch env")),
	Refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Filter:       key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Detail:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open detail")),
	Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Back:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/cancel")),
	Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// shortHelp is the footer hint line on the flag list screen.
func (k keyMap) shortHelp() []key.Binding {
	return []key.Binding{k.Detail, k.Toggle, k.New, k.Archive, k.Filter, k.ShowArchived, k.Env, k.Project, k.Refresh, k.Quit}
}

// detailHelp is the footer hint line while the detail pane is focused.
func (k keyMap) detailHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "scroll")),
		k.Toggle,
		k.Archive,
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to list")),
		k.Quit,
	}
}
