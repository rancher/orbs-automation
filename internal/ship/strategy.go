// Package ship is the domain of gero ship: bumping one consumer repository to a
// release of a producer. It holds the state check that decides whether a PR is
// needed and the strategies that know where a consumer keeps the value. It is
// framework-free and talks to the world through small interfaces defined here,
// so tests use fakes (docs/specs/PUSH_TO_REPO.md).
package ship

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/semver"
	"github.com/rancher/orbs-automation/internal/ship/templates"
	"github.com/rancher/orbs-automation/internal/strategyspec"
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
	// Write returns the file src with the value set to value, changing nothing
	// else in it. Setting the value it already has returns src unchanged.
	Write(src []byte, p Params, value string) ([]byte, error)
	// Version finds the release version a value holds: the version in v1.2.3,
	// in rancher/scc-operator:v1.2.3 or in an image reference with a registry
	// and port. ok is false for a value with no version in it, such as latest.
	Version(value string) (v semver.Version, ok bool)
	// Describe says what a bump from one value to another changes, in a phrase
	// for the PR body's list of changes, such as "Updated `key` in `file`".
	Describe(p Params, from, to string) string
}

var strategies = map[string]Strategy{
	// yamlBump keeps the release's value at one key of a YAML file: a bare tag by
	// default, or an image reference such as rancher/scc-operator:v0.5.3 when the
	// producer's value template says so.
	"yamlBump": yamlKey{},
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
// file, updateKey (a dotted path), and optionally value, a template over
// ValueVars. Their declarations, defaults included, are in strategyspec.
type yamlKey struct{}

func (yamlKey) File(p Params) (string, error) { return required(p, "file") }

func (yamlKey) Value(p Params, vars map[string]string) (string, error) {
	text := p["value"]
	if text == "" {
		text = paramDefault("yamlBump", "value")
	}
	if err := templates.Validate(text, ValueVars); err != nil {
		return "", fmt.Errorf("param value %q: %w", text, err)
	}
	return templates.Render(text, vars)
}

func (yamlKey) Write(src []byte, p Params, value string) ([]byte, error) {
	key, err := required(p, "updateKey")
	if err != nil {
		return nil, err
	}
	out, _, err := yamledit.Set(src, key, value)
	return out, err
}

// versionToken is a version inside a longer value: a tag after a colon, a bare
// version, or one with a prerelease or build part.
var versionToken = regexp.MustCompile(`v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`)

func (yamlKey) Version(value string) (semver.Version, bool) {
	token := versionToken.FindString(value)
	if token == "" {
		return semver.Version{}, false
	}
	return semver.Parse(token)
}

func (yamlKey) Describe(p Params, from, to string) string {
	return fmt.Sprintf("Updated `%s` in `%s` from `%s` to `%s`", p["updateKey"], p["file"], from, to)
}

func (yamlKey) Read(src []byte, p Params) (string, error) {
	key, err := required(p, "updateKey")
	if err != nil {
		return "", err
	}
	return yamledit.Get(src, key)
}

// paramDefault is a param's default from the strategy's declaration.
func paramDefault(strategy, key string) string {
	spec, _ := strategyspec.Lookup(strategy)
	param, _ := spec.Param(key)
	return param.Default
}

func required(p Params, name string) (string, error) {
	if p[name] == "" {
		return "", fmt.Errorf("the strategy needs the param %q", name)
	}
	return p[name], nil
}
