package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var allRegistries = []string{RegistryDockerHub, RegistryPrimeStaging, RegistryPrimeProduction}

func TestEffectiveImagesDefaultsToTheRepository(t *testing.T) {
	p := Project{Repo: Repo{Name: "rancher/scc-operator"}}

	assert.Equal(t, []Image{{Name: "rancher/scc-operator", Registries: allRegistries}}, p.EffectiveImages())
	assert.Empty(t, p.Images, "the field stays as written")
}

func TestEffectiveImagesKeepsWhatIsListed(t *testing.T) {
	p := Project{Repo: Repo{Name: "rancher/rancher"}, Images: []Image{
		{Name: "rancher/rancher", Registries: []string{RegistryDockerHub}},
		{Name: "rancher/rancher-agent"},
	}}

	assert.Equal(t, []Image{
		{Name: "rancher/rancher", Registries: []string{RegistryDockerHub}},
		{Name: "rancher/rancher-agent", Registries: allRegistries},
	}, p.EffectiveImages(), "an image with no registries goes to all of them")
}

func TestEffectiveImagesDoesNotShareTheRegistryList(t *testing.T) {
	p := Project{Repo: Repo{Name: "o/a"}}
	first := p.EffectiveImages()
	first[0].Registries[0] = "changed"
	assert.Equal(t, allRegistries, p.EffectiveImages()[0].Registries)
}

func TestImageAppliesTo(t *testing.T) {
	tests := []struct {
		name  string
		since string
		line  string
		want  bool
	}{
		{"no since applies to every line", "", "v2.12", true},
		{"the line itself", "v2.16", "v2.16", true},
		{"a newer line", "v2.16", "v2.17", true},
		{"an older line", "v2.16", "v2.15", false},
		{"numbers, not text: 9 is below 16", "v2.16", "v2.9", false},
		{"numbers, not text: 100 is above 16", "v2.16", "v2.100", true},
		{"a newer major", "v2.16", "v3.0", true},
		{"an older major", "v2.16", "v1.99", false},
		{"no leading v", "2.16", "v2.17", true},
		{"a line that is not a number applies", "v2.16", "main", true},
		{"a since that is not a number applies", "latest", "v2.15", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Image{Name: "o/a", Since: tt.since}.AppliesTo(tt.line))
		})
	}
}

func TestEffectiveImagesKeepsSince(t *testing.T) {
	p := Project{Repo: Repo{Name: "rancher/rancher"}, Images: []Image{
		{Name: "rancher/rancher"},
		{Name: "rancher/rancher-server", Since: "v2.16"},
	}}
	got := p.EffectiveImages()
	assert.Empty(t, got[0].Since)
	assert.Equal(t, "v2.16", got[1].Since)
}
