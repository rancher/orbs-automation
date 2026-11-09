package bump

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/gomodedit"
	"github.com/rancher/orbs-automation/internal/platform/semver"
	"github.com/rancher/orbs-automation/internal/platform/templates"
	"github.com/rancher/orbs-automation/internal/platform/yamledit"
)

// ErrFormatNotBuilt is returned for a holding whose format gero cannot read or write
// yet.
var ErrFormatNotBuilt = errors.New("format not built")

// A format reads the value at a key in a file and replaces it, leaving the rest of
// the file as it was. Only the formats here are built: a holding in another one,
// such as dockerfile, is refused with ErrFormatNotBuilt.
type format struct {
	get func(src []byte, key string) (string, error)
	set func(src []byte, key, value string) (out []byte, old string, err error)
}

var formats = map[string]format{
	"yaml":  {get: yamledit.Get, set: yamledit.Set},
	"gomod": {get: gomodedit.Get, set: gomodedit.Set},
}

// ReleaseOf makes the variables of a release of the producer for what a holding
// holds of it: the tag, the artifact's image or file, and where the file is among
// the release's files.
func ReleaseOf(h *chain.Holding, tag, commit string) (Release, error) {
	a := h.Artifact()
	if a == nil {
		return Release{}, fmt.Errorf("Holding %q holds no artifact of a release", h.Name())
	}
	rel := newRelease(tag, tagPatternOf(h), h.Producer().Slug(), commit)
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

// newRelease makes the variables that come from a tag, for a stream whose tags
// match a pattern. The prefix is the pattern up to its last slash, when the tag
// starts with it; the bare tag is the tag without it, and the version is that
// without a leading v.
func newRelease(tag, pattern, sourceRepo, commit string) Release {
	prefix := chain.TagPrefix(pattern, tag)
	bare := strings.TrimPrefix(tag, prefix)
	return Release{Tag: tag, Prefix: prefix, BareTag: bare, Version: strings.TrimPrefix(bare, "v"), SourceRepo: sourceRepo, Commit: commit}
}

// tagPatternOf is the pattern of the tags of the stream a holding's artifact is
// produced by, or none when the holding has no artifact.
func tagPatternOf(h *chain.Holding) string {
	if a := h.Artifact(); a != nil && a.Stream() != nil {
		return a.Stream().TagPattern()
	}
	return ""
}

// Place is where a holding is found in a consumer's repo for one target.
type Place struct {
	File   string
	Key    string
	Format string
}

// PlacesOf renders where a holding is found for a line and a branch of the
// consumer, one place for each of its locations that exists for the line: the
// file and key with the line, the holding's stage and the branch. A location
// limited to other lines is left out, so a holding can have none for a line.
func PlacesOf(h *chain.Holding, line, branch string) ([]Place, error) {
	vars := map[string]string{"line": line, "stage": h.Doc.Spec.Holds.Stage, "branch": branch}
	var places []Place
	for _, at := range h.Doc.Spec.At {
		if !at.Lines.Contains(line) {
			continue
		}
		file, err := templates.Render(at.File, vars)
		if err != nil {
			return nil, fmt.Errorf("Holding %q: at.file %q: %w", h.Name(), at.File, err)
		}
		key, err := templates.Render(at.Key, vars)
		if err != nil {
			return nil, fmt.Errorf("Holding %q: at.key %q: %w", h.Name(), at.Key, err)
		}
		places = append(places, Place{File: file, Key: key, Format: at.Format})
	}
	return places, nil
}

// FormOf renders how a holding writes the release: its form, with the release's
// variables.
func FormOf(h *chain.Holding, rel Release) (string, error) {
	form := h.Doc.Spec.Holds.Form
	value, err := templates.Render(form, map[string]string{
		"image": rel.Image, "tag": rel.Tag, "prefix": rel.Prefix, "bareTag": rel.BareTag, "version": rel.Version, "url": rel.URL, "file": rel.File,
		"sourceRepo": rel.SourceRepo, "commit": rel.Commit, "base": "", "upstream": "",
	})
	if err != nil {
		return "", fmt.Errorf("Holding %q: form %q: %w", h.Name(), form, err)
	}
	return value, nil
}

// resolveHolding finds the places and the value a holding wants for a target. It
// refuses a holding with no location for the line, and a format that is not
// built, before anything is read or written.
func resolveHolding(h *chain.Holding, line, branch string, rel Release) ([]Place, string, error) {
	places, err := PlacesOf(h, line, branch)
	if err != nil {
		return nil, "", err
	}
	if len(places) == 0 {
		return nil, "", fmt.Errorf("Holding %q has no location for %s", h.Name(), line)
	}
	for _, place := range places {
		if _, built := formats[place.Format]; !built {
			return nil, "", fmt.Errorf("%w: Holding %q is in %s, which gero cannot read or write yet", ErrFormatNotBuilt, h.Name(), place.Format)
		}
	}
	want, err := FormOf(h, rel)
	if err != nil {
		return nil, "", err
	}
	return places, want, nil
}

// CheckHolding reads what a holding holds, in each of its locations, in src for a
// line and a branch, and says how it compares with what the release wants. It
// never writes.
func CheckHolding(ctx context.Context, src Tree, h *chain.Holding, line, branch string, rel Release) (Check, error) {
	places, want, err := resolveHolding(h, line, branch, rel)
	if err != nil {
		return Check{}, err
	}
	check := Check{Want: want, UpToDate: true}
	for _, place := range places {
		data, err := src.ReadFile(ctx, place.File)
		if err != nil {
			return Check{}, fmt.Errorf("read %s: %w", place.File, err)
		}
		current, err := formats[place.Format].get(data, place.Key)
		if err != nil {
			return Check{}, fmt.Errorf("read %s: %w", place.File, err)
		}
		check.Locations = append(check.Locations, LocationCheck{
			Place: place, Current: current, UpToDate: current == want,
			Relation: relation(current, want), Change: change(place, current, want),
		})
	}
	first := check.Locations[0]
	check.Current, check.Relation, check.Change = first.Current, first.Relation, first.Change
	for _, loc := range check.Locations {
		if !loc.UpToDate {
			check.UpToDate = false
			check.Relation, check.Change = loc.Relation, loc.Change
			break
		}
	}
	return check, nil
}

// ApplyHolding writes the release into each of the holding's locations in dst,
// changing only those values, and says what changed. Every location is read and
// set before any file is written, so a location that cannot be written leaves the
// tree as it was.
func ApplyHolding(ctx context.Context, dst WritableTree, h *chain.Holding, line, branch string, rel Release) (Result, error) {
	places, want, err := resolveHolding(h, line, branch, rel)
	if err != nil {
		return Result{}, err
	}
	pending := map[string][]byte{} // each file as it is once the places so far are set
	var order []string             // the files that changed, in the order they first did
	var res Result
	for _, place := range places {
		data, read := pending[place.File]
		if !read {
			if data, err = dst.ReadFile(ctx, place.File); err != nil {
				return Result{}, fmt.Errorf("read %s: %w", place.File, err)
			}
		}
		out, old, err := formats[place.Format].set(data, place.Key, want)
		if err != nil {
			return Result{}, fmt.Errorf("set %s in %s: %w", place.Key, place.File, err)
		}
		pending[place.File] = data
		if old == want {
			continue
		}
		pending[place.File] = out
		if !slices.Contains(order, place.File) {
			order = append(order, place.File)
		}
		res.Changes = append(res.Changes, change(place, old, want))
	}
	for _, file := range order {
		if err := dst.WriteFile(ctx, file, pending[file]); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", file, err)
		}
	}
	res.Changed = len(res.Changes) > 0
	return res, nil
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
