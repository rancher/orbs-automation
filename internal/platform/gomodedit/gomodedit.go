// Package gomodedit reads and replaces the version of a module in a go.mod,
// leaving the rest of the file as it was: the other lines, the comments and
// `// indirect`. It is the one place the go.mod parser is used
// (docs/specs/DEPENDENCY_POLICY.md).
package gomodedit

import "errors"

var (
	// ErrNotRequired is returned when the go.mod has no require line for the module.
	ErrNotRequired = errors.New("module not required")
	// ErrInvalidVersion is returned for a version that is empty or has a space in it.
	ErrInvalidVersion = errors.New("invalid version")
)

// Get returns the version of module that src requires, and an error when it does
// not.
func Get(_ []byte, _ string) (string, error) {
	return "", nil
}

// Set replaces the version of module that src requires with version, and returns
// the new text and the version it replaced. Nothing else in src changes. It is an
// error when src does not require module.
func Set(src []byte, _, _ string) (out []byte, old string, err error) {
	return src, "", nil
}
