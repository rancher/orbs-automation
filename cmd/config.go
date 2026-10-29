package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/rancher/orbs-automation/internal/cli/configdir"
	"github.com/rancher/orbs-automation/internal/cli/configshow"
	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/errs"
	"github.com/spf13/cobra"
)

// resolveConfigDir finds the central config directory from s and the working
// directory. Nothing found is not an error here: config show reports it unset.
func resolveConfigDir(s clicfg.Settings) (configdir.Result, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return configdir.Result{}, err
	}
	res, err := configdir.Resolve(s, cwd, os.DirFS(cwd))
	if errors.Is(err, configdir.ErrNotFound) {
		return configdir.Result{}, nil
	}
	return res, err
}

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Validate and inspect the application config",
	}
	configCmd.AddCommand(
		&cobra.Command{
			Use:   "validate",
			Short: "Validate the application config",
			RunE: func(*cobra.Command, []string) error {
				return errs.NotImplemented("gero config validate")
			},
		},
		&cobra.Command{
			Use:   "show",
			Short: "Show the resolved CLI config and where each value came from",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				s := settingsFrom(cmd)
				dir, err := resolveConfigDir(s)
				if err != nil {
					return err
				}
				out, err := configshow.Render(s, dir, s.Output)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), out)
				return err
			},
		},
	)
	rootCmd.AddCommand(configCmd)
}
