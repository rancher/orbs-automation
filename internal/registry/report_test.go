package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
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

// sccOperator is a project with one universal stream and one image.
func sccOperator(t *testing.T) *chain.Project {
	t.Helper()
	g := chaintest.Load(t,
		chaintest.Doc("Project", "scc-operator", "", "  repo: {name: rancher/scc-operator}\n"),
		chaintest.Doc("Stream", "scc-operator", "{project: scc-operator}", "  universal: true\n"),
		chaintest.Doc("Artifact", "scc-operator", "{stream: scc-operator}", "  kind: image\n  name: rancher/scc-operator\n"),
	)
	return g.Project("scc-operator")
}

// rancher is a project with the lines v2.16 and v2.15, and the images given.
func rancher(t *testing.T, images ...string) *chain.Project {
	t.Helper()
	docs := []string{
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "rancher", "", "  repo: {name: rancher/rancher}\n"),
		chaintest.Doc("Stream", "rancher", "{project: rancher}", "  lines:\n    - {line: v2.16, series: \"2.16.x\"}\n    - {line: v2.15, series: \"2.15.x\"}\n"),
	}
	for i, spec := range images {
		docs = append(docs, chaintest.Doc("Artifact", fmt.Sprintf("image-%d", i), "{stream: rancher}", spec))
	}
	return chaintest.Load(t, docs...).Project("rancher")
}

const (
	rancherImage = "  kind: image\n  name: rancher/rancher\n  registries: [dockerhub]\n"
	serverImage  = "  kind: image\n  name: rancher/rancher-server\n  registries: [dockerhub]\n  lines: {since: v2.16}\n"
)

func TestCheckProjectStableTag(t *testing.T) {
	got := CheckProject(context.Background(), sccOperator(t), &fakeProber{}, "v0.5.3")

	assert.Equal(t, "rancher/scc-operator", got.Project)
	assert.True(t, got.Stable)
	require.Len(t, got.Rows, 3)
	assert.Equal(t, "docker.io/rancher/scc-operator:v0.5.3", got.Rows[0].Reference)
	assert.False(t, got.Failed())
}

func TestCheckProjectPrereleaseSkipsPrimeProduction(t *testing.T) {
	p := &fakeProber{}
	got := CheckProject(context.Background(), sccOperator(t), p, "v0.5.3-rc.1")

	assert.Equal(t, Skipped, got.Rows[2].Status)
	assert.Equal(t, "prerelease", got.Rows[2].Detail)
	assert.Len(t, p.asked, 2)
}

func TestCheckProjectFails(t *testing.T) {
	p := &fakeProber{results: map[string]Result{"stgregistry.suse.com rancher/scc-operator:v1": {Status: Unreachable, Detail: "500"}}}
	got := CheckProject(context.Background(), sccOperator(t), p, "v1")
	assert.True(t, got.Failed())
	assert.Equal(t, "500", got.Rows[1].Detail)
}

func TestCheckProjectLeavesOutAnImageBeforeItsLine(t *testing.T) {
	f := &fakeProber{}
	got := CheckProject(context.Background(), rancher(t, rancherImage, serverImage), f, "v2.15.3")
	require.Len(t, got.Rows, 1)
	assert.Equal(t, "rancher/rancher", got.Rows[0].Image)
	assert.Len(t, f.asked, 1)
	assert.Empty(t, got.Warnings)
}

func TestCheckProjectATagInNoLineWarns(t *testing.T) {
	got := CheckProject(context.Background(), rancher(t, serverImage), &fakeProber{}, "v1.0.0")
	require.Len(t, got.Rows, 1, "a plain check of every image")
	require.Len(t, got.Warnings, 1)
	assert.Contains(t, got.Warnings[0], "none of the release lines")
}

func TestCheckProjectKeepsConfigOrder(t *testing.T) {
	got := CheckProject(context.Background(), sccOperator(t), &fakeProber{}, "v1.0.0")
	assert.Equal(t, []string{v1alpha2.RegistryDockerHub, v1alpha2.RegistryPrimeStaging, v1alpha2.RegistryPrimeProduction},
		[]string{got.Rows[0].Registry, got.Rows[1].Registry, got.Rows[2].Registry})
}

func TestCheckProjectWithNoImageHasNoRows(t *testing.T) {
	got := CheckProject(context.Background(), rancher(t), &fakeProber{}, "v2.16.0")

	assert.Empty(t, got.Rows)
	assert.False(t, got.Failed())
}

func TestReportFailedIsFalseWhenEmpty(t *testing.T) {
	assert.False(t, Report{}.Failed())
}
