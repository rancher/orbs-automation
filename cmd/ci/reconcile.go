package ci

import (
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile release state against GitHub",
	RunE: func(*cobra.Command, []string) error {
		return errs.NotImplemented("gero ci:reconcile")
	},
}
