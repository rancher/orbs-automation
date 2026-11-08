// Package config is the engine that loads one configuration object: it decodes
// YAML into a target strictly, fills the defaults the target has, and asks the
// target to validate itself, collecting every finding, errors and warnings, in a
// Report. It knows nothing of kinds, files or references between documents: a
// caller that reads many documents (internal/apis/meta) calls it once for each
// and checks the rules across them itself.
//
// The order is the one internal/workspace runs today for the application config:
// decode, then defaults, then validate. Validation reads the defaulted values.
// A default fills only a field that has a conservative value of its own; a value
// that depends on something else stays as written and is resolved by a getter on
// the type (docs/specs/WORKSPACE.md, "Raw and effective values").
package config
