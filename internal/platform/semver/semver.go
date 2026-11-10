// Package semver classifies release tags the way the check-semver script
// does. It performs no I/O and never reads the environment.
package semver

import (
	"regexp"
	"strings"
)

// prereleaseRe is the script's exact prerelease check. It is deliberately not
// the SemVer grammar: "1.2.3-" and "01.2.3-rc" match.
var prereleaseRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+-`)

// Info is the result of classifying a tag.
type Info struct {
	Input         string
	Version       string // Input with one leading "v" stripped
	HasPrerelease bool
	HasBuildMeta  bool

	// Always zero: the SemVer grammar is read by Parse, not by Classify
	// (docs/specs/CHECK_SEMVER.md).
	IsSemver            bool
	Major, Minor, Patch uint64
	Prerelease          string
	BuildMeta           string
}

// Classify reports whether tag carries a prerelease or build metadata. Any
// text is accepted: non-SemVer input is a result, not a failure.
func Classify(tag string) Info {
	version := strings.TrimPrefix(tag, "v")
	return Info{
		Input:         tag,
		Version:       version,
		HasPrerelease: prereleaseRe.MatchString(version),
		HasBuildMeta:  strings.Contains(version, "+"),
	}
}
