// Package dockerfileedit reads and replaces the value of one ARG or ENV in a
// Dockerfile, leaving every other byte of the file as it was.
package dockerfileedit

import "errors"

var (
	// ErrNotFound is returned when no ARG or ENV of the name has a value in the file.
	ErrNotFound = errors.New("no ARG or ENV with a value")
	// ErrAmbiguous is returned when the name is given a value in more than one place,
	// so it does not name one value.
	ErrAmbiguous = errors.New("given a value more than once")
	// ErrInvalidValue is returned for a new value that cannot be written on one
	// instruction line as it is.
	ErrInvalidValue = errors.New("invalid value")
)

// Get returns the value that an ARG or ENV of name is given in src.
func Get(src []byte, name string) (string, error) {
	return "", nil
}

// Set replaces the value that an ARG or ENV of name is given in src with value,
// and returns the new file and the value it replaced.
func Set(src []byte, name, value string) (out []byte, old string, err error) {
	return src, "", nil
}
