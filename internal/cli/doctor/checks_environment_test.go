package doctor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCheck(t *testing.T) {
	c := NewVersionCheck(Build{Version: "0.4.0", Commit: "abc1234", Date: "2026-10-29", OS: "linux", Arch: "arm64"})

	assert.Equal(t, "version", c.Name())
	assert.Equal(t, "environment", c.Category())
	assert.False(t, c.Online())
	assert.Empty(t, c.Needs())
	assert.NotEmpty(t, c.Description())
	assert.Equal(t, Result{Status: StatusOK, Message: "gero 0.4.0 on linux/arm64"}, c.Run(context.Background()))
}

func TestVersionCheckOfADevBuild(t *testing.T) {
	c := NewVersionCheck(Build{Version: "dev", Commit: "none", Date: "unknown", OS: "darwin", Arch: "amd64"})

	assert.Equal(t, Result{Status: StatusOK, Message: "gero dev on darwin/amd64"}, c.Run(context.Background()))
}
