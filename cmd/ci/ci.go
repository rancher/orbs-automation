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
	// ContextCIOnly marks a command that requires GITHUB_ACTIONS and a token.
	ContextCIOnly = "ci-only"
)

func subCommandList() []*cobra.Command {
	return []*cobra.Command{
		reconcileCmd,
	}
}

func init() {
	for _, cmd := range subCommandList() {
		cmd.Use = fmt.Sprintf("%s:%s", groups.CIGroup.ID, cmd.Use)
		cmd.GroupID = groups.CIGroup.ID
		cmd.Annotations = map[string]string{ContextAnnotation: ContextCIOnly}
	}
}

func RegisterSubcommands(cmd *cobra.Command) {
	for _, subCmd := range subCommandList() {
		cmd.AddCommand(subCmd)
	}
}
