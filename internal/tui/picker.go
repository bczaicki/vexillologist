package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bczaicki/vexillologist/internal/ld"
)

// pickerItem is one selectable row in the project or environment picker.
type pickerItem struct {
	key      string
	name     string
	guarded  bool
	descNote string
}

func (i pickerItem) Title() string {
	if i.guarded {
		return i.key + " " + styleWarn.Render("⚠ critical")
	}
	return i.key
}

func (i pickerItem) Description() string {
	if i.descNote != "" {
		return i.name + " — " + i.descNote
	}
	return i.name
}

func (i pickerItem) FilterValue() string { return i.key + " " + i.name }

// picker wraps a bubbles list for choosing a project or environment.
type picker struct {
	list list.Model
}

func newPicker(title string) picker {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	return picker{list: l}
}

func (p *picker) setProjects(projects []ld.Project) {
	items := make([]list.Item, len(projects))
	for i, proj := range projects {
		items[i] = pickerItem{key: proj.Key, name: proj.Name}
	}
	p.list.SetItems(items)
}

func (p *picker) setEnvironments(envs []ld.Environment, extraGuarded []string) {
	items := make([]list.Item, len(envs))
	for i, env := range envs {
		items[i] = pickerItem{
			key:     env.Key,
			name:    env.Name,
			guarded: isGuardedEnv(env, extraGuarded),
		}
	}
	p.list.SetItems(items)
}

func (p *picker) setSize(w, h int) {
	p.list.SetSize(w, h)
}

// selected returns the highlighted item's key, or "" when the list is empty.
func (p *picker) selected() string {
	item, ok := p.list.SelectedItem().(pickerItem)
	if !ok {
		return ""
	}
	return item.key
}

func (p *picker) filtering() bool {
	return p.list.FilterState() == list.Filtering
}

func (p *picker) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
}

func (p *picker) view() string {
	return p.list.View()
}

// isGuardedEnv mirrors the CLI's guardrail: LD's critical flag, prod-like
// keys, or user-configured guarded environments.
func isGuardedEnv(env ld.Environment, extraGuarded []string) bool {
	if env.Critical || env.Key == "prod" || env.Key == "production" {
		return true
	}
	for _, key := range extraGuarded {
		if key == env.Key {
			return true
		}
	}
	return false
}
