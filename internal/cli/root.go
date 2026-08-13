// Package cli implements the vex command tree.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/phillips-connect/vexillologist/internal/config"
	"github.com/phillips-connect/vexillologist/internal/ld"
	"github.com/phillips-connect/vexillologist/internal/output"
)

// Exit codes, stable for agents to branch on.
const (
	ExitOK           = 0
	ExitAPIError     = 1
	ExitUsage        = 2
	ExitConfirmation = 3
	ExitNotFound     = 4
)

// Version is stamped at build time via -ldflags.
var Version = "dev"

// globals holds the values of persistent flags plus resolved config.
type globals struct {
	apiToken string
	project  string
	env      string
	baseURL  string
	jsonOut  bool
	yes      bool
	dryRun   bool

	resolved config.Resolved
	// newClient builds the API client; tests replace it with a fake.
	newClient func() (ld.API, error)
}

// codedError pairs an error with the process exit code it should produce.
type codedError struct {
	code int
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

func usageErrorf(format string, args ...any) error {
	return &codedError{code: ExitUsage, err: fmt.Errorf(format, args...)}
}

func confirmationError(msg string) error {
	return &codedError{code: ExitConfirmation, err: errors.New(msg)}
}

// exitCodeFor maps an error to the vex exit code contract.
func exitCodeFor(err error) int {
	var coded *codedError
	if errors.As(err, &coded) {
		return coded.code
	}
	if ld.IsNotFound(err) {
		return ExitNotFound
	}
	return ExitAPIError
}

// TUIOptions carries the resolved session settings into the TUI without the
// cli package importing the tui package.
type TUIOptions struct {
	Token       string
	BaseURL     string
	Project     string
	Environment string
	Guarded     []string
}

// TUIRunner launches the interactive TUI with resolved settings.
type TUIRunner func(opts TUIOptions) error

// NewRootCmd builds the vex command tree. tuiRunner runs the interactive TUI
// (nil until wired up) so the cli package doesn't import the tui package.
func NewRootCmd(tuiRunner TUIRunner) *cobra.Command {
	g := &globals{}

	root := &cobra.Command{
		Use:           "vex",
		Short:         "vexillologist — a LaunchDarkly feature-flag TUI and CLI",
		Long:          "vexillologist (vex) views and manages LaunchDarkly feature flags.\nRun with no arguments for the interactive TUI; use subcommands for scripting.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load()
			if err != nil {
				return usageErrorf("loading config: %v", err)
			}
			g.resolved = config.Resolve(config.Flags{
				Token:       g.apiToken,
				Project:     g.project,
				Environment: g.env,
			}, file)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if tuiRunner == nil {
				return cmd.Help()
			}
			return tuiRunner(g.tuiOptions())
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&g.apiToken, "api-token", "", "LaunchDarkly API access token (or set LD_API_TOKEN)")
	pf.StringVarP(&g.project, "project", "p", "", "LaunchDarkly project key")
	pf.StringVarP(&g.env, "env", "e", "", "LaunchDarkly environment key")
	pf.StringVar(&g.baseURL, "base-url", "", "override the LaunchDarkly API base URL")
	pf.BoolVar(&g.jsonOut, "json", false, "emit machine-readable JSON on stdout")
	pf.BoolVar(&g.yes, "yes", false, "skip confirmation prompts")
	pf.BoolVar(&g.dryRun, "dry-run", false, "print the request that would be sent without sending it")

	g.newClient = func() (ld.API, error) {
		if g.resolved.Token == "" {
			return nil, usageErrorf("no API token: set LD_API_TOKEN, pass --api-token, or run `vex config set token=...`")
		}
		var opts []ld.Option
		if g.baseURL != "" {
			opts = append(opts, ld.WithBaseURL(g.baseURL))
		}
		return ld.NewClient(g.resolved.Token, opts...), nil
	}

	root.AddCommand(
		newProjectsCmd(g),
		newEnvsCmd(g),
		newFlagsCmd(g),
		newConfigCmd(g),
	)
	if tuiRunner != nil {
		root.AddCommand(&cobra.Command{
			Use:   "tui",
			Short: "Launch the interactive TUI (same as running vex with no arguments)",
			RunE: func(cmd *cobra.Command, args []string) error {
				return tuiRunner(g.tuiOptions())
			},
		})
	}
	return root
}

func (g *globals) tuiOptions() TUIOptions {
	return TUIOptions{
		Token:       g.resolved.Token,
		BaseURL:     g.baseURL,
		Project:     g.resolved.Project,
		Environment: g.resolved.Environment,
		Guarded:     g.resolved.Guarded,
	}
}

// Execute runs the command tree and returns the process exit code, printing
// errors as JSON envelopes (stdout) or human text (stderr) per --json.
func Execute(tuiRunner TUIRunner) int {
	root := NewRootCmd(tuiRunner)
	err := root.Execute()
	if err == nil {
		return ExitOK
	}

	code := exitCodeFor(err)
	if jsonRequested(root) {
		var apiErr *ld.APIError
		detail := output.ErrorDetail{Code: errorCodeName(code), Message: err.Error()}
		if errors.As(err, &apiErr) && apiErr.Code != "" {
			detail.Code = apiErr.Code
		}
		_ = output.JSON(os.Stdout, output.ErrorResult{OK: false, Error: detail})
	} else {
		fmt.Fprintf(os.Stderr, "vex: %v\n", err)
	}
	return code
}

func jsonRequested(root *cobra.Command) bool {
	f := root.PersistentFlags().Lookup("json")
	return f != nil && f.Value.String() == "true"
}

func errorCodeName(code int) string {
	switch code {
	case ExitUsage:
		return "usage_error"
	case ExitConfirmation:
		return "confirmation_required"
	case ExitNotFound:
		return "not_found"
	default:
		return "api_error"
	}
}

// requireProject returns the resolved project key or a usage error.
func (g *globals) requireProject() (string, error) {
	if g.resolved.Project == "" {
		return "", usageErrorf("no project: pass -p/--project, set LD_PROJECT, or run `vex config set project=...`")
	}
	return g.resolved.Project, nil
}

// requireEnv returns the resolved environment key or a usage error.
func (g *globals) requireEnv() (string, error) {
	if g.resolved.Environment == "" {
		return "", usageErrorf("no environment: pass -e/--env, set LD_ENVIRONMENT, or run `vex config set environment=...`")
	}
	return g.resolved.Environment, nil
}
