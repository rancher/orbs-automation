package chain

import "github.com/rancher/orbs-automation/internal/apis/v1alpha2"

// Stream is what a project releases.
type Stream struct {
	Doc *v1alpha2.Stream
}

// Name is the name of the document.
func (s *Stream) Name() string { return "" }

// Project is the project that releases the stream.
func (s *Stream) Project() *Project { return nil }

// Scheme is how the stream's releases are listed and ordered.
func (s *Stream) Scheme() string { return "" }

// TagPattern is the glob a release's tag matches.
func (s *Stream) TagPattern() string { return "" }

// TagMatches reports whether a tag is a release of the stream: it matches the
// pattern and none of the excludes.
func (s *Stream) TagMatches(_ string) bool { return false }

// Series is the version series of a line, such as 0.9.x. ok is false when the
// stream does not list the line or lists it with no series.
func (s *Stream) Series(_ string) (series string, ok bool) { return "", false }

// Serves reports whether releases of the stream are for the line: it is
// universal, or it lists the line.
func (s *Stream) Serves(_ string) bool { return false }

// Stages are the stages of the stream: those written, else its scheme's, which for
// a chart is the stages of the project's branches.
func (s *Stream) Stages() []v1alpha2.Stage { return nil }

// LineOf returns the line a tag is of, for a stream whose lines are version
// series: the line whose series holds the tag's version. ok is false for a tag in
// no series, for a universal stream, and for a calver stream, whose tag carries
// its line and is read by the tag scheme.
func (s *Stream) LineOf(_ string) (line string, ok bool) { return "", false }
