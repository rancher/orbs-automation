package updatedispatcher

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/rancher/orbs-automation/internal/platform/semver"
	"github.com/rancher/orbs-automation/internal/ship/templates"
	"github.com/rancher/orbs-automation/internal/yamledit"
)

// YAMLBump keeps a release's value at one key of a YAML file. The method fixes
// the file with `with.file`. A call passes `updateKey`, the key to read and set
// (a dotted path for a nested key), and optionally `value`, a template over
// {image}, {tag}, {version} and {sourceRepo}. Without `value` it writes the tag.
type YAMLBump struct{}

var _ UpdateStrategy = YAMLBump{}

// Name is how config refers to the strategy.
func (YAMLBump) Name() string { return "yamlBump" }

// Requires lists the input a method must offer.
func (YAMLBump) Requires() []string { return []string{"updateKey"} }

// Configure reads `with`: the file, and nothing else.
func (YAMLBump) Configure(with map[string]string) (Updater, error) {
	for key := range with {
		if key != "file" {
			return nil, fmt.Errorf("yamlBump takes only with.file, not %q", key)
		}
	}
	if with["file"] == "" {
		return nil, errors.New("yamlBump needs with.file, the file to edit")
	}
	return yamlUpdater{file: with["file"]}, nil
}

type yamlUpdater struct{ file string }

// defaultValue is what is written when the call passes no `value`.
const defaultValue = "{tag}"

func (u yamlUpdater) want(in Inputs, rel Release) (string, error) {
	text := in["value"]
	if text == "" {
		text = defaultValue
	}
	vars := []string{"image", "tag", "version", "sourceRepo"}
	if err := templates.Validate(text, vars); err != nil {
		return "", fmt.Errorf("input value %q: %w", text, err)
	}
	return templates.Render(text, map[string]string{
		"image": rel.Image, "tag": rel.Tag, "version": rel.Version, "sourceRepo": rel.SourceRepo,
	})
}

func (u yamlUpdater) key(in Inputs) (string, error) {
	if in["updateKey"] == "" {
		return "", errors.New("yamlBump needs the input updateKey")
	}
	return in["updateKey"], nil
}

// Check reads the file and says what it holds, what it should hold, and how the
// two compare.
func (u yamlUpdater) Check(ctx context.Context, src Tree, in Inputs, rel Release) (Check, error) {
	key, err := u.key(in)
	if err != nil {
		return Check{}, err
	}
	want, err := u.want(in, rel)
	if err != nil {
		return Check{}, err
	}
	data, err := src.ReadFile(ctx, u.file)
	if err != nil {
		return Check{}, fmt.Errorf("read %s: %w", u.file, err)
	}
	current, err := yamledit.Get(data, key)
	if err != nil {
		return Check{}, fmt.Errorf("read %s: %w", u.file, err)
	}
	return Check{
		Current: current, Want: want, UpToDate: current == want, Relation: relation(current, want),
		Change: u.change(key, current, want),
	}, nil
}

// Apply sets the key, changing only its value in the file.
func (u yamlUpdater) Apply(ctx context.Context, dst WritableTree, in Inputs, rel Release) (Result, error) {
	key, err := u.key(in)
	if err != nil {
		return Result{}, err
	}
	want, err := u.want(in, rel)
	if err != nil {
		return Result{}, err
	}
	data, err := dst.ReadFile(ctx, u.file)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", u.file, err)
	}
	out, old, err := yamledit.Set(data, key, want)
	if err != nil {
		return Result{}, fmt.Errorf("set %s in %s: %w", key, u.file, err)
	}
	if old == want {
		return Result{}, nil
	}
	if err := dst.WriteFile(ctx, u.file, out); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", u.file, err)
	}
	return Result{Changed: true, Changes: []Change{u.change(key, old, want)}}, nil
}

func (u yamlUpdater) change(key, from, to string) Change {
	return Change{
		Path: u.file, From: from, To: to,
		Description: fmt.Sprintf("Updated `%s` in `%s` from `%s` to `%s`", key, u.file, from, to),
	}
}

// versionToken is a version inside a longer value: a tag after a colon, a bare
// version, or one with a prerelease or build part.
var versionToken = regexp.MustCompile(`v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`)

// relation compares the versions found in two values, and says unknown when
// either has none, so a value like latest is never judged.
func relation(current, want string) Relation {
	cv, ok := versionIn(current)
	if !ok {
		return RelationUnknown
	}
	wv, ok := versionIn(want)
	if !ok {
		return RelationUnknown
	}
	switch semver.Compare(wv, cv) {
	case 1:
		return RelationNewer
	case -1:
		return RelationOlder
	}
	return RelationSame
}

func versionIn(value string) (semver.Version, bool) {
	token := versionToken.FindString(value)
	if token == "" {
		return semver.Version{}, false
	}
	return semver.Parse(token)
}
