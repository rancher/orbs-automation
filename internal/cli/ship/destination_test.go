package ship

import (
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

func TestRenderTextNamesTheProducerOfACompanionsImages(t *testing.T) {
	p := withCompanion()
	p.Images = []domain.ReleaseImages{{Producer: "rancher/lasso", Release: "v0.2.5", Report: registry.Report{Rows: []registry.Row{{Image: "github.com/rancher/lasso", Registry: "goproxy", Status: "found"}}}}}

	out := renderText(p, false)

	assert.Contains(t, out, "Images (rancher/lasso v0.2.5)")
}
