// Package config loads the central application config: the YAML documents under
// config/ (Project, RancherLines, External). It is framework-free and takes an
// fs.FS, so it is table-testable. It does not read CLI settings; the caller
// locates the directory (see internal/workspace). Shapes are documented in
// config/README.md.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"go.yaml.in/yaml/v3"
)

// APIVersion is the only apiVersion the loader accepts until gero v1.0.0.
const APIVersion = "gero/v1alpha1"

// Document kinds.
const (
	KindProject      = "Project"
	KindRancherLines = "RancherLines"
	KindExternal     = "External"
)

// Config is everything found in one config directory, as written (no defaults
// applied yet).
type Config struct {
	Projects  []Project
	Externals []External
	Lines     *RancherLines // nil when the directory has no RancherLines document
}

// Project is a repo we model.
type Project struct {
	Name         string       `yaml:"-"`
	Repo         Repo         `yaml:"repo"`
	Release      Release      `yaml:"release"`
	PullRequests PullRequests `yaml:"pullRequests"`
	Sources      []Source     `yaml:"sources"`
}

// Repo identifies the project's repository.
type Repo struct {
	Name          string `yaml:"name"` // owner/name slug
	Owned         bool   `yaml:"owned"`
	DefaultBranch string `yaml:"defaultBranch"`
}

// Release is the project's release policy.
type Release struct {
	Branches           string        `yaml:"branches"` // single or perLine
	TagPattern         string        `yaml:"tagPattern"`
	RequiresPrerelease bool          `yaml:"requiresPrerelease"`
	Lines              []ProjectLine `yaml:"lines"`
}

// ProjectLine maps one Rancher line to this project's series and branch.
type ProjectLine struct {
	Line    string `yaml:"line"`
	Version string `yaml:"version"`
	Branch  string `yaml:"branch"`
}

// PullRequests holds the conventions for PRs opened against the project.
type PullRequests struct {
	Title        string   `yaml:"title"`
	BranchPrefix string   `yaml:"branchPrefix"`
	Labels       []string `yaml:"labels"`
}

// Source is a project this one takes updates from.
type Source struct {
	From   string `yaml:"from"`
	Accept Accept `yaml:"accept"`
	Update Update `yaml:"update"`
}

// Accept says which of a source's releases are taken.
type Accept struct {
	When  string `yaml:"when"`  // stable or all
	Lines string `yaml:"lines"` // all or universal
}

// Update says how a source's release is applied.
type Update struct {
	Strategy string            `yaml:"strategy"`
	Params   map[string]string `yaml:"params"`
	PostEdit []string          `yaml:"postEdit"`
}

// RancherLines is the set of Rancher lines and facts about each.
type RancherLines struct {
	Name  string `yaml:"-"`
	Lines []Line `yaml:"lines"`
}

// Line is one Rancher line.
type Line struct {
	ID         string          `yaml:"id"`
	Phase      string          `yaml:"phase"` // development, ga, eom or eol
	Kubernetes KubernetesRange `yaml:"kubernetes"`
}

// KubernetesRange is the supported Kubernetes minor range of a line.
type KubernetesRange struct {
	Min string `yaml:"min"`
	Max string `yaml:"max"`
}

// External is a third-party project we track or notify.
type External struct {
	Name          string         `yaml:"-"`
	Repo          Repo           `yaml:"repo"`
	Relationships []Relationship `yaml:"relationships"`
}

// Relationship ties an External to one of our projects.
type Relationship struct {
	Project string `yaml:"project"`
	Action  string `yaml:"action"` // trackOnly, react or notify
}

// envelope is the shape shared by every document.
type envelope[T any] struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Name       string `yaml:"name"`
	Spec       T      `yaml:"spec"`
}

// fileKinds maps a file name suffix to the one kind it may hold.
var fileKinds = []struct{ suffix, kind string }{
	{".project.yaml", KindProject},
	{".external.yaml", KindExternal},
	{"rancher.lines.yaml", KindRancherLines},
}

// Load reads every config file at the root of fsys: <name>.project.yaml,
// <name>.external.yaml and rancher.lines.yaml. Files may hold several
// documents. Unknown keys, an unsupported apiVersion, a kind that does not
// match the file name suffix and duplicate names are errors that name the file.
func Load(fsys fs.FS) (Config, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	var cfg Config
	seen := map[string]string{} // kind/name to the file that defined it
	for _, e := range entries {
		kind, ok := kindForFile(e.Name())
		if e.IsDir() || !ok {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return Config{}, fmt.Errorf("config: %w", err)
		}
		if err := loadFile(&cfg, seen, e.Name(), kind, data); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func kindForFile(name string) (string, bool) {
	for _, fk := range fileKinds {
		if strings.HasSuffix(name, fk.suffix) {
			return fk.kind, true
		}
	}
	return "", false
}

func loadFile(cfg *Config, seen map[string]string, file, fileKind string, data []byte) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for i := 1; ; i++ {
		var node yaml.Node
		if err := dec.Decode(&node); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if err := loadDocument(cfg, seen, file, fileKind, &node); err != nil {
			return fmt.Errorf("%s: document %d: %w", file, i, err)
		}
	}
}

func loadDocument(cfg *Config, seen map[string]string, file, fileKind string, node *yaml.Node) error {
	var head envelope[yaml.Node]
	if err := node.Decode(&head); err != nil {
		return err
	}
	if head.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion %q is not supported, want %q", head.APIVersion, APIVersion)
	}
	if head.Kind != fileKind {
		return fmt.Errorf("kind %q does not belong in this file, want %q", head.Kind, fileKind)
	}
	if head.Name == "" {
		return errors.New("name is required")
	}
	// yaml.Node.Decode cannot be strict, so re-encode the document and decode it
	// again with unknown keys rejected.
	raw, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	key := head.Kind + "/" + head.Name
	if prev, dup := seen[key]; dup {
		return fmt.Errorf("duplicate %s %q (also in %s)", head.Kind, head.Name, prev)
	}
	seen[key] = file

	switch head.Kind {
	case KindProject:
		p, err := decodeStrict[Project](raw)
		if err != nil {
			return err
		}
		p.Name = head.Name
		cfg.Projects = append(cfg.Projects, p)
	case KindExternal:
		x, err := decodeStrict[External](raw)
		if err != nil {
			return err
		}
		x.Name = head.Name
		cfg.Externals = append(cfg.Externals, x)
	case KindRancherLines:
		l, err := decodeStrict[RancherLines](raw)
		if err != nil {
			return err
		}
		if cfg.Lines != nil {
			return errors.New("duplicate RancherLines document")
		}
		l.Name = head.Name
		cfg.Lines = &l
	}
	return nil
}

func decodeStrict[T any](raw []byte) (T, error) {
	var env envelope[T]
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	err := dec.Decode(&env)
	return env.Spec, err
}
