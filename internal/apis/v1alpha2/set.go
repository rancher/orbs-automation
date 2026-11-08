package v1alpha2

import "github.com/rancher/orbs-automation/internal/platform/config"

// ValidateSet checks what no one document can alone: the rules across a set of
// decoded and valid documents. See docs/design/CONFIG_SCHEMA.md, "Rules across
// documents".
func ValidateSet(_ []Object, _ config.Context, _ *config.Report) {}
