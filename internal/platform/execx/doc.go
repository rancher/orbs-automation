// Package execx is the one place gero starts other programs. It runs an
// allowed binary with an argument vector and hands back what happened: the
// raw stdout, the raw stderr and the exit code. It does nothing else.
//
// Scope, on purpose:
//
//   - It does not print, color, trim, split or otherwise format output. A
//     caller that parses a result owns that, and a UI layer owns how it looks.
//   - It does not use a shell. A command is a name and an argument vector,
//     never a string to be parsed.
//   - It runs only the binaries it was built with (see New), found on PATH
//     once, so a name that is not on the list is refused and a later change to
//     PATH cannot swap the program.
//   - It has no sudo or privilege helper. Add one only when a command proves it
//     needs one, and then as its own reviewed decision.
//   - Every run takes a context, and cancelling it kills the process. A caller
//     that wants a time limit sets it on the context.
//
// The Runner is a concrete type. A package that runs programs defines the
// small interface it needs where it consumes it (as internal/cli/doctor does
// for gh and git), so tests use a fake and not a real binary.
//
// # Future API surface
//
// Streaming is not built, and is listed here so it grows without reshaping
// what exists. A long-running command, for example a clone, may want live
// output. The shape would be optional writers on Cmd:
//
//	type Cmd struct {
//	    // ...
//	    Stdout io.Writer // when set, stdout goes here as it is produced
//	    Stderr io.Writer // when set, stderr goes here as it is produced
//	}
//
// With a writer set, the matching Result field would still hold the bytes, so
// a caller sees the same thing either way, and a nil writer (the default)
// keeps today's capture-only behavior. The package would still never print on
// its own: where live output goes is the caller's choice. Stdin is left out
// until something needs it. Build this when a command needs it, not before.
package execx
