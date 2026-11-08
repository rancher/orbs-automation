// Package clicfg resolves the CLI config of one gero invocation (flags, GERO_*
// environment, the user file and defaults) into an immutable Settings value.
// It is framework-free (no cobra) and does no git inference: unset stays empty
// and a resolver on top of Settings fills it in from the working directory.
//
// The package is split by concern:
//
//   - schema.go is the shape of the user file, the one file a person writes for
//     themselves. It is a plain file of keys, with no apiVersion or kind and no
//     versions to migrate between, so it lives here and not in internal/apis,
//     which holds the versioned and kinded application config
//   - settings.go is the resolved Settings, what a command is given, and the
//     values a setting may take
//   - load.go resolves Settings from the layers, flag over environment over file
//     over default, and derives the flag and environment names of a key
//   - terminal.go is how the terminal is told to look
//   - context.go says whether the process runs in CI, and userfile.go where the
//     user file and the state live
//
// See docs/specs/CLI_CONFIG.md.
package clicfg
