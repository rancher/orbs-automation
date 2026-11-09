// Package regexedit reads and replaces one value of any text file by a pattern
// with a capture group, leaving every other byte of the file as it was. It is the
// escape hatch for a file that is not in a format gero reads, or is not valid in
// it, such as a chart's values with a placeholder the build fills in.
package regexedit

import "errors"

var (
	// ErrInvalidPattern is returned for a pattern that does not compile or does not
	// have exactly one capture group.
	ErrInvalidPattern = errors.New("invalid pattern")
	// ErrNoMatch is returned when the pattern matches nowhere in the file, or its
	// group does not take part in the match.
	ErrNoMatch = errors.New("no match")
	// ErrAmbiguous is returned when the pattern matches in more than one place, so
	// it does not name one value.
	ErrAmbiguous = errors.New("matches more than once")
	// ErrInvalidValue is returned for a new value of more than one line.
	ErrInvalidValue = errors.New("invalid value")
)

// Get returns the text the capture group of pattern matches in src.
func Get(_ []byte, _ string) (string, error) {
	return "", nil
}

// Set replaces the text the capture group of pattern matches in src with value,
// and returns the new file and the text it replaced. Only those bytes change.
func Set(src []byte, _, _ string) (out []byte, old string, err error) {
	return src, "", nil
}
