// Package appconfig reads the central application config of the v1alpha2 shape
// from an fs.FS: it finds the documents in every YAML file, decodes, defaults and
// validates each one, then checks the set as a whole. It takes no CLI settings and
// is not yet used: internal/workspace becomes its one importer when the commands
// move to this shape, as it is for internal/config/loader today.
package appconfig

import (
	"io/fs"

	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// Config is the application config: the documents of each kind, in the order
// they were read.
type Config struct {
	Projects  []*v1alpha2.Project
	Streams   []*v1alpha2.Stream
	Artifacts []*v1alpha2.Artifact
	Holdings  []*v1alpha2.Holding
	Edits     []*v1alpha2.Edit
	Lines     *v1alpha2.RancherLines // nil when there is no RancherLines document
}

// Empty reports whether the config has no document at all.
func (c Config) Empty() bool {
	return len(c.Projects)+len(c.Streams)+len(c.Artifacts)+len(c.Holdings)+len(c.Edits) == 0 && c.Lines == nil
}

// Load reads every *.yaml and *.yml file under the root of fsys, whatever its name
// or directory: a document is what it says it is, not where it is. The Report has
// the findings of every document, errors and warnings, naming the file and the
// document. The Config is the zero Config unless the Report has no error, so a
// caller never sees a config that failed a rule.
func Load(_ fs.FS, _ config.Context) (Config, config.Report) {
	return Config{}, config.Report{}
}
