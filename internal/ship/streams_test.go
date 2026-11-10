package ship

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACheckChoosesTheStreamTheConsumerHoldsForTheLine(t *testing.T) {
	// svc holds lib's v1 stream for v2.15 and newer, and its v2 stream for v2.14 and older.
	tests := []struct {
		name     string
		in       Input
		wantTag  string
		branches []string
	}{
		{"--line on the newer stream", Input{Lines: []string{"v2.15"}}, "v1.1.0", []string{"release/v2.15"}},
		{"--line on the older stream", Input{Lines: []string{"v2.14"}}, "v2.1.0", []string{"release/v2.14"}},
		{"a line as the argument", Input{Arg: "v2.14"}, "v2.1.0", []string{"release/v2.14"}},
		{"a line as the argument, without its v", Input{Arg: "2.15"}, "v1.1.0", []string{"release/v2.15"}},
		{"a tag", Input{Arg: "v1.0.0"}, "v1.0.0", []string{"main", "release/v2.15"}},
		{"a tag of the other stream", Input{Arg: "v2.0.0"}, "v2.0.0", []string{"release/v2.14"}},
		{"a tag and a line of its stream", Input{Arg: "v1.0.0", Lines: []string{"v2.15"}}, "v1.0.0", []string{"release/v2.15"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in
			in.Team, in.Project, in.To = testTeam, "lib", "svc"

			got, err := Check(ctx, twoStreams(t), &fakeProber{}, in)

			require.NoError(t, err)
			require.Len(t, got.Releases, 1)
			assert.Equal(t, tt.wantTag, got.Releases[0].Tag)
			assert.Equal(t, tt.branches, branchesOf(got))
		})
	}
}

func TestACheckWithNoLineAndNoTagAsksForOneWhenTheConsumerTakesSeveralStreams(t *testing.T) {
	_, err := Check(ctx, twoStreams(t), &fakeProber{}, Input{Team: testTeam, Project: "lib", To: "svc"})

	require.ErrorIs(t, err, ErrAmbiguousConsumer)
	assert.ErrorContains(t, err, "name a line", "it says what to do")
	assert.ErrorContains(t, err, "lib-one (v2.16, v2.15)", "and which stream is held for which lines")
	assert.ErrorContains(t, err, "lib-two (v2.14)")
}

func TestACheckRefusesALineNoStreamIsHeldFor(t *testing.T) {
	ws := twoStreams(t)
	ws.links[1].HoldsLines = "{since: v2.15, until: v2.15}" // v2.14 is now held by none

	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "lib", To: "svc", Lines: []string{"v2.14"}})

	require.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "v2.14")
	assert.ErrorContains(t, err, "lib-one (v2.16, v2.15)", "it lists the lines that are held")
}

func TestACheckRefusesLinesOnDifferentStreams(t *testing.T) {
	_, err := Check(ctx, twoStreams(t), &fakeProber{}, Input{Team: testTeam, Project: "lib", To: "svc", Lines: []string{"v2.15", "v2.14"}})

	require.ErrorIs(t, err, ErrAmbiguousConsumer)
	assert.ErrorContains(t, err, "different streams")
	assert.ErrorContains(t, err, "one stream")
}

func TestACheckRefusesATagAndALineOfDifferentStreams(t *testing.T) {
	_, err := Check(ctx, twoStreams(t), &fakeProber{}, Input{Team: testTeam, Project: "lib", To: "svc", Arg: "v1.0.0", Lines: []string{"v2.14"}})

	require.ErrorIs(t, err, ErrNoTargets, "the stream of the tag is not held for that line")
}

func TestAConsumerWithOneRouteTakesALineAsTheArgumentToo(t *testing.T) {
	got, err := Check(ctx, withDevLine(fixture(t)), &fakeProber{}, Input{Team: testTeam, Project: "scc-operator", To: "rancher", Arg: "v2.15"})

	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.15"}, branchesOf(got), "a universal stream has no lines of its own, so the line is the consumer's")
	require.Len(t, got.Releases, 1)
	assert.Equal(t, "v0.5.3", got.Releases[0].Tag)
}

func TestACheckRefusesALineTwoStreamsAreHeldFor(t *testing.T) {
	ws := twoStreams(t)
	ws.links[1].HoldsLines = "{since: v2.14}" // both streams are held for v2.15 now

	_, err := Check(ctx, ws, &fakeProber{}, Input{Team: testTeam, Project: "lib", To: "svc", Lines: []string{"v2.15"}})

	require.ErrorIs(t, err, ErrAmbiguousConsumer)
	assert.ErrorContains(t, err, "more than one stream")
	assert.ErrorContains(t, err, "lib-one")
	assert.ErrorContains(t, err, "lib-two")
}
