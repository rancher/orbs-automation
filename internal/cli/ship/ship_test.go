package ship

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkspace struct {
	err   error
	files map[string]string
}

func (f *fakeWorkspace) Consumers(string) (config.Project, []config.Consumer, error) {
	if f.err != nil {
		return config.Project{}, nil, f.err
	}
	producer := config.Project{
		Name: "scc-operator", Repo: config.Repo{Name: "rancher/scc-operator"}, Release: config.Release{TagPattern: "v*"},
	}
	consumer := config.Consumer{
		Project: config.Project{Name: "rancher", Repo: config.Repo{Name: "rancher/rancher"}},
		Source: config.Source{
			From: "scc-operator", Accept: config.Accept{When: config.AcceptStable},
			Update: config.Update{Strategy: "buildYamlBump", Params: map[string]string{"file": "build.yaml", "key": "image"}},
		},
	}
	return producer, []config.Consumer{consumer}, nil
}

func (f *fakeWorkspace) UpstreamRefs(_ context.Context, slug string) (git.Refs, error) {
	if slug == "rancher/scc-operator" {
		return git.Refs{Tags: []git.Tag{{Name: "v0.5.3", Commit: "1111111aaaaaaaa"}}}, nil
	}
	return git.Refs{Branches: []git.Branch{{Name: "main", Commit: "2222222bbbbbbbb"}}}, nil
}

func (f *fakeWorkspace) UpstreamFile(context.Context, string, string, string) ([]byte, error) {
	return []byte(f.files["build.yaml"]), nil
}

type fakeProber struct{ result registry.Result }

func (f fakeProber) Exists(context.Context, string, string, string) registry.Result { return f.result }

var in = domain.Input{Project: "scc-operator", Branch: "main"}

func readyWorkspace() *fakeWorkspace {
	return &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.5.2\n"}}
}

func TestRunReady(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, clicfg.OutputText, in, false)

	require.NoError(t, err)
	for _, want := range []string{
		"rancher/scc-operator", "v0.5.3", "stable", "rancher/rancher", "main", "2222222",
		"buildYamlBump", "build.yaml", "rancher/scc-operator:v0.5.2", "rancher/scc-operator:v0.5.3",
		"ready", "Nothing was written",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "2222222bbbbbbbb", "commits are shortened")
}

func TestRunAlreadyUpToDate(t *testing.T) {
	ws := &fakeWorkspace{files: map[string]string{"build.yaml": "image: rancher/scc-operator:v0.5.3\n"}}
	out, err := run(context.Background(), ws, fakeProber{}, clicfg.OutputText, in, false)
	require.NoError(t, err)
	assert.Contains(t, out, "already up to date")
	assert.NotContains(t, out, "Images", "the images were not checked")
}

func TestRunBlockedKeepsTheReport(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.NotFound}}, clicfg.OutputText, in, false)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.Contains(t, out, "blocked")
	assert.Contains(t, out, "not found", "the missing images are listed")
}

func TestRunErrorsPrintNothing(t *testing.T) {
	boom := errors.New("boom")
	out, err := run(context.Background(), &fakeWorkspace{err: boom}, fakeProber{}, clicfg.OutputText, in, false)
	assert.ErrorIs(t, err, boom)
	assert.Empty(t, out)
}

func TestRunJSON(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.Found}}, clicfg.OutputJSON, in, false)
	require.NoError(t, err)
	var got domain.Plan
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, domain.StatusReady, got.Status)
	assert.Equal(t, "2222222bbbbbbbb", got.Target.Commit, "JSON carries the full commit")
}

func TestRunJSONWhenBlocked(t *testing.T) {
	out, err := run(context.Background(), readyWorkspace(), fakeProber{result: registry.Result{Status: registry.NotFound}}, clicfg.OutputJSON, in, false)
	assert.ErrorIs(t, err, ErrBlocked)
	assert.True(t, json.Valid([]byte(out)))
}

func TestRunRejectsOtherOutputFormats(t *testing.T) {
	_, err := run(context.Background(), readyWorkspace(), fakeProber{}, clicfg.OutputEnv, in, false)
	assert.ErrorContains(t, err, "not supported by ship")
	_, err = Run(context.Background(), clicfg.Settings{Output: clicfg.OutputEnv}, t.TempDir(), Options{Project: "scc-operator"})
	assert.ErrorContains(t, err, "not supported by ship")
}
