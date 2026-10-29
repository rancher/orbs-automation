// Package errs holds the error values shared across gero.
package errs

import (
	"errors"
	"fmt"
)

// ErrNotImplemented marks functionality that is designed but not built yet.
var ErrNotImplemented = errors.New("not implemented")

// NotImplemented returns an error wrapping ErrNotImplemented that names the
// unfinished feature and, when intent is not empty, describes what the user
// can expect once it is built, for example:
//
//	gero config validate: not implemented
//
//	Intended behavior: Checks every config file and lists each problem found.
func NotImplemented(feature, intent string) error {
	if intent == "" {
		return fmt.Errorf("%s: %w", feature, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w\n\nIntended behavior: %s", feature, ErrNotImplemented, intent)
}
