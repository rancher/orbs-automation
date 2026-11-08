package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHonorsLevel(t *testing.T) {
	tests := []struct {
		level     string
		wantDebug bool
		wantInfo  bool
		wantWarn  bool
		wantError bool
	}{
		{clicfg.LogDebug, true, true, true, true},
		{clicfg.LogInfo, false, true, true, true},
		{clicfg.LogWarn, false, false, true, true},
		{clicfg.LogError, false, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, tt.level, false)

			assert.Equal(t, tt.wantDebug, l.Enabled(nil, slog.LevelDebug))
			assert.Equal(t, tt.wantInfo, l.Enabled(nil, slog.LevelInfo))
			assert.Equal(t, tt.wantWarn, l.Enabled(nil, slog.LevelWarn))
			assert.Equal(t, tt.wantError, l.Enabled(nil, slog.LevelError))
		})
	}
}

func TestNewTextHandler(t *testing.T) {
	var buf bytes.Buffer

	New(&buf, clicfg.LogInfo, false).Info("hello", "k", "v")

	assert.Contains(t, buf.String(), "level=INFO")
	assert.Contains(t, buf.String(), "msg=hello")
	assert.Contains(t, buf.String(), "k=v")
}

func TestNewJSONHandler(t *testing.T) {
	var buf bytes.Buffer

	New(&buf, clicfg.LogInfo, true).Info("hello", "k", "v")

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, "INFO", got["level"])
	assert.Equal(t, "hello", got["msg"])
	assert.Equal(t, "v", got["k"])
}

func TestNewUnknownLevelFallsBackToWarn(t *testing.T) {
	l := New(&bytes.Buffer{}, "loud", false)

	assert.False(t, l.Enabled(nil, slog.LevelInfo))
	assert.True(t, l.Enabled(nil, slog.LevelWarn))
}
