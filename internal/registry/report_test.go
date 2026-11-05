package registry

import (
	"context"
	"sync"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProber answers from results, keyed "host image:tag", and finds everything
// else. It records what it was asked.
type fakeProber struct {
	mu      sync.Mutex
	results map[string]Result
	asked   []string
}

func (f *fakeProber) Exists(_ context.Context, host, image, tag string) Result {
	key := host + " " + image + ":" + tag
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, key)
	if r, ok := f.results[key]; ok {
		return r
	}
	return Result{Status: Found}
}

func scc() config.Project {
	return config.Project{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}}
}

func TestCheckProjectStableTag(t *testing.T) {
	got := CheckProject(context.Background(), scc(), &fakeProber{}, "v0.5.3")

	assert.Equal(t, "rancher/scc-operator", got.Project)
	assert.True(t, got.Stable)
	require.Len(t, got.Rows, 3)
	assert.Equal(t, "docker.io/rancher/scc-operator:v0.5.3", got.Rows[0].Reference)
	assert.False(t, got.Failed())
}

func TestCheckProjectPrereleaseSkipsPrimeProduction(t *testing.T) {
	p := &fakeProber{}
	got := CheckProject(context.Background(), scc(), p, "v0.5.3-rc.1")

	assert.Equal(t, Skipped, got.Rows[2].Status)
	assert.Equal(t, "prerelease", got.Rows[2].Detail)
	assert.Len(t, p.asked, 2)
}

func TestCheckProjectFails(t *testing.T) {
	p := &fakeProber{results: map[string]Result{"stgregistry.suse.com rancher/scc-operator:v1": {Status: Unreachable, Detail: "500"}}}
	got := CheckProject(context.Background(), scc(), p, "v1")
	assert.True(t, got.Failed())
	assert.Equal(t, "500", got.Rows[1].Detail)
}

func TestCheckProjectLeavesOutAnImageBeforeItsLine(t *testing.T) {
	p := config.Project{
		Repo:    config.Repo{Name: "rancher/rancher"},
		Release: config.Release{Lines: []config.ProjectLine{{Line: "v2.16"}, {Line: "v2.15"}}},
		Images: []config.Image{
			{Name: "rancher/rancher", Registries: []string{config.RegistryDockerHub}},
			{Name: "rancher/rancher-server", Since: "v2.16", Registries: []string{config.RegistryDockerHub}},
		},
	}
	f := &fakeProber{}
	got := CheckProject(context.Background(), p, f, "v2.15.3")
	require.Len(t, got.Rows, 1)
	assert.Equal(t, "rancher/rancher", got.Rows[0].Image)
	assert.Len(t, f.asked, 1)
	assert.Empty(t, got.Warnings)
}

func TestCheckProjectATagInNoLineWarns(t *testing.T) {
	p := config.Project{
		Repo:    config.Repo{Name: "rancher/rancher"},
		Release: config.Release{Lines: []config.ProjectLine{{Line: "v2.16"}}},
		Images:  []config.Image{{Name: "rancher/rancher-server", Since: "v2.16", Registries: []string{config.RegistryDockerHub}}},
	}
	got := CheckProject(context.Background(), p, &fakeProber{}, "v1.0.0")
	require.Len(t, got.Rows, 1, "a plain check of every image")
	require.Len(t, got.Warnings, 1)
	assert.Contains(t, got.Warnings[0], "none of the release lines")
}

func TestCheckProjectKeepsConfigOrder(t *testing.T) {
	got := CheckProject(context.Background(), scc(), &fakeProber{}, "v1.0.0")
	assert.Equal(t, []string{config.RegistryDockerHub, config.RegistryPrimeStaging, config.RegistryPrimeProduction},
		[]string{got.Rows[0].Registry, got.Rows[1].Registry, got.Rows[2].Registry})
}

func TestReportFailedIsFalseWhenEmpty(t *testing.T) {
	assert.False(t, Report{}.Failed())
}
