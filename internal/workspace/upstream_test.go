package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const upstreamUserFile = `upstream: upstream
repos:
  overrides:
    rancher/scc-operator:
      upstream: rancher
`

func TestUpstreamFor(t *testing.T) {
	w := Open(settings(t, upstreamUserFile, nil, nil), t.TempDir())

	assert.Equal(t, "rancher", w.UpstreamFor("rancher/scc-operator"), "a per-repo override wins")
	assert.Equal(t, "rancher", w.UpstreamFor("Rancher/SCC-Operator"), "slugs compare case-insensitively")
	assert.Equal(t, "upstream", w.UpstreamFor("rancher/rancher"), "otherwise the global setting")
}

func TestUpstreamForUnset(t *testing.T) {
	w := Open(settings(t, "", nil, nil), t.TempDir())
	assert.Empty(t, w.UpstreamFor("rancher/rancher"))
}

func TestPushSettingsCollectsTheUsersSettingsForARepository(t *testing.T) {
	w := Open(settings(t, `
upstream: upstream
fork: origin
githubUser: danpock
pushTo: fork
repos:
  overrides:
    rancher/rancher:
      pushTo: upstream
    rancher/scc-operator:
      upstream: rancher
`, nil, nil), t.TempDir())

	assert.Equal(t, PushSettings{UserRepo: "upstream", UserDefault: "fork", Fork: "origin", Upstream: "upstream", GitHubUser: "danpock"},
		w.PushSettings("rancher/rancher"))
	assert.Equal(t, PushSettings{UserDefault: "fork", Fork: "origin", Upstream: "rancher", GitHubUser: "danpock"},
		w.PushSettings("rancher/scc-operator"), "the per-repo upstream applies, and no per-repo pushTo is set")
	assert.Equal(t, "upstream", w.PushSettings("RANCHER/Rancher").UserRepo, "the slug is matched without regard to case")
	assert.Equal(t, PushSettings{UserDefault: "fork", Fork: "origin", Upstream: "upstream", GitHubUser: "danpock"},
		w.PushSettings("rancher/charts"), "a repository with no override has only the user's own")
}

func TestPushSettingsOfAnEmptyUserFileIsEmpty(t *testing.T) {
	w := Open(settings(t, "", nil, nil), t.TempDir())

	assert.Equal(t, PushSettings{}, w.PushSettings("rancher/rancher"))
}
