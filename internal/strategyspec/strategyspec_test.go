package strategyspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYamlBumpSpec(t *testing.T) {
	spec, ok := Lookup("yamlBump")
	require.True(t, ok)
	assert.Equal(t, "yamlBump", spec.Name)

	file, ok := spec.Param("file")
	require.True(t, ok)
	assert.True(t, file.Required)
	assert.True(t, file.ConsumerOnly, "a producer must not choose which file the consumer edits")

	key, ok := spec.Param("updateKey")
	require.True(t, ok)
	assert.True(t, key.Required)
	assert.False(t, key.ConsumerOnly, "the producer says which key is its own")

	value, ok := spec.Param("value")
	require.True(t, ok)
	assert.False(t, value.Required)
	assert.Equal(t, "{tag}", value.Default)
}

func TestChartsPackageBumpIsDeclared(t *testing.T) {
	// The mock configs use it, so validation must know it, though gero does not
	// implement it yet.
	spec, ok := Lookup("chartsPackageBump")
	require.True(t, ok)
	pkg, ok := spec.Param("package")
	require.True(t, ok)
	assert.True(t, pkg.Required)
}

func TestUnknownStrategy(t *testing.T) {
	_, ok := Lookup("buildYamlBump")
	assert.False(t, ok, "yamlBump replaced buildYamlBump and yamlKeyBump")
	spec, _ := Lookup("yamlBump")
	_, ok = spec.Param("nope")
	assert.False(t, ok)
}

func TestNames(t *testing.T) {
	assert.Equal(t, []string{"chartsPackageBump", "yamlBump"}, Names())
}

func TestEveryParamHasADescription(t *testing.T) {
	for _, name := range Names() {
		spec, _ := Lookup(name)
		for _, p := range spec.Params {
			assert.NotEmpty(t, p.Description, "%s.%s", name, p.Key)
		}
	}
}
