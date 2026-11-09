// Package ci holds the commands that only run inside GitHub Actions. They
// are named with the group prefix (ci:reconcile) and listed under the CI
// help group.
package ci

import (
	"fmt"

	"github.com/rancher/orbs-automation/cmd/groups"
	"github.com/spf13/cobra"
)

const (
	// ContextAnnotation is the cobra annotation key that declares where a
	// command may run.
	ContextAnnotation = "gero/context"
	// ContextCIOnly marks a command that requires the CI context and a token.
	ContextCIOnly = "ci-only"
	// ContextWorkstationOnly marks a command that runs only on an engineer's
	// workstation: not in CI, and not inside gero repl.
	ContextWorkstationOnly = "workstation-only"
)

func subCommandList() []*cobra.Command {
	return []*cobra.Command{
		newReconcileCmd(),
	}
}

// RegisterSubcommands adds new instances of the CI commands to cmd, named with
// the group prefix and marked CI-only. Each call builds them again, so a command
// tree built for each REPL line shares no flag state with another.
func RegisterSubcommands(cmd *cobra.Command) {
	for _, subCmd := range subCommandList() {
		subCmd.Use = fmt.Sprintf("%s:%s", groups.CIGroup.ID, subCmd.Use)
		subCmd.GroupID = groups.CIGroup.ID
		subCmd.Annotations = map[string]string{ContextAnnotation: ContextCIOnly}
		cmd.AddCommand(subCmd)
	}
}
