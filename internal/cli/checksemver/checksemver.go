// Package checksemver holds the logic of gero semver check, a port of the
// check-semver script. It is framework-free: cmd/ owns the cobra wiring.
package checksemver

import (
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/platform/semver"
)

// ErrNoTag is returned when neither an argument nor GITHUB_REF_NAME names a tag.
var ErrNoTag = errors.New("no tag given: pass one as an argument or set GITHUB_REF_NAME")

// ResolveTag picks the tag to classify: the first argument, else
// GITHUB_REF_NAME. An empty argument falls through to the variable and extra
// arguments are ignored.
func ResolveTag(args []string, getenv func(string) string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}
	if tag := getenv("GITHUB_REF_NAME"); tag != "" {
		return tag, nil
	}
	return "", ErrNoTag
}

// Classify reports the version facts for tag. It is the one place this command
// calls the semver library, so cmd/ never imports it and later options (such as
// the SemVer grammar) are mapped to semver options here.
func Classify(tag string) semver.Info {
	return semver.Classify(tag)
}

// Render formats info as the default env output: the script's two KEY=value
// lines, ready to append to $GITHUB_OUTPUT. Other Info fields are ignored here;
// they belong to the opt-in formats.
func Render(info semver.Info) string {
	return fmt.Sprintf("HAS_PRERELEASE=%t\nHAS_BUILD_META=%t\n", info.HasPrerelease, info.HasBuildMeta)
}
