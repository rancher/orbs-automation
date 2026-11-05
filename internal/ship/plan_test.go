package ship

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/rancher/orbs-automation/internal/yamledit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	producer     config.Project
	consumers    []config.Consumer
	consumersErr error
	refs         map[string]git.Refs // by repository slug
	refsErr      map[string]error
	files        map[string]string // "slug@ref:path"
	asked        []string
}

func (f *fakeWorkspace) Consumers(key string) (config.Project, []config.Consumer, error) {
	f.asked = append(f.asked, "consumers "+key)
	return f.producer, f.consumers, f.consumersErr
}

func (f *fakeWorkspace) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	f.asked = append(f.asked, "refs "+slug)
	return f.refs[slug], f.refsErr[slug]
}

func (f *fakeWorkspace) UpstreamFile(_ context.Context, slug, ref, path string) ([]byte, error) {
	f.asked = append(f.asked, "file "+slug+"@"+ref+":"+path)
	content, ok := f.files[slug+"@"+ref+":"+path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return []byte(content), nil
}

type fakeProber struct {
	mu      sync.Mutex
	results map[string]registry.Result
	calls   int
}

func (f *fakeProber) Exists(_ context.Context, host, image, tag string) registry.Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if r, ok := f.results[host+" "+image+":"+tag]; ok {
		return r
	}
	return registry.Result{Status: registry.Found}
}

func riConsumer(strategy string) config.Consumer {
	return config.Consumer{
		Project: config.Project{
			Name: "rancher", Repo: config.Repo{Name: "rancher/rancher", DefaultBranch: "main"},
			Release: config.Release{Branches: "perLine", Lines: []config.ProjectLine{{Line: "v2.15", Branch: "release/v2.15"}, {Line: "v2.14", Branch: "release/v2.14"}}},
		},
		Source: config.Source{
			From:   "scc-operator",
			Accept: config.Accept{When: config.AcceptStable, Lines: config.AcceptLinesAll},
			Update: config.Update{Strategy: strategy, Params: map[string]string{"file": "build.yaml", "key": "defaultSccOperatorImage"}},
		},
	}
}

func fixture() *fakeWorkspace {
	return &fakeWorkspace{
		producer:  universal(),
		consumers: []config.Consumer{riConsumer("buildYamlBump")},
		refs: map[string]git.Refs{
			"rancher/scc-operator": {Tags: tags("v0.5.2", "v0.5.3", "v0.6.0-rc.1")},
			"rancher/rancher": {Branches: []git.Branch{
				{Name: "main", Commit: "sha-main"}, {Name: "release/v2.15", Commit: "sha-215"},
			}},
		},
		files: map[string]string{
			"rancher/rancher@sha-215:build.yaml": "defaultSccOperatorImage: rancher/scc-operator:v0.5.2\n",
			"rancher/rancher@sha-main:build.yaml": "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n",
		},
	}
}

var ctx = context.Background()

func TestCheckReady(t *testing.T) {
	ws, p := fixture(), &fakeProber{}

	got, err := Check(ctx, ws, p, Input{Project: "scc-operator", Branch: "release/v2.15"})

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", got.Producer)
	assert.Equal(t, Release{Tag: "v0.5.3", Commit: "sha-v0.5.3", Stable: true}, got.Release)
	assert.Equal(t, "rancher/rancher", got.Consumer)
	assert.Equal(t, "buildYamlBump", got.Strategy)
	assert.Equal(t, Target{Branch: "release/v2.15", Commit: "sha-215"}, got.Target)
	assert.Equal(t, "build.yaml", got.File)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", got.Current)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", got.Want)
	assert.Equal(t, StatusReady, got.Status)
	assert.Empty(t, got.Blockers)
	require.NotNil(t, got.Images)
	assert.False(t, got.Images.Failed())
	assert.Equal(t, "scc-operator", ws.asked[0][len("consumers "):])
}

func TestCheckReadsTheFileAtTheCommitItListed(t *testing.T) {
	ws := fixture()
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Branch: "release/v2.15"})
	require.NoError(t, err)
	assert.Contains(t, ws.asked, "file rancher/rancher@sha-215:build.yaml", "the head commit, so the read is exact")
}

func TestCheckAlreadyUpToDate(t *testing.T) {
	p := &fakeProber{}
	got, err := Check(ctx, fixture(), p, Input{Project: "scc-operator", Branch: "main"})
	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, got.Status)
	assert.Equal(t, got.Current, got.Want)
	assert.Nil(t, got.Images, "nothing else runs")
	assert.Zero(t, p.calls)
}

func TestCheckBlockedByAMissingImage(t *testing.T) {
	p := &fakeProber{results: map[string]registry.Result{
		"registry.rancher.com rancher/scc-operator:v0.5.3": {Status: registry.NotFound},
	}}
	got, err := Check(ctx, fixture(), p, Input{Project: "scc-operator", Branch: "release/v2.15"})
	require.NoError(t, err)
	assert.Equal(t, StatusBlocked, got.Status)
	require.Len(t, got.Blockers, 1)
	assert.Contains(t, got.Blockers[0], "images")
	assert.True(t, got.Images.Failed())
}

func TestCheckAnExplicitTagAndALine(t *testing.T) {
	got, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.5.2", Branch: "release/v2.15"})
	require.NoError(t, err)
	assert.Equal(t, "v0.5.2", got.Release.Tag)
	assert.Equal(t, StatusUpToDate, got.Status, "the consumer already holds v0.5.2")
}

func TestCheckARefusedPrerelease(t *testing.T) {
	_, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.6.0-rc.1", Branch: "main"})
	assert.ErrorIs(t, err, ErrPrerelease)

	got, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.6.0-rc.1", Branch: "main", Prerelease: true})
	require.NoError(t, err)
	assert.Equal(t, "v0.6.0-rc.1", got.Release.Tag)
	assert.False(t, got.Release.Stable)
}

func TestCheckABranchIsRequired(t *testing.T) {
	_, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator"})
	assert.ErrorIs(t, err, ErrBranchRequired)
	assert.ErrorContains(t, err, "release/v2.15", "the message names the consumer's release branches")
	assert.ErrorContains(t, err, "release/v2.14")
}

func TestCheckABranchTheConsumerDoesNotHave(t *testing.T) {
	_, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Branch: "release/v2.14"})
	assert.ErrorIs(t, err, ErrBranchNotFound)
	assert.ErrorContains(t, err, "release/v2.14")
}

func TestCheckConsumers(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		ws := fixture()
		ws.consumers = nil
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Branch: "main"})
		assert.ErrorIs(t, err, ErrNoConsumer)
	})
	t.Run("several and none chosen", func(t *testing.T) {
		ws := fixture()
		other := riConsumer("buildYamlBump")
		other.Project.Name, other.Project.Repo.Name = "charts", "rancher/charts"
		ws.consumers = append(ws.consumers, other)
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Branch: "main"})
		assert.ErrorIs(t, err, ErrAmbiguousConsumer)
		assert.ErrorContains(t, err, "charts")
		assert.ErrorContains(t, err, "rancher")
	})
	t.Run("chosen by name or slug", func(t *testing.T) {
		for _, to := range []string{"charts", "rancher/charts", "Rancher/Charts"} {
			ws := fixture()
			other := riConsumer("buildYamlBump")
			other.Project.Name, other.Project.Repo.Name = "charts", "rancher/charts"
			ws.consumers = append(ws.consumers, other)
			ws.refs["rancher/charts"] = ws.refs["rancher/rancher"]
			ws.files["rancher/charts@sha-main:build.yaml"] = "defaultSccOperatorImage: x\n"
			got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", To: to, Branch: "main"})
			require.NoError(t, err, to)
			assert.Equal(t, "rancher/charts", got.Consumer)
		}
	})
	t.Run("a --to that is not a consumer", func(t *testing.T) {
		_, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", To: "nope", Branch: "main"})
		assert.ErrorIs(t, err, ErrNoConsumer)
	})
}

func TestCheckUnknownStrategy(t *testing.T) {
	ws := fixture()
	ws.consumers = []config.Consumer{riConsumer("chartsPackageBump")}
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Branch: "main"})
	assert.ErrorIs(t, err, ErrUnknownStrategy)
}

func TestCheckReadFailures(t *testing.T) {
	t.Run("the file is missing", func(t *testing.T) {
		ws := fixture()
		delete(ws.files, "rancher/rancher@sha-215:build.yaml")
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Branch: "release/v2.15"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "build.yaml")
		assert.ErrorContains(t, err, "release/v2.15")
	})
	t.Run("the key is missing", func(t *testing.T) {
		ws := fixture()
		ws.files["rancher/rancher@sha-215:build.yaml"] = "other: 1\n"
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Branch: "release/v2.15"})
		assert.ErrorIs(t, err, yamledit.ErrKeyNotFound)
		assert.ErrorContains(t, err, "defaultSccOperatorImage")
	})
}

func TestCheckHostErrors(t *testing.T) {
	boom := errors.New("network down")
	for slug := range map[string]bool{"rancher/scc-operator": true, "rancher/rancher": true} {
		ws := fixture()
		ws.refsErr = map[string]error{slug: boom}
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Branch: "main"})
		assert.ErrorIs(t, err, boom, slug)
	}
}

func TestCheckProducerErrors(t *testing.T) {
	ws := fixture()
	ws.consumersErr = workspace.ErrUnknownProject
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "nope", Branch: "main"})
	assert.ErrorIs(t, err, workspace.ErrUnknownProject)
}

func TestCheckValueVariables(t *testing.T) {
	ws := fixture()
	c := riConsumer("buildYamlBump")
	c.Source.Update.Params = map[string]string{
		"file": "build.yaml", "key": "defaultSccOperatorImage", "value": "{sourceRepo}@{version}",
	}
	ws.consumers = []config.Consumer{c}
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Branch: "release/v2.15"})
	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator@0.5.3", got.Want)
}
