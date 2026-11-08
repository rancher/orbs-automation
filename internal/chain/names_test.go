package chain_test

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/calver"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/tagscheme"
	"github.com/stretchr/testify/assert"
)

func TestTheNamesTheDomainUsesAreTheOnesTheConfigAccepts(t *testing.T) {
	for config, domain := range map[string]string{
		v1alpha2.LayoutSingle:            chain.LayoutSingle,
		v1alpha2.LayoutPerLine:           chain.LayoutPerLine,
		v1alpha2.HoldsRelease:            chain.HoldsRelease,
		v1alpha2.AcceptAll:               chain.AcceptAll,
		v1alpha2.RegistryDockerHub:       chain.RegistryDockerHub,
		v1alpha2.RegistryPrimeStaging:    chain.RegistryPrimeStaging,
		v1alpha2.RegistryPrimeProduction: chain.RegistryPrimeProduction,
		v1alpha2.SchemeSemver:            tagscheme.Semver,
		v1alpha2.SchemeCalverLineStage:   tagscheme.CalverLineStage,
		v1alpha2.StageDev:                calver.StageDev,
		v1alpha2.StageStable:             calver.StageStable,
	} {
		assert.Equal(t, config, domain)
	}
}

// chartVersion is accepted in config and has no scheme in the domain yet, so the
// stream of one is not read: tagscheme.ForStream says so.
func TestASchemeTheConfigAcceptsHasADomainOwnerOrIsNotBuilt(t *testing.T) {
	owned := map[string]bool{tagscheme.Semver: true, tagscheme.CalverLineStage: true}
	unbuilt := map[string]bool{v1alpha2.SchemeChartVersion: true}
	for _, name := range []string{v1alpha2.SchemeSemver, v1alpha2.SchemeCalverLineStage, v1alpha2.SchemeChartVersion} {
		assert.True(t, owned[name] != unbuilt[name], name)
	}
}
