package ship

import (
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/registry"
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withDestination() domain.Plan {
	p := previewPlan()
	p.Destination = []domain.Destination{{
		Project: "rancher/rancher", Line: "v2.15", Stream: "wrangler-v3-7", Branch: "release/v2.15",
		Held: []domain.HeldValue{{Name: "rancher-holds-wrangler-3-7", Current: "v3.7.1"}},
	}}
	return p
}

func TestReplayHasAnEventForTheDestinationAfterTheTargets(t *testing.T) {
	events := replay(withDestination())

	assert.Equal(t, []domain.EventKind{
		domain.EventResolved, domain.EventTargetChecked, domain.EventTargetsChecked, domain.EventDestination, domain.EventDone,
	}, kinds(events))
	assert.Empty(t, events[0].Plan.Destination, "the header only, as a run emits it")
	require.NotNil(t, events[3].Destination)
	assert.Equal(t, "v2.15", events[3].Destination.Line)
	assert.Len(t, events[4].Plan.Destination, 1, "the whole plan at the end")
}

func TestRenderTextShowsWhatTheDestinationHolds(t *testing.T) {
	out := renderText(withDestination(), false)

	assert.Contains(t, out, "Destination")
	assert.Contains(t, out, "v2.15: rancher/rancher holds v3.7.1 through wrangler-v3-7, on release/v2.15")
}

func TestRenderTextShowsADestinationThatCouldNotBeRead(t *testing.T) {
	p := withDestination()
	p.Destination[0].Held, p.Destination[0].Error = nil, "no such file"

	out := renderText(p, false)

	assert.Contains(t, out, "v2.15: rancher/rancher could not be read: no such file")
}

func TestRenderTextHasNoDestinationSectionWithoutOne(t *testing.T) {
	assert.NotContains(t, renderText(previewPlan(), false), "Destination")
}

func withCompanion() domain.Plan {
	p := previewPlan()
	p.Companions = []domain.Companion{{
		Producer: "rancher/lasso", Module: "github.com/rancher/lasso", Tag: "v0.2.5", Version: "v0.2.5",
		RequiredBy: "rancher/wrangler v3.7.4", File: "go.mod",
	}}
	p.Images = []domain.ReleaseImages{{Release: "v0.5.3"}, {Producer: "rancher/lasso", Release: "v0.2.5"}}
	return p
}

func TestRenderTextSaysWhichReleasesGoWithTheOneAskedFor(t *testing.T) {
	out := renderText(withCompanion(), false)

	assert.Contains(t, out, "with  github.com/rancher/lasso v0.2.5 of rancher/lasso, which rancher/wrangler v3.7.4 requires")
}

func TestRenderTextSaysAPinThatCouldNotBeResolved(t *testing.T) {
	p := withCompanion()
	p.Companions[0].Tag, p.Companions[0].Error = "", "rancher/lasso has no tag v0.2.9"

	out := renderText(p, false)

	assert.Contains(t, out, "github.com/rancher/lasso: rancher/lasso has no tag v0.2.9")
}

func TestReplayCarriesTheProducerOfACompanionsImages(t *testing.T) {
	var producers []string
	for _, e := range replay(withCompanion()) {
		if e.Kind == domain.EventImagesChecked {
			producers = append(producers, e.Producer)
		}
	}

	assert.Equal(t, []string{"", "rancher/lasso"}, producers)
}

func TestRenderTextPutsACompanionsArtifactsInTheTableOfTheRelease(t *testing.T) {
	p := withCompanion()
	p.Images = []domain.ReleaseImages{
		{Release: "v0.5.3", Report: registry.Report{Rows: []registry.Row{{Image: "rancher/scc-operator", Label: "Docker Hub", Status: "found", Reference: "rancher/scc-operator:v0.5.3"}}}},
		{Producer: "rancher/lasso", Release: "v0.2.5", Report: registry.Report{Rows: []registry.Row{{Image: "github.com/rancher/lasso", Label: "Go module proxy", Status: "found", Reference: "github.com/rancher/lasso@v0.2.5"}}}},
	}

	out := renderText(p, false)

	assert.Equal(t, 1, strings.Count(out, "Artifacts"), "one table, and no second heading for the companion")
	assert.NotContains(t, out, "Images")
	assert.Contains(t, out, "ARTIFACT")
	assert.NotContains(t, out, "IMAGE ")
	scc, lasso := strings.Index(out, "rancher/scc-operator:v0.5.3"), strings.Index(out, "github.com/rancher/lasso@v0.2.5")
	assert.Greater(t, scc, 0)
	assert.Greater(t, lasso, scc, "the companion's rows follow the release's, in the same table")
}

func TestAnArtifactTableThatIsNotACompanionsStartsItsOwn(t *testing.T) {
	p := withCompanion()
	p.Releases = []domain.Release{{Tag: "v0.9.1"}, {Tag: "v0.8.3"}}
	row := func(image string) registry.Report {
		return registry.Report{Rows: []registry.Row{{Image: image, Label: "Docker Hub", Status: "found"}}}
	}
	p.Images = []domain.ReleaseImages{
		{Release: "v0.9.1", Report: row("a/new")},
		{Producer: "rancher/lasso", Release: "v0.2.5", Report: row("github.com/rancher/lasso")},
		{Release: "v0.8.3", Report: row("a/old")},
	}

	out := renderText(p, false)

	assert.Contains(t, out, "Artifacts (v0.9.1)")
	assert.Contains(t, out, "Artifacts (v0.8.3)")
	assert.Equal(t, 2, strings.Count(out, "Artifacts ("), "the companion joins the first release's table")
}

// wranglerAndLasso is a plan that ships wrangler into a consumer that holds it, and
// the lasso that goes with it, which the consumer holds already.
func wranglerAndLasso() domain.Plan {
	p := previewPlan()
	p.Producer = "rancher/wrangler"
	p.Targets[0].Line = "v2.15"
	p.Targets[0].Holdings = []domain.HoldingPlan{
		{Name: "bro-holds-wrangler-3-8", Project: "wrangler", File: "go.mod", Current: "v3.7.2", Want: "v3.8.0"},
		{Name: "bro-holds-wrangler-3-8+lasso-v0-2-module", Project: "lasso", Pinned: true, Release: "v0.2.10", File: "go.mod", Current: "v0.2.10", Want: "v0.2.10", UpToDate: true},
	}
	return p
}

func TestRenderTextHasARowForEachArtifactOfATarget(t *testing.T) {
	out := renderText(wranglerAndLasso(), false)

	assert.Regexp(t, `(?m)^  LINE\s+BRANCH\s+HEAD\s+ARTIFACT\s+NOW\s+STATUS$`, out)
	assert.Regexp(t, `(?m)^  v2\.15\s+release/v2\.15\s+2222222\s+wrangler\s+v3\.7\.2\s+ready$`, out)
	assert.Regexp(t, `(?m)^  v2\.15\s+release/v2\.15\s+2222222\s+lasso\s+v0\.2\.10\s+already up to date$`, out, "lasso is read beside wrangler, and is held already")
}

func TestRenderTextARowForACompanionSaysItIsReadyWhenItIsToBeRaised(t *testing.T) {
	p := wranglerAndLasso()
	p.Targets[0].Holdings[1].Current, p.Targets[0].Holdings[1].UpToDate = "v0.2.3", false

	out := renderText(p, false)

	assert.Regexp(t, `(?m)^  v2\.15\s+release/v2\.15\s+2222222\s+lasso\s+v0\.2\.3\s+ready$`, out)
}

func TestRenderTextARowOfATargetThatIsNotReadyTakesTheTargetsStatus(t *testing.T) {
	p := wranglerAndLasso()
	p.Targets[0].Status = domain.StatusBlocked

	out := renderText(p, false)

	assert.Regexp(t, `(?m)^  v2\.15\s+release/v2\.15\s+2222222\s+lasso\s+v0\.2\.10\s+blocked$`, out)
}

func TestRenderTextTheTableOfSeveralReleasesNamesTheReleaseOfEachArtifact(t *testing.T) {
	p := wranglerAndLasso()
	p.Releases = []domain.Release{{Tag: "v3.8.0"}, {Tag: "v3.7.4"}}
	p.Targets[0].Release = "v3.8.0"

	out := renderText(p, false)

	assert.Regexp(t, `(?m)^  LINE\s+BRANCH\s+RELEASE\s+HEAD\s+ARTIFACT\s+NOW\s+WANT\s+STATUS$`, out)
	assert.Regexp(t, `(?m)^  v2\.15\s+release/v2\.15\s+v3\.8\.0\s+2222222\s+wrangler\s+v3\.7\.2\s+v3\.8\.0\s+ready$`, out)
	assert.Regexp(t, `(?m)^  v2\.15\s+release/v2\.15\s+v0\.2\.10\s+2222222\s+lasso\s+v0\.2\.10\s+v0\.2\.10\s+already up to date$`, out, "a companion's release is its own tag")
}

func TestRenderTextTheDestinationSaysWhoseReleaseEachEntryHolds(t *testing.T) {
	p := wranglerAndLasso()
	p.Destination = []domain.Destination{
		{Project: "rancher/rancher", Producer: "wrangler", Line: "v2.15", Stream: "wrangler-v3-8", Branch: "release/v2.15", Held: []domain.HeldValue{{Current: "v3.8.0"}}},
		{Project: "rancher/rancher", Producer: "lasso", Line: "v2.15", Stream: "lasso-v0-2", Branch: "release/v2.15", Held: []domain.HeldValue{{Current: "v0.2.10"}}},
		{Project: "rancher/rancher", Producer: "lasso", Line: "v2.14", Error: "no such file"},
	}

	out := renderText(p, false)

	assert.Contains(t, out, "v2.15: rancher/rancher holds v3.8.0 of wrangler through wrangler-v3-8, on release/v2.15")
	assert.Contains(t, out, "v2.15: rancher/rancher holds v0.2.10 of lasso through lasso-v0-2, on release/v2.15")
	assert.Contains(t, out, "v2.14: rancher/rancher (lasso) could not be read: no such file")
	assert.Equal(t, 1, strings.Count(out, "Destination"), "one heading")
}
