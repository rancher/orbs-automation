package cmd

import (
	"time"

	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

func init() {
	doctorCmd := &cobra.Command{
		Use:   "doctor [check...]",
		Short: "Check that this workstation is set up to use gero",
		RunE: func(*cobra.Command, []string) error {
			return errs.NotImplemented("gero doctor",
				"Read-only. Checks the tools, config and GitHub access gero needs on this workstation, "+
					"and says how to fix whatever is wrong. Offline unless --online.")
		},
	}
	f := doctorCmd.Flags()
	f.Bool("online", false, "also run the checks that need the network")
	f.Duration("timeout", 10*time.Second, "bound for each check that needs the network")
	f.Bool("strict", false, "exit non-zero on warnings too")
	f.Bool("list-checks", false, "list the checks and exit")
	rootCmd.AddCommand(doctorCmd)
}
