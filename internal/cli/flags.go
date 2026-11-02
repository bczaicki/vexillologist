package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bczaicki/vexillologist/internal/ld"
	"github.com/bczaicki/vexillologist/internal/output"
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
		newFlagsSettingsCmd(g),
		newFlagsArchiveCmd(g),
		newFlagsRestoreCmd(g),
		newFlagsStatusesCmd(g),
	)
	return cmd
}

func newFlagsListCmd(g *globals) *cobra.Command {
	var filter, sortBy string
	var archived bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List flags in a project, with per-environment state, status, and age",
		RunE: func(cmd *cobra.Command, args []string) error {
			sortMode, err := ld.ParseSortMode(sortBy)
			if err != nil {
				return usageErrorf("%v", err)
			}
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

			// Statuses power the STATUS column and status sorting; their
			// absence shouldn't break a plain listing.
			statuses, statusErr := client.GetFlagStatuses(cmd.Context(), project, env)
			if statusErr != nil && sortMode == ld.SortStatus {
				return statusErr
			}

			ld.SortFlags(flags, sortMode, env, statuses)

			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), flags)
			}
			rows := make([][]string, len(flags))
			for i, f := range flags {
				state, changed := "-", "-"
				if envCfg, ok := f.Environments[env]; ok {
					state = onOff(envCfg.On)
					changed = output.RelativeAge(envCfg.LastModified)
				}
				status := "-"
				if s, ok := statuses[f.Key]; ok {
					status = s.Name
				}
				rows[i] = []string{f.Key, f.Name, state, changed, status, strings.Join(f.Tags, ",")}
			}
			header := []string{"KEY", "NAME", strings.ToUpper(env), "CHANGED", "STATUS", "TAGS"}
			if err := output.Table(cmd.OutOrStdout(), header, rows); err != nil {
				return err
			}
			if statusErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: flag statuses unavailable: %v\n", statusErr)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "substring filter on flag key/name")
	cmd.Flags().BoolVar(&archived, "archived", false, "list archived flags instead of active ones")
	cmd.Flags().StringVar(&sortBy, "sort", string(ld.SortKey),
		"sort order: key, name, created (newest first), longest-on (on-flags, oldest change first), recent, status (archive candidates first)")
	return cmd
}

func newFlagsStatusesCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "statuses",
		Short: "Show every flag's evaluation status in one environment (new/active/launched/inactive)",
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
			statuses, err := client.GetFlagStatuses(cmd.Context(), project, env)
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), statuses)
			}
			keys := make([]string, 0, len(statuses))
			for key := range statuses {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			rows := make([][]string, len(keys))
			for i, key := range keys {
				s := statuses[key]
				lastRequested := "-"
				if s.LastRequested != nil {
					lastRequested = output.RelativeAge(s.LastRequested.UnixMilli()) + " ago"
				}
				rows[i] = []string{key, s.Name, lastRequested}
			}
			return output.Table(cmd.OutOrStdout(), []string{"KEY", "STATUS", "LAST REQUESTED"}, rows)
		},
	}
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
	fmt.Fprintf(out, "  kind: %s   archived: %v\n", f.Kind, f.Archived)
	fmt.Fprintf(out, "  settings: %s\n", f.SettingsSummary())
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
	var temporary, permanent, clientSide, mobile bool
	cmd := &cobra.Command{
		Use:   "create <flag-key>",
		Short: "Create a new boolean flag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if temporary && permanent {
				return usageErrorf("--temporary and --permanent are mutually exclusive")
			}
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			params := ld.CreateFlagParams{
				Key:         args[0],
				Name:        name,
				Description: description,
				Tags:        tags,
				Temporary:   !permanent,
				ClientSideAvailability: &ld.ClientSideAvailability{
					UsingEnvironmentID: clientSide,
					UsingMobileKey:     mobile,
				},
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
	cmd.Flags().BoolVar(&clientSide, "client-side", true, "make the flag available to client-side SDKs (client-side ID)")
	cmd.Flags().BoolVar(&mobile, "mobile", false, "make the flag available to mobile SDKs (mobile key)")
	cmd.Flags().BoolVar(&permanent, "permanent", false, "mark the flag as permanent (flags are temporary by default)")
	cmd.Flags().BoolVar(&temporary, "temporary", false, "mark the flag as temporary")
	_ = cmd.Flags().MarkDeprecated("temporary", "flags are temporary unless --permanent is passed")
	return cmd
}

func newFlagsSettingsCmd(g *globals) *cobra.Command {
	var clientSide, mobile, permanent bool
	var comment string
	cmd := &cobra.Command{
		Use:   "settings <flag-key>",
		Short: "Change a flag's SDK availability or temporary/permanent setting (affects ALL environments)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Only the settings actually passed are changed.
			var update ld.FlagSettingsUpdate
			if cmd.Flags().Changed("client-side") {
				update.ClientSide = &clientSide
			}
			if cmd.Flags().Changed("mobile") {
				update.Mobile = &mobile
			}
			if cmd.Flags().Changed("permanent") {
				update.Permanent = &permanent
			}
			if update.Empty() {
				return usageErrorf("pass at least one of --client-side, --mobile, or --permanent")
			}
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			flagKey := args[0]
			if g.dryRun {
				return emitDryRun(cmd, g, "updateFlagSettings", "", ld.FlagSettingsPatch(update, comment))
			}
			client, err := g.newClient()
			if err != nil {
				return err
			}
			// Settings are flag-wide. Taking a flag away from an SDK breaks
			// every app evaluating it, so that gets the type-the-key
			// confirmation archiving does.
			desc := fmt.Sprintf("Set %s on flag %q in project %q — this affects ALL environments",
				update.Describe(), flagKey, project)
			if err := g.confirm(desc, flagKey, update.DisablesSDK()); err != nil {
				return err
			}
			flag, err := client.UpdateFlagSettings(cmd.Context(), project, flagKey, update, comment)
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), output.MutationResult{
					OK: true, Action: "updateFlagSettings", Flag: flag,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ %s: %s\n", flagKey, flag.SettingsSummary())
			return nil
		},
	}
	cmd.Flags().BoolVar(&clientSide, "client-side", false, "make the flag available to client-side SDKs (--client-side=false to remove)")
	cmd.Flags().BoolVar(&mobile, "mobile", false, "make the flag available to mobile SDKs (--mobile=false to remove)")
	cmd.Flags().BoolVar(&permanent, "permanent", false, "mark the flag permanent (--permanent=false for temporary)")
	cmd.Flags().StringVar(&comment, "comment", "via vex", "audit-log comment for the change")
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

func newFlagsRestoreCmd(g *globals) *cobra.Command {
	var comment string
	cmd := &cobra.Command{
		Use:   "restore <flag-key>",
		Short: "Restore an archived flag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			flagKey := args[0]
			if g.dryRun {
				return emitDryRun(cmd, g, "restoreFlag", "", ld.RestoreFlagPatch(comment))
			}
			client, err := g.newClient()
			if err != nil {
				return err
			}
			desc := fmt.Sprintf("Restore archived flag %q in project %q", flagKey, project)
			if err := g.confirm(desc, "", false); err != nil {
				return err
			}
			flag, err := client.RestoreFlag(cmd.Context(), project, flagKey, comment)
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), output.MutationResult{
					OK: true, Action: "restoreFlag", Flag: flag,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ restored flag %s\n", flagKey)
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
