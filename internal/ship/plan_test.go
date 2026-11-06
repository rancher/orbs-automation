package ship

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/git"
	"github.com/rancher/orbs-automation/internal/registry"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
	"github.com/rancher/orbs-automation/internal/workspace"
	"github.com/rancher/orbs-automation/internal/yamledit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// link is a route as the tests build it: the producer's edge, the consumer
// project and the consumer's method. fakeWorkspace turns the links into a config
// and the real update dispatcher engine.
type link struct {
	Consumer config.Project
	Method   config.UpdateMethod
	Edge     config.Edge
}

type fakeWorkspace struct {
	producer   config.Project
	links      []link
	projectErr error
	refs       map[string]git.Refs // by repository slug
	refsErr    map[string]error
	files      map[string]string // "slug@ref:path"
	asked      []string
	cloneDir   string // the engineer's clone, for Ship
	cloneErr   error
	push       PushSettings
}

// Project returns the producer with the edges of the links, as the loaded config
// has it.
func (f *fakeWorkspace) Project(key string) (config.Project, error) {
	f.asked = append(f.asked, "project "+key)
	producer := f.producer
	producer.Downstream = nil
	for _, l := range f.links {
		producer.Downstream = append(producer.Downstream, l.Edge)
	}
	return producer, f.projectErr
}

// Updates builds the real engine from the links, as the workspace does from the
// loaded config.
func (f *fakeWorkspace) Updates() (*updatedispatcher.Engine, error) {
	producer := f.producer
	producer.Downstream = nil
	projects := []config.Project{producer}
	for _, l := range f.links {
		producer.Downstream = append(producer.Downstream, l.Edge)
		consumer := l.Consumer
		consumer.UpdateMethods = []config.UpdateMethod{l.Method}
		projects = append(projects, consumer)
	}
	projects[0] = producer
	return updatedispatcher.NewEngine(updatedispatcher.Builtin(), config.Config{Projects: projects})
}

// CloneDir and Remote answer from the fields, for the tests that make commits.
func (f *fakeWorkspace) CloneDir(string) (string, error) { return f.cloneDir, f.cloneErr }

func (f *fakeWorkspace) Remote(string) (string, error) { return "upstream", nil }

// PushSettings answers from the field.
func (f *fakeWorkspace) PushSettings(string) PushSettings { return f.push }

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

// riLink is the route from scc-operator to rancher's build-yaml-bump method, as
// the config writes it: the consumer fixes the file, offers updateKey and value,
// and the producer says which key and that the value is an image reference.
func riLink(strategy string) link {
	return link{
		Consumer: config.Project{
			Name: "rancher", Repo: config.Repo{Name: "rancher/rancher", DefaultBranch: "main"},
			Release: config.Release{Branches: "perLine", Lines: []config.ProjectLine{{Line: "v2.15", Branch: "release/v2.15"}, {Line: "v2.14", Branch: "release/v2.14"}}},
		},
		Method: config.UpdateMethod{
			Name: "build-yaml-bump", Strategy: strategy,
			With:          map[string]string{"file": "build.yaml"},
			AllowedParams: []config.AllowedParam{{Key: "updateKey"}, {Key: "value", Default: "{tag}"}},
			Accept:        config.Accept{When: config.AcceptStable},
		},
		Edge: config.Edge{
			To: "rancher", Method: "build-yaml-bump", // lines omitted: derived from the producer
			Params: map[string]string{"updateKey": "defaultSccOperatorImage", "value": "{image}:{tag}"},
		},
	}
}

func fixture() *fakeWorkspace {
	return &fakeWorkspace{
		producer: universal(),
		links:    []link{riLink("yamlBump")},
		refs: map[string]git.Refs{
			"rancher/scc-operator": {Tags: tags("v0.5.2", "v0.5.3", "v0.6.0-rc.1")},
			"rancher/rancher": {Branches: []git.Branch{
				{Name: "main", Commit: "sha-main"}, {Name: "release/v2.15", Commit: "sha-215"},
			}},
		},
		files: map[string]string{
			"rancher/rancher@sha-215:build.yaml":  "defaultSccOperatorImage: rancher/scc-operator:v0.5.2\n",
			"rancher/rancher@sha-main:build.yaml": "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n",
		},
	}
}

var ctx = context.Background()

func TestCheckReady(t *testing.T) {
	ws, p := fixture(), &fakeProber{}

	got, err := Check(ctx, ws, p, Input{Project: "scc-operator", Lines: []string{"v2.15"}})

	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator", got.Producer)
	assert.Equal(t, []Release{{Tag: "v0.5.3", Commit: "sha-v0.5.3", Stable: true}}, got.Releases)
	assert.Equal(t, "rancher/rancher", got.Consumer)
	assert.Equal(t, "build-yaml-bump", got.Method)
	assert.Equal(t, "yamlBump", got.Strategy)
	assert.Equal(t, Target{Branch: "release/v2.15", Commit: "sha-215"}, got.Targets[0].Target)
	assert.Equal(t, "build.yaml", got.File)
	assert.Equal(t, "rancher/scc-operator:v0.5.2", got.Targets[0].Current)
	assert.Equal(t, "rancher/scc-operator:v0.5.3", got.Want)
	assert.Equal(t, StatusReady, got.Status)
	assert.Empty(t, got.Blockers)
	require.Len(t, got.Images, 1)
	assert.Equal(t, "v0.5.3", got.Images[0].Release)
	assert.False(t, got.Images[0].Failed())
	assert.Equal(t, "scc-operator", ws.asked[0][len("project "):])
}

func TestCheckReadsTheFileAtTheCommitItListed(t *testing.T) {
	ws := fixture()
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Contains(t, ws.asked, "file rancher/rancher@sha-215:build.yaml", "the head commit, so the read is exact")
}

func TestCheckAlreadyUpToDate(t *testing.T) {
	p := &fakeProber{}
	got, err := Check(ctx, withDevLine(fixture()), p, Input{Project: "scc-operator", Lines: []string{"v2.16"}})
	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, got.Status)
	assert.Equal(t, got.Targets[0].Current, got.Want)
	assert.Empty(t, got.Images, "nothing else runs")
	assert.Zero(t, p.calls)
}

func TestCheckBlockedByAMissingImage(t *testing.T) {
	p := &fakeProber{results: map[string]registry.Result{
		"registry.rancher.com rancher/scc-operator:v0.5.3": {Status: registry.NotFound},
	}}
	got, err := Check(ctx, fixture(), p, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Equal(t, StatusBlocked, got.Status)
	require.Len(t, got.Blockers, 1)
	assert.Contains(t, got.Blockers[0], "images")
	require.Len(t, got.Images, 1)
	assert.True(t, got.Images[0].Failed())
}

func TestCheckAnExplicitTagAndALine(t *testing.T) {
	got, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.5.2", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	require.Len(t, got.Releases, 1)
	assert.Equal(t, "v0.5.2", got.Releases[0].Tag)
	assert.Equal(t, StatusUpToDate, got.Status, "the consumer already holds v0.5.2")
}

func TestCheckARefusedPrerelease(t *testing.T) {
	_, err := Check(ctx, withDevLine(fixture()), &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.6.0-rc.1", Lines: []string{"v2.16"}})
	assert.ErrorIs(t, err, ErrPrerelease)

	got, err := Check(ctx, withDevLine(fixture()), &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.6.0-rc.1", Lines: []string{"v2.16"}, Prerelease: true})
	require.NoError(t, err)
	require.Len(t, got.Releases, 1)
	assert.Equal(t, "v0.6.0-rc.1", got.Releases[0].Tag)
	assert.False(t, got.Releases[0].Stable)
}

func TestCheckABranchTheConsumerDoesNotHave(t *testing.T) {
	_, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.14"}})
	assert.ErrorIs(t, err, ErrBranchNotFound)
	assert.ErrorContains(t, err, "release/v2.14")
}

func TestCheckConsumers(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		ws := withDevLine(fixture())
		ws.links = nil
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.16"}})
		assert.ErrorIs(t, err, ErrNoConsumer)
	})
	t.Run("several and none chosen", func(t *testing.T) {
		ws := withDevLine(fixture())
		other := riLink("yamlBump")
		other.Consumer.Name, other.Consumer.Repo.Name, other.Edge.To = "charts", "rancher/charts", "charts"
		ws.links = append(ws.links, other)
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.16"}})
		assert.ErrorIs(t, err, ErrAmbiguousConsumer)
		assert.ErrorContains(t, err, "charts")
		assert.ErrorContains(t, err, "rancher")
	})
	t.Run("chosen by name or slug", func(t *testing.T) {
		for _, to := range []string{"charts", "rancher/charts", "Rancher/Charts"} {
			ws := withDevLine(fixture())
			other := riLink("yamlBump")
			other.Consumer.Name, other.Consumer.Repo.Name, other.Edge.To = "charts", "rancher/charts", "charts"
			other.Consumer.Release.Lines = ws.links[0].Consumer.Release.Lines
			ws.links = append(ws.links, other)
			ws.refs["rancher/charts"] = ws.refs["rancher/rancher"]
			ws.files["rancher/charts@sha-main:build.yaml"] = "defaultSccOperatorImage: x\n"
			got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", To: to, Lines: []string{"v2.16"}})
			require.NoError(t, err, to)
			assert.Equal(t, "rancher/charts", got.Consumer)
		}
	})
	t.Run("a --to that is not a consumer", func(t *testing.T) {
		_, err := Check(ctx, withDevLine(fixture()), &fakeProber{}, Input{Project: "scc-operator", To: "nope", Lines: []string{"v2.16"}})
		assert.ErrorIs(t, err, ErrNoConsumer)
	})
}

func TestCheckUnknownStrategy(t *testing.T) {
	ws := withDevLine(fixture())
	ws.links = []link{riLink("chartsPackageBump")}
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.16"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chartsPackageBump", "the engine says which strategy is not known")
}

func TestCheckReadFailures(t *testing.T) {
	t.Run("the file is missing", func(t *testing.T) {
		ws := fixture()
		delete(ws.files, "rancher/rancher@sha-215:build.yaml")
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
		require.Error(t, err)
		assert.ErrorContains(t, err, "build.yaml")
		assert.ErrorContains(t, err, "release/v2.15")
	})
	t.Run("the key is missing", func(t *testing.T) {
		ws := fixture()
		ws.files["rancher/rancher@sha-215:build.yaml"] = "other: 1\n"
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
		assert.ErrorIs(t, err, yamledit.ErrKeyNotFound)
		assert.ErrorContains(t, err, "defaultSccOperatorImage")
	})
}

func TestCheckHostErrors(t *testing.T) {
	boom := errors.New("network down")
	for slug := range map[string]bool{"rancher/scc-operator": true, "rancher/rancher": true} {
		ws := withDevLine(fixture())
		ws.refsErr = map[string]error{slug: boom}
		_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.16"}})
		assert.ErrorIs(t, err, boom, slug)
	}
}

func TestCheckProducerErrors(t *testing.T) {
	ws := withDevLine(fixture())
	ws.projectErr = workspace.ErrUnknownProject
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "nope", Lines: []string{"v2.16"}})
	assert.ErrorIs(t, err, workspace.ErrUnknownProject)
}

func TestCheckValueVariables(t *testing.T) {
	ws := fixture()
	c := riLink("yamlBump")
	c.Edge.Params = map[string]string{
		"updateKey": "defaultSccOperatorImage", "value": "{sourceRepo}@{version}",
	}
	ws.links = []link{c}
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Equal(t, "rancher/scc-operator@0.5.3", got.Want)
}

func TestCheckPreviewsWhatARunWouldWrite(t *testing.T) {
	got, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	require.NotNil(t, got.Targets[0].Preview)
	assert.Equal(t, "bot/scc-operator-v0.5.3-v2.15", got.Targets[0].Preview.Branch)
	assert.Equal(t, "[v2.15] Update scc-operator to v0.5.3", got.Targets[0].Preview.Title)
	assert.Contains(t, got.Targets[0].Preview.Body, "rancher/scc-operator:v0.5.2")
	assert.Contains(t, got.Targets[0].Preview.CommitMessage, "Automation: gero ship")
}

func TestCheckPreviewUsesTheUsersBranchPrefix(t *testing.T) {
	got, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}, BranchPrefix: "dan/"})
	require.NoError(t, err)
	assert.Equal(t, "dan/v0.5.3-v2.15", got.Targets[0].Preview.Branch)
}

func TestCheckHasNoPreviewWhenAlreadyUpToDate(t *testing.T) {
	got, err := Check(ctx, withDevLine(fixture()), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.16"}})
	require.NoError(t, err)
	assert.Nil(t, got.Targets[0].Preview)
}

func TestCheckPreviewIsThereWhenBlocked(t *testing.T) {
	p := &fakeProber{results: map[string]registry.Result{"registry.rancher.com rancher/scc-operator:v0.5.3": {Status: registry.NotFound}}}
	got, err := Check(ctx, fixture(), p, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.Equal(t, StatusBlocked, got.Status)
	assert.NotNil(t, got.Targets[0].Preview, "so the reader sees what would have been written")
}

func TestCheckABadPlaceholderIsAnError(t *testing.T) {
	ws := fixture()
	c := riLink("yamlBump")
	c.Consumer.PullRequests.Title = "Update {nope}"
	ws.links = []link{c}
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	assert.ErrorContains(t, err, "nope")
}

func TestCheckABlockedDowngrade(t *testing.T) {
	// main holds v0.5.3; shipping the older v0.5.2 is a downgrade.
	got, err := Check(ctx, withDevLine(fixture()), &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.5.2", Lines: []string{"v2.16"}})

	require.NoError(t, err)
	assert.True(t, got.Targets[0].Downgrade)
	assert.Equal(t, StatusBlocked, got.Status)
	require.NotEmpty(t, got.Targets[0].Blockers)
	assert.Contains(t, got.Targets[0].Blockers[0], "downgrade")
	assert.NotContains(t, got.Targets[0].Blockers[0], "0.5.3", "the values are in the table, so the text is the same for every branch and can be grouped")
	assert.Contains(t, got.Targets[0].Blockers[0], "--allow-downgrade")
	assert.Empty(t, got.Images, "a blocked downgrade stops here, as an up to date consumer does")
	assert.Nil(t, got.Targets[0].Preview)
}

func TestCheckAllowedDowngradeIsAWarning(t *testing.T) {
	got, err := Check(ctx, withDevLine(fixture()), &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.5.2", Lines: []string{"v2.16"}, AllowDowngrade: true})
	require.NoError(t, err)
	assert.True(t, got.Targets[0].Downgrade, "it is still a downgrade")
	assert.Equal(t, StatusReady, got.Status)
	assert.Empty(t, got.Targets[0].Blockers)
	require.NotEmpty(t, got.Targets[0].Warnings)
	assert.Contains(t, got.Targets[0].Warnings[0], "downgrade")
	assert.NotEmpty(t, got.Images, "once it is allowed the rest of the check runs")
	assert.NotNil(t, got.Targets[0].Preview)
}

func TestCheckNewerIsNotADowngrade(t *testing.T) {
	got, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.False(t, got.Targets[0].Downgrade, "v0.5.2 to v0.5.3")
}

func TestCheckAlreadyUpToDateIsNotADowngrade(t *testing.T) {
	got, err := Check(ctx, withDevLine(fixture()), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.16"}})
	require.NoError(t, err)
	assert.False(t, got.Targets[0].Downgrade)
	assert.Equal(t, StatusUpToDate, got.Status)
}

func TestCheckAPrereleaseOfTheSameVersionIsADowngrade(t *testing.T) {
	// main holds v0.5.3; v0.5.3-rc.1 is lower, though it has the same numbers.
	ws := withDevLine(fixture())
	ws.refs["rancher/scc-operator"] = git.Refs{Tags: tags("v0.5.3", "v0.5.3-rc.1")}
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Arg: "v0.5.3-rc.1", Lines: []string{"v2.16"}, Prerelease: true})
	require.NoError(t, err)
	assert.True(t, got.Targets[0].Downgrade)
}

func TestCheckFromAPrereleaseToTheStableReleaseIsNotADowngrade(t *testing.T) {
	ws := fixture()
	ws.files["rancher/rancher@sha-215:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3-rc.1\n"
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.False(t, got.Targets[0].Downgrade)
	assert.Equal(t, StatusReady, got.Status)
}

func TestCheckAValueWithNoVersionIsNotJudged(t *testing.T) {
	ws := fixture()
	ws.files["rancher/rancher@sha-215:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:latest\n"
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	assert.False(t, got.Targets[0].Downgrade, "there is nothing to compare")
	assert.Equal(t, StatusReady, got.Status)
}

// loopFixture is rancher with two release lines, each with a branch and a file,
// taking updates from scc-operator, which is universal.
func loopFixture() *fakeWorkspace {
	ws := fixture()
	ws.refs["rancher/rancher"] = git.Refs{Branches: []git.Branch{
		{Name: "main", Commit: "sha-main"}, {Name: "release/v2.15", Commit: "sha-215"}, {Name: "release/v2.14", Commit: "sha-214"},
	}}
	ws.files["rancher/rancher@sha-214:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.1\n"
	return ws
}

func branchesOf(p Plan) []string {
	out := make([]string, len(p.Targets))
	for i, t := range p.Targets {
		out[i] = t.Branch
	}
	return out
}

func TestCheckWithNoBranchChecksEveryBranchAUniversalReleaseGoesTo(t *testing.T) {
	got, err := Check(ctx, loopFixture(), &fakeProber{}, Input{Project: "scc-operator"})

	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.15", "release/v2.14"}, branchesOf(got), "the consumer's release branches, in config order")
	assert.Equal(t, "rancher/scc-operator:v0.5.2", got.Targets[0].Current)
	assert.Equal(t, "rancher/scc-operator:v0.5.1", got.Targets[1].Current)
	assert.Equal(t, StatusReady, got.Status)
}

func TestCheckChecksTheImagesOnceForAllTheTargets(t *testing.T) {
	p := &fakeProber{}
	_, err := Check(ctx, loopFixture(), p, Input{Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, 3, p.calls, "one check per registry for the release, not one per branch")
}

func TestCheckNoImagesWhenEveryTargetIsUpToDate(t *testing.T) {
	ws := loopFixture()
	ws.files["rancher/rancher@sha-215:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n"
	ws.files["rancher/rancher@sha-214:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n"
	p := &fakeProber{}
	got, err := Check(ctx, ws, p, Input{Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, got.Status)
	assert.Empty(t, got.Images)
	assert.Zero(t, p.calls)
}

func TestCheckAMixOfUpToDateAndReady(t *testing.T) {
	ws := loopFixture()
	ws.files["rancher/rancher@sha-215:build.yaml"] = "defaultSccOperatorImage: rancher/scc-operator:v0.5.3\n"
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, StatusUpToDate, got.Targets[0].Status)
	assert.Equal(t, StatusReady, got.Targets[1].Status)
	assert.Equal(t, StatusReady, got.Status)
	assert.NotEmpty(t, got.Images, "one of them needs a PR")
	assert.Nil(t, got.Targets[0].Preview)
	assert.NotNil(t, got.Targets[1].Preview)
}

func TestCheckABranchThatCannotBeReadFailsOnlyItself(t *testing.T) {
	ws := loopFixture()
	delete(ws.files, "rancher/rancher@sha-214:build.yaml")
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator"})

	require.NoError(t, err, "one branch does not stop the others")
	assert.Equal(t, StatusReady, got.Targets[0].Status)
	assert.Equal(t, StatusFailed, got.Targets[1].Status)
	assert.Contains(t, got.Targets[1].Error, "build.yaml")
	assert.Equal(t, "release/v2.14", got.Targets[1].Branch)
	assert.Equal(t, StatusBlocked, got.Status, "a failed branch blocks the whole")
}

func TestCheckABranchTheHostLacksFailsOnlyItself(t *testing.T) {
	ws := loopFixture()
	ws.refs["rancher/rancher"] = git.Refs{Branches: []git.Branch{{Name: "release/v2.15", Commit: "sha-215"}}}
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, StatusReady, got.Targets[0].Status)
	assert.Equal(t, StatusFailed, got.Targets[1].Status)
	assert.Contains(t, got.Targets[1].Error, "release/v2.14")
}

func TestCheckABranchFlagChecksOnlyThatBranch(t *testing.T) {
	got, err := Check(ctx, loopFixture(), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.14"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.14"}, branchesOf(got))
}

func TestCheckABranchFlagOutsideTheReleaseBranchesIsStillAllowed(t *testing.T) {
	got, err := Check(ctx, withDevLine(loopFixture()), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.16"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, branchesOf(got))
}

func TestCheckANamedBranchThatCannotBeReadIsAnError(t *testing.T) {
	ws := loopFixture()
	delete(ws.files, "rancher/rancher@sha-214:build.yaml")
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.14"}})
	require.Error(t, err, "asked for that one branch, so failing to read it is the answer")
	assert.ErrorContains(t, err, "build.yaml")
}

// A producer with lines of its own feeds the consumer's branch for the line of
// the release.
func linedLoop(t *testing.T) *fakeWorkspace {
	t.Helper()
	ws := loopFixture()
	producer := config.Project{
		Name: "kuberlr", Repo: config.Repo{Name: "rancher/kuberlr", DefaultBranch: "main"},
		Release: config.Release{TagPattern: "v*", Lines: []config.ProjectLine{
			{Line: "v2.16", Version: "9.x"}, {Line: "v2.15", Version: "8.x"},
		}},
	}
	c := riLink("yamlBump")
	c.Consumer.Release.Lines = []config.ProjectLine{
		{Line: "v2.16"}, // development: the default branch
		{Line: "v2.15", Branch: "release/v2.15"},
		{Line: "v2.14", Branch: "release/v2.14"},
	}
	ws.producer, ws.links = producer, []link{c}
	ws.refs["rancher/kuberlr"] = git.Refs{Tags: tags("v8.1.0", "v9.0.0", "v7.0.0")}
	ws.files["rancher/rancher@sha-main:build.yaml"] = "defaultSccOperatorImage: rancher/kuberlr:v9.0.0\n"
	return ws
}

func TestCheckALinedProducerFeedsTheConsumersBranchForTheLine(t *testing.T) {
	for tag, want := range map[string]string{"v8.1.0": "release/v2.15", "v9.0.0": "main"} {
		ws := linedLoop(t)
		got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "kuberlr", Arg: tag})
		require.NoError(t, err, tag)
		assert.Equal(t, []string{want}, branchesOf(got), tag)
	}
}

func TestCheckATagInNoLineOfTheProducerHasNoTarget(t *testing.T) {
	ws := linedLoop(t)
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "kuberlr", Arg: "v7.0.0"})
	assert.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "--line")
}

func TestCheckAConsumerWithoutTheLineHasNoTarget(t *testing.T) {
	ws := linedLoop(t)
	c := ws.links[0]
	c.Consumer.Release.Lines = []config.ProjectLine{{Line: "v2.14", Branch: "release/v2.14"}}
	ws.links[0] = c
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "kuberlr", Arg: "v8.1.0"})
	assert.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "v2.15")
}

func TestCheckAnExplicitUniversalAcceptFeedsEveryBranchWhateverTheProducerHas(t *testing.T) {
	ws := linedLoop(t)
	c := ws.links[0]
	c.Edge.Lines = config.AcceptLinesUniversal
	ws.links[0] = c
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "kuberlr", Arg: "v8.1.0"})
	require.NoError(t, err)
	assert.Equal(t, []string{"main", "release/v2.15", "release/v2.14"}, branchesOf(got))
}

func TestCheckAConsumerWithNoReleaseBranchesHasNoTarget(t *testing.T) {
	ws := loopFixture()
	c := ws.links[0]
	c.Consumer.Release = config.Release{Branches: config.BranchesPerLine}
	ws.links[0] = c
	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator"})
	assert.ErrorIs(t, err, ErrNoTargets)
}

func TestCheckASingleBranchConsumerTakesEveryReleaseOnItsDefaultBranch(t *testing.T) {
	ws := loopFixture()
	c := ws.links[0]
	c.Consumer.Release = config.Release{Branches: config.BranchesSingle}
	ws.links[0] = c
	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator"})
	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, branchesOf(got))
}

func TestCheckCarriesADryRunIntoThePlan(t *testing.T) {
	got, err := Check(ctx, loopFixture(), &fakeProber{}, Input{Project: "scc-operator", DryRun: true})
	require.NoError(t, err)
	assert.True(t, got.DryRun)

	got, err = Check(ctx, loopFixture(), &fakeProber{}, Input{Project: "scc-operator"})
	require.NoError(t, err)
	assert.False(t, got.DryRun)
}

func TestCheckADryRunChecksTheSameThings(t *testing.T) {
	// A dry run is the same check: it previews and checks the images, so it shows
	// what a real run would do.
	got, err := Check(ctx, loopFixture(), &fakeProber{}, Input{Project: "scc-operator", DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, StatusReady, got.Status)
	assert.NotEmpty(t, got.Images)
	assert.NotNil(t, got.Targets[0].Preview)
}

func TestCheckNamesEachTargetByItsRancherLine(t *testing.T) {
	ws := fixture()

	onALine, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})
	require.NoError(t, err)
	single := fixture()
	single.links[0].Consumer.Release = config.Release{Branches: config.BranchesSingle}
	inNoLine, err := Check(ctx, single, &fakeProber{}, Input{Project: "scc-operator"})
	require.NoError(t, err)
	require.Len(t, inNoLine.Targets, 1)

	assert.Equal(t, "v2.15", onALine.Targets[0].Line, "a line means the same in every project, which a branch does not")
	assert.Empty(t, inNoLine.Targets[0].Line, "a branch that is in no line has none, and is named by its branch")
}

func TestCheckALineIsTheBranchTheConsumerKeepsItOn(t *testing.T) {
	for _, line := range []string{"v2.15", "2.15"} {
		got, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{line}})
		require.NoError(t, err, line)
		assert.Equal(t, []string{"release/v2.15"}, branchesOf(got), line)
	}
}

func TestCheckALineWithNoBranchOfItsOwnIsTheDefaultBranch(t *testing.T) {
	ws := fixture()
	ws.links[0].Consumer.Release.Lines = append(ws.links[0].Consumer.Release.Lines, config.ProjectLine{Line: "v2.16"})

	got, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.16"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"main"}, branchesOf(got))
	assert.Equal(t, "v2.16", got.Targets[0].Line)
}

func TestCheckALineTheConsumerDoesNotHaveSaysWhichItDoes(t *testing.T) {
	_, err := Check(ctx, fixture(), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.99"}})

	require.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "v2.99")
	assert.ErrorContains(t, err, "v2.15, v2.14")
}

func TestCheckALineOfAConsumerWithoutLinesIsAnError(t *testing.T) {
	ws := fixture()
	ws.links[0].Consumer.Release.Lines = nil

	_, err := Check(ctx, ws, &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.15"}})

	require.ErrorIs(t, err, ErrNoTargets)
	assert.ErrorContains(t, err, "no release lines")
}

// withDevLine gives the consumer the line in development, v2.16, which has no
// branch of its own and so lives on the default branch.
func withDevLine(ws *fakeWorkspace) *fakeWorkspace {
	for i := range ws.links {
		c := &ws.links[i].Consumer.Release
		c.Lines = append(slices.Clone(c.Lines), config.ProjectLine{Line: "v2.16"})
	}
	return ws
}

func TestCheckSeveralLinesAreEachATargetOnceInTheOrderGiven(t *testing.T) {
	got, err := Check(ctx, loopFixture(), &fakeProber{}, Input{Project: "scc-operator", Lines: []string{"v2.14", "2.15", "v2.14"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"release/v2.14", "release/v2.15"}, branchesOf(got))
}
