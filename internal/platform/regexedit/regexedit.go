// Package regexedit reads and replaces one value of any text file by a pattern
// with a capture group, leaving every other byte of the file as it was. It is the
// escape hatch for a file that is not in a format gero reads, or is not valid in
// it, such as a chart's values with a placeholder the build fills in.
package regexedit

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

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

// Get returns the text the capture group of pattern matches in src. The pattern is
// read with (?m), so ^ and $ are the start and the end of a line, and it must have
// exactly one capture group and match in exactly one place: a pattern names a
// value, and one that could name two is an error to be anchored.
func Get(src []byte, pattern string) (string, error) {
	loc, err := match(src, pattern)
	if err != nil {
		return "", err
	}
	return string(src[loc[0]:loc[1]]), nil
}

// Set replaces the text the capture group of pattern matches in src with value,
// and returns the new file and the text it replaced. Only those bytes change. The
// value must be one line. The pattern is read as Get reads it.
func Set(src []byte, pattern, value string) (out []byte, old string, err error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, "", fmt.Errorf("%w: the new value must be one line", ErrInvalidValue)
	}
	loc, err := match(src, pattern)
	if err != nil {
		return nil, "", err
	}
	old = string(src[loc[0]:loc[1]])
	out = make([]byte, 0, len(src)-(loc[1]-loc[0])+len(value))
	out = append(out, src[:loc[0]]...)
	out = append(out, value...)
	out = append(out, src[loc[1]:]...)
	return out, old, nil
}

// match finds where the capture group of the pattern is in src: its start and its
// end.
func match(src []byte, pattern string) ([2]int, error) {
	re, err := regexp.Compile("(?m)" + pattern)
	if err != nil {
		return [2]int{}, fmt.Errorf("%w `%s`: %v", ErrInvalidPattern, pattern, err)
	}
	if n := re.NumSubexp(); n != 1 {
		return [2]int{}, fmt.Errorf("%w `%s`: it has %d capture groups, and it needs exactly one", ErrInvalidPattern, pattern, n)
	}
	found := re.FindAllSubmatchIndex(src, 2)
	switch {
	case len(found) == 0:
		return [2]int{}, fmt.Errorf("%w: the pattern `%s` matches nowhere in the file", ErrNoMatch, pattern)
	case len(found) > 1:
		return [2]int{}, fmt.Errorf("%w: the pattern `%s` matches in more than one place; anchor it with ^ and $, or add what comes before or after the value", ErrAmbiguous, pattern)
	}
	if found[0][2] < 0 {
		return [2]int{}, fmt.Errorf("%w: the capture group of the pattern `%s` takes no part in its match", ErrNoMatch, pattern)
	}
	return [2]int{found[0][2], found[0][3]}, nil
}
