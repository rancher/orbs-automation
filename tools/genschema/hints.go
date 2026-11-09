package main

import (
	"fmt"

	"github.com/invopop/jsonschema"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
)

// hint adds to one field what its tag and type cannot say. def is the Go name of
// the struct and key the YAML key of the field; for a list, the hint is of an item.
type hint struct {
	def, key string
	enum     []string
	pattern  string
}

// The allowed values of the fields that have a fixed set, built from the
// constants the rules check against, so the schema and the rules cannot differ.
var (
	pushTo = []string{v1alpha2.PushToFork, v1alpha2.PushToUpstream}

	appHints = []hint{
		{def: "ArtifactSpec", key: "kind", enum: []string{v1alpha2.ArtifactImage, v1alpha2.ArtifactFile, v1alpha2.ArtifactGoMod}},
		{def: "ArtifactSpec", key: "registries", enum: []string{v1alpha2.RegistryDockerHub, v1alpha2.RegistryPrimeStaging, v1alpha2.RegistryPrimeProduction}},
		{def: "At", key: "format", enum: []string{v1alpha2.FormatYAML, v1alpha2.FormatDockerfile, v1alpha2.FormatGoMod, v1alpha2.FormatRegex}},
		{def: "Holds", key: "kind", enum: []string{v1alpha2.HoldsRelease, v1alpha2.HoldsCommit, v1alpha2.HoldsBranch}},
		{def: "Holds", key: "accept", enum: []string{v1alpha2.AcceptStable, v1alpha2.AcceptAll}},
		{def: "Step", key: "from", enum: []string{v1alpha2.FromConsumer, v1alpha2.FromProducer, v1alpha2.FromCentral}},
		{def: "PullRequests", key: "pushTo", enum: pushTo},
		{def: "Branches", key: "layout", enum: []string{v1alpha2.LayoutSingle, v1alpha2.LayoutPerLine}},
		{def: "StreamSpec", key: "scheme", enum: []string{v1alpha2.SchemeSemver, v1alpha2.SchemeCalverLineStage, v1alpha2.SchemeChartVersion}},
		{def: "Line", key: "phase", enum: []string{lines.PhaseDevelopment, lines.PhaseGA, lines.PhaseEOM, lines.PhaseEOL}},
	}

	userHints = []hint{
		{def: "fileConfig", key: "output", enum: []string{clicfg.OutputText, clicfg.OutputJSON, clicfg.OutputEnv}},
		{def: "fileConfig", key: "color", enum: []string{clicfg.ColorAuto, clicfg.ColorAlways, clicfg.ColorNever}},
		{def: "fileConfig", key: "pushTo", enum: pushTo},
		{def: "fileRepoOverride", key: "pushTo", enum: pushTo},
	}
)

// applyHint puts a hint on the field it names, and says so when there is none, so
// a renamed field cannot leave a hint that does nothing.
func applyHint(defs jsonschema.Definitions, h hint) error {
	def := defs[h.def]
	if def == nil || def.Properties == nil {
		return fmt.Errorf("hint on %s.%s: there is no struct %s", h.def, h.key, h.def)
	}
	field, ok := def.Properties.Get(h.key)
	if !ok {
		return fmt.Errorf("hint on %s.%s: %s has no field with that key", h.def, h.key, h.def)
	}
	if field.Type == "array" && field.Items != nil {
		field = field.Items
	}
	for _, v := range h.enum {
		field.Enum = append(field.Enum, v)
	}
	field.Pattern = h.pattern
	return nil
}

func applyHints(defs jsonschema.Definitions, hints []hint) error {
	for _, h := range hints {
		if err := applyHint(defs, h); err != nil {
			return err
		}
	}
	return nil
}
