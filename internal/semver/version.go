package semver

// Version is a tag read with the SemVer grammar, for ordering. It is separate
// from Classify, which is the lenient check-semver port and accepts any text.
type Version struct {
	Major, Minor, Patch uint64
	Prerelease          string // dot-separated identifiers, without the leading "-"
}

// IsPrerelease reports whether the version has a prerelease part.
func (v Version) IsPrerelease() bool { return v.Prerelease != "" }

// Parse reads tag as [v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]. Numbers have no
// leading zeros, and the identifiers are non-empty runs of [0-9A-Za-z-]. Build
// metadata is checked and dropped, since it does not take part in ordering. ok
// is false for anything else, which is not an error: repositories hold tags
// that are not versions.
func Parse(tag string) (v Version, ok bool) {
	return Version{}, false
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
func Compare(a, b Version) int {
	return 0
}
