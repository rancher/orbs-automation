package cmd

import (
	"github.com/rancher/orbs-automation/internal/platform/errs"
	"github.com/spf13/cobra"
)

// addWorkflows adds the command to root.
func addWorkflows(root *cobra.Command) {
	workflowsCmd := &cobra.Command{
		Use:   "workflows",
		Short: "Render and check workflows in target repositories",
	}
	workflowsCmd.AddCommand(
		&cobra.Command{
			Use:   "sync",
			Short: "Render canonical workflows into a target repository",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero workflows sync",
					"Renders the canonical workflows into .github/workflows/ of a target repository, "+
						"shows a dry-run diff first, then opens a PR or commits the changes. Honors --dry-run.")
			},
		},
		&cobra.Command{
			Use:   "verify",
			Short: "Check rendered workflows for drift",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero workflows verify",
					"Read-only. Compares a target repository's rendered workflows with the canonical "+
						"templates and reports any drift.")
			},
		},
	)
	root.AddCommand(workflowsCmd)
}
