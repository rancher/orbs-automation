package v1alpha2

// Stream is what a project releases: the versions, and the lines they serve.
type Stream struct {
	TypeMeta   `yaml:",inline"`
	ObjectMeta `yaml:",inline"`
	Spec       StreamSpec `yaml:"spec"`
}

// StreamSpec is the fields of a Stream.
type StreamSpec struct {
	Scheme             string       `yaml:"scheme"` // semver, calverLineStage or chartVersion
	TagPattern         string       `yaml:"tagPattern"`
	TagExclude         []string     `yaml:"tagExclude"`
	Stages             []Stage      `yaml:"stages"`
	Lines              []StreamLine `yaml:"lines"`
	Universal          bool         `yaml:"universal"`
	RequiresPrerelease bool         `yaml:"requiresPrerelease"`
}

// Stage is a tier of a stream's releases.
type Stage struct {
	Name       string `yaml:"name"`
	Prerelease bool   `yaml:"prerelease"`
}

// StreamLine is one Rancher line a stream serves, and its version series.
type StreamLine struct {
	Line   string `yaml:"line"`
	Series string `yaml:"series"`
}
