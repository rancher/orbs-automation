package version

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringDefaults(t *testing.T) {
	assert.Equal(t, "dev (commit none, built unknown)", String())
}

func TestJSON(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldV, oldC, oldD })
	Version, Commit, Date = "1.2.3", "abc1234", "2025-01-02T03:04:05Z"

	b, err := JSON()

	require.NoError(t, err)
	var got map[string]string
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, map[string]string{"version": "1.2.3", "commit": "abc1234", "date": "2025-01-02T03:04:05Z"}, got)
}

func TestStringUsesInjectedValues(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldV, oldC, oldD })
	Version, Commit, Date = "1.2.3", "abc1234", "2025-01-02T03:04:05Z"

	assert.Equal(t, "1.2.3 (commit abc1234, built 2025-01-02T03:04:05Z)", String())
}
