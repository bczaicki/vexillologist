package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phillips-connect/vexillologist/internal/ld"
)

const requestTimeout = 30 * time.Second

type projectsLoadedMsg []ld.Project
type envsLoadedMsg []ld.Environment
type flagsLoadedMsg []ld.FeatureFlag
type flagDetailMsg *ld.FeatureFlag
type flagToggledMsg struct {
	flag *ld.FeatureFlag
	env  string
	on   bool
}
type flagCreatedMsg *ld.FeatureFlag
type flagArchivedMsg struct{ key string }
type errMsg struct{ err error }

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}

func loadProjects(client ld.API) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		projects, err := client.ListProjects(ctx)
		if err != nil {
			return errMsg{err}
		}
		return projectsLoadedMsg(projects)
	}
}

func loadEnvs(client ld.API, project string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		envs, err := client.ListEnvironments(ctx, project)
		if err != nil {
			return errMsg{err}
		}
		return envsLoadedMsg(envs)
	}
}

func loadFlags(client ld.API, project, env string, archived bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		flags, err := client.ListFlags(ctx, project, ld.ListFlagsOptions{EnvKeys: []string{env}, Archived: archived})
		if err != nil {
			return errMsg{err}
		}
		return flagsLoadedMsg(flags)
	}
}

func loadFlagDetail(client ld.API, project, key, env string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		flag, err := client.GetFlag(ctx, project, key, []string{env})
		if err != nil {
			return errMsg{err}
		}
		return flagDetailMsg(flag)
	}
}

func toggleFlag(client ld.API, project, key, env string, on bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		flag, err := client.ToggleFlag(ctx, project, key, env, on, "via vex tui")
		if err != nil {
			return errMsg{err}
		}
		return flagToggledMsg{flag: flag, env: env, on: on}
	}
}

func createFlag(client ld.API, project string, params ld.CreateFlagParams) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		flag, err := client.CreateFlag(ctx, project, params)
		if err != nil {
			return errMsg{err}
		}
		return flagCreatedMsg(flag)
	}
}

func archiveFlag(client ld.API, project, key string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		if _, err := client.ArchiveFlag(ctx, project, key, "via vex tui"); err != nil {
			return errMsg{err}
		}
		return flagArchivedMsg{key: key}
	}
}
