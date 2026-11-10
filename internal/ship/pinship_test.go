package ship

import (
	"context"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/chain/chaintest"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinWorkspace is the world of pinGraph served as a workspace: wrangler is released
// as v1.7.3 and v1.7.4, whose go.mod requires lasso v0.2.5, bro holds both modules in
// go.mod on its main branch and on r15, and lasso has v0.2.4 to v0.2.6.
type pinWorkspace struct {
	g    *chain.Graph
	host pinHost
}

func (w pinWorkspace) Project(key string) (*chain.Project, error) { return w.g.Project(key), nil }

func (w pinWorkspace) Bump() (*bump.Engine, error) { return bump.NewEngine(bump.Builtin(), w.g) }

func (w pinWorkspace) UpstreamRefs(c context.Context, slug string) (git.Refs, error) {
	return w.host.UpstreamRefs(c, slug)
}

func (w pinWorkspace) UpstreamFile(c context.Context, slug, ref, path string) ([]byte, error) {
	return w.host.UpstreamFile(c, slug, ref, path)
}

func (pinWorkspace) CloneDir(string) (string, error)  { return "", nil }
func (pinWorkspace) Remote(string) (string, error)    { return "upstream", nil }
func (pinWorkspace) PushSettings(string) PushSettings { return PushSettings{} }

// broGoMod is bro's go.mod holding wrangler at v1.7.1 and lasso at the version given.
func broGoMod(la string) string {
	return "module acme/bro\n\nrequire (\n\tgithub.com/acme/la " + la + "\n\tgithub.com/acme/wr v1.7.1\n)\n"
}

func newPinWorkspace(t *testing.T) pinWorkspace {
	t.Helper()
	g, _ := pinGraph(t, laStream)
	return pinWorkspace{g: g, host: pinHost{
		files: map[string]string{
			"acme/wr@sha-v1.7.4:go.mod": "", // not read: the tag's commit is not a ref
			"acme/wr@v1.7.4:go.mod":     "module github.com/acme/wr\n\nrequire github.com/acme/la v0.2.5\n",
			"acme/bro@sha-main:go.mod":  broGoMod("v0.2.3"),
			"acme/bro@sha-r15:go.mod":   broGoMod("v0.2.3"),
		},
		tags: map[string][]git.Tag{
			"acme/wr": tags("v1.7.3", "v1.7.4"),
			"acme/la": tags("v0.2.4", "v0.2.5", "v0.2.6"),
		},
		branches: map[string][]git.Branch{"acme/bro": {{Name: "main", Commit: "sha-main"}, {Name: "r15", Commit: "sha-r15"}}},
	}}
}

func shipWrangler(w pinWorkspace) (Plan, error) {
	return Check(ctx, w, &fakeProber{}, Input{Team: testTeam, Project: "wr", To: "bro", Arg: "v1.7.4"})
}

func holdingNamed(t *testing.T, tp TargetPlan, name string) HoldingPlan {
	t.Helper()
	for _, h := range tp.Holdings {
		if h.Name == name {
			return h
		}
	}
	require.Failf(t, "no such holding", "%s is not in %v", name, tp.Holdings)
	return HoldingPlan{}
}

// r15Target is the target of the plan on bro's r15 branch.
func r15Target(t *testing.T, p Plan) TargetPlan {
	t.Helper()
	for _, tp := range p.Targets {
		if tp.Branch == "r15" {
			return tp
		}
	}
	require.Failf(t, "no such target", "r15 is not in %v", branchesOf(p))
	return TargetPlan{}
}

func TestACheckShipsTheLassoThePinResolvesWithWrangler(t *testing.T) {
	got, err := shipWrangler(newPinWorkspace(t))

	require.NoError(t, err)
	assert.Equal(t, StatusReady, got.Status, "%v", got.Blockers)
	require.Len(t, got.Companions, 1)
	c := got.Companions[0]
	assert.Equal(t, "v0.2.5", c.Tag)
	assert.Equal(t, "acme/la", c.Producer)
	assert.Equal(t, "acme/wr v1.7.4", c.RequiredBy)
	assert.Empty(t, c.Error)
	tp := r15Target(t, got)
	require.Len(t, tp.Holdings, 2, "wrangler's holding, and the lasso that follows it")
	wr, la := holdingNamed(t, tp, "bro-holds-wr"), holdingNamed(t, tp, "bro-holds-wr+la-mod")
	assert.False(t, wr.Pinned)
	assert.True(t, la.Pinned)
	assert.Equal(t, "v1.7.4", wr.Want)
	assert.Equal(t, "v0.2.5", la.Want, "the tag the pin derived, and not the latest lasso")
	assert.Equal(t, "v0.2.3", la.Current)
	assert.False(t, la.UpToDate)
	assert.Equal(t, []string{"bro-holds-wr"}, got.Holdings, "the plan's holdings are the producer's own")
}

func TestACheckKeepsALassoNewerThanThePinResolvesAndSaysSo(t *testing.T) {
	w := newPinWorkspace(t)
	w.host.files["acme/bro@sha-r15:go.mod"] = broGoMod("v0.2.9")
	w.host.files["acme/bro@sha-main:go.mod"] = broGoMod("v0.2.9")

	got, err := shipWrangler(w)

	require.NoError(t, err)
	tp := r15Target(t, got)
	assert.Equal(t, StatusReady, tp.Status, "wrangler is still to be written")
	assert.True(t, holdingNamed(t, tp, "bro-holds-wr+la-mod").UpToDate, "a floor that is met")
	require.Len(t, tp.Warnings, 1)
	assert.Contains(t, tp.Warnings[0], "v0.2.9")
	assert.Contains(t, tp.Warnings[0], "v0.2.5", "it names what the pin resolved")
	assert.Contains(t, tp.Warnings[0], "acme/wr v1.7.4", "and what requires it")
	assert.Contains(t, tp.Warnings[0], "kept")
}

func TestACheckBlocksAReleaseWhoseCompanionCannotBeDerived(t *testing.T) {
	w := newPinWorkspace(t)
	w.host.files["acme/wr@v1.7.4:go.mod"] = "module github.com/acme/wr\n\nrequire github.com/acme/la v0.2.9\n" // lasso has no such tag

	got, err := shipWrangler(w)

	require.NoError(t, err, "the plan says it, and does not stop")
	assert.Equal(t, StatusBlocked, got.Status)
	require.Len(t, got.Blockers, 1)
	assert.Contains(t, got.Blockers[0], "v0.2.9")
	assert.Contains(t, got.Blockers[0], "no tag")
	require.Len(t, got.Companions, 1)
	assert.NotEmpty(t, got.Companions[0].Error)
	assert.Len(t, r15Target(t, got).Holdings, 1, "no lasso is planned for a version that cannot be shipped")
}

func TestACheckSkipsACompanionWhoseFilesDoNotRequireTheModule(t *testing.T) {
	w := newPinWorkspace(t)
	w.host.files["acme/bro@sha-r15:go.mod"] = "module acme/bro\n\nrequire github.com/acme/wr v1.7.1\n"
	w.host.files["acme/bro@sha-main:go.mod"] = "module acme/bro\n\nrequire github.com/acme/wr v1.7.1\n"

	got, err := shipWrangler(w)

	require.NoError(t, err)
	tp := r15Target(t, got)
	assert.Len(t, tp.Holdings, 1, "there is no lasso to hold")
	assert.Equal(t, StatusReady, tp.Status)
}

func TestACheckLooksForTheCompanionsReleaseInItsRegistries(t *testing.T) {
	got, err := shipWrangler(newPinWorkspace(t))

	require.NoError(t, err)
	var producers []string
	for _, im := range got.Images {
		producers = append(producers, im.Producer+" "+im.Release)
	}
	assert.Equal(t, []string{" v1.7.4", "acme/la v0.2.5"}, producers, "the release asked for, then the companion, which names its producer")
}

func TestThePRBodyListsTheCompanionAndWhatRequiresIt(t *testing.T) {
	got, err := shipWrangler(newPinWorkspace(t))

	require.NoError(t, err)
	tp := r15Target(t, got)
	require.NotNil(t, tp.Preview)
	assert.Contains(t, tp.Preview.Body, "github.com/acme/la")
	assert.Contains(t, tp.Preview.Body, "v0.2.5, which acme/wr v1.7.4 requires")
}

func TestATargetOfAPlanTakesTheCompanionsAndTheirReleasesToTheEngine(t *testing.T) {
	plan, res, err := check(ctx, newPinWorkspace(t), &fakeProber{}, Input{Team: testTeam, Project: "wr", To: "bro", Arg: "v1.7.4"}, nil)
	require.NoError(t, err)

	target := res.targetOf(r15Target(t, plan))

	require.Len(t, target.Holdings, 2)
	pinned := target.Holdings[1]
	assert.True(t, pinned.Virtual())
	assert.Equal(t, "v0.2.5", target.Pinned[pinned])
	assert.Equal(t, "r15", target.Branch)
}

func TestHoldingsOfLassoTraceTheVirtualHoldingAsAnyOther(t *testing.T) {
	w := newPinWorkspace(t)

	got, err := Holdings(ctx, w, w.g.Project("la"))

	require.NoError(t, err)
	require.Len(t, got, 1, "the pin makes bro a consumer of lasso")
	assert.Equal(t, "acme/bro", got[0].Consumer)
	assert.Empty(t, got[0].Error)
	require.NotEmpty(t, got[0].Targets)
	held := got[0].Targets[0].Held[0]
	assert.Equal(t, "bro-holds-wr+la-mod", held.Name)
	assert.Equal(t, "v0.2.3", held.Current)
	assert.Equal(t, "v0.2.6", held.Stable.Tag, "against the latest lasso, until status reads what the pin requires")
}

func TestHoldingsOfLassoSayWhenNoFileOfTheVirtualHoldingRequiresIt(t *testing.T) {
	w := newPinWorkspace(t)
	w.host.files["acme/bro@sha-main:go.mod"] = "module acme/bro\n\nrequire github.com/acme/wr v1.7.1\n"
	w.host.files["acme/bro@sha-r15:go.mod"] = "module acme/bro\n\nrequire github.com/acme/wr v1.7.1\n"

	got, err := Holdings(ctx, w, w.g.Project("la"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotEmpty(t, got[0].Targets)
	held := got[0].Targets[0].Held[0]
	assert.Contains(t, held.Error, "requires", "no panic, and the row says why it holds nothing")
}

// withDestination is the world with rancher, the final destination, holding wrangler
// for v2.15 and newer as bro does, so it holds lasso through the pin; it is on v1.7.4
// and lasso v0.2.5 on both branches.
func withPinDestination(t *testing.T) pinWorkspace {
	t.Helper()
	g, _ := pinGraph(t, laStream,
		chaintest.Doc("Project", "dest", "", "  repo: {name: acme/dest}\n  finalDestination: true\n  branches:\n    layout: perLine\n    lines: [{line: v2.16}, {line: v2.15, branch: r15}]\n"),
		chaintest.Doc("Holding", "dest-holds-wr", "{project: dest, artifact: wr-mod}", "  at: {file: go.mod, key: github.com/acme/wr, format: gomod}\n  holds: {kind: release, form: \"{bareTag}\", accept: all, lines: {since: v2.15}}\n"),
	)
	w := pinWorkspace{g: g, host: newPinWorkspace(t).host}
	w.host.files["acme/dest@sha-dmain:go.mod"] = "module acme/dest\n\nrequire (\n\tgithub.com/acme/la v0.2.5\n\tgithub.com/acme/wr v1.7.4\n)\n"
	w.host.files["acme/dest@sha-d15:go.mod"] = "module acme/dest\n\nrequire (\n\tgithub.com/acme/la v0.2.5\n\tgithub.com/acme/wr v1.7.4\n)\n"
	w.host.branches["acme/dest"] = []git.Branch{{Name: "main", Commit: "sha-dmain"}, {Name: "r15", Commit: "sha-d15"}}
	return w
}

func TestACheckLooksForACompanionsReleaseEvenWhenTheConsumerAlreadyHoldsIt(t *testing.T) {
	w := newPinWorkspace(t)
	w.host.files["acme/bro@sha-r15:go.mod"] = broGoMod("v0.2.5")
	w.host.files["acme/bro@sha-main:go.mod"] = broGoMod("v0.2.5")

	got, err := shipWrangler(w)

	require.NoError(t, err)
	var producers []string
	for _, im := range got.Images {
		producers = append(producers, im.Producer+" "+im.Release)
	}
	assert.Equal(t, []string{" v1.7.4", "acme/la v0.2.5"}, producers, "it goes out with the release, so its artifact is checked whatever bro holds")
	assert.True(t, holdingNamed(t, r15Target(t, got), "bro-holds-wr+la-mod").UpToDate)
}

func TestAHoldingOfAPlanNamesTheProjectOfItsArtifactAndTheReleaseOfACompanion(t *testing.T) {
	got, err := shipWrangler(newPinWorkspace(t))

	require.NoError(t, err)
	tp := r15Target(t, got)
	assert.Equal(t, "wr", holdingNamed(t, tp, "bro-holds-wr").Project)
	la := holdingNamed(t, tp, "bro-holds-wr+la-mod")
	assert.Equal(t, "la", la.Project)
	assert.Equal(t, "v0.2.5", la.Release, "the tag of the companion, which is not the target's")
	assert.Empty(t, holdingNamed(t, tp, "bro-holds-wr").Release, "the target's own release is the target's")
	require.Len(t, got.Companions, 1)
	assert.Equal(t, "la", got.Companions[0].Name)
}

func TestACheckReadsTheDestinationForTheCompanionToo(t *testing.T) {
	got, err := shipWrangler(withPinDestination(t))

	require.NoError(t, err)
	require.Len(t, got.Destination, 4, "wrangler and lasso on each of the two lines")
	var seen []string
	for _, d := range got.Destination {
		seen = append(seen, d.Line+" "+d.Producer)
		assert.Equal(t, "acme/dest", d.Project)
		assert.Empty(t, d.Error)
		require.Len(t, d.Held, 1)
	}
	assert.ElementsMatch(t, []string{"v2.16 wr", "v2.16 la", "v2.15 wr", "v2.15 la"}, seen)
	for _, d := range got.Destination {
		if d.Producer == "la" {
			assert.Equal(t, "v0.2.5", d.Held[0].Current, "what the destination really holds of lasso")
			assert.Equal(t, "la", d.Stream)
		}
	}
}

func TestACheckKeepsTheTitleAndTheCommitToWranglerWhenTheCompanionIsNotBumped(t *testing.T) {
	w := newPinWorkspace(t)
	w.host.files["acme/bro@sha-r15:go.mod"] = broGoMod("v0.2.5")

	got, err := shipWrangler(w)

	require.NoError(t, err)
	tp := r15Target(t, got)
	require.NotNil(t, tp.Preview)
	assert.Equal(t, "[r15] Update wr to v1.7.4", tp.Preview.Title)
	assert.Contains(t, tp.Preview.CommitMessage, "Update wr to v1.7.4\n")
}

func TestACheckNamesTheCompanionInTheTitleAndTheCommitWhenItIsBumped(t *testing.T) {
	got, err := shipWrangler(newPinWorkspace(t))

	require.NoError(t, err)
	tp := r15Target(t, got)
	require.NotNil(t, tp.Preview)
	assert.Equal(t, "[r15] Update wr to v1.7.4 and la to v0.2.5", tp.Preview.Title)
	assert.True(t, strings.HasPrefix(tp.Preview.CommitMessage, "Update wr to v1.7.4 and la to v0.2.5\n"), tp.Preview.CommitMessage)
	assert.Contains(t, tp.Preview.Branch, "wr-v1.7.4-r15", "the branch is named for the release asked for, so it is the same whether lasso goes with it or not")
	assert.NotContains(t, tp.Preview.Branch, "la")
}
