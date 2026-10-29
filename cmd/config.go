package cmd

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/configshow"
	"github.com/rancher/orbs-automation/internal/cli/configvalidate"
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
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				out, err := configvalidate.Run(settingsFrom(cmd), cwd)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), out)
				return err
			},
		},
		&cobra.Command{
			Use:   "show",
			Short: "Show the resolved CLI config and where each value came from",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				out, err := configshow.Run(settingsFrom(cmd), cwd)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), out)
				return err
			},
		},
	)
	rootCmd.AddCommand(configCmd)
}
