package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringDefaults(t *testing.T) {
	assert.Equal(t, "dev (commit none, built unknown)", String())
}

func TestStringUsesInjectedValues(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldV, oldC, oldD })
	Version, Commit, Date = "1.2.3", "abc1234", "2025-01-02T03:04:05Z"

	assert.Equal(t, "1.2.3 (commit abc1234, built 2025-01-02T03:04:05Z)", String())
}
