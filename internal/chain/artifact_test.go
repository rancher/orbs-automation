package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func artifact(name, stream, spec string) string {
	return chaintest.Doc("Artifact", name, "{stream: "+stream+"}", spec)
}

func TestAStreamAndAProjectKnowTheirArtifacts(t *testing.T) {
	g := chaintest.Load(t,
		project("a", "  repo: {name: acme/a}\n"),
		stream("one", "a", "  universal: true\n"),
		stream("two", "a", "  universal: true\n"),
		artifact("img", "one", "  kind: image\n  name: acme/a\n"),
		artifact("file", "two", "  kind: file\n  name: a-{version}.tgz\n"),
		artifact("img2", "one", "  kind: image\n  name: acme/a-agent\n"),
	)
	a := g.Project("a")

	require.Len(t, a.Streams()[0].Artifacts(), 2)
	assert.Equal(t, "img", a.Streams()[0].Artifacts()[0].Name())
	assert.Equal(t, "img2", a.Streams()[0].Artifacts()[1].Name())
	require.Len(t, a.Streams()[1].Artifacts(), 1)
	require.Len(t, a.Artifacts(), 3)
	assert.Equal(t, []string{"acme/a", "acme/a-agent", "a-{version}.tgz"}, []string{a.Artifacts()[0].Ref(), a.Artifacts()[1].Ref(), a.Artifacts()[2].Ref()})
	assert.Equal(t, "one", a.Artifacts()[0].Stream().Name())
	assert.True(t, a.Artifacts()[0].IsImage())
	assert.False(t, a.Artifacts()[2].IsImage())
}

func TestTheRegistriesOfAnArtifact(t *testing.T) {
	g := chaintest.Load(t,
		project("a", "  repo: {name: acme/a}\n"),
		stream("s", "a", "  universal: true\n"),
		artifact("default", "s", "  kind: image\n  name: acme/a\n"),
		artifact("written", "s", "  kind: image\n  name: acme/b\n  registries: [dockerhub]\n"),
		artifact("file", "s", "  kind: file\n  name: a.tgz\n"),
	)
	arts := g.Project("a").Artifacts()

	assert.Equal(t, []string{"dockerhub", "prime-staging"}, arts[0].Registries(false))
	assert.Equal(t, []string{"dockerhub", "prime-staging", "prime-production"}, arts[0].Registries(true))
	assert.Equal(t, []string{"dockerhub"}, arts[1].Registries(false))
	assert.Equal(t, []string{"dockerhub"}, arts[1].Registries(true), "what is written is what is checked")
	assert.Empty(t, arts[2].Registries(true))
}

func TestAnArtifactAppliesToTheLinesItIsProducedFor(t *testing.T) {
	g := chaintest.Load(t,
		chaintest.Lines("v2.16", "v2.15"),
		project("a", "  repo: {name: acme/a}\n"),
		stream("s", "a", "  universal: true\n"),
		artifact("every", "s", "  kind: image\n  name: acme/a\n"),
		artifact("since", "s", "  kind: image\n  name: acme/b\n  lines: {since: v2.16}\n"),
	)
	arts := g.Project("a").Artifacts()

	assert.True(t, arts[0].AppliesTo("v2.15"))
	assert.False(t, arts[1].AppliesTo("v2.15"))
	assert.True(t, arts[1].AppliesTo("v2.16"))
	assert.True(t, arts[1].AppliesTo("2.17"))
}
