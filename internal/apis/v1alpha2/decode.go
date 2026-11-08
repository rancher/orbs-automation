package v1alpha2

import (
	"io"

	"github.com/rancher/orbs-automation/internal/apis/meta"
)

// AddToScheme registers the kinds of this version with a Scheme.
func AddToScheme(s *meta.Scheme) {
	s.AddKind(APIVersion, KindProject, func() Object { return new(Project) })
	s.AddKind(APIVersion, KindStream, func() Object { return new(Stream) })
	s.AddKind(APIVersion, KindArtifact, func() Object { return new(Artifact) })
	s.AddKind(APIVersion, KindHolding, func() Object { return new(Holding) })
	s.AddKind(APIVersion, KindEdit, func() Object { return new(Edit) })
	s.AddKind(APIVersion, KindRancherLines, func() Object { return new(RancherLines) })
}

// Decode reads every document of a YAML stream of this version into the typed
// object of its kind; see meta.Scheme.Decode. A caller that reads several
// versions builds one Scheme with each version's AddToScheme.
func Decode(r io.Reader, source string) ([]Object, error) {
	s := meta.NewScheme()
	AddToScheme(s)
	return s.Decode(r, source)
}
