package registry

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/chain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	tests := []struct {
		role, label, host, name string
	}{
		{chain.RegistryDockerHub, "Docker Hub", "registry-1.docker.io", "docker.io"},
		{chain.RegistryPrimeStaging, "Prime Staging", "stgregistry.suse.com", "stgregistry.suse.com"},
		{chain.RegistryPrimeProduction, "Prime Production", "registry.rancher.com", "registry.rancher.com"},
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
	for _, role := range []string{chain.RegistryDockerHub, chain.RegistryPrimeStaging, chain.RegistryPrimeProduction} {
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
		{"v0.5.3", chain.RegistryDockerHub, true},
		{"v0.5.3", chain.RegistryPrimeStaging, true},
		{"v0.5.3", chain.RegistryPrimeProduction, true},
		{"v0.5.3-rc.1", chain.RegistryDockerHub, true},
		{"v0.5.3-rc.1", chain.RegistryPrimeStaging, true},
		{"v0.5.3-rc.1", chain.RegistryPrimeProduction, false},
		{"0.5.3-hotfix-1", chain.RegistryPrimeProduction, false},
		{"v0.5.3+build.1", chain.RegistryPrimeProduction, true},
		{"main", chain.RegistryPrimeProduction, true},
		{"v0.5.3", "ecr", false},
	}
	for _, tt := range tests {
		t.Run(tt.tag+" in "+tt.role, func(t *testing.T) {
			assert.Equal(t, tt.want, Applies(tt.role, tt.tag))
		})
	}
}

func TestReference(t *testing.T) {
	hub, _ := Lookup(chain.RegistryDockerHub)
	prod, _ := Lookup(chain.RegistryPrimeProduction)
	assert.Equal(t, "docker.io/rancher/scc-operator:v0.5.3", hub.Reference("rancher/scc-operator", "v0.5.3"))
	assert.Equal(t, "registry.rancher.com/rancher/scc-operator:v0.5.3", prod.Reference("rancher/scc-operator", "v0.5.3"))
}
