package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phillips-connect/vexillologist/internal/cli"
	"github.com/phillips-connect/vexillologist/internal/ld"
)

// Run launches the interactive TUI. It satisfies cli.TUIRunner.
func Run(opts cli.TUIOptions) error {
	if opts.Token == "" {
		return errors.New("no API token: set LD_API_TOKEN, pass --api-token, or run `vex config set token=...`")
	}
	var ldOpts []ld.Option
	if opts.BaseURL != "" {
		ldOpts = append(ldOpts, ld.WithBaseURL(opts.BaseURL))
	}
	client := ld.NewClient(opts.Token, ldOpts...)

	program := tea.NewProgram(New(client, opts), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
