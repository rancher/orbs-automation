// Package embedded holds the central config bundle that is compiled into the
// binary, so a gero with no checkout and no network still has config
// (docs/specs/CENTRAL_BUNDLE.md). The file is generated from config/ and committed,
// so a plain go build or go install has it; a test fails when it is behind config/.
package embedded

import _ "embed"

// The header is the compile tool's default one, not a release's: the committed bytes
// change only when config/ does. The release of the binary is what it was built
// as, not what this header says.
//
//go:generate sh -c "cd ../../.. && go run ./tools/compilebundle --out internal/bundle/embedded"

//go:embed gero-config-bundle.yaml
var data []byte

// Bytes is the embedded bundle, in the format bundle.Parse reads.
func Bytes() []byte {
	return data
}
