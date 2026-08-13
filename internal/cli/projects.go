package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/phillips-connect/vexillologist/internal/output"
)

func newProjectsCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Work with LaunchDarkly projects",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all projects in the account",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := g.newClient()
			if err != nil {
				return err
			}
			projects, err := client.ListProjects(cmd.Context())
			if err != nil {
				return err
			}
			if g.jsonOut {
				return output.JSON(cmd.OutOrStdout(), projects)
			}
			rows := make([][]string, len(projects))
			for i, p := range projects {
				rows[i] = []string{p.Key, p.Name, strings.Join(p.Tags, ",")}
			}
			return output.Table(cmd.OutOrStdout(), []string{"KEY", "NAME", "TAGS"}, rows)
		},
	})
	return cmd
}
