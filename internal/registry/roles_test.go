package registry

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	tests := []struct {
		role, label, host, name string
	}{
		{v1alpha2.RegistryDockerHub, "Docker Hub", "registry-1.docker.io", "docker.io"},
		{v1alpha2.RegistryPrimeStaging, "Prime Staging", "stgregistry.suse.com", "stgregistry.suse.com"},
		{v1alpha2.RegistryPrimeProduction, "Prime Production", "registry.rancher.com", "registry.rancher.com"},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			r, ok := Lookup(tt.role)
			require.True(t, ok)
			assert.Equal(t, Registry{Role: tt.role, Label: tt.label, Host: tt.host, Name: tt.name}, r)
		})
	}
}

func TestLookupUnknownRole(t *testing.T) {
	_, ok := Lookup("ecr")
	assert.False(t, ok)
}

func TestEveryConfigRoleHasARegistry(t *testing.T) {
	for _, role := range []string{v1alpha2.RegistryDockerHub, v1alpha2.RegistryPrimeStaging, v1alpha2.RegistryPrimeProduction} {
		_, ok := Lookup(role)
		assert.True(t, ok, role)
	}
}

func TestApplies(t *testing.T) {
	tests := []struct {
		tag  string
		role string
		want bool
	}{
		{"v0.5.3", v1alpha2.RegistryDockerHub, true},
		{"v0.5.3", v1alpha2.RegistryPrimeStaging, true},
		{"v0.5.3", v1alpha2.RegistryPrimeProduction, true},
		{"v0.5.3-rc.1", v1alpha2.RegistryDockerHub, true},
		{"v0.5.3-rc.1", v1alpha2.RegistryPrimeStaging, true},
		{"v0.5.3-rc.1", v1alpha2.RegistryPrimeProduction, false},
		{"0.5.3-hotfix-1", v1alpha2.RegistryPrimeProduction, false},
		{"v0.5.3+build.1", v1alpha2.RegistryPrimeProduction, true},
		{"main", v1alpha2.RegistryPrimeProduction, true},
		{"v0.5.3", "ecr", false},
	}
	for _, tt := range tests {
		t.Run(tt.tag+" in "+tt.role, func(t *testing.T) {
			assert.Equal(t, tt.want, Applies(tt.role, tt.tag))
		})
	}
}

func TestReference(t *testing.T) {
	hub, _ := Lookup(v1alpha2.RegistryDockerHub)
	prod, _ := Lookup(v1alpha2.RegistryPrimeProduction)
	assert.Equal(t, "docker.io/rancher/scc-operator:v0.5.3", hub.Reference("rancher/scc-operator", "v0.5.3"))
	assert.Equal(t, "registry.rancher.com/rancher/scc-operator:v0.5.3", prod.Reference("rancher/scc-operator", "v0.5.3"))
}
