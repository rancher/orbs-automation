package chain_test

import (
	"testing"
	"testing/fstest"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAGraphKeepsTheWarningsAcknowledgedApartFromTheOthers(t *testing.T) {
	g, rep := chaintest.LoadFS(fstest.MapFS{
		"lines.yaml": {Data: []byte("apiVersion: gero/v1alpha2\nkind: RancherLines\nname: rancher\nspec:\n  lines:\n    - id: v2.15\n      dates: {ga: 2026-08-27}\n")},
		"p.yaml":     {Data: []byte("apiVersion: gero/v1alpha2\nkind: Project\nname: p\nacknowledge:\n  - {rule: default-branch-past-development, reason: read from main by design}\nspec:\n  repo: {name: acme/a}\n  branches:\n    layout: perLine\n    lines: [{line: v2.15}]\n")},
		"q.yaml":     {Data: []byte("apiVersion: gero/v1alpha2\nkind: Project\nname: q\nspec:\n  repo: {name: acme/b}\n  branches:\n    layout: perLine\n    lines: [{line: v2.15}]\n")},
	})
	require.NoError(t, rep.Err())

	require.Len(t, g.Warnings(), 1, "q is not acknowledged")
	assert.Equal(t, `Project "q"`, g.Warnings()[0].Label)
	require.Len(t, g.Acknowledged(), 1)
	assert.Equal(t, `Project "p"`, g.Acknowledged()[0].Label)
	assert.Equal(t, "read from main by design", g.Acknowledged()[0].Reason)
}
