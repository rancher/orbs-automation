package semver

import "errors"

// Bump is the part of a version that a new release raises.
type Bump int

// The parts, from the smallest. Patch is the zero value, the usual next release.
const (
	Patch Bump = iota
	Minor
	Major
)

// ErrInvalidLabel is returned for a prerelease label that is not one identifier:
// empty, all digits, or with a character other than [0-9A-Za-z-].
var ErrInvalidLabel = errors.New("invalid prerelease label")

// String is the version without a "v": 1.2.4-rc.1.
//
// STUB: empty until a later commit.
func (v Version) String() string { return "" }

// NextStable is the stable version that follows v. After a prerelease it is the
// version being previewed, 1.2.4-rc.2 to 1.2.4, and bump is not used. After a
// stable version it raises the part bump names and zeroes those below it: 1.2.3
// to 1.2.4, 1.3.0 or 2.0.0.
//
// STUB: v itself until a later commit.
func (v Version) NextStable(bump Bump) Version { return v }

// NextPrerelease is the prerelease version that follows v, with label as its
// name (rc). After a stable version it is the first prerelease of the next
// version, which bump raises as for NextStable: 1.2.3 to 1.2.4-rc.1, 1.3.0-rc.1
// or 2.0.0-rc.1. After a prerelease it is the next iteration of the same version
// and bump is not used: 1.2.4-rc.1 to 1.2.4-rc.2, rc.9 to rc.10, rc2 to rc3 (the
// style of the last identifier is kept), rc to rc.1. A label other than the
// current one starts over on the same version: 1.2.4-beta.2 with label rc is
// 1.2.4-rc.1. An empty label keeps the current one, and after a stable version
// there is none to keep, so it is an error (ErrInvalidLabel), as is any label that
// is not one identifier.
//
// STUB: v itself until a later commit.
func (v Version) NextPrerelease(bump Bump, label string) (Version, error) { return v, nil }
