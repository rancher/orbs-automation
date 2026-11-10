package registry

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/goproxy"
	"github.com/rancher/orbs-automation/internal/platform/ociregistry"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProber answers from results, keyed "host image:tag", and finds everything
// else. It records what it was asked.
type fakeProber struct {
	mu      sync.Mutex
	results map[string]ociregistry.Result
	asked   []string
}

func (f *fakeProber) Exists(_ context.Context, host, image, tag string) ociregistry.Result {
	key := host + " " + image + ":" + tag
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, key)
	if r, ok := f.results[key]; ok {
		return r
	}
	return ociregistry.Result{Status: ociregistry.Found}
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
	p := &fakeProber{results: map[string]ociregistry.Result{"stgregistry.suse.com rancher/scc-operator:v1": {Status: ociregistry.Unreachable, Detail: "500"}}}
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
	assert.Equal(t, []string{chain.RegistryDockerHub, chain.RegistryPrimeStaging, chain.RegistryPrimeProduction},
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

// fakeModules is a fakeProber that can also look for Go modules: it answers from
// modules, keyed "module@version", and finds everything else.
type fakeModules struct {
	fakeProber
	modules map[string]goproxy.Result
	asked   []string
}

func (f *fakeModules) ModuleExists(_ context.Context, module, version string) goproxy.Result {
	key := module + "@" + version
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, key)
	if r, ok := f.modules[key]; ok {
		return r
	}
	return goproxy.Result{Status: goproxy.Found}
}

const helmtoolsModule = "github.com/rancher/ob-charts-tool/helmtools"

// helmtools is a project whose module is released with a path prefix on its tag.
func helmtools(t *testing.T) *chain.Project {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Doc("Project", "ob-charts-tool", "", "  repo: {name: rancher/ob-charts-tool, managedBy: orbs}\n"),
		chaintest.Doc("Stream", "helmtools", "{project: ob-charts-tool}", "  tagPattern: \"helmtools/v*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "helmtools-module", "{stream: helmtools}", "  kind: gomod\n  name: "+helmtoolsModule+"\n"),
	).Project("ob-charts-tool")
}

// wrangler is a project with a stream and a module for each of two minors, both
// the same module path.
func wrangler(t *testing.T) *chain.Project {
	t.Helper()
	return chaintest.Load(t,
		chaintest.Doc("Project", "wrangler", "", "  repo: {name: rancher/wrangler}\n"),
		chaintest.Doc("Stream", "wrangler-3-8", "{project: wrangler}", "  tagPattern: \"v3.8.*\"\n  universal: true\n"),
		chaintest.Doc("Stream", "wrangler-3-7", "{project: wrangler}", "  tagPattern: \"v3.7.*\"\n  universal: true\n"),
		chaintest.Doc("Artifact", "wrangler-3-8-module", "{stream: wrangler-3-8}", "  kind: gomod\n  name: github.com/rancher/wrangler/v3\n"),
		chaintest.Doc("Artifact", "wrangler-3-7-module", "{stream: wrangler-3-7}", "  kind: gomod\n  name: github.com/rancher/wrangler/v3\n"),
	).Project("wrangler")
}

func TestCheckProjectLooksForAModuleOnTheProxy(t *testing.T) {
	p := &fakeModules{}

	got := CheckProject(context.Background(), helmtools(t), p, "helmtools/v0.1.2")

	require.Len(t, got.Rows, 1)
	assert.Equal(t, Row{
		Image: helmtoolsModule, Registry: "goproxy", Label: "Go module proxy",
		Reference: helmtoolsModule + "@v0.1.2", Status: string(goproxy.Found),
	}, got.Rows[0], "the version is the tag without the prefix of its stream")
	assert.Equal(t, []string{helmtoolsModule + "@v0.1.2"}, p.asked)
	assert.False(t, got.Failed())
}

func TestCheckProjectAModuleTheProxyDoesNotHaveFails(t *testing.T) {
	for name, res := range map[string]goproxy.Result{
		"not found":   {Status: goproxy.NotFound},
		"unreachable": {Status: goproxy.Unreachable, Detail: "the proxy answered 502 Bad Gateway"},
	} {
		t.Run(name, func(t *testing.T) {
			p := &fakeModules{modules: map[string]goproxy.Result{helmtoolsModule + "@v0.1.2": res}}

			got := CheckProject(context.Background(), helmtools(t), p, "helmtools/v0.1.2")

			require.Len(t, got.Rows, 1)
			assert.True(t, got.Failed())
			assert.Equal(t, string(res.Status), got.Rows[0].Status)
			assert.Equal(t, res.Detail, got.Rows[0].Detail)
		})
	}
}

func TestCheckProjectOnlyLooksForTheModulesOfTheStreamsTheTagIsOf(t *testing.T) {
	p := &fakeModules{}

	got := CheckProject(context.Background(), wrangler(t), p, "v3.7.2")

	require.Len(t, got.Rows, 1, "the 3.8 stream's module is not a release of this tag")
	assert.Equal(t, "github.com/rancher/wrangler/v3@v3.7.2", got.Rows[0].Reference)
	assert.Equal(t, []string{"github.com/rancher/wrangler/v3@v3.7.2"}, p.asked)
}

func TestCheckProjectATagOfNoStreamHasNoModuleToLookFor(t *testing.T) {
	p := &fakeModules{}

	got := CheckProject(context.Background(), wrangler(t), p, "v3.9.0")

	assert.Empty(t, got.Rows)
	assert.Empty(t, p.asked)
}

func TestCheckProjectLooksForTheModuleOfAPrereleaseToo(t *testing.T) {
	p := &fakeModules{}

	got := CheckProject(context.Background(), wrangler(t), p, "v3.8.0-rc.3")

	require.Len(t, got.Rows, 1)
	assert.Equal(t, string(goproxy.Found), got.Rows[0].Status, "the proxy has prereleases, so they are not skipped")
	assert.Equal(t, []string{"github.com/rancher/wrangler/v3@v3.8.0-rc.3"}, p.asked)
}

func TestCheckProjectWithNoWayToAskAboutModulesSaysSo(t *testing.T) {
	got := CheckProject(context.Background(), helmtools(t), &fakeProber{}, "helmtools/v0.1.2")

	require.Len(t, got.Rows, 1)
	assert.Equal(t, Skipped, got.Rows[0].Status)
	assert.Contains(t, got.Rows[0].Detail, "proxy")
	assert.False(t, got.Failed(), "a module that was not asked about is not one that is missing")
}

func TestCheckProjectListsTheImagesBeforeTheModules(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Doc("Project", "tool", "", "  repo: {name: acme/tool}\n"),
		chaintest.Doc("Stream", "tool", "{project: tool}", "  universal: true\n"),
		chaintest.Doc("Artifact", "a-module", "{stream: tool}", "  kind: gomod\n  name: github.com/acme/tool\n"),
		chaintest.Doc("Artifact", "z-image", "{stream: tool}", "  kind: image\n  name: acme/tool\n  registries: [dockerhub]\n"),
	)

	got := CheckProject(context.Background(), g.Project("tool"), &fakeModules{}, "v1.0.0")

	require.Len(t, got.Rows, 2)
	assert.Equal(t, "acme/tool", got.Rows[0].Image)
	assert.Equal(t, "github.com/acme/tool", got.Rows[1].Image)
}

func TestCheckProjectAModuleOfSomeLinesIsLeftOutForAnOlderLine(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		chaintest.Doc("Project", "rancher", "", "  repo: {name: rancher/rancher}\n"),
		chaintest.Doc("Stream", "rancher", "{project: rancher}", "  lines:\n    - {line: v2.16, series: \"2.16.x\"}\n    - {line: v2.15, series: \"2.15.x\"}\n"),
		chaintest.Doc("Artifact", "rancher-module", "{stream: rancher}", "  kind: gomod\n  name: github.com/rancher/rancher/pkg/apis\n  lines: {since: v2.16}\n"),
	)
	p := &fakeModules{}

	older := CheckProject(context.Background(), g.Project("rancher"), p, "v2.15.1")
	newer := CheckProject(context.Background(), g.Project("rancher"), p, "v2.16.0")

	assert.Empty(t, older.Rows)
	require.Len(t, newer.Rows, 1)
	assert.Equal(t, "github.com/rancher/rancher/pkg/apis@v2.16.0", newer.Rows[0].Reference)
}
