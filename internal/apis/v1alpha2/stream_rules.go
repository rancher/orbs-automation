package v1alpha2

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/config"
)

// The schemes a Stream may name, and the defaults of a Stream.
const (
	SchemeSemver          = "semver"
	SchemeCalverLineStage = "calverLineStage"
	SchemeChartVersion    = "chartVersion"

	DefaultScheme     = SchemeSemver
	DefaultTagPattern = "v*"
)

// A series is a major (9.x), or for semver a major and a minor (0.9.x); a chart's
// is a major only.
var (
	semverSeries = regexp.MustCompile(`^([0-9]+)(?:\.([0-9]+))?\.x$`)
	chartSeries  = regexp.MustCompile(`^([0-9]+)\.x$`)
)

// SetDefaults fills the scheme and the tag pattern.
func (s *Stream) SetDefaults() {
	if s.Spec.Scheme == "" {
		s.Spec.Scheme = DefaultScheme
	}
	if s.Spec.TagPattern == "" {
		s.Spec.TagPattern = DefaultTagPattern
	}
}

// Validate checks what a Stream can alone: its scheme, tag patterns, stages and
// lines, and that the series of its lines do not overlap. That its lines are real
// Rancher lines is checked across documents.
func (s *Stream) Validate(_ config.Context, r *config.Report) {
	spec := s.Spec
	if !oneOf(r, "spec.scheme", spec.Scheme, SchemeSemver, SchemeCalverLineStage, SchemeChartVersion) {
		return
	}
	if _, err := path.Match(spec.TagPattern, ""); err != nil {
		r.Errorf("spec.tagPattern", "%q is not a pattern: %v", spec.TagPattern, err)
	}
	for i, p := range spec.TagExclude {
		if _, err := path.Match(p, ""); err != nil {
			r.Errorf(fmt.Sprintf("spec.tagExclude[%d]", i), "%q is not a pattern: %v", p, err)
		}
	}

	seenStage := map[string]bool{}
	for i, st := range spec.Stages {
		at := fmt.Sprintf("spec.stages[%d].name", i)
		switch {
		case st.Name == "":
			r.Errorf(at, "is required")
		case seenStage[st.Name]:
			r.Errorf(at, "%q is listed twice", st.Name)
		}
		seenStage[st.Name] = true
	}

	if spec.Universal && len(spec.Lines) > 0 {
		r.Errorf("spec.lines", "are not given when the stream is universal, which serves every line")
	}
	if !spec.Universal && len(spec.Lines) == 0 {
		r.Errorf("spec", "serves nothing: give its lines or universal")
	}
	s.validateLines(r)
}

func (s *Stream) validateLines(r *config.Report) {
	spec := s.Spec
	seen := map[string]bool{}
	type taken struct {
		line  string
		major uint64
		minor int64 // -1: the whole major
	}
	var series []taken
	for i, l := range spec.Lines {
		at := fmt.Sprintf("spec.lines[%d]", i)
		switch {
		case l.Line == "":
			r.Errorf(at+".line", "is required")
		case seen[l.Line]:
			r.Errorf(at+".line", "%q is listed twice", l.Line)
		default:
			checkLineID(r, at+".line", l.Line)
		}
		seen[l.Line] = true

		switch spec.Scheme {
		case SchemeCalverLineStage:
			if l.Series != "" {
				r.Errorf(at+".series", "is not given: the tag of this scheme carries the line")
			}
			continue
		case SchemeChartVersion:
			if l.Series == "" {
				r.Errorf(at+".series", "is required, such as 111.x")
				continue
			}
			if !chartSeries.MatchString(l.Series) {
				r.Errorf(at+".series", "%q must be a major series such as 111.x", l.Series)
				continue
			}
		default:
			if l.Series == "" {
				r.Errorf(at+".series", "is required, such as 9.x or 0.9.x")
				continue
			}
			if !semverSeries.MatchString(l.Series) {
				r.Errorf(at+".series", "%q must be a series such as 9.x or 0.9.x", l.Series)
				continue
			}
		}

		major, minor := parseSeries(l.Series)
		for _, t := range series {
			if t.major == major && (t.minor < 0 || minor < 0 || t.minor == minor) {
				r.Errorf(at+".series", "%q overlaps the series of line %s", l.Series, t.line)
				break
			}
		}
		series = append(series, taken{line: l.Line, major: major, minor: minor})
	}
}

// parseSeries reads a series that already matched the grammar: its major, and its
// minor or -1 when it has none.
func parseSeries(series string) (major uint64, minor int64) {
	parts := strings.Split(strings.TrimSuffix(series, ".x"), ".")
	major, _ = strconv.ParseUint(parts[0], 10, 64)
	minor = -1
	if len(parts) > 1 {
		m, _ := strconv.ParseInt(parts[1], 10, 64)
		minor = m
	}
	return major, minor
}
