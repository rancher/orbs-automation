package cmd

import (
	"fmt"

	"github.com/rancher/orbs-automation/cmd/version"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/spf13/cobra"
)

// addVersion adds the command to root.
func addVersion(root *cobra.Command) {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the gero version",
		Long: `Print the version, commit and build date of this gero binary. The text is
the same as gero --version; with --json it is an object with the keys version,
commit and date.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if settingsFrom(cmd).Output == clicfg.OutputJSON {
				b, err := version.JSON()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "gero version %s\n", version.String()); err != nil {
				return err
			}
			return printHeader(cmd)
		},
	}
	addHeaderFlag(versionCmd)
	root.AddCommand(versionCmd)
}
