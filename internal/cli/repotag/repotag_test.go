package repotag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/git/releasetag"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

const (
	head215  = "8c613980aabbccddeeff00112233445566778899"
	headMain = "7cc6dab0aabbccddeeff00112233445566778899"
)

type fakeWorkspace struct {
	project  config.Project
	refs     git.Refs
	refsErr  error
	pathErr  error
	upstream string
}

func (f fakeWorkspace) Project(string) (config.Project, error) { return f.project, nil }
func (f fakeWorkspace) RepoPath(string) (workspace.RepoPath, error) {
	return workspace.RepoPath{Path: "/work"}, f.pathErr
}
func (f fakeWorkspace) UpstreamFor(string) string { return f.upstream }
func (f fakeWorkspace) UpstreamRefs(context.Context, string) (git.Refs, error) {
	return f.refs, f.refsErr
}

// shell is a producer with a version series for each Rancher line, in a clone
// whose upstream remote is called upstream.
func shell() fakeWorkspace {
	return fakeWorkspace{
		project: config.Project{
			Name: "shell", Repo: config.Repo{Name: "rancher/shell", DefaultBranch: "main"},
			Release: config.Release{Branching: config.BranchingPerLine, TagPattern: "v*", Lines: []config.ProjectLine{
				{Line: "v2.16", Version: "0.9.x"},
				{Line: "v2.15", Version: "0.8.x", Branch: "release/v2.15"},
			}},
		},
		refs: git.Refs{
			Branches: []git.Branch{{Name: "main", Commit: headMain}, {Name: "release/v2.15", Commit: head215}},
			Tags: []git.Tag{
				{Name: "v0.8.2", Commit: "1111"}, {Name: "v0.8.3", Commit: "2222"}, {Name: "v0.9.0", Commit: "3333"},
			},
		},
		upstream: "upstream",
	}
}

// fakeGit records what was done to the clone, and holds its local tags.
type fakeGit struct {
	heads     map[string]string // what fetching a branch gives, when it is not what the host said
	local     map[string]string // the local tags
	made      []string
	pushed    []string
	fetchErr  error
	createErr error
	pushErr   error
}

func (f *fakeGit) Fetch(_ context.Context, _, _, branch string) (string, error) {
	return f.heads[branch], f.fetchErr
}

func (f *fakeGit) TagCommit(_ context.Context, _, name string) (string, bool, error) {
	c, ok := f.local[name]
	return c, ok, nil
}

func (f *fakeGit) CreateTag(_ context.Context, dir, name, commit string) error {
	f.made = append(f.made, dir+" "+name+" "+commit)
	return f.createErr
}

func (f *fakeGit) PushTag(_ context.Context, dir, remote, name string) error {
	f.pushed = append(f.pushed, dir+" "+remote+" "+name)
	return f.pushErr
}

// tagging runs the command and keeps what it wrote and what it did to git.
type tagging struct {
	out bytes.Buffer
	git *fakeGit
}

// run fetches what the host has, unless the test says the branch moved.
func (g *tagging) run(ws fakeWorkspace, format string, dryRun bool, opts Options) error {
	if g.git == nil {
		g.git = &fakeGit{}
	}
	if g.git.heads == nil {
		g.git.heads = map[string]string{}
		for _, b := range ws.refs.Branches {
			g.git.heads[b.Name] = b.Commit
		}
	}
	return run(bg, &g.out, ws, g.git, format, dryRun, opts)
}

func plain() Options { return Options{Line: "v2.15", Tag: "v0.8.4-rc.1"} }

func pushing(answer string) Options {
	opts := plain()
	opts.Push, opts.Interactive, opts.In = true, true, strings.NewReader(answer)
	return opts
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args              []string
		project, line, tg string
	}{
		{[]string{"v2.15"}, "", "v2.15", ""},
		{[]string{"release/v2.15"}, "", "release/v2.15", ""},
		{[]string{"v2.15", "v0.8.4-rc.1"}, "", "v2.15", "v0.8.4-rc.1"},
		{[]string{"shell", "v2.15"}, "shell", "v2.15", ""},
		{[]string{"rancher/shell", "release/v2.15"}, "rancher/shell", "release/v2.15", ""},
		{[]string{"shell", "v2.15", "v0.8.4-rc.1"}, "shell", "v2.15", "v0.8.4-rc.1"},
		{[]string{"shell", "main", "v0.8.4"}, "shell", "main", "v0.8.4"},
		{[]string{"main", "0.8.4"}, "", "main", "0.8.4"},
	}
	for _, tt := range tests {
		project, line, tag := ParseArgs(tt.args)

		assert.Equal(t, [3]string{tt.project, tt.line, tt.tg}, [3]string{project, line, tag}, "%v", tt.args)
	}
}

func TestRunMakesTheTagInTheCloneAtTheHeadOfTheLinesBranchAndAsksNothing(t *testing.T) {
	var g tagging

	err := g.run(shell(), "text", false, plain())

	require.NoError(t, err)
	assert.Equal(t, []string{"/work v0.8.4-rc.1 " + head215}, g.git.made, "on release/v2.15's head, as the host has it")
	assert.Empty(t, g.git.pushed, "pushing is the engineer's, or --push")
	out := g.out.String()
	assert.Contains(t, out, "Tagged v0.8.4-rc.1 on release/v2.15 at 8c61398 in rancher/shell")
	assert.Contains(t, out, "git push upstream v0.8.4-rc.1", "and how to send it")
	assert.NotContains(t, out, "[y/N]", "a tag in the clone is undone with git tag -d, so nothing is asked")
}

func TestRunWithPushAsksOnceThenMakesTheTagAndPushesItToTheUpstream(t *testing.T) {
	var g tagging

	err := g.run(shell(), "text", false, pushing("y\n"))

	require.NoError(t, err)
	assert.Equal(t, []string{"/work v0.8.4-rc.1 " + head215}, g.git.made)
	assert.Equal(t, []string{"/work upstream v0.8.4-rc.1"}, g.git.pushed)
	out := g.out.String()
	assert.Contains(t, out, "Push the tag v0.8.4-rc.1 on release/v2.15 at 8c61398 in rancher/shell to upstream? [y/N]")
	assert.Contains(t, out, "Tagged and pushed v0.8.4-rc.1")
	assert.NotContains(t, out, "git push upstream", "it was pushed")
}

func TestRunWithPushDeclinedChangesNothing(t *testing.T) {
	var g tagging

	err := g.run(shell(), "text", false, pushing("n\n"))

	require.ErrorIs(t, err, ErrDeclined)
	assert.Empty(t, g.git.made, "asked before anything is made")
	assert.Empty(t, g.git.pushed)
}

func TestRunWithPushAndYesDoesNotAsk(t *testing.T) {
	var g tagging
	opts := pushing("")
	opts.Yes, opts.Interactive, opts.In = true, false, nil

	err := g.run(shell(), "text", false, opts)

	require.NoError(t, err)
	assert.Len(t, g.git.pushed, 1)
	assert.NotContains(t, g.out.String(), "[y/N]")
}

func TestRunWithPushAndNoTerminalOrYesIsRefusedBeforeAnyWrite(t *testing.T) {
	var g tagging
	opts := pushing("")
	opts.Interactive, opts.In = false, nil

	err := g.run(shell(), "text", false, opts)

	require.ErrorIs(t, err, ErrNeedsApproval)
	assert.Empty(t, g.git.made)
	assert.Empty(t, g.git.pushed)
}

func TestRunADryRunSaysWhatItWouldDoAndChangesNothing(t *testing.T) {
	var g tagging
	err := g.run(shell(), "text", true, plain())
	require.NoError(t, err)
	assert.Contains(t, g.out.String(), "Would tag v0.8.4-rc.1 on release/v2.15 at 8c61398 in rancher/shell")

	var p tagging
	opts := pushing("")
	opts.In = nil
	err = p.run(shell(), "text", true, opts)
	require.NoError(t, err)
	assert.Contains(t, p.out.String(), "and push it to upstream")
	assert.NotContains(t, p.out.String(), "[y/N]")

	for _, x := range []*tagging{&g, &p} {
		assert.Empty(t, x.git.made)
		assert.Empty(t, x.git.pushed)
	}
}

func TestRunATagTheHostHasOnTheCommitIsDoneAndNothingIsSent(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.1", Commit: head215})
	opts := pushing("")
	opts.Yes, opts.In = true, nil

	err := g.run(ws, "text", false, opts)

	require.NoError(t, err)
	assert.Empty(t, g.git.made)
	assert.Empty(t, g.git.pushed)
	assert.Contains(t, g.out.String(), "already tagged")
}

func TestRunATagThatIsOnlyInTheCloneIsNotMadeAgainAndCanBePushed(t *testing.T) {
	g := tagging{git: &fakeGit{local: map[string]string{"v0.8.4-rc.1": head215}}}
	err := g.run(shell(), "text", false, plain())
	require.NoError(t, err)
	assert.Empty(t, g.git.made)
	assert.Contains(t, g.out.String(), "already in your clone")
	assert.Contains(t, g.out.String(), "git push upstream v0.8.4-rc.1")

	p := tagging{git: &fakeGit{local: map[string]string{"v0.8.4-rc.1": head215}}}
	opts := pushing("y\n")
	err = p.run(shell(), "text", false, opts)
	require.NoError(t, err)
	assert.Empty(t, p.git.made)
	assert.Equal(t, []string{"/work upstream v0.8.4-rc.1"}, p.git.pushed)
	assert.Contains(t, p.out.String(), "Pushed v0.8.4-rc.1")
}

func TestRunATagThatIsInTheCloneOnAnotherCommitIsNeverMoved(t *testing.T) {
	g := tagging{git: &fakeGit{local: map[string]string{"v0.8.4-rc.1": "eeeeeeee"}}}

	err := g.run(shell(), "text", false, plain())

	require.ErrorIs(t, err, releasetag.ErrTagExists)
	assert.Empty(t, g.git.made)
}

func TestRunATagTheHostHasOnAnotherCommitIsNeverMoved(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.1", Commit: "ffffffff"})

	err := g.run(ws, "text", false, plain())

	require.ErrorIs(t, err, releasetag.ErrTagExists)
	assert.Empty(t, g.git.made)
}

func TestRunATagTheLineCannotHaveIsAnErrorBeforeAnyWriteOrQuestion(t *testing.T) {
	var g tagging
	opts := pushing("y\n")
	opts.Tag = "v0.9.1" // the series of v2.16

	err := g.run(shell(), "text", false, opts)

	require.ErrorIs(t, err, releasetag.ErrNotInSeries)
	assert.Empty(t, g.git.made)
	assert.NotContains(t, g.out.String(), "[y/N]")
}

func TestRunALineTheProjectDoesNotHaveListsTheOnesItDoes(t *testing.T) {
	var g tagging
	opts := plain()
	opts.Line = "v2.99"

	err := g.run(shell(), "text", false, opts)

	require.Error(t, err)
	assert.ErrorContains(t, err, "v2.99")
	assert.ErrorContains(t, err, "v2.16, v2.15")
	assert.Empty(t, g.git.made)
}

func TestRunTheLineMayBeWrittenWithoutTheVOrAsItsBranch(t *testing.T) {
	for _, line := range []string{"2.15", "V2.15", "release/v2.15"} {
		var g tagging
		opts := plain()
		opts.Line = line

		err := g.run(shell(), "text", false, opts)

		require.NoError(t, err, line)
		assert.Equal(t, []string{"/work v0.8.4-rc.1 " + head215}, g.git.made, line)
	}
}

func TestRunTheDevelopmentLineIsTheDefaultBranchWithItsOwnSeries(t *testing.T) {
	var g tagging
	opts := plain()
	opts.Line, opts.Tag = "v2.16", "v0.9.1-rc.1"

	err := g.run(shell(), "text", false, opts)

	require.NoError(t, err)
	assert.Equal(t, []string{"/work v0.9.1-rc.1 " + headMain}, g.git.made)
}

func TestRunAProjectWithNoLinesTagsABranchOfItsOwnWithNoSeries(t *testing.T) {
	var g tagging
	ws := shell()
	ws.project.Release = config.Release{Branching: config.BranchingSingle, TagPattern: "v*"}
	ws.project.Repo.Name = "rancher/scc-operator"
	ws.refs.Tags = []git.Tag{{Name: "v0.5.3", Commit: "1111"}}
	opts := plain()
	opts.Line, opts.Tag = "main", "v0.5.4-rc.1"

	err := g.run(ws, "text", false, opts)

	require.NoError(t, err)
	assert.Equal(t, []string{"/work v0.5.4-rc.1 " + headMain}, g.git.made)
}

func TestRunWithNoTagChoosesThePrereleaseThatFollowsTheNewestOfTheLine(t *testing.T) {
	var g tagging
	opts := plain()
	opts.Tag = ""

	err := g.run(shell(), "text", false, opts)

	require.NoError(t, err)
	assert.Equal(t, []string{"/work v0.8.4-rc.1 " + head215}, g.git.made)
}

func TestRunStablePromotesThePrereleaseOnTheHeadAndSaysSo(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.2", Commit: head215})
	opts := pushing("y\n")
	opts.Tag, opts.Stable = "", true

	err := g.run(ws, "text", false, opts)

	require.NoError(t, err)
	assert.Equal(t, []string{"/work v0.8.4 " + head215}, g.git.made)
	assert.Contains(t, g.out.String(), "Push the tag v0.8.4 on release/v2.15 at 8c61398, which is v0.8.4-rc.2 with no commits since, in rancher/shell to upstream?")
}

func TestRunStableThatPromotesSaysWhichPrereleaseInJSONAndInADryRun(t *testing.T) {
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.2", Commit: head215})
	opts := Options{Line: "v2.15", Stable: true}

	var j tagging
	require.NoError(t, j.run(ws, "json", false, opts))
	var res Result
	require.NoError(t, json.Unmarshal(j.out.Bytes(), &res), j.out.String())
	assert.Equal(t, "v0.8.4", res.Tag)
	assert.Equal(t, "v0.8.4-rc.2", res.Promotes)

	var d tagging
	require.NoError(t, d.run(ws, "text", true, opts))
	assert.Contains(t, d.out.String(), "Would tag v0.8.4 on release/v2.15 at 8c61398, which is v0.8.4-rc.2 with no commits since, in rancher/shell")
	assert.Empty(t, d.git.made)
}

func TestRunStableRefusesToPromoteWhenTheBranchHasMovedOn(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4-rc.2", Commit: "ccccccccdddd"})
	opts := plain()
	opts.Tag, opts.Stable = "", true

	err := g.run(ws, "text", false, opts)

	require.ErrorIs(t, err, releasetag.ErrMovedSincePrerelease)
	assert.Empty(t, g.git.made)
}

func TestRunAProjectThatRequiresAPrereleaseHasNoStableAfterAStable(t *testing.T) {
	var g tagging
	ws := shell()
	ws.project.Release.RequiresPrerelease = true
	opts := plain()
	opts.Tag, opts.Stable = "", true

	err := g.run(ws, "text", false, opts)

	require.ErrorIs(t, err, releasetag.ErrNeedsPrerelease)
	assert.Empty(t, g.git.made)
}

func TestRunANamedStableTagOfAProjectThatRequiresAPrereleaseNeedsOneOnTheHead(t *testing.T) {
	var g tagging
	ws := shell()
	ws.project.Release.RequiresPrerelease = true
	opts := plain()
	opts.Tag = "v0.8.4"

	err := g.run(ws, "text", false, opts)

	require.ErrorIs(t, err, releasetag.ErrNeedsPrerelease)
	assert.Empty(t, g.git.made)
}

func TestRunMinorAndMajorRaiseTheVersionAfterAStableTag(t *testing.T) {
	for bump, want := range map[Bump]string{BumpMinor: "v0.6.0-rc.1", BumpMajor: "v1.0.0-rc.1"} {
		var g tagging
		ws := shell()
		ws.project.Release = config.Release{Branching: config.BranchingSingle, TagPattern: "v*"}
		ws.refs.Tags = []git.Tag{{Name: "v0.5.3", Commit: "1111"}}

		err := g.run(ws, "text", false, Options{Line: "main", Bump: bump})

		require.NoError(t, err)
		assert.Equal(t, []string{"/work " + want + " " + headMain}, g.git.made)
	}
}

func TestRunWithNothingNewOnTheHeadChoosesNothing(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Tags = append(ws.refs.Tags, git.Tag{Name: "v0.8.4", Commit: head215})
	opts := plain()
	opts.Tag = ""

	err := g.run(ws, "text", false, opts)

	require.ErrorIs(t, err, releasetag.ErrNothingNew)
	assert.Empty(t, g.git.made)
}

func TestRunATagAndAWayToChooseOneAreNotBothAllowed(t *testing.T) {
	for _, opts := range []Options{
		{Line: "v2.15", Tag: "v0.8.4", Stable: true},
		{Line: "v2.15", Tag: "v0.8.4", Bump: BumpMinor},
	} {
		var g tagging

		err := g.run(shell(), "text", false, opts)

		require.ErrorIs(t, err, ErrTagAndChoice)
		assert.Empty(t, g.git.made)
	}
}

func TestRunABranchThatMovedWhileItWasReadIsAnErrorBeforeAnyWrite(t *testing.T) {
	g := tagging{git: &fakeGit{heads: map[string]string{"release/v2.15": "9999999999"}}}

	err := g.run(shell(), "text", false, plain())

	require.Error(t, err)
	assert.ErrorContains(t, err, "release/v2.15")
	assert.ErrorContains(t, err, "again")
	assert.Empty(t, g.git.made)
}

func TestRunABranchTheHostDoesNotHaveIsAnError(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refs.Branches = ws.refs.Branches[:1] // no release/v2.15

	err := g.run(ws, "text", false, plain())

	require.Error(t, err)
	assert.ErrorContains(t, err, "release/v2.15")
	assert.Empty(t, g.git.made)
}

func TestRunFailuresToReachTheHostOrTheCloneAreErrors(t *testing.T) {
	var g tagging
	ws := shell()
	ws.refsErr = errors.New("network down")
	require.ErrorContains(t, g.run(ws, "text", false, plain()), "network down")

	ws = shell()
	ws.pathErr = errors.New("not found")
	require.ErrorContains(t, tagging{}.run2(ws), "repos.search")

	ws = shell()
	ws.upstream = ""
	require.ErrorContains(t, tagging{}.run2(ws), "upstream")

	f := tagging{git: &fakeGit{fetchErr: errors.New("could not fetch")}}
	require.ErrorContains(t, f.run(shell(), "text", false, plain()), "could not fetch")
}

func (tagging) run2(ws fakeWorkspace) error {
	var g tagging
	return g.run(ws, "text", false, plain())
}

func TestRunAFailureToMakeTheTagIsAnErrorAndNothingIsPushed(t *testing.T) {
	g := tagging{git: &fakeGit{createErr: errors.New("fatal: bad object")}}
	opts := pushing("y\n")

	err := g.run(shell(), "text", false, opts)

	require.ErrorContains(t, err, "bad object")
	assert.Empty(t, g.git.pushed)
	assert.NotContains(t, g.out.String(), "Tagged")
}

func TestRunAFailureToPushSaysTheTagIsInTheCloneAndHowToTryAgain(t *testing.T) {
	g := tagging{git: &fakeGit{pushErr: errors.New("the remote rejected the push")}}

	err := g.run(shell(), "text", false, pushing("y\n"))

	require.Error(t, err)
	assert.ErrorContains(t, err, "rejected")
	assert.ErrorContains(t, err, "v0.8.4-rc.1 is in your clone")
	assert.Len(t, g.git.made, 1)
}

func TestRunJSONReportsWhatWasDone(t *testing.T) {
	var g tagging

	err := g.run(shell(), "json", false, plain())

	require.NoError(t, err)
	var res Result
	require.NoError(t, json.Unmarshal(g.out.Bytes(), &res), g.out.String())
	assert.Equal(t, Result{
		Project: "rancher/shell", Branch: "release/v2.15", Commit: head215, Tag: "v0.8.4-rc.1", Newest: "v0.8.3", Action: "tagged",
		Remote: "upstream",
	}, res)
}

func TestRunJSONWithPushNeverAsksSoItNeedsYes(t *testing.T) {
	var g tagging
	opts := pushing("y\n")

	err := g.run(shell(), "json", false, opts)

	require.ErrorIs(t, err, ErrNeedsApproval)
	assert.Empty(t, g.git.made)
}

func TestRunAnOutputFormatItDoesNotSupportIsAnErrorBeforeAnything(t *testing.T) {
	var g tagging

	err := g.run(shell(), "env", false, plain())

	require.Error(t, err)
	assert.Empty(t, g.git.made)
}
