package cmd

import (
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Validate and inspect the application config",
	}
	configCmd.AddCommand(
		&cobra.Command{
			Use:   "validate",
			Short: "Validate the application config",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero config validate")
			},
		},
		&cobra.Command{
			Use:   "show",
			Short: "Show the application config",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero config show")
			},
		},
	)
	rootCmd.AddCommand(configCmd)
}
