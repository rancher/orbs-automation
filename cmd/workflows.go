package cmd

import (
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

func init() {
	workflowsCmd := &cobra.Command{
		Use:   "workflows",
		Short: "Render and check workflows in target repositories",
	}
	workflowsCmd.AddCommand(
		&cobra.Command{
			Use:   "sync",
			Short: "Render canonical workflows into a target repository",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero workflows sync")
			},
		},
		&cobra.Command{
			Use:   "verify",
			Short: "Check rendered workflows for drift",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero workflows verify")
			},
		},
	)
	rootCmd.AddCommand(workflowsCmd)
}
