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
