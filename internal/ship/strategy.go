// Package ship is the domain of gero ship: bumping one consumer repository to a
// release of a producer. It holds the state check that decides whether a PR is
// needed and the strategies that know where a consumer keeps the value. It is
// framework-free and talks to the world through small interfaces defined here,
// so tests use fakes (docs/specs/PUSH_TO_REPO.md).
package ship

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/ship/templates"
	"github.com/rancher/orbs-automation/internal/yamledit"
)

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
	// Describe says what a bump from one value to another changes, in a phrase
	// for the PR body's list of changes, such as "Updated `key` in `file`".
	Describe(p Params, from, to string) string
}

var strategies = map[string]Strategy{
	// buildYamlBump sets an image reference, such as rancher/scc-operator:v0.5.3.
	"buildYamlBump": yamlKey{defaultValue: "{image}:{tag}"},
	// yamlKeyBump sets a bare value, by default the tag.
	"yamlKeyBump": yamlKey{defaultValue: "{tag}"},
}

// LookupStrategy returns the strategy of a name.
func LookupStrategy(name string) (Strategy, error) {
	if s, ok := strategies[name]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("%w %q: gero implements %s", ErrUnknownStrategy, name, strings.Join(StrategyNames(), ", "))
}

// StrategyNames lists the strategies gero implements, sorted.
func StrategyNames() []string {
	names := make([]string, 0, len(strategies))
	for n := range strategies {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// yamlKey keeps a release's value at one key of a YAML file. The params are
// file and key (a dotted path), and optionally value, a template over
// ValueVars that replaces the strategy's default.
type yamlKey struct{ defaultValue string }

func (yamlKey) File(p Params) (string, error) { return required(p, "file") }

func (y yamlKey) Value(p Params, vars map[string]string) (string, error) {
	text := y.defaultValue
	if v := p["value"]; v != "" {
		text = v
	}
	if err := templates.Validate(text, ValueVars); err != nil {
		return "", fmt.Errorf("param value %q: %w", text, err)
	}
	return templates.Render(text, vars)
}

func (yamlKey) Describe(p Params, from, to string) string {
	return fmt.Sprintf("Updated `%s` in `%s` from `%s` to `%s`", p["key"], p["file"], from, to)
}

func (yamlKey) Read(src []byte, p Params) (string, error) {
	key, err := required(p, "key")
	if err != nil {
		return "", err
	}
	return yamledit.Get(src, key)
}

func required(p Params, name string) (string, error) {
	if p[name] == "" {
		return "", fmt.Errorf("the strategy needs the param %q", name)
	}
	return p[name], nil
}
