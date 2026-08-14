package cli

import (
	"github.com/spf13/cobra"

	"github.com/bczaicki/vexillologist/internal/output"
)

func newEnvsCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "envs",
		Aliases: []string{"environments"},
		Short:   "Work with LaunchDarkly environments",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List environments in a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := g.requireProject()
			if err != nil {
				return err
			}
			client, err := g.newClient()
			if err != nil {
				return err
			}
			envs, err := client.ListEnvironments(cmd.Context(), project)
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), envs)
			}
			rows := make([][]string, len(envs))
			for i, e := range envs {
				critical := ""
				if e.Critical {
					critical = "yes"
				}
				rows[i] = []string{e.Key, e.Name, critical}
			}
			return output.Table(cmd.OutOrStdout(), []string{"KEY", "NAME", "CRITICAL"}, rows)
		},
	})
	return cmd
}
