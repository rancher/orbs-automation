package groups

import "github.com/spf13/cobra"

var CIGroup cobra.Group = cobra.Group{
	ID:    "ci",
	Title: "CI-Only Commands:",
}
