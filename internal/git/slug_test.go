package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https", "https://github.com/rancher/rancher", "rancher/rancher"},
		{"https with .git", "https://github.com/rancher/rancher.git", "rancher/rancher"},
		{"https with trailing slash", "https://github.com/rancher/rancher/", "rancher/rancher"},
		{"https with credentials", "https://user:token@github.com/rancher/scc-operator.git", "rancher/scc-operator"},
		{"http", "http://github.com/rancher/rancher.git", "rancher/rancher"},
		{"https with CI token user", "https://x-access-token:abc@github.com/rancher/rancher", "rancher/rancher"},
		{"case is preserved", "HTTPS://GitHub.com/Rancher/Rancher.git", "Rancher/Rancher"},
		{"scp-like ssh", "git@github.com:rancher/rancher.git", "rancher/rancher"},
		{"scp-like ssh without .git", "git@github.com:danpock/rancher", "danpock/rancher"},
		{"ssh scheme", "ssh://git@github.com/rancher/rancher.git", "rancher/rancher"},
		{"ssh scheme with port", "ssh://git@github.com:22/rancher/rancher.git", "rancher/rancher"},
		{"git scheme", "git://github.com/rancher/rancher.git", "rancher/rancher"},
		{"other host", "https://gitlab.example.com/rancher/rancher.git", "rancher/rancher"},
		{"nested group keeps the last two segments", "https://gitlab.example.com/group/sub/proj.git", "sub/proj"},
		{"dot in name", "git@github.com:rancher/rancher.io.git", "rancher/rancher.io"},
		{"surrounding space", "  git@github.com:rancher/rancher.git\n", "rancher/rancher"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SlugFromURL(tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSlugFromURLRejects(t *testing.T) {
	for _, url := range []string{
		"",
		"rancher",
		"https://github.com/rancher",
		"https://github.com/",
		"git@github.com:rancher",
		"/home/dan/src/rancher",
		"file:///home/dan/src/rancher",
	} {
		t.Run(url, func(t *testing.T) {
			_, err := SlugFromURL(url)
			assert.ErrorIs(t, err, ErrNotRemoteURL)
		})
	}
}
