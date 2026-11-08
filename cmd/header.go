package cmd

import (
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/platform/header"
	"github.com/spf13/cobra"
)

// addHeaderFlag adds --egg to a command: a flag that adds the header art to its
// text output. It is hidden, so it is in no help and no generated docs.
func addHeaderFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("egg", false, "")
	_ = cmd.Flags().MarkHidden("egg")
}

// headerTitle is the line above the art.
const headerTitle = "RR-CORE v20.0 :: Energy Absorption Matrix Online"

// printHeader writes the header art when --egg is set, for text output only: json
// and env output are read by programs and must stay as they are.
func printHeader(cmd *cobra.Command) error {
	if on, _ := cmd.Flags().GetBool("egg"); !on || settingsFrom(cmd).Output != clicfg.OutputText {
		return nil
	}
	art := headerTitle + "\n" + header.Print()
	if !strings.HasSuffix(art, "\n") {
		art += "\n"
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), art)
	return err
}
