package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bczaicki/vexillologist/internal/config"
	"github.com/bczaicki/vexillologist/internal/output"
)

func newConfigCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the vexillologist config file",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Show the config file contents and its location",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			file, err := config.Load()
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), map[string]any{
					"path":                 path,
					"project":              file.Project,
					"environment":          file.Environment,
					"token_set":            file.Token != "",
					"guarded_environments": file.GuardedEnvironments,
				})
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "config file: %s\n", path)
			fmt.Fprintf(out, "project:     %s\n", orUnset(file.Project))
			fmt.Fprintf(out, "environment: %s\n", orUnset(file.Environment))
			tokenState := "not set"
			if file.Token != "" {
				tokenState = "set (prefer LD_API_TOKEN instead)"
			}
			fmt.Fprintf(out, "token:       %s\n", tokenState)
			if len(file.GuardedEnvironments) > 0 {
				fmt.Fprintf(out, "guarded:     %s\n", strings.Join(file.GuardedEnvironments, ", "))
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set key=value [key=value ...]",
		Short: "Set config values (project, environment, token)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load()
			if err != nil {
				return err
			}
			for _, arg := range args {
				key, value, found := strings.Cut(arg, "=")
				if !found {
					return usageErrorf("expected key=value, got %q", arg)
				}
				switch key {
				case "project":
					file.Project = value
				case "environment", "env":
					file.Environment = value
				case "token":
					file.Token = value
					fmt.Fprintln(cmd.ErrOrStderr(), "note: storing the token in the config file; the LD_API_TOKEN env var is preferred")
				default:
					return usageErrorf("unknown config key %q (valid: project, environment, token)", key)
				}
			}
			if err := config.Save(file); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ config saved")
			return nil
		},
	})

	return cmd
}

func orUnset(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}
