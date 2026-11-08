package chain

import (
	"path"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/platform/git"
)

// Stream is what a project releases.
type Stream struct {
	Doc       *v1alpha2.Stream
	project   *Project
	artifacts []*Artifact
}

// Name is the name of the document.
func (s *Stream) Name() string { return s.Doc.Name }

// Artifacts returns what each release of the stream produces, in the order read.
func (s *Stream) Artifacts() []*Artifact { return s.artifacts }

// Project is the project that releases the stream.
func (s *Stream) Project() *Project { return s.project }

// Scheme is how the stream's releases are listed and ordered.
func (s *Stream) Scheme() string { return s.Doc.Spec.Scheme }

// TagPattern is the glob a release's tag matches.
func (s *Stream) TagPattern() string { return s.Doc.Spec.TagPattern }

// TagMatches reports whether a tag is a release of the stream: it matches the
// pattern and none of the excludes.
func (s *Stream) TagMatches(tag string) bool {
	if ok, _ := path.Match(s.Doc.Spec.TagPattern, tag); !ok {
		return false
	}
	for _, ex := range s.Doc.Spec.TagExclude {
		if ok, _ := path.Match(ex, tag); ok {
			return false
		}
	}
	return true
}

// Series is the version series of a line, such as 0.9.x. ok is false when the
// stream does not list the line or lists it with no series.
func (s *Stream) Series(line string) (series string, ok bool) {
	for _, l := range s.Doc.Spec.Lines {
		if lineKey(l.Line) == lineKey(line) {
			return l.Series, l.Series != ""
		}
	}
	return "", false
}

// Serves reports whether releases of the stream are for the line: it is
// universal, or it lists the line.
func (s *Stream) Serves(line string) bool {
	if s.Doc.Spec.Universal {
		return true
	}
	for _, l := range s.Doc.Spec.Lines {
		if lineKey(l.Line) == lineKey(line) {
			return true
		}
	}
	return false
}

// Stages are the stages of the stream: those written, else its scheme's, which for
// a chart is the stages of the project's branches.
func (s *Stream) Stages() []v1alpha2.Stage {
	return s.Doc.EffectiveStages(s.project.Doc.Spec.Branches.Stages)
}

// LineOf returns the line a tag is of, for a stream whose lines are version
// series: the line whose series holds the tag's version. ok is false for a tag in
// no series, for a universal stream, and for a calver stream, whose tag carries
// its line and is read by the tag scheme.
func (s *Stream) LineOf(tag string) (line string, ok bool) {
	if s.Doc.Spec.Universal || s.Doc.Spec.Scheme == v1alpha2.SchemeCalverLineStage {
		return "", false
	}
	for _, l := range s.Doc.Spec.Lines {
		if l.Series != "" && git.InSeries(tag, l.Series) {
			return l.Line, true
		}
	}
	return "", false
}
