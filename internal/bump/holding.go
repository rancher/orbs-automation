package bump

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/semver"
	"github.com/rancher/orbs-automation/internal/platform/templates"
	"github.com/rancher/orbs-automation/internal/platform/yamledit"
)

// ErrFormatNotBuilt is returned for a holding whose format gero cannot read or write
// yet.
var ErrFormatNotBuilt = errors.New("format not built")

// formatYAML is the format gero can read and write.
const formatYAML = "yaml"

// ReleaseOf makes the variables of a release of the producer for what a holding
// holds of it: the tag, the artifact's image or file, and where the file is among
// the release's files.
func ReleaseOf(h *chain.Holding, tag, commit string) (Release, error) {
	a := h.Artifact()
	if a == nil {
		return Release{}, fmt.Errorf("Holding %q holds no artifact of a release", h.Name())
	}
	rel := Release{Tag: tag, Version: strings.TrimPrefix(tag, "v"), SourceRepo: h.Producer().Slug(), Commit: commit}
	if a.IsImage() {
		rel.Image = a.Ref()
		return rel, nil
	}
	file, err := templates.Render(a.Ref(), map[string]string{"tag": rel.Tag, "version": rel.Version})
	if err != nil {
		return Release{}, fmt.Errorf("Artifact %q: name %q: %w", a.Name(), a.Ref(), err)
	}
	rel.File = file
	rel.URL = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", rel.SourceRepo, tag, file)
	return rel, nil
}

// Place is where a holding is found in a consumer's repo for one target.
type Place struct {
	File   string
	Key    string
	Format string
}

// PlaceOf renders where a holding is found for a line and a branch of the
// consumer: its file and key with the line, the holding's stage and the branch.
func PlaceOf(h *chain.Holding, line, branch string) (Place, error) {
	vars := map[string]string{"line": line, "stage": h.Doc.Spec.Holds.Stage, "branch": branch}
	at := h.Doc.Spec.At
	file, err := templates.Render(at.File, vars)
	if err != nil {
		return Place{}, fmt.Errorf("Holding %q: at.file %q: %w", h.Name(), at.File, err)
	}
	key, err := templates.Render(at.Key, vars)
	if err != nil {
		return Place{}, fmt.Errorf("Holding %q: at.key %q: %w", h.Name(), at.Key, err)
	}
	return Place{File: file, Key: key, Format: at.Format}, nil
}

// FormOf renders how a holding writes the release: its form, with the release's
// variables.
func FormOf(h *chain.Holding, rel Release) (string, error) {
	form := h.Doc.Spec.Holds.Form
	value, err := templates.Render(form, map[string]string{
		"image": rel.Image, "tag": rel.Tag, "version": rel.Version, "url": rel.URL, "file": rel.File,
		"sourceRepo": rel.SourceRepo, "commit": rel.Commit, "base": "", "upstream": "",
	})
	if err != nil {
		return "", fmt.Errorf("Holding %q: form %q: %w", h.Name(), form, err)
	}
	return value, nil
}

// resolveHolding finds the place and the value a holding wants for a target, and refuses
// a format that is not built.
func resolveHolding(h *chain.Holding, line, branch string, rel Release) (Place, string, error) {
	place, err := PlaceOf(h, line, branch)
	if err != nil {
		return Place{}, "", err
	}
	if place.Format != formatYAML {
		return Place{}, "", fmt.Errorf("%w: Holding %q is in %s, which gero cannot read or write yet", ErrFormatNotBuilt, h.Name(), place.Format)
	}
	want, err := FormOf(h, rel)
	if err != nil {
		return Place{}, "", err
	}
	return place, want, nil
}

// CheckHolding reads what a holding holds in src for a line and a branch, and
// says how it compares with what the release wants. It never writes.
func CheckHolding(ctx context.Context, src Tree, h *chain.Holding, line, branch string, rel Release) (Check, error) {
	place, want, err := resolveHolding(h, line, branch, rel)
	if err != nil {
		return Check{}, err
	}
	data, err := src.ReadFile(ctx, place.File)
	if err != nil {
		return Check{}, fmt.Errorf("read %s: %w", place.File, err)
	}
	current, err := yamledit.Get(data, place.Key)
	if err != nil {
		return Check{}, fmt.Errorf("read %s: %w", place.File, err)
	}
	return Check{
		Current: current, Want: want, UpToDate: current == want, Relation: relation(current, want),
		Change: change(place, current, want),
	}, nil
}

// ApplyHolding writes the release into the holding's place in dst, changing only
// that value, and says what changed.
func ApplyHolding(ctx context.Context, dst WritableTree, h *chain.Holding, line, branch string, rel Release) (Result, error) {
	place, want, err := resolveHolding(h, line, branch, rel)
	if err != nil {
		return Result{}, err
	}
	data, err := dst.ReadFile(ctx, place.File)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", place.File, err)
	}
	out, old, err := yamledit.Set(data, place.Key, want)
	if err != nil {
		return Result{}, fmt.Errorf("set %s in %s: %w", place.Key, place.File, err)
	}
	if old == want {
		return Result{}, nil
	}
	if err := dst.WriteFile(ctx, place.File, out); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", place.File, err)
	}
	return Result{Changed: true, Changes: []Change{change(place, old, want)}}, nil
}

func change(place Place, from, to string) Change {
	return Change{
		Path: place.File, From: from, To: to,
		Description: fmt.Sprintf("Updated `%s` in `%s` from `%s` to `%s`", place.Key, place.File, from, to),
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
