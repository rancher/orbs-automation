package clicfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaultToken(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"no tokens", nil, false},
		{"only GH_TOKEN", map[string]string{"GH_TOKEN": "a"}, false},
		{"only GITHUB_TOKEN", map[string]string{"GITHUB_TOKEN": "a"}, true},
		{"GH_TOKEN equals GITHUB_TOKEN", map[string]string{"GH_TOKEN": "a", "GITHUB_TOKEN": "a"}, true},
		{"GH_TOKEN differs", map[string]string{"GH_TOKEN": "b", "GITHUB_TOKEN": "a"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Load(Inputs{Getenv: env(tt.env)})

			assert.NoError(t, err)
			assert.Equal(t, tt.want, s.DefaultToken)
		})
	}
}
