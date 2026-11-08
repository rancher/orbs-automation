package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/invopop/jsonschema"

	"github.com/rancher/orbs-automation/internal/apis/meta"
	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
	"github.com/rancher/orbs-automation/internal/platform/clicfg"
)

const module = "github.com/rancher/orbs-automation"

// commentDirs are the packages whose Go comments become descriptions, relative to
// the repository root.
var commentDirs = []string{
	"internal/apis/lines",
	"internal/apis/meta",
	"internal/apis/v1alpha2",
	"internal/platform/clicfg",
}

const defsPrefix = "#/$defs/"

// build returns the schemas to write, by file name relative to the output
// directory, with slashes. It reads the Go sources for comments, so it runs from
// the repository root.
func build() (map[string][]byte, error) {
	r, err := newReflector()
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}

	app, err := kindSchemas(r)
	if err != nil {
		return nil, err
	}
	for kind, s := range app.byKind {
		if files["v1alpha2/"+strings.ToLower(kind)+".schema.json"], err = encode(s); err != nil {
			return nil, err
		}
	}
	if files["gero.v1alpha2.schema.json"], err = encode(app.all); err != nil {
		return nil, err
	}

	user := r.ReflectFromType(reflect.TypeOf(clicfg.UserFile()))
	if err := applyHints(user.Definitions, userHints); err != nil {
		return nil, err
	}
	user.Title = "gero user config"
	if files["user-config.schema.json"], err = encode(user); err != nil {
		return nil, err
	}
	return files, nil
}

func newReflector() (*jsonschema.Reflector, error) {
	r := &jsonschema.Reflector{
		FieldNameTag:               "yaml",
		RequiredFromJSONSchemaTags: true,
		Anonymous:                  true,
		Mapper:                     mapper,
	}
	for _, dir := range commentDirs {
		if err := r.AddGoComments(module, dir); err != nil {
			return nil, fmt.Errorf("read the comments of %s: %w", dir, err)
		}
	}
	return r, nil
}

// mapper describes the types that decode themselves from more than one form, which
// reflection cannot see.
func mapper(t reflect.Type) *jsonschema.Schema {
	text := func() *jsonschema.Schema { return &jsonschema.Schema{Type: "string"} }
	listOfText := func() *jsonschema.Schema { return &jsonschema.Schema{Type: "array", Items: text()} }
	switch t {
	case reflect.TypeFor[time.Time]():
		return &jsonschema.Schema{Type: "string", Format: "date"}
	case reflect.TypeFor[v1alpha2.Branch]():
		return &jsonschema.Schema{OneOf: []*jsonschema.Schema{
			text(),
			{Type: "object", AdditionalProperties: text()},
		}}
	case reflect.TypeFor[v1alpha2.Command]():
		return &jsonschema.Schema{OneOf: []*jsonschema.Schema{text(), listOfText()}}
	case reflect.TypeFor[meta.Refs]():
		return &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{text(), listOfText()},
		}}
	}
	return nil
}

// kinds are the schemas of the application config.
type kinds struct {
	byKind map[string]*jsonschema.Schema // one for each kind, with the definitions it reaches
	all    *jsonschema.Schema            // every kind, picked by the kind of a document
}

func kindSchemas(r *jsonschema.Reflector) (kinds, error) {
	scheme := meta.NewScheme()
	v1alpha2.AddToScheme(scheme)

	defs := jsonschema.Definitions{}
	kindNames := scheme.Kinds(v1alpha2.APIVersion)
	for _, kind := range kindNames {
		obj, _ := scheme.New(v1alpha2.APIVersion, kind)
		for name, def := range r.ReflectFromType(reflect.TypeOf(obj)).Definitions {
			defs[name] = def
		}
	}
	if err := applyHints(defs, appHints); err != nil {
		return kinds{}, err
	}
	for _, kind := range kindNames {
		if err := pinKind(defs, kind); err != nil {
			return kinds{}, err
		}
	}

	out := kinds{byKind: map[string]*jsonschema.Schema{}}
	all := &jsonschema.Schema{
		Version:     jsonschema.Version,
		Title:       "gero " + v1alpha2.APIVersion + " documents",
		Definitions: defs,
	}
	for _, kind := range kindNames {
		out.byKind[kind] = &jsonschema.Schema{
			Version:     jsonschema.Version,
			Title:       "gero " + v1alpha2.APIVersion + " " + kind,
			Ref:         defsPrefix + kind,
			Definitions: reachable(defs, kind),
		}
		is := jsonschema.NewProperties()
		is.Set("kind", &jsonschema.Schema{Const: kind})
		all.AllOf = append(all.AllOf, &jsonschema.Schema{
			If:   &jsonschema.Schema{Properties: is, Required: []string{"kind"}},
			Then: &jsonschema.Schema{Ref: defsPrefix + kind},
		})
	}
	out.all = all
	return out, nil
}

// pinKind makes the envelope of a kind exact: the one apiVersion and kind it can
// have, a name that is a slug, and the four keys every document has.
func pinKind(defs jsonschema.Definitions, kind string) error {
	def := defs[kind]
	if def == nil || def.Properties == nil {
		return fmt.Errorf("no definition for the kind %s", kind)
	}
	for key, want := range map[string]any{"apiVersion": v1alpha2.APIVersion, "kind": kind} {
		field, ok := def.Properties.Get(key)
		if !ok {
			return fmt.Errorf("%s has no %s", kind, key)
		}
		field.Const = want
	}
	name, ok := def.Properties.Get("name")
	if !ok {
		return fmt.Errorf("%s has no name", kind)
	}
	name.Pattern = meta.SlugPattern
	def.Required = []string{"apiVersion", "kind", "name", "spec"}
	return nil
}

// reachable returns the definitions a definition refers to, itself included, and
// those they refer to in turn.
func reachable(defs jsonschema.Definitions, root string) jsonschema.Definitions {
	out := jsonschema.Definitions{}
	var visit func(name string)
	var walk func(s *jsonschema.Schema)
	walk = func(s *jsonschema.Schema) {
		if s == nil {
			return
		}
		if name, ok := strings.CutPrefix(s.Ref, defsPrefix); ok {
			visit(name)
		}
		if s.Properties != nil {
			for p := s.Properties.Oldest(); p != nil; p = p.Next() {
				walk(p.Value)
			}
		}
		walk(s.Items)
		walk(s.AdditionalProperties)
		for _, o := range s.OneOf {
			walk(o)
		}
	}
	visit = func(name string) {
		if _, done := out[name]; done || defs[name] == nil {
			return
		}
		out[name] = defs[name]
		walk(defs[name])
	}
	visit(root)
	return out
}

// encode writes a schema as indented JSON with a trailing newline, without
// escaping the characters of a description or a pattern.
func encode(s *jsonschema.Schema) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
