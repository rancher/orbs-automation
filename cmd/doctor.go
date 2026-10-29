package cmd

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/rancher/orbs-automation/cmd/version"
	"github.com/rancher/orbs-automation/internal/cli/doctor"
	"github.com/spf13/cobra"
)

func init() {
	doctorCmd := &cobra.Command{
		Use:   "doctor [check...]",
		Short: "Check that this workstation is set up to use gero",
		Long: `Check that this workstation is set up to use gero, and say how to fix what is
not. It is read-only and never prints a token.

By default it runs only the checks that work offline. --online adds the ones
that need the network, each bounded by --timeout; a timeout is a warning, since
a flaky network is not a broken setup. Name one or more checks to run just
those (and the checks they need): naming an online check runs it without
--online. --list-checks shows every check.

The exit code is 0 when no check failed, warnings included, and 1 when one
failed or, with --strict, warned.`,
		Example: `  # Check this workstation
  gero doctor

  # Include the checks that call GitHub
  gero doctor --online

  # Run one check, as json
  gero doctor auth-api --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags, err := doctorFlags(cmd, args)
			if err != nil {
				return err
			}
			build := doctor.Build{
				Version: version.Version, Commit: version.Commit, Date: version.Date,
				OS: runtime.GOOS, Arch: runtime.GOARCH,
			}
			// The report is written even when a check failed and the command
			// exits non-zero.
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			home, _ := os.UserHomeDir()
			env := doctor.Environment{Settings: settingsFrom(cmd), Cwd: cwd, Home: home, Getenv: os.Getenv}
			out, err := doctor.Main(cmd.Context(), env, flags, build)
			if out != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), out)
			}
			return err
		},
	}
	f := doctorCmd.Flags()
	f.Bool("online", false, "also run the checks that need the network")
	f.Duration("timeout", 10*time.Second, "bound for each check that needs the network")
	f.Bool("strict", false, "exit non-zero on warnings too")
	f.Bool("list-checks", false, "list the checks and exit")
	rootCmd.AddCommand(doctorCmd)
}

// doctorFlags reads gero doctor's own flags into the plain values the logic takes.
func doctorFlags(cmd *cobra.Command, names []string) (doctor.Flags, error) {
	f := cmd.Flags()
	online, err := f.GetBool("online")
	if err != nil {
		return doctor.Flags{}, err
	}
	timeout, err := f.GetDuration("timeout")
	if err != nil {
		return doctor.Flags{}, err
	}
	strict, err := f.GetBool("strict")
	if err != nil {
		return doctor.Flags{}, err
	}
	list, err := f.GetBool("list-checks")
	if err != nil {
		return doctor.Flags{}, err
	}
	return doctor.Flags{Online: online, Timeout: timeout, Strict: strict, ListChecks: list, Names: names}, nil
}
