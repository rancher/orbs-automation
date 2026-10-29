package ci

import (
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile release state against GitHub",
	RunE: func(*cobra.Command, []string) error {
		return errs.NotImplemented("gero ci:reconcile",
			"Runs only in GitHub Actions, on a cron. Checks tags, PRs and check runs on GitHub, "+
				"then updates the release tracking issues: state, labels and progress comments.")
	},
}
