// Package version holds the build information injected at release time. The
// variables are set with -ldflags -X (see .goreleaser.yaml); a plain go build
// leaves the placeholders. It lives under cmd/ on purpose: only the wiring layer
// reads these globals, and anything deeper is handed the value as an argument.
package version

import (
	"encoding/json"
	"fmt"
)

// Set by GoReleaser through -ldflags -X.
var (
	// Version is the release version without the leading "v", or the
	// snapshot name.
	Version = "dev"
	// Commit is the commit SHA the binary was built from.
	Commit = "none"
	// Date is the build date, RFC3339.
	Date = "unknown"
)

// String describes the build, for example "1.2.3 (commit abc1234, built 2025-01-02T03:04:05Z)".
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}

// JSON describes the build as a JSON object with the keys version, commit and date.
func JSON() ([]byte, error) {
	return json.Marshal(struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Date    string `json:"date"`
	}{Version, Commit, Date})
}
