package chainload_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/chainload"
	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = config.Context{Now: chaintest.Now}

func project(name, spec string) string { return chaintest.Doc("Project", name, "", spec) }

func files(m map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, data := range m {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

func TestLoadJoinsTheConfigAndKeepsItsWarnings(t *testing.T) {
	g, rep := chainload.Load(files(map[string]string{
		"lines.yaml": "apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n    - id: v2.15\n      dates: {ga: 2026-08-27}\n",
		"p.yaml":     project("p", "  repo: {name: acme/a}\n  branches:\n    layout: perLine\n    lines: [{line: v2.15}]\n"),
	}), ctx)

	require.NoError(t, rep.Err())
	require.NotNil(t, g)
	assert.Equal(t, "p", g.Project("p").Name())
	require.Len(t, g.Warnings(), 1)
	assert.Contains(t, g.Warnings()[0].Error(), "p.yaml")
	assert.False(t, g.Empty())
}

func TestLoadOfABadConfigIsNoGraphAndTheProblems(t *testing.T) {
	g, rep := chainload.Load(files(map[string]string{"bad.yaml": project("p", "  repo: {name: nope}\n")}), ctx)

	assert.Nil(t, g)
	require.Error(t, rep.Err())
	assert.True(t, strings.Contains(rep.Err().Error(), "bad.yaml"), rep.Err().Error())
}

func TestLoadOfNothingIsAnEmptyGraph(t *testing.T) {
	g, rep := chainload.Load(files(map[string]string{"README.md": "hello"}), ctx)

	require.NoError(t, rep.Err())
	require.NotNil(t, g)
	assert.True(t, g.Empty())
	assert.Empty(t, g.Projects())
	assert.Empty(t, g.Warnings())
}
