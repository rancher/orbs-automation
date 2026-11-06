package imagesexist

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	project config.Project
	err     error
	asked   string
}

func (f *fakeWorkspace) Project(key string) (config.Project, error) {
	f.asked = key
	return f.project, f.err
}

// fakeChecker answers from results, keyed "host image:tag", and is found by
// default. It records what it was asked, and can be slowed to check ordering.
type fakeChecker struct {
	mu      sync.Mutex
	results map[string]registry.Result
	asked   []string
	delay   map[string]time.Duration
}

func (f *fakeChecker) Exists(_ context.Context, host, image, tag string) registry.Result {
	key := host + " " + image + ":" + tag
	time.Sleep(f.delay[host])
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, key)
	if r, ok := f.results[key]; ok {
		return r
	}
	return registry.Result{Status: registry.Found}
}

func sccProject() config.Project {
	return config.Project{Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}}
}

var ctx = context.Background()

func TestCheckStableTagGoesToEveryRegistry(t *testing.T) {
	ws := &fakeWorkspace{project: sccProject()}
	c := &fakeChecker{}

	got, err := check(ctx, ws, c, "", "v0.5.3")

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", got.Project)
	assert.Equal(t, "v0.5.3", got.Tag)
	assert.True(t, got.Stable)
	assert.Equal(t, []Row{
		{Image: "rancher/scc-operator", Registry: "dockerhub", Label: "Docker Hub",
			Reference: "docker.io/rancher/scc-operator:v0.5.3", Status: "found"},
		{Image: "rancher/scc-operator", Registry: "prime-staging", Label: "Prime Staging",
			Reference: "stgregistry.suse.com/rancher/scc-operator:v0.5.3", Status: "found"},
		{Image: "rancher/scc-operator", Registry: "prime-production", Label: "Prime Production",
			Reference: "registry.rancher.com/rancher/scc-operator:v0.5.3", Status: "found"},
	}, got.Rows)
	assert.False(t, got.Failed())
	assert.Empty(t, ws.asked, "no project asks for the working directory's")
}

func TestCheckPrereleaseSkipsPrimeProduction(t *testing.T) {
	c := &fakeChecker{}
	got, err := check(ctx, &fakeWorkspace{project: sccProject()}, c, "", "v0.5.3-rc.1")

	require.NoError(t, err)
	assert.False(t, got.Stable)
	require.Len(t, got.Rows, 3)
	assert.Equal(t, Row{
		Image: "rancher/scc-operator", Registry: "prime-production", Label: "Prime Production",
		Reference: "registry.rancher.com/rancher/scc-operator:v0.5.3-rc.1", Status: Skipped, Detail: "prerelease",
	}, got.Rows[2])
	assert.Len(t, c.asked, 2, "a skipped registry is not asked")
	assert.False(t, got.Failed(), "skipped is not a failure")
}

func TestCheckFailsOnNotFoundAndOnUnreachable(t *testing.T) {
	for name, res := range map[string]registry.Result{
		"not found":   {Status: registry.NotFound},
		"unreachable": {Status: registry.Unreachable, Detail: "500 Internal Server Error"},
	} {
		t.Run(name, func(t *testing.T) {
			c := &fakeChecker{results: map[string]registry.Result{"stgregistry.suse.com rancher/scc-operator:v0.5.3": res}}
			got, err := check(ctx, &fakeWorkspace{project: sccProject()}, c, "", "v0.5.3")
			require.NoError(t, err)
			assert.True(t, got.Failed())
			assert.Equal(t, string(res.Status), got.Rows[1].Status)
			assert.Equal(t, res.Detail, got.Rows[1].Detail)
		})
	}
}

func TestCheckUsesTheProjectsImages(t *testing.T) {
	p := sccProject()
	p.Images = []config.Image{
		{Name: "rancher/scc-operator", Registries: []string{config.RegistryDockerHub}},
		{Name: "rancher/scc-operator-extras"},
	}
	got, err := check(ctx, &fakeWorkspace{project: p}, &fakeChecker{}, "", "v1.0.0")
	require.NoError(t, err)

	var images []string
	for _, r := range got.Rows {
		images = append(images, r.Image+" in "+r.Registry)
	}
	assert.Equal(t, []string{
		"rancher/scc-operator in dockerhub",
		"rancher/scc-operator-extras in dockerhub",
		"rancher/scc-operator-extras in prime-staging",
		"rancher/scc-operator-extras in prime-production",
	}, images, "config order, narrowed where the image lists registries")
}

func TestCheckKeepsConfigOrderWhateverFinishesFirst(t *testing.T) {
	c := &fakeChecker{delay: map[string]time.Duration{"registry-1.docker.io": 30 * time.Millisecond}}
	got, err := check(ctx, &fakeWorkspace{project: sccProject()}, c, "", "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "dockerhub", got.Rows[0].Registry, "the slow one is still first")
	assert.Equal(t, "prime-production", got.Rows[2].Registry)
}

func TestCheckPassesTheProjectKey(t *testing.T) {
	ws := &fakeWorkspace{project: sccProject()}
	_, err := check(ctx, ws, &fakeChecker{}, "scc-operator", "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "scc-operator", ws.asked)
}

func TestCheckProjectErrors(t *testing.T) {
	for _, want := range []error{workspace.ErrNotProject, workspace.ErrUnknownProject, workspace.ErrNotFound} {
		_, err := check(ctx, &fakeWorkspace{err: want}, &fakeChecker{}, "", "v1.0.0")
		assert.ErrorIs(t, err, want)
	}
}

func TestRenderText(t *testing.T) {
	c := &fakeChecker{results: map[string]registry.Result{
		"stgregistry.suse.com rancher/scc-operator:v0.5.3-rc.1": {Status: registry.NotFound},
	}}
	r, err := check(ctx, &fakeWorkspace{project: sccProject()}, c, "", "v0.5.3-rc.1")
	require.NoError(t, err)

	out := renderText(r)

	for _, want := range []string{
		"rancher/scc-operator", "v0.5.3-rc.1", "prerelease",
		"IMAGE", "REGISTRY", "STATUS", "REFERENCE",
		"Docker Hub", "found", "docker.io/rancher/scc-operator:v0.5.3-rc.1",
		"Prime Staging", "not found",
		"Prime Production", "skipped (prerelease)",
		"1 not found",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRenderTextAllFound(t *testing.T) {
	r, err := check(ctx, &fakeWorkspace{project: sccProject()}, &fakeChecker{}, "", "v0.5.3")
	require.NoError(t, err)
	out := renderText(r)
	assert.Contains(t, out, "stable")
	assert.Contains(t, out, "All required images were found")
}

func TestRenderTextShowsWhyUnreachable(t *testing.T) {
	c := &fakeChecker{results: map[string]registry.Result{
		"registry-1.docker.io rancher/scc-operator:v1": {Status: registry.Unreachable, Detail: "429 Too Many Requests"},
	}}
	r, err := check(ctx, &fakeWorkspace{project: sccProject()}, c, "", "v1")
	require.NoError(t, err)
	out := renderText(r)
	assert.Contains(t, out, "unreachable (429 Too Many Requests)")
	assert.Contains(t, out, "1 unreachable")
}

func TestRenderJSON(t *testing.T) {
	r, err := check(ctx, &fakeWorkspace{project: sccProject()}, &fakeChecker{}, "", "v0.5.3")
	require.NoError(t, err)
	out, err := renderJSON(r)
	require.NoError(t, err)
	var back Report
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, r, back)
	assert.Contains(t, out, `"results"`)
}

func TestRunRejectsOtherOutputFormats(t *testing.T) {
	_, err := Run(ctx, clicfg.Settings{Output: clicfg.OutputEnv}, t.TempDir(), "", "v1.0.0")
	assert.ErrorContains(t, err, "not supported by repo images exist")
}

func TestFailedIsFalseForAnEmptyReport(t *testing.T) {
	assert.False(t, Report{}.Failed())
}

func riProject() config.Project {
	return config.Project{
		Name: "rancher",
		Repo: config.Repo{Name: "rancher/rancher"},
		Release: config.Release{Branching: "perLine", Lines: []config.ProjectLine{
			{Line: "v2.16"}, {Line: "v2.15"},
		}},
		Images: []config.Image{
			{Name: "rancher/rancher", Registries: []string{config.RegistryDockerHub}},
			{Name: "rancher/rancher-server", Since: "v2.16", Registries: []string{config.RegistryDockerHub}},
		},
	}
}

func rowFor(t *testing.T, r Report, image string) Row {
	t.Helper()
	for _, row := range r.Rows {
		if row.Image == image {
			return row
		}
	}
	require.Failf(t, "no row", "image %s", image)
	return Row{}
}

func TestCheckLeavesOutAnImageBeforeItsLine(t *testing.T) {
	c := &fakeChecker{}
	got, err := check(ctx, &fakeWorkspace{project: riProject()}, c, "", "v2.15.3")
	require.NoError(t, err)

	require.Len(t, got.Rows, 1, "an image the tag is too old for is not in the report at all")
	assert.Equal(t, "rancher/rancher", got.Rows[0].Image)
	assert.Equal(t, "found", got.Rows[0].Status)
	assert.Equal(t, []string{"registry-1.docker.io rancher/rancher:v2.15.3"}, c.asked, "and it is not asked for")
	assert.False(t, got.Failed())
	assert.Empty(t, got.Warnings)
}

func TestCheckChecksAnImageFromItsLineOn(t *testing.T) {
	for _, tag := range []string{"v2.16.0", "v2.16.0-rc.1"} {
		c := &fakeChecker{}
		got, err := check(ctx, &fakeWorkspace{project: riProject()}, c, "", tag)
		require.NoError(t, err)
		assert.Equal(t, "found", rowFor(t, got, "rancher/rancher-server").Status, tag)
	}
}

func TestCheckFindsTheLineThroughTheSeries(t *testing.T) {
	p := config.Project{
		Name: "kuberlr-kubectl",
		Repo: config.Repo{Name: "rancher/kuberlr-kubectl"},
		Release: config.Release{Branching: "perLine", Lines: []config.ProjectLine{
			{Line: "v2.16", Version: "9.x"}, {Line: "v2.15", Version: "8.x"},
		}},
		Images: []config.Image{{Name: "rancher/kuberlr-kubectl-new", Since: "v2.16", Registries: []string{config.RegistryDockerHub}}},
	}
	old, err := check(ctx, &fakeWorkspace{project: p}, &fakeChecker{}, "", "v8.1.2")
	require.NoError(t, err)
	assert.Empty(t, old.Rows, "8.x is v2.15, before the image's line")

	current, err := check(ctx, &fakeWorkspace{project: p}, &fakeChecker{}, "", "v9.0.1")
	require.NoError(t, err)
	require.Len(t, current.Rows, 1, "9.x is v2.16")
	assert.Equal(t, "found", current.Rows[0].Status)
}

func TestCheckATagInNoLineIsAPlainCheckWithAWarning(t *testing.T) {
	c := &fakeChecker{}
	got, err := check(ctx, &fakeWorkspace{project: riProject()}, c, "", "v1.0.0")
	require.NoError(t, err)

	assert.Equal(t, "found", rowFor(t, got, "rancher/rancher-server").Status, "no since rule is applied")
	assert.Len(t, c.asked, 2, "every image is checked")
	require.Len(t, got.Warnings, 1)
	assert.Contains(t, got.Warnings[0], "v1.0.0")
	assert.Contains(t, got.Warnings[0], "none of the release lines")
	assert.False(t, got.Failed(), "a warning is not a failure")
}

func TestCheckNoWarningWhenNoImageHasASince(t *testing.T) {
	got, err := check(ctx, &fakeWorkspace{project: sccProject()}, &fakeChecker{}, "", "v1.0.0")
	require.NoError(t, err)
	assert.Empty(t, got.Warnings, "with nothing to compare, an unknown line does not matter")

	p := riProject()
	p.Images = p.Images[:1] // no since
	got, err = check(ctx, &fakeWorkspace{project: p}, &fakeChecker{}, "", "v1.0.0")
	require.NoError(t, err)
	assert.Empty(t, got.Warnings)
}

func TestRenderTextHasNoRowForAnImageBeforeItsLine(t *testing.T) {
	skipped, err := check(ctx, &fakeWorkspace{project: riProject()}, &fakeChecker{}, "", "v2.15.3")
	require.NoError(t, err)
	out := renderText(skipped)
	assert.NotContains(t, out, "rancher-server")
	assert.NotContains(t, out, "before v2.16")
}

func TestRenderTextShowsWarnings(t *testing.T) {
	warned, err := check(ctx, &fakeWorkspace{project: riProject()}, &fakeChecker{}, "", "v1.0.0")
	require.NoError(t, err)
	out := renderText(warned)
	assert.Contains(t, out, "Warning: ")
	assert.Contains(t, out, "none of the release lines")
}

func TestRenderTextWhenNoImageIsExpected(t *testing.T) {
	p := riProject()
	p.Images = []config.Image{{Name: "rancher/rancher-server", Since: "v2.16", Registries: []string{config.RegistryDockerHub}}}
	r, err := check(ctx, &fakeWorkspace{project: p}, &fakeChecker{}, "", "v2.15.3")
	require.NoError(t, err)
	assert.Empty(t, r.Rows)
	assert.False(t, r.Failed())
	out := renderText(r)
	assert.Contains(t, out, "No image is expected for this tag")
	assert.NotContains(t, out, "All required images were found")
	assert.NotContains(t, out, "REFERENCE", "no empty table")
}

func TestRenderJSONHasTheWarnings(t *testing.T) {
	warned, err := check(ctx, &fakeWorkspace{project: riProject()}, &fakeChecker{}, "", "v1.0.0")
	require.NoError(t, err)
	out, err := renderJSON(warned)
	require.NoError(t, err)
	assert.Contains(t, out, `"warnings"`)
	var back Report
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, warned, back)
}
