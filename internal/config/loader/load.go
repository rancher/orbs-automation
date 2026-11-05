// Package loader reads the central application config from an fs.FS and checks
// it: YAML decoding, defaults and validation. It is framework-free and takes
// no CLI settings; internal/workspace locates the directory and is the only
// importer, so commands never see config that skipped defaults or validation.
// The types are in internal/config.
package loader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"go.yaml.in/yaml/v3"
)

// envelope is the shape shared by every document.
type envelope[T any] struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Name       string `yaml:"name"`
	Spec       T      `yaml:"spec"`
}

// fileKinds maps a file name suffix to the one kind it may hold.
var fileKinds = []struct{ suffix, kind string }{
	{".project.yaml", config.KindProject},
	{".external.yaml", config.KindExternal},
	{"rancher.lines.yaml", config.KindRancherLines},
}

// Load reads every config file at the root of fsys: <name>.project.yaml,
// <name>.external.yaml and rancher.lines.yaml. Files may hold several
// documents. Unknown keys, an unsupported apiVersion, a kind that does not
// match the file name suffix and duplicate names are errors that name the file.
func Load(fsys fs.FS) (config.Config, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return config.Config{}, fmt.Errorf("config: %w", err)
	}
	var cfg config.Config
	seen := map[string]string{} // kind/name to the file that defined it
	for _, e := range entries {
		kind, ok := kindForFile(e.Name())
		if e.IsDir() || !ok {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return config.Config{}, fmt.Errorf("config: %w", err)
		}
		if err := loadFile(&cfg, seen, e.Name(), kind, data); err != nil {
			return config.Config{}, err
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

func loadFile(cfg *config.Config, seen map[string]string, file, fileKind string, data []byte) error {
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

func loadDocument(cfg *config.Config, seen map[string]string, file, fileKind string, node *yaml.Node) error {
	var head envelope[yaml.Node]
	if err := node.Decode(&head); err != nil {
		return err
	}
	if head.APIVersion != config.APIVersion {
		return fmt.Errorf("apiVersion %q is not supported, want %q", head.APIVersion, config.APIVersion)
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
	case config.KindProject:
		p, err := decodeStrict[config.Project](raw)
		if err != nil {
			return err
		}
		p.Name = head.Name
		cfg.Projects = append(cfg.Projects, p)
	case config.KindExternal:
		x, err := decodeStrict[config.External](raw)
		if err != nil {
			return err
		}
		x.Name = head.Name
		cfg.Externals = append(cfg.Externals, x)
	case config.KindRancherLines:
		l, err := decodeStrict[config.RancherLines](raw)
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
