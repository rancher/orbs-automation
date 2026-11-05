// Package strategyspec declares the strategies gero knows by name and the params
// each one takes, apart from what the strategy does. Config validation needs
// the declarations, to check that a method and the edges into it use params the
// strategy has, and it cannot import the code that runs them (internal/ship), so
// the names and params live here and the behavior is in ship, matched by name.
package strategyspec

import "slices"

// Param is one param a strategy consumes.
type Param struct {
	Key         string
	Description string
	Required    bool   // a producer or the method must supply it, unless it has a Default
	Default     string // what it is when nobody supplies it
	// ConsumerOnly marks a param that chooses what is edited, such as the file.
	// Only the consumer's method may set it, never a producer's edge.
	ConsumerOnly bool
}

// Spec is a strategy's name and the params it consumes.
type Spec struct {
	Name   string
	Params []Param
}

// Lookup returns the spec of a strategy name.
func Lookup(name string) (Spec, bool) {
	s, ok := specs[name]
	return s, ok
}

var specs = map[string]Spec{
	// yamlBump sets one key of a YAML file to a value made from the release.
	"yamlBump": {Name: "yamlBump", Params: []Param{
		{Key: "file", Required: true, ConsumerOnly: true, Description: "The file to edit, relative to the repository root"},
		{Key: "updateKey", Required: true, Description: "The key to read and set, as a dotted path for a nested key"},
		{Key: "value", Default: "{tag}", Description: "What to write: a template over {image}, {tag}, {version} and {sourceRepo}"},
	}},
	// chartsPackageBump is declared so config naming it validates; gero does not
	// implement it yet.
	"chartsPackageBump": {Name: "chartsPackageBump", Params: []Param{
		{Key: "package", Required: true, Description: "The chart package to bump"},
	}},
}

// Names lists the strategies that have a spec, sorted.
func Names() []string {
	names := make([]string, 0, len(specs))
	for n := range specs {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Param returns the strategy's param of a key.
func (s Spec) Param(key string) (Param, bool) {
	for _, p := range s.Params {
		if p.Key == key {
			return p, true
		}
	}
	return Param{}, false
}
