package semver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

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
func (v Version) String() string {
	out := strconv.FormatUint(v.Major, 10) + "." + strconv.FormatUint(v.Minor, 10) + "." + strconv.FormatUint(v.Patch, 10)
	if v.Prerelease != "" {
		out += "-" + v.Prerelease
	}
	return out
}

// NextStable is the stable version that follows v. After a prerelease it is the
// version being previewed, 1.2.4-rc.2 to 1.2.4, and bump is not used. After a
// stable version it raises the part bump names and zeroes those below it: 1.2.3
// to 1.2.4, 1.3.0 or 2.0.0.
func (v Version) NextStable(bump Bump) Version {
	if v.IsPrerelease() {
		return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
	}
	return v.raised(bump)
}

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
func (v Version) NextPrerelease(bump Bump, label string) (Version, error) {
	if label != "" && !validLabel(label) {
		return Version{}, fmt.Errorf("%w %q: it must be one identifier of letters, digits and hyphens, not all digits", ErrInvalidLabel, label)
	}
	if !v.IsPrerelease() {
		if label == "" {
			return Version{}, fmt.Errorf("%w: a stable version has no label to keep, so one is needed (rc)", ErrInvalidLabel)
		}
		next := v.raised(bump)
		next.Prerelease = label + ".1"
		return next, nil
	}
	next := Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
	if label != "" && label != currentLabel(v.Prerelease) {
		next.Prerelease = label + ".1"
		return next, nil
	}
	next.Prerelease = iterate(v.Prerelease)
	return next, nil
}

// raised is v with the part bump names raised and the parts below it zeroed.
func (v Version) raised(bump Bump) Version {
	switch bump {
	case Major:
		return Version{Major: v.Major + 1}
	case Minor:
		return Version{Major: v.Major, Minor: v.Minor + 1}
	}
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

// validLabel reports whether s is one prerelease identifier that is not a number.
func validLabel(s string) bool {
	if s == "" || allDigits(s) {
		return false
	}
	for _, r := range s {
		if !isIdentifierChar(r) {
			return false
		}
	}
	return true
}

// currentLabel is the name of a prerelease: its first identifier without the
// digits it may end in, so rc.2 and rc2 are both rc.
func currentLabel(prerelease string) string {
	first, _, _ := strings.Cut(prerelease, ".")
	text, _ := splitTrailingDigits(first)
	return text
}

// iterate is the next iteration of a prerelease. The last identifier counts up
// when it is a number (rc.9 to rc.10) or ends in one (rc9 to rc10), and a last
// identifier that has none gets a .1 (rc to rc.1).
func iterate(prerelease string) string {
	ids := strings.Split(prerelease, ".")
	last := ids[len(ids)-1]
	text, digits := splitTrailingDigits(last)
	if digits == "" {
		return prerelease + ".1"
	}
	ids[len(ids)-1] = text + increment(digits)
	return strings.Join(ids, ".")
}

// increment adds one to a string of digits, without parsing it, so a long run
// cannot overflow.
func increment(digits string) string {
	b := []byte(digits)
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] < '9' {
			b[i]++
			return string(b)
		}
		b[i] = '0'
	}
	return "1" + string(b)
}
