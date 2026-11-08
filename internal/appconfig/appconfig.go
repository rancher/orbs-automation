// Package appconfig reads the central application config of the v1alpha2 shape
// from an fs.FS: it finds the documents in every YAML file, decodes, defaults and
// validates each one, then checks the set as a whole. It takes no CLI settings;
// internal/chain builds the model commands read from what it returns, and
// internal/chainload and internal/workspace are the only ones that call it.
package appconfig

import (
	"bytes"
	"io/fs"
	"path/filepath"

	"github.com/rancher/orbs-automation/internal/apis/meta"
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
//
// The set is checked only once every document is valid, as a document that was
// dropped would make the documents that refer to it look wrong.
func Load(fsys fs.FS, ctx config.Context) (Config, config.Report) {
	scheme := meta.NewScheme()
	v1alpha2.AddToScheme(scheme)

	var rep config.Report
	var objs []v1alpha2.Object
	origin := map[v1alpha2.Object]string{}
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isYAML(path) {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		found, fileRep := scheme.DecodeReport(bytes.NewReader(data), path, ctx)
		rep.Findings = append(rep.Findings, fileRep.Findings...)
		for _, o := range found {
			origin[o] = path
		}
		objs = append(objs, found...)
		return nil
	})
	if err != nil {
		rep.Findings = append(rep.Findings, config.Finding{Msg: err.Error()})
	}
	if rep.Err() != nil {
		return Config{}, rep
	}
	v1alpha2.ValidateSetFrom(objs, func(o v1alpha2.Object) string { return origin[o] }, ctx, &rep)
	if rep.Err() != nil {
		return Config{}, rep
	}
	return build(objs), rep
}

// isYAML reports whether a path is a YAML file by its extension.
func isYAML(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml"
}

// build sorts the documents into a Config, in the order read.
func build(objs []v1alpha2.Object) Config {
	var c Config
	for _, o := range objs {
		switch o := o.(type) {
		case *v1alpha2.Project:
			c.Projects = append(c.Projects, o)
		case *v1alpha2.Stream:
			c.Streams = append(c.Streams, o)
		case *v1alpha2.Artifact:
			c.Artifacts = append(c.Artifacts, o)
		case *v1alpha2.Holding:
			c.Holdings = append(c.Holdings, o)
		case *v1alpha2.Edit:
			c.Edits = append(c.Edits, o)
		case *v1alpha2.RancherLines:
			if c.Lines == nil {
				c.Lines = o
			}
		}
	}
	return c
}
