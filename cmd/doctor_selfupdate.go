package cmd

import (
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/cmd/ci"
	"github.com/rancher/orbs-automation/cmd/version"
	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/rancher/orbs-automation/internal/cli/selfupdate"
	"github.com/rancher/orbs-automation/internal/cli/termfacts"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/spf13/cobra"
)

// buildUpdater makes the Updater of the running gero. It is a variable so a
// test can give another that does not reach GitHub.
var buildUpdater = selfupdate.Build

// newDoctorSelfUpdate builds gero doctor self-update. It is for an engineer's
// workstation, outside the REPL, and is marked so (ci.ContextWorkstationOnly):
// hidden says whether this tree hides it from help and completion.
func newDoctorSelfUpdate(hidden bool) *cobra.Command {
	var checkOnly, yes, prerelease bool
	c := &cobra.Command{
		Use:    "self-update",
		Short:  "Update gero to the newest release",
		Hidden: hidden,
		Long: `Update gero to the newest release, in place.

It looks for a newer release on GitHub and, if there is one, shows it and asks
before replacing this binary. Nothing is installed unless the release proves to
be ours: its checksum manifest and its archive must each carry a Sigstore
signature made by this repository's release workflow on a version tag, and the
archive must match the manifest. A release without those signatures, or signed
by anyone else, is refused, and there is no flag to skip the check.

A gero that a package manager installed (Homebrew, apt), or that your user
cannot write, is not touched: it says so and names where to update it instead. A
development build is left alone. It runs on a workstation, not in CI and not in
gero repl.

--check says whether a newer release exists and stops; --dry-run says what would
be installed. A gero that is a pre-release also looks at pre-releases; --pre-release
lets a stable one do the same, and --pre-release=false keeps a pre-release to
stable releases. Without a terminal, pass --yes to approve.`,
		Example: `  # Update, after being shown the release and asked
  gero doctor self-update

  # Only say whether there is a newer release
  gero doctor self-update --check

  # Move a stable gero to the newest pre-release
  gero doctor self-update --pre-release`,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{ci.ContextAnnotation: ci.ContextWorkstationOnly},
		RunE: func(cmd *cobra.Command, _ []string) error {
			s := settingsFrom(cmd)
			// Unset, a pre-release gero follows pre-releases and a stable one does not.
			var prereleases *bool
			if cmd.Flags().Changed("pre-release") {
				prereleases = &prerelease
			}
			updater, err := buildUpdater(version.Version, prereleases)
			if err != nil {
				return err
			}
			out, err := selfupdate.Run(cmd.Context(), selfupdate.Options{
				Updater: updater, CheckOnly: checkOnly, DryRun: s.DryRun,
				Yes: yes || s.NonInteractive, Interactive: termfacts.IsTerminal(os.Stdin),
				In: cmd.InOrStdin(), Out: cmd.OutOrStdout(), Terminal: s.Terminal,
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}
	c.Flags().BoolVar(&checkOnly, "check", false, "only say whether a newer release exists")
	c.Flags().BoolVarP(&yes, "yes", "y", false, "approve the update without asking (env GERO_NON_INTERACTIVE)")
	c.Flags().BoolVar(&prerelease, "pre-release", false, "consider pre-releases (--pre-release=false for stable only); by default a pre-release gero does and a stable one does not")
	return c
}

// hideWorkstationOnly reports whether a tree built now hides the
// workstation-only commands: in CI, or inside gero repl, where they cannot run.
func hideWorkstationOnly(getenv func(string) string) bool {
	// A GERO_CONTEXT that is not valid is reported when the settings load, and
	// leaves the command listed.
	inCI, _ := clicfg.InCI(getenv)
	return inCI || getenv(repl.EnvRepl) != ""
}
