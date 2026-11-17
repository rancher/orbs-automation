package semver

import (
	"cmp"
	"strconv"
	"strings"
)

// Version is a tag read with the SemVer grammar, for ordering. It is separate
// from Classify, which is the lenient check-semver port and accepts any text.
type Version struct {
	Major, Minor, Patch uint64
	Prerelease          string // dot-separated identifiers, without the leading "-"
	Build               string // dot-separated identifiers of the build metadata, without the leading "+"
}

// IsPrerelease reports whether the version has a prerelease part.
func (v Version) IsPrerelease() bool { return v.Prerelease != "" }

// Parse reads tag as [v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]. Numbers have no
// leading zeros, and the identifiers are non-empty runs of [0-9A-Za-z-]. Build
// metadata is checked and kept in Build, and does not take part in precedence
// except where Compare says. ok is false for anything else, which is not an
// error: repositories hold tags that are not versions.
func Parse(tag string) (v Version, ok bool) {
	rest := strings.TrimPrefix(tag, "v")
	rest, build, hasBuild := strings.Cut(rest, "+")
	if hasBuild && !validIdentifiers(build, false) {
		return Version{}, false
	}
	core, pre, hasPre := strings.Cut(rest, "-")
	if hasPre && !validIdentifiers(pre, true) {
		return Version{}, false
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var n [3]uint64
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') || !allDigits(part) {
			return Version{}, false
		}
		num, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return Version{}, false
		}
		n[i] = num
	}
	return Version{Major: n[0], Minor: n[1], Patch: n[2], Prerelease: pre, Build: build}, true
}

// validIdentifiers reports whether s is dot-separated, non-empty identifiers of
// [0-9A-Za-z-]. A prerelease number has no leading zero, as SemVer requires.
func validIdentifiers(s string, prerelease bool) bool {
	for _, id := range strings.Split(s, ".") {
		if id == "" {
			return false
		}
		for _, r := range id {
			if !isIdentifierChar(r) {
				return false
			}
		}
		if prerelease && allDigits(id) && len(id) > 1 && id[0] == '0' {
			return false
		}
	}
	return true
}

func isIdentifierChar(r rune) bool {
	return r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// Compare orders a and b by SemVer precedence, returning -1, 0 or 1. A version
// with a prerelease is lower than the same version without one. Prerelease
// identifiers compare one by one: numbers numerically and lower than text, text
// in ASCII order, and a shorter list lower than a longer one that starts the
// same.
//
// One deliberate difference from the SemVer text: an identifier that ends in
// digits, such as rc2 or alpha10, compares by its text prefix and then by that
// number, so rc2 is lower than rc10. Plain ASCII order would put rc10 first,
// which is wrong for the tags this team publishes.
//
// A second difference: two versions that are the same by precedence are told apart by
// the revision in their build metadata (compareBuild), so that a respin such as
// v1.35.0+k3s3 is newer than v1.35.0+k3s1.
func Compare(a, b Version) int {
	if c := comparePrecedence(a, b); c != 0 {
		return c
	}
	return compareBuild(a.Build, b.Build)
}

// comparePrecedence is the order SemVer gives two versions: the numbers, then the prerelease.
func comparePrecedence(a, b Version) int {
	if c := cmp.Or(cmp.Compare(a.Major, b.Major), cmp.Compare(a.Minor, b.Minor), cmp.Compare(a.Patch, b.Patch)); c != 0 {
		return c
	}
	switch {
	case a.Prerelease == b.Prerelease:
		return 0
	case a.Prerelease == "":
		return 1
	case b.Prerelease == "":
		return -1
	}
	as, bs := strings.Split(a.Prerelease, "."), strings.Split(b.Prerelease, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareIdentifier(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(as), len(bs))
}

// compareBuild breaks a tie of two versions that are the same by precedence, by the
// revision their build metadata carries: +k3s3 is newer than +k3s1, and +rke2r2 than
// +rke2r1, which is how a respin of the same release is told from the first. It
// compares identifier by identifier, and only identifiers of the same text that end in
// digits (k3s1 and k3s3, not one and two); anything else is equal, as SemVer has it.
func compareBuild(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		aText, aDigits := splitTrailingDigits(as[i])
		bText, bDigits := splitTrailingDigits(bs[i])
		if aText != bText || aDigits == "" || bDigits == "" {
			return 0
		}
		return compareDigits(aDigits, bDigits)
	}
	return 0
}

func compareIdentifier(a, b string) int {
	aNum, bNum := allDigits(a), allDigits(b)
	switch {
	case aNum && bNum:
		return compareDigits(a, b)
	case aNum:
		return -1
	case bNum:
		return 1
	}
	aText, aDigits := splitTrailingDigits(a)
	bText, bDigits := splitTrailingDigits(b)
	if c := strings.Compare(aText, bText); c != 0 || aDigits == "" || bDigits == "" {
		return cmp.Or(c, cmp.Compare(len(aDigits), len(bDigits)))
	}
	return compareDigits(aDigits, bDigits)
}

// splitTrailingDigits splits "rc10" into "rc" and "10".
func splitTrailingDigits(s string) (text, digits string) {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	return s[:i], s[i:]
}

// compareDigits compares two digit strings by value without parsing them, so
// a long run cannot overflow. Leading zeros are ignored.
func compareDigits(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	return cmp.Or(cmp.Compare(len(a), len(b)), strings.Compare(a, b))
}
