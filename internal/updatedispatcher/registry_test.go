package updatedispatcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type namedStrategy struct{ name string }

func (n namedStrategy) Name() string     { return n.name }
func (namedStrategy) Requires() []string { return nil }
func (namedStrategy) Configure(map[string]string) (Updater, error) {
	return nil, nil
}

func TestBuiltinHasYamlBump(t *testing.T) {
	r := Builtin()
	s, err := r.Get("yamlBump")
	require.NoError(t, err)
	assert.Equal(t, "yamlBump", s.Name(), "the name is claimed by the strategy itself")
	assert.Equal(t, []string{"yamlBump"}, r.Names())
}

func TestRegistryUnknownStrategyListsWhatExists(t *testing.T) {
	_, err := Builtin().Get("buildYamlBump")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "buildYamlBump")
	assert.Contains(t, err.Error(), "yamlBump")
}

func TestRegistryRegistersByTheNameTheStrategyClaims(t *testing.T) {
	r := NewStrategyRegistry()
	r.Register(namedStrategy{"b"})
	r.Register(namedStrategy{"a"})
	assert.Equal(t, []string{"a", "b"}, r.Names(), "sorted")
}

func TestRegistryTwoStrategiesClaimingOneNamePanics(t *testing.T) {
	r := NewStrategyRegistry()
	r.Register(namedStrategy{"x"})
	assert.Panics(t, func() { r.Register(namedStrategy{"x"}) })
}
