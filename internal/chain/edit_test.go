package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
)

func TestAnEditKnowsItsEnv(t *testing.T) {
	g := chaintest.Load(t, pair(
		holding("holds-img", "{project: hold, artifact: img}", holdsImage),
		chaintest.Doc("Edit", "regen", "{holding: holds-img}", "  env:\n    GOTOOLCHAIN: \"{goToolchain}\"\n  postEdit:\n    - {run: [go, generate]}\n"),
		chaintest.Doc("Edit", "plain", "{holding: holds-img}", "  postEdit:\n    - {run: x}\n"),
	)...)

	edits := g.Holdings()[0].Edits()
	assert.Equal(t, map[string]string{"GOTOOLCHAIN": "{goToolchain}"}, edits[0].Env())
	assert.Empty(t, edits[1].Env())
}
