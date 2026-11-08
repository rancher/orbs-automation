// Package calver reads the tags of projects whose lines are found in their tags
// and not in per-line branches, such as rancher/rancher-assets. A scheme says
// which line and stage a tag is for and how tags are ordered. It works on tag
// names alone, with no repository and no I/O (docs/specs/CHAIN_TRACING.md).
package calver

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
)

// Tag is a tag name read by a scheme.
type Tag struct {
	Name  string
	Line  string    // the Rancher line, such as v2.16
	Stage string    // v1alpha2.StageDev or v1alpha2.StageStable
	Time  time.Time // UTC, to the minute
}

// Scheme is one way a project spells a line, a stage and a time in its tags.
type Scheme interface {
	// Name is what the scheme of a stream calls it in config.
	Name() string
	// Parse reads a tag name. It is false for a name that is not a release of the
	// scheme, which is then not a release of any line.
	Parse(name string) (Tag, bool)
	// Compare orders two tags of one line and stage: negative when a is older.
	Compare(a, b Tag) int
}

// LineStage is calverLineStage: v{minor}-{YYYYMMDD}T{HHMM}Z, with -dev after it
// for the dev stage, as the Assets tags are.
type LineStage struct{}

// Name is calverLineStage.
func (LineStage) Name() string { return v1alpha2.SchemeCalverLineStage }

// lineStage is the whole tag: the line, the time and the optional dev suffix.
var lineStage = regexp.MustCompile(`^(v\d+\.\d+)-(\d{8}T\d{4}Z)(-dev)?$`)

// timeLayout is the timestamp of a tag, to the minute and in UTC.
const timeLayout = "20060102T1504Z"

// Parse reads a calverLineStage tag. The suffix is the stage: -dev is dev and none
// is release. A name with an impossible date or time is not a release.
func (LineStage) Parse(name string) (Tag, bool) {
	m := lineStage.FindStringSubmatch(name)
	if m == nil {
		return Tag{}, false
	}
	t, err := time.Parse(timeLayout, m[2])
	if err != nil {
		return Tag{}, false
	}
	stage := v1alpha2.StageStable
	if m[3] != "" {
		stage = v1alpha2.StageDev
	}
	return Tag{Name: name, Line: m[1], Stage: stage, Time: t}, true
}

// Compare orders by time, then by name, so two tags of one minute still have an
// order.
func (LineStage) Compare(a, b Tag) int {
	if c := a.Time.Compare(b.Time); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}

// Registry maps the names schemes claim to their code.
type Registry struct {
	schemes map[string]Scheme
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{schemes: map[string]Scheme{}} }

// Builtin returns a registry with every scheme gero has. The list is explicit on
// purpose, as bump.Builtin's is.
func Builtin() *Registry {
	r := NewRegistry()
	r.Register(LineStage{})
	return r
}

// Register adds a scheme under the name it claims. Two schemes claiming one name
// is a programming error, so it panics.
func (r *Registry) Register(s Scheme) {
	if _, dup := r.schemes[s.Name()]; dup {
		panic(fmt.Sprintf("calver: two schemes claim the name %q", s.Name()))
	}
	r.schemes[s.Name()] = s
}

// Get returns the scheme of a name; the error lists what exists.
func (r *Registry) Get(name string) (Scheme, error) {
	if s, ok := r.schemes[name]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("tag scheme %q is not known: use one of %s", name, strings.Join(r.Names(), ", "))
}

// Names lists the schemes, sorted.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.schemes))
	for n := range r.schemes {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Select returns the tags among names that are for the line and stage, newest
// first. A name that is not a release of the scheme is skipped.
func Select(s Scheme, names []string, line, stage string) []Tag {
	var out []Tag
	for _, name := range names {
		if t, ok := s.Parse(name); ok && t.Line == line && t.Stage == stage {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b Tag) int { return s.Compare(b, a) })
	return out
}
