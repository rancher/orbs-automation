package ci

import (
	"context"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/ciship"
	"github.com/rancher/orbs-automation/internal/cli/logging"
	"github.com/rancher/orbs-automation/internal/cli/ship"
	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/spf13/cobra"
)

func newShipCmd(settings func(*cobra.Command) clicfg.Settings) *cobra.Command {
	var in ship.Options
	cmd := &cobra.Command{
		Use:   "ship [project] [tag|line]",
		Short: "Ship a release of a project to the projects that take updates from it, from a workflow",
		Long: `Ship a release of <project> to its consumers, as the bot, from a workflow.

It does what gero ship does, with the same arguments: a workflow gives the tag
its build produced, and the PRs go to the consumers of that tag's line. It never
asks, so it takes none of the flags that answer or skip a question, and it always
pushes to the upstream of the consumer.

It needs a token that reaches the consumers, in GH_TOKEN: the workflow's own
GITHUB_TOKEN reaches only its own repository, so it is refused before anything is
written. GERO_BOT_USER names the bot the commits and PRs are made as.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			in.Project, in.Arg = ship.ParseArgs(args)
			s := settings(cmd)
			log := logging.New(cmd.ErrOrStderr(), s.LogLevel, s.CI)
			return ciship.Run(ctx, cmd.OutOrStdout(), log, s, cwd, in)
		},
	}
	ship.BindFlags(cmd.Flags(), &in)
	return cmd
}
