// Package strategyspec declares the strategies gero knows by name and the params
// each one takes, apart from what the strategy does. Config validation needs
// the declarations, to check that a method and the edges into it use params the
// strategy has, and it cannot import the code that runs them (internal/ship), so
// the names and params live here and the behavior is in ship, matched by name.
package strategyspec

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
	return Spec{}, false
}

// Names lists the strategies that have a spec, sorted.
func Names() []string {
	return nil
}

// Param returns the strategy's param of a key.
func (s Spec) Param(key string) (Param, bool) {
	return Param{}, false
}
