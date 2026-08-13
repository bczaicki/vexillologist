// Package tui implements the interactive vexillologist interface.
package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/phillips-connect/vexillologist/internal/cli"
	"github.com/phillips-connect/vexillologist/internal/ld"
)

type screen int

const (
	screenProjectPicker screen = iota
	screenEnvPicker
	screenFlagList
	screenCreateForm
)

// Model is the root bubbletea model: shared session state, the active
// screen's sub-model, and an optional confirm-modal overlay.
type Model struct {
	client  ld.API
	project string
	env     string
	envs    []ld.Environment
	guarded []string

	screen  screen
	picker  picker
	flags   flagList
	form    createForm
	confirm *confirmModal

	spinner spinner.Model
	loading bool
	status  string
	err     error

	width, height int
}

// New builds the root model from resolved CLI options.
func New(client ld.API, opts cli.TUIOptions) Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := Model{
		client:  client,
		project: opts.Project,
		env:     opts.Environment,
		guarded: opts.Guarded,
		flags:   newFlagList(),
		spinner: sp,
		loading: true,
	}
	switch {
	case m.project == "":
		m.screen = screenProjectPicker
		m.picker = newPicker("projects")
	case m.env == "":
		m.screen = screenEnvPicker
		m.picker = newPicker("environments — " + m.project)
	default:
		m.screen = screenFlagList
		// Initialized even though unused: setSize touches it on resize, and
		// a zero-value bubbles list panics.
		m.picker = newPicker("projects")
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	switch m.screen {
	case screenProjectPicker:
		cmds = append(cmds, loadProjects(m.client))
	case screenEnvPicker:
		cmds = append(cmds, loadEnvs(m.client, m.project))
	case screenFlagList:
		// envs are needed for the guarded check on toggles.
		cmds = append(cmds, loadEnvs(m.client, m.project), loadFlags(m.client, m.project, m.env))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		contentHeight := m.height - 1 // status bar
		m.picker.setSize(m.width, contentHeight)
		m.flags.setSize(m.width, contentHeight)
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		sp, cmd := m.spinner.Update(msg)
		m.spinner = sp
		return m, cmd

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case projectsLoadedMsg:
		m.loading = false
		m.picker.setProjects(msg)
		return m, nil

	case envsLoadedMsg:
		m.envs = msg
		if m.screen == screenEnvPicker {
			m.loading = false
			m.picker.setEnvironments(msg, m.guarded)
		}
		return m, nil

	case flagsLoadedMsg:
		m.loading = false
		m.flags.setFlags(msg, m.env)
		m.status = fmt.Sprintf("%d flags", len(msg))
		return m, nil

	case flagDetailMsg:
		m.loading = false
		m.flags.setFullDetail(msg, m.env)
		return m, nil

	case flagToggledMsg:
		m.loading = false
		m.flags.updateFlag(msg.flag, m.env)
		m.status = fmt.Sprintf("✓ %s is now %s in %s", msg.flag.Key, onOff(msg.on), msg.env)
		return m, nil

	case flagCreatedMsg:
		m.loading = false
		m.screen = screenFlagList
		m.status = fmt.Sprintf("✓ created %s", (*ld.FeatureFlag)(msg).Key)
		// Reload so the new flag appears with proper env state.
		m.loading = true
		return m, loadFlags(m.client, m.project, m.env)

	case flagArchivedMsg:
		m.loading = false
		m.flags.removeFlag(msg.key, m.env)
		m.status = fmt.Sprintf("✓ archived %s", msg.key)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, m.routeToScreen(msg)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// A new keypress clears transient status/error lines.
	m.err = nil

	// The confirm modal captures all keys while open.
	if m.confirm != nil {
		done, cmd := m.confirm.update(msg)
		if done {
			m.confirm = nil
			if cmd != nil {
				m.loading = true
				return m, tea.Batch(cmd, m.spinner.Tick)
			}
		}
		return m, cmd
	}

	// While a list filter prompt is open, keys belong to the list.
	filtering := (m.screen == screenProjectPicker || m.screen == screenEnvPicker) && m.picker.filtering() ||
		m.screen == screenFlagList && m.flags.filtering()
	if filtering {
		return m, m.routeToScreen(msg)
	}

	if m.screen == screenCreateForm {
		params, done, cmd := m.form.update(msg)
		if done {
			m.screen = screenFlagList
			if params != nil {
				m.loading = true
				return m, tea.Batch(createFlag(m.client, m.project, *params), m.spinner.Tick)
			}
		}
		return m, cmd
	}

	if key.Matches(msg, keys.Quit) {
		return m, tea.Quit
	}

	switch m.screen {
	case screenProjectPicker:
		if key.Matches(msg, keys.Detail) {
			if selected := m.picker.selected(); selected != "" {
				m.project = selected
				m.screen = screenEnvPicker
				m.picker = newPicker("environments — " + m.project)
				m.picker.setSize(m.width, m.height-1)
				m.loading = true
				return m, tea.Batch(loadEnvs(m.client, m.project), m.spinner.Tick)
			}
			return m, nil
		}

	case screenEnvPicker:
		if key.Matches(msg, keys.Detail) {
			if selected := m.picker.selected(); selected != "" {
				m.env = selected
				m.screen = screenFlagList
				m.loading = true
				return m, tea.Batch(loadFlags(m.client, m.project, m.env), m.spinner.Tick)
			}
			return m, nil
		}
		if key.Matches(msg, keys.Back) {
			m.screen = screenProjectPicker
			m.picker = newPicker("projects")
			m.picker.setSize(m.width, m.height-1)
			m.loading = true
			return m, tea.Batch(loadProjects(m.client), m.spinner.Tick)
		}

	case screenFlagList:
		switch {
		case key.Matches(msg, keys.Detail):
			if flag := m.flags.selected(); flag != nil {
				m.loading = true
				return m, tea.Batch(loadFlagDetail(m.client, m.project, flag.Key, m.env), m.spinner.Tick)
			}
			return m, nil

		case key.Matches(msg, keys.Toggle):
			return m.startToggle()

		case key.Matches(msg, keys.Archive):
			return m.startArchive()

		case key.Matches(msg, keys.New):
			m.form = newCreateForm()
			m.screen = screenCreateForm
			return m, nil

		case key.Matches(msg, keys.Refresh):
			m.loading = true
			return m, tea.Batch(loadFlags(m.client, m.project, m.env), m.spinner.Tick)

		case key.Matches(msg, keys.Env):
			m.screen = screenEnvPicker
			m.picker = newPicker("environments — " + m.project)
			m.picker.setSize(m.width, m.height-1)
			m.loading = true
			return m, tea.Batch(loadEnvs(m.client, m.project), m.spinner.Tick)

		case key.Matches(msg, keys.Project):
			m.screen = screenProjectPicker
			m.picker = newPicker("projects")
			m.picker.setSize(m.width, m.height-1)
			m.loading = true
			return m, tea.Batch(loadProjects(m.client), m.spinner.Tick)
		}
	}

	return m, m.routeToScreen(msg)
}

// startToggle opens the appropriate confirm modal for toggling the selected flag.
func (m Model) startToggle() (tea.Model, tea.Cmd) {
	flag := m.flags.selected()
	if flag == nil {
		return m, nil
	}
	envCfg, ok := flag.Environments[m.env]
	if !ok {
		m.err = errors.New("no environment data for " + m.env)
		return m, nil
	}
	turnOn := !envCfg.On
	prompt := fmt.Sprintf("Turn %s %s in %s?", strings.ToUpper(onOff(turnOn)), flag.Key, m.env)
	expected := ""
	if m.envIsGuarded() {
		expected = m.env
	}
	m.confirm = newConfirmModal(prompt, expected, toggleFlag(m.client, m.project, flag.Key, m.env, turnOn))
	return m, nil
}

// startArchive opens the strong confirm modal for archiving the selected flag.
func (m Model) startArchive() (tea.Model, tea.Cmd) {
	flag := m.flags.selected()
	if flag == nil {
		return m, nil
	}
	prompt := fmt.Sprintf("Archive %s? This affects ALL environments.", flag.Key)
	m.confirm = newConfirmModal(prompt, flag.Key, archiveFlag(m.client, m.project, flag.Key))
	return m, nil
}

func (m *Model) envIsGuarded() bool {
	for _, env := range m.envs {
		if env.Key == m.env {
			return isGuardedEnv(env, m.guarded)
		}
	}
	// Unknown env (envs not loaded yet): err on the side of caution.
	return true
}

// routeToScreen forwards a message to the active screen's sub-model.
func (m *Model) routeToScreen(msg tea.Msg) tea.Cmd {
	switch m.screen {
	case screenProjectPicker, screenEnvPicker:
		return m.picker.update(msg)
	case screenFlagList:
		return m.flags.update(msg, m.env)
	}
	return nil
}

func (m Model) View() string {
	var content string
	switch m.screen {
	case screenProjectPicker, screenEnvPicker:
		content = m.picker.view()
	case screenFlagList:
		content = m.flags.view()
	case screenCreateForm:
		content = m.form.view()
	}

	if m.confirm != nil {
		content = overlayCenter(m.width, max(1, m.height-1), m.confirm.view())
	}

	status := m.status
	if m.loading {
		status = m.spinner.View() + "loading…"
	}
	bar := renderStatusBar(m.width, m.project, m.env, status, m.err)

	contentHeight := max(1, m.height-1)
	content = fitHeight(content, contentHeight)
	return content + "\n" + bar
}

// fitHeight pads or truncates content to exactly h lines so the status bar
// stays pinned to the bottom row.
func fitHeight(content string, h int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
