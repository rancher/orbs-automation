package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	semverCmd := &cobra.Command{
		Use:   "semver",
		Short: "Work with version strings",
	}
	semverCmd.AddCommand(&cobra.Command{
		Use:   "check [tag]",
		Short: "Derive version facts from a semver string",
		Args:  cobra.ArbitraryArgs,
		RunE:  func(*cobra.Command, []string) error { return nil },
	})
	rootCmd.AddCommand(semverCmd)
}
