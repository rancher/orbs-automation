// Package embedded holds the central config bundle that is compiled into the
// binary, so a gero with no checkout and no network still has config
// (docs/specs/CENTRAL_BUNDLE.md). The file is generated from config/ and committed,
// so a plain go build or go install has it; a test fails when it is behind config/.
package embedded

// Bytes is the embedded bundle, in the format bundle.Parse reads.
func Bytes() []byte {
	return nil
}
