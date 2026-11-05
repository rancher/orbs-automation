// Package ship is the domain of gero ship: bumping one consumer repository to a
// release of a producer. It holds the state check that decides whether a PR is
// needed and the strategies that know where a consumer keeps the value. It is
// framework-free and talks to the world through small interfaces defined here,
// so tests use fakes (docs/specs/PUSH_TO_REPO.md).
package ship

import "errors"

// Params are the params of a source's update, as written in config.
type Params map[string]string

// ValueVars are the variables a strategy's value template may use.
var ValueVars = []string{"image", "tag", "version", "sourceRepo"}

// ErrUnknownStrategy is returned for an update.strategy gero has no
// implementation of.
var ErrUnknownStrategy = errors.New("unknown strategy")

// Strategy knows how one kind of consumer keeps the value of a release: where
// it is, what it should become and how to read what it is now. Writing it is a
// later step. The name in a source's update.strategy picks one.
type Strategy interface {
	// File is the path, in the consumer, of the file the strategy edits.
	File(p Params) (string, error)
	// Value is what the consumer should hold for a release: the value template
	// of the params, or the strategy's default, filled from vars (ValueVars).
	Value(p Params, vars map[string]string) (string, error)
	// Read returns the value the file src holds now.
	Read(src []byte, p Params) (string, error)
}

// LookupStrategy returns the strategy of a name.
func LookupStrategy(name string) (Strategy, error) {
	return nil, nil
}

// StrategyNames lists the strategies gero implements, sorted.
func StrategyNames() []string {
	return nil
}
