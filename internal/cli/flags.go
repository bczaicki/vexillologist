package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/phillips-connect/vexillologist/internal/ld"
	"github.com/phillips-connect/vexillologist/internal/output"
)

func newFlagsCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flags",
		Short: "Work with feature flags",
	}
	cmd.AddCommand(
		newFlagsListCmd(g),
		newFlagsGetCmd(g),
		newFlagsToggleCmd(g),
		newFlagsCreateCmd(g),
		newFlagsArchiveCmd(g),
	)
	return cmd
}

func newFlagsListCmd(g *globals) *cobra.Command {
	var filter string
	var archived bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List flags in a project, with per-environment on/off state",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			env, err := g.requireEnv()
			if err != nil {
				return err
			}
			client, err := g.newClient()
			if err != nil {
				return err
			}
			flags, err := client.ListFlags(cmd.Context(), project, ld.ListFlagsOptions{
				EnvKeys:  []string{env},
				Filter:   filter,
				Archived: archived,
			})
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), flags)
			}
			rows := make([][]string, len(flags))
			for i, f := range flags {
				state := "-"
				if envCfg, ok := f.Environments[env]; ok {
					if envCfg.On {
						state = "on"
					} else {
						state = "off"
					}
				}
				rows[i] = []string{f.Key, f.Name, state, strings.Join(f.Tags, ",")}
			}
			return output.Table(cmd.OutOrStdout(), []string{"KEY", "NAME", strings.ToUpper(env), "TAGS"}, rows)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "substring filter on flag key/name")
	cmd.Flags().BoolVar(&archived, "archived", false, "list archived flags instead of active ones")
	return cmd
}

func newFlagsGetCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "get <flag-key>",
		Short: "Show one flag's full configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			client, err := g.newClient()
			if err != nil {
				return err
			}
			// Environment is optional here: without it, LD returns all envs.
			var envKeys []string
			if g.resolved.Environment != "" {
				envKeys = []string{g.resolved.Environment}
			}
			flag, err := client.GetFlag(cmd.Context(), project, args[0], envKeys)
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), flag)
			}
			return printFlagHuman(cmd, flag)
		},
	}
}

func printFlagHuman(cmd *cobra.Command, f *ld.FeatureFlag) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s  (%s)\n", f.Key, f.Name)
	if f.Description != "" {
		fmt.Fprintf(out, "  %s\n", f.Description)
	}
	fmt.Fprintf(out, "  kind: %s   archived: %v   temporary: %v\n", f.Kind, f.Archived, f.Temporary)
	if len(f.Tags) > 0 {
		fmt.Fprintf(out, "  tags: %s\n", strings.Join(f.Tags, ", "))
	}
	if len(f.Variations) > 0 {
		fmt.Fprintln(out, "  variations:")
		for i, v := range f.Variations {
			name := v.Name
			if name != "" {
				name = " (" + name + ")"
			}
			fmt.Fprintf(out, "    [%d] %v%s\n", i, v.Value, name)
		}
	}
	if len(f.Environments) > 0 {
		fmt.Fprintln(out, "  environments:")
		for key, envCfg := range f.Environments {
			state := "off"
			if envCfg.On {
				state = "on"
			}
			fmt.Fprintf(out, "    %-16s %-4s rules: %d, individual targets: %d\n",
				key, state, len(envCfg.Rules), len(envCfg.Targets))
		}
	}
	return nil
}

func newFlagsToggleCmd(g *globals) *cobra.Command {
	var on, off bool
	var comment string
	cmd := &cobra.Command{
		Use:   "toggle <flag-key>",
		Short: "Turn a flag's targeting on or off in one environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if on == off {
				return usageErrorf("pass exactly one of --on or --off")
			}
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			envKey, err := g.requireEnv()
			if err != nil {
				return err
			}

			flagKey := args[0]
			action := "turnFlagOff"
			if on {
				action = "turnFlagOn"
			}

			if g.dryRun {
				return emitDryRun(cmd, g, action, envKey, ld.ToggleFlagPatch(envKey, on, comment))
			}

			client, err := g.newClient()
			if err != nil {
				return err
			}

			env, err := lookupEnv(cmd.Context(), client, project, envKey)
			if err != nil {
				return err
			}
			guarded := isGuardedEnv(env, g.resolved.Guarded)
			desc := fmt.Sprintf("Turn %s %q in environment %q (project %q)", onOff(on), flagKey, envKey, project)
			if err := g.confirm(desc, envKey, guarded); err != nil {
				return err
			}

			flag, err := client.ToggleFlag(cmd.Context(), project, flagKey, envKey, on, comment)
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), output.MutationResult{
					OK: true, Action: action, Environment: envKey, Critical: guarded, Flag: flag,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ %s is now %s in %s\n", flagKey, onOff(on), envKey)
			return nil
		},
	}
	cmd.Flags().BoolVar(&on, "on", false, "turn targeting on")
	cmd.Flags().BoolVar(&off, "off", false, "turn targeting off")
	cmd.Flags().StringVar(&comment, "comment", "via vex", "audit-log comment for the change")
	return cmd
}

func newFlagsCreateCmd(g *globals) *cobra.Command {
	var name, description string
	var tags []string
	var temporary bool
	cmd := &cobra.Command{
		Use:   "create <flag-key>",
		Short: "Create a new boolean flag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			params := ld.CreateFlagParams{
				Key:         args[0],
				Name:        name,
				Description: description,
				Tags:        tags,
				Temporary:   temporary,
			}
			if params.Name == "" {
				params.Name = args[0]
			}
			if g.dryRun {
				return emitDryRun(cmd, g, "createFlag", "", params)
			}
			client, err := g.newClient()
			if err != nil {
				return err
			}
			flag, err := client.CreateFlag(cmd.Context(), project, params)
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), output.MutationResult{
					OK: true, Action: "createFlag", Flag: flag,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ created flag %s (%s)\n", flag.Key, flag.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "human-readable flag name (defaults to the key)")
	cmd.Flags().StringVar(&description, "description", "", "flag description")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "comma-separated tags")
	cmd.Flags().BoolVar(&temporary, "temporary", false, "mark the flag as temporary")
	return cmd
}

func newFlagsArchiveCmd(g *globals) *cobra.Command {
	var comment string
	cmd := &cobra.Command{
		Use:   "archive <flag-key>",
		Short: "Archive a flag (affects ALL environments)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			flagKey := args[0]
			if g.dryRun {
				return emitDryRun(cmd, g, "archiveFlag", "", ld.ArchiveFlagPatch(comment))
			}
			client, err := g.newClient()
			if err != nil {
				return err
			}
			// Archiving is flag-wide, so it always gets the strong,
			// type-the-key confirmation.
			desc := fmt.Sprintf("Archive flag %q in project %q — this affects ALL environments", flagKey, project)
			if err := g.confirm(desc, flagKey, true); err != nil {
				return err
			}
			flag, err := client.ArchiveFlag(cmd.Context(), project, flagKey, comment)
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), output.MutationResult{
					OK: true, Action: "archiveFlag", Flag: flag,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ archived flag %s\n", flagKey)
			return nil
		},
	}
	cmd.Flags().StringVar(&comment, "comment", "via vex", "audit-log comment for the change")
	return cmd
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// lookupEnv fetches one environment so mutations can check its critical flag.
func lookupEnv(ctx context.Context, client ld.API, project, envKey string) (ld.Environment, error) {
	envs, err := client.ListEnvironments(ctx, project)
	if err != nil {
		return ld.Environment{}, err
	}
	for _, e := range envs {
		if e.Key == envKey {
			return e, nil
		}
	}
	return ld.Environment{}, &ld.APIError{StatusCode: 404, Code: "not_found",
		Message: fmt.Sprintf("environment %q not found in project %q", envKey, project)}
}

// emitDryRun prints what would be sent and exits successfully.
func emitDryRun(cmd *cobra.Command, g *globals, action, envKey string, payload any) error {
	if g.jsonOut {
		return output.JSON(cmd.OutOrStdout(), output.MutationResult{
			OK: true, Action: action, Environment: envKey, DryRun: true, Patch: payload,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "dry run — would send %s:\n", action)
	return output.JSON(cmd.OutOrStdout(), payload)
}
