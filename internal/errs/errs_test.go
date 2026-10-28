package errs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotImplemented(t *testing.T) {
	err := NotImplemented("gero config validate")

	assert.ErrorIs(t, err, ErrNotImplemented)
	assert.EqualError(t, err, "gero config validate: not implemented")
}
