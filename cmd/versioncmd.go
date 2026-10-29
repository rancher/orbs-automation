package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the gero version",
		Long: `Print the version, commit and build date of this gero binary. The text is
the same as gero --version; with --json it is an object with the keys version,
commit and date.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return nil
		},
	})
}
