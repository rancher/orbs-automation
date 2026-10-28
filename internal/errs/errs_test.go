package errs

import (
	"errors"
	"testing"
)

func TestNotImplemented(t *testing.T) {
	err := NotImplemented("gero config validate")

	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("errors.Is(%v, ErrNotImplemented) = false, want true", err)
	}
	if got, want := err.Error(), "gero config validate: not implemented"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
