// Package tagscheme is what every reader of a project's releases asks of its tags:
// which lines they carry, which line a tag is in, the newest of a line, whether a
// tag is stable and how two compare. A stream's scheme follows from its scheme
// setting, so ship, repo status, repo tag and lines resolve call it and carry no scheme code of their own
// (docs/specs/CHAIN_TRACING.md). It does no I/O.
package tagscheme

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/rancher/orbs-automation/internal/calver"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/platform/semver"
)

// Scheme answers the questions of one way of naming releases.
type Scheme interface {
	// Name is the release.tagScheme that selects it.
	Name() string
	// Valid reports whether a name is a release tag of the scheme at all.
	Valid(tag string) bool
	// Lines is the Rancher lines of the project, in order. Semver gives the lines
	// the config lists, whatever the tags; a scheme that finds lines in its tags
	// gives the ones the tags carry, newest first.
	Lines(tags []git.Tag) []string
	// LineOf is the line a tag is in, or "" when it is in none.
	LineOf(tag string) string
	// Latest is the newest stable tag of a line and the newest prerelease tag newer
	// than it, each nil when there is none. The line "" is every line. For a scheme
	// with stages a prerelease is a tag of the dev stage.
	Latest(tags []git.Tag, line string) git.Latest
	// IsStable reports whether a tag is not a prerelease.
	IsStable(tag string) bool
	// Stage is the stage a tag is of (calver.StageDev or calver.StageStable), or ""
	// for a scheme whose tags carry none, and for a name that is not a release.
	Stage(tag string) string
	// Compare orders two tags: negative when a is older, zero when they are equal
	// or either is not a release tag of the scheme.
	Compare(a, b string) int
}

// semverScheme is the scheme of a project with per-line branches, and the default:
// tags are versions, and a line is the version series the config gives it.
type semverScheme struct {
	stream  *chain.Stream // the stream whose lines and series these are
	pattern string
	accepts func(tag string) bool // nil: every tag the pattern matches
}

// The names of the schemes a stream may use in config.
const (
	Semver          = "semver"
	CalverLineStage = calver.LineStageName
)

func (semverScheme) Name() string { return Semver }

func (semverScheme) Valid(tag string) bool {
	_, ok := semver.Parse(tag)
	return ok
}

func (s semverScheme) Lines([]git.Tag) []string { return s.stream.Lines() }

func (s semverScheme) LineOf(tag string) string {
	line, _ := s.stream.LineOf(tag)
	return line
}

func (s semverScheme) Latest(tags []git.Tag, line string) git.Latest {
	if line != "" {
		series, ok := s.stream.Series(line)
		if !ok {
			return git.Latest{}
		}
		var inLine []git.Tag
		parsed, parsedOK := semver.ParseSeries(series) // read once, not for each tag
		for _, t := range tags {
			if v, ok := semver.Parse(t.Name); ok && parsedOK && parsed.Contains(v) {
				inLine = append(inLine, t)
			}
		}
		tags = inLine
	}
	if s.accepts != nil {
		var kept []git.Tag
		for _, t := range tags {
			if s.accepts(t.Name) {
				kept = append(kept, t)
			}
		}
		tags = kept
	}
	return git.LatestReleases(tags, s.pattern)
}

func (semverScheme) Stage(string) string { return "" }

func (semverScheme) IsStable(tag string) bool { return !semver.Classify(tag).HasPrerelease }

func (semverScheme) Compare(a, b string) int {
	x, okA := semver.Parse(a)
	y, okB := semver.Parse(b)
	if !okA || !okB {
		return 0
	}
	return semver.Compare(x, y)
}

// calverScheme is the scheme of a project that finds its lines in its tags, over a
// scheme of internal/calver. A prerelease is a tag of the dev stage.
type calverScheme struct {
	lines []string
	inner calver.Scheme
}

func (c calverScheme) Name() string { return c.inner.Name() }

func (c calverScheme) Valid(tag string) bool {
	_, ok := c.inner.Parse(tag)
	return ok
}

// Lines are the lines the config lists, whatever the tags carry: the project is
// used for those lines and not for the others its repository has tags of.
func (c calverScheme) Lines([]git.Tag) []string { return c.lines }

func (c calverScheme) LineOf(tag string) string {
	t, _ := c.inner.Parse(tag)
	return t.Line
}

// Latest is the newest tag of the stable stage, and the newest of the dev stage
// when it is strictly newer than that: a dev tag of the same minute as a release,
// which is the same commit, is not a newer one.
func (c calverScheme) Latest(tags []git.Tag, line string) git.Latest {
	lines := c.Lines(tags)
	if line != "" {
		if !slices.Contains(lines, line) {
			return git.Latest{}
		}
		lines = []string{line}
	}
	release := c.newest(tags, lines, calver.StageStable)
	dev := c.newest(tags, lines, calver.StageDev)
	var out git.Latest
	if release != nil {
		out.Stable = &release.tag
	}
	if dev != nil && (release == nil || dev.at.After(release.at)) {
		out.Prerelease = &dev.tag
	}
	return out
}

// stamped is a tag of the host with the time its name carries.
type stamped struct {
	tag git.Tag
	at  time.Time
}

// newest is the newest tag of a stage among the lines, by the time in its name.
func (c calverScheme) newest(tags []git.Tag, lines []string, stage string) *stamped {
	byName := make(map[string]git.Tag, len(tags))
	for _, t := range tags {
		if _, dup := byName[t.Name]; !dup {
			byName[t.Name] = t
		}
	}
	var best *stamped
	for _, line := range lines {
		selected := calver.Select(c.inner, names(tags), line, stage)
		if len(selected) == 0 {
			continue
		}
		if top := selected[0]; best == nil || top.Time.After(best.at) {
			best = &stamped{tag: byName[top.Name], at: top.Time}
		}
	}
	return best
}

func (c calverScheme) Stage(tag string) string {
	t, _ := c.inner.Parse(tag)
	return t.Stage
}

func (c calverScheme) IsStable(tag string) bool {
	t, ok := c.inner.Parse(tag)
	return ok && t.Stage == calver.StageStable
}

// Compare orders by the date in the names, so a newer date with the same line is a
// newer image, and the prod and dev tags of one commit, which share a date, are
// equal.
func (c calverScheme) Compare(a, b string) int {
	x, okA := c.inner.Parse(a)
	y, okB := c.inner.Parse(b)
	if !okA || !okB {
		return 0
	}
	return x.Time.Compare(y.Time)
}

func names(tags []git.Tag) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.Name
	}
	return out
}

// ForStream returns the scheme of a stream: semver for a semver stream, over the
// lines it lists and the tags it accepts, and the calver scheme it names for a
// calverLineStage stream. A chartVersion stream has none yet.
func ForStream(st *chain.Stream) (Scheme, error) {
	switch st.Scheme() {
	case Semver:
		return semverScheme{stream: st, pattern: st.TagPattern(), accepts: st.TagMatches}, nil
	case CalverLineStage:
		inner, err := calver.Builtin().Get(st.Scheme())
		if err != nil {
			return nil, fmt.Errorf("stream %s: %w", st.Name(), err)
		}
		return calverScheme{lines: st.Lines(), inner: inner}, nil
	}
	return nil, fmt.Errorf("stream %s: the scheme %s is not built", st.Name(), st.Scheme())
}

// ForProject returns the scheme of the one stream of a project. A project with no
// stream, or with several, has no one scheme, and the caller names the stream.
func ForProject(p *chain.Project) (Scheme, error) {
	st, err := p.OnlyStream()
	switch {
	case errors.Is(err, chain.ErrSeveralStreams):
		return nil, fmt.Errorf("%w: name the one to use", err)
	case err != nil:
		return nil, err
	}
	return ForStream(st)
}

// Newest is the newest tag of a line of any kind, a prerelease or a stable tag,
// for the one who asks what a line could have next. A line the stream gives no
// series, as for a universal stream, has every tag the stream accepts: nothing
// tells its tags from the rest of the repository's. It is nil when there is
// none, and an error for a stream whose scheme is not built.
func Newest(st *chain.Stream, tags []git.Tag, line string) (*git.Tag, error) {
	scheme, err := ForStream(st)
	if err != nil {
		return nil, err
	}
	if _, ok := st.Series(line); !ok {
		line = ""
	}
	return scheme.Latest(tags, line).Newest(), nil
}
