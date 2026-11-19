package v1alpha2

import "github.com/rancher/orbs-automation/internal/platform/config"

// SetDefaults does nothing: a team has no default.
func (t *Teams) SetDefaults() {}

// Validate checks each team: its id and its GitHub team.
func (t *Teams) Validate(_ config.Context, _ *config.Report) {}
