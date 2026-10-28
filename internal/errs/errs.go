// Package errs holds the error values shared across gero.
package errs

import (
	"errors"
	"fmt"
)

// ErrNotImplemented marks functionality that is designed but not built yet.
var ErrNotImplemented = errors.New("not implemented")

// NotImplemented returns an error wrapping ErrNotImplemented that names the
// unfinished feature, for example "gero config validate: not implemented".
func NotImplemented(feature string) error {
	return fmt.Errorf("%s: %w", feature, ErrNotImplemented)
}
