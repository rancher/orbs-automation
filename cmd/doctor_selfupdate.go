package cmd

import (
	"fmt"

	"github.com/rancher/orbs-automation/cmd/ci"
	"github.com/rancher/orbs-automation/internal/cli/repl"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/spf13/cobra"
)

// newDoctorSelfUpdate builds gero doctor self-update. It is for an engineer's
// workstation, outside the REPL, and is marked so (ci.ContextWorkstationOnly):
// hidden says whether this tree hides it from help and completion.
func newDoctorSelfUpdate(hidden bool) *cobra.Command {
	return &cobra.Command{
		Use:         "self-update",
		Short:       "Update gero on this workstation",
		Hidden:      hidden,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{ci.ContextAnnotation: ci.ContextWorkstationOnly},
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "gero doctor self-update: not built yet")
			return err
		},
	}
}

// hideWorkstationOnly reports whether a tree built now hides the
// workstation-only commands: in CI, or inside gero repl, where they cannot run.
func hideWorkstationOnly(getenv func(string) string) bool {
	// A GERO_CONTEXT that is not valid is reported when the settings load, and
	// leaves the command listed.
	inCI, _ := clicfg.InCI(getenv)
	return inCI || getenv(repl.EnvRepl) != ""
}
