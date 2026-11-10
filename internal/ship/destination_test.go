package ship

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withDestination is twoStreams with a final destination that holds lib too, and
// reads it on the lines the way svc does: the v1 stream for v2.15 and newer, the v2
// stream for v2.14 and older.
func withDestination(t *testing.T) *fakeWorkspace {
	t.Helper()
	ws := twoStreams(t)
	dest := projectSpec{
		Name: "dest", Repo: "acme/dest", FinalDestination: true,
		Lines: []lineSpec{{"v2.16", ""}, {"v2.15", "release/v2.15"}, {"v2.14", "release/v2.14"}},
	}
	ws.links = append(ws.links,
		link{Consumer: dest, Artifact: "lib-one-image", Key: "one", HoldsLines: "{since: v2.15}", Form: "{tag}"},
		link{Consumer: dest, Artifact: "lib-two-image", Key: "two", HoldsLines: "{until: v2.14}", Form: "{tag}"},
	)
	ws.refs["acme/dest"] = git.Refs{Branches: []git.Branch{{Name: "main", Commit: "sha-dmain"}, {Name: "release/v2.15", Commit: "sha-d215"}, {Name: "release/v2.14", Commit: "sha-d214"}}}
	ws.files["acme/dest@sha-dmain:build.yaml"] = "one: v1.1.0\ntwo: v2.0.0\n"
	ws.files["acme/dest@sha-d215:build.yaml"] = "one: v1.1.0\ntwo: v2.0.0\n"
	ws.files["acme/dest@sha-d214:build.yaml"] = "one: v1.0.0\ntwo: v2.1.0\n"
	return ws
}

func shipLib(ws *fakeWorkspace, in Input) (Plan, error) {
	in.Team, in.Project, in.To = testTeam, "lib", "svc"
	return Check(ctx, ws, &fakeProber{}, in)
}

func TestACheckReadsTheDestinationForTheLineItShipsTo(t *testing.T) {
	got, err := shipLib(withDestination(t), Input{Lines: []string{"v2.15"}})

	require.NoError(t, err)
	require.Len(t, got.Destination, 1)
	d := got.Destination[0]
	assert.Equal(t, "acme/dest", d.Project)
	assert.Equal(t, "v2.15", d.Line)
	assert.Equal(t, "lib-one", d.Stream)
	assert.Equal(t, "release/v2.15", d.Branch)
	assert.Empty(t, d.Error)
	require.Len(t, d.Held, 1)
	assert.Equal(t, "v1.1.0", d.Held[0].Current, "what the destination really holds, read from the host")
	require.Len(t, got.Targets, 1)
	assert.Empty(t, got.Targets[0].Warnings, "the destination agrees with the consumer's holding")
}

func TestACheckReadsTheDestinationOnceForEachLine(t *testing.T) {
	got, err := shipLib(withDestination(t), Input{Arg: "v1.1.0"})

	require.NoError(t, err)
	var lines []string
	for _, d := range got.Destination {
		lines = append(lines, d.Line)
	}
	assert.Equal(t, []string{"v2.16", "v2.15"}, lines, "a release for two branches reads the destination for each line")
}

func TestACheckDoesNotReadTheDestinationWhenItIsTheConsumer(t *testing.T) {
	in := Input{Team: testTeam, Project: "lib", To: "dest", Lines: []string{"v2.15"}}

	got, err := Check(ctx, withDestination(t), &fakeProber{}, in)

	require.NoError(t, err)
	assert.Empty(t, got.Destination)
}

func TestACheckHasNoDestinationWhenNoneHoldsTheProducerOnTheLine(t *testing.T) {
	got, err := shipLib(twoStreams(t), Input{Lines: []string{"v2.15"}})

	require.NoError(t, err)
	assert.Empty(t, got.Destination)

	ws := withDestination(t)
	ws.links[2].HoldsLines = "{since: v2.16}" // the destination holds lib-one for v2.16 only
	got, err = shipLib(ws, Input{Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Empty(t, got.Destination, "the destination holds nothing of lib-one on v2.15, and lib-two is for v2.14")
}

func TestACheckWarnsWhenTheDestinationHoldsAnotherStreamForTheLine(t *testing.T) {
	ws := withDestination(t)
	ws.links[2].HoldsLines = "{since: v2.16}"
	ws.links[3].HoldsLines = "{until: v2.15}" // the destination is on lib-two for v2.15

	got, err := shipLib(ws, Input{Lines: []string{"v2.15"}})

	require.NoError(t, err, "the consumer's holding still decides the route")
	require.Len(t, got.Destination, 1)
	assert.Equal(t, "lib-two", got.Destination[0].Stream)
	require.Len(t, got.Targets, 1)
	require.Len(t, got.Targets[0].Warnings, 1)
	w := got.Targets[0].Warnings[0]
	assert.Contains(t, w, "acme/dest")
	assert.Contains(t, w, "lib-two")
	assert.Contains(t, w, "lib-one", "it names both streams")
	assert.Equal(t, StatusReady, got.Targets[0].Status, "a warning blocks nothing")
}

func TestACheckWarnsWhenWhatTheDestinationReallyHoldsIsNotTheStream(t *testing.T) {
	ws := withDestination(t)
	ws.files["acme/dest@sha-d215:build.yaml"] = "one: v2.5.0\ntwo: v2.0.0\n" // config says v1, the file says otherwise

	got, err := shipLib(ws, Input{Lines: []string{"v2.15"}})

	require.NoError(t, err)
	require.Len(t, got.Targets, 1)
	require.Len(t, got.Targets[0].Warnings, 1)
	assert.Contains(t, got.Targets[0].Warnings[0], "v2.5.0")
	assert.Contains(t, got.Targets[0].Warnings[0], "lib-one")
}

func TestACheckTakesAnUnreadableDestinationAsAWarning(t *testing.T) {
	ws := withDestination(t)
	delete(ws.files, "acme/dest@sha-d215:build.yaml")

	got, err := shipLib(ws, Input{Lines: []string{"v2.15"}})

	require.NoError(t, err)
	require.Len(t, got.Destination, 1)
	assert.NotEmpty(t, got.Destination[0].Error)
	require.Len(t, got.Targets, 1)
	require.Len(t, got.Targets[0].Warnings, 1)
	assert.Contains(t, got.Targets[0].Warnings[0], "could not read")
	assert.Equal(t, StatusReady, got.Targets[0].Status)
}

func TestACheckTakesAnUnreachableDestinationBranchAsAWarning(t *testing.T) {
	ws := withDestination(t)
	ws.refsErr = map[string]error{"acme/dest": assert.AnError}

	got, err := shipLib(ws, Input{Lines: []string{"v2.15"}})

	require.NoError(t, err)
	require.Len(t, got.Destination, 1)
	assert.NotEmpty(t, got.Destination[0].Error)
	assert.Contains(t, got.Targets[0].Warnings[0], "could not read")
}
