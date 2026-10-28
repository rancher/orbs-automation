package cmd

import (
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

func init() {
	linesCmd := &cobra.Command{
		Use:   "lines",
		Short: "Work with release lines",
	}
	linesCmd.AddCommand(&cobra.Command{
		Use:   "resolve",
		Short: "Resolve release lines",
		RunE: func(*cobra.Command, []string) error {
			return errs.NotImplemented("gero lines resolve")
		},
	})
	rootCmd.AddCommand(linesCmd)
}
