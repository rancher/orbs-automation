package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// Load decodes one YAML document into target, sets its defaults and validates it.
// source names the document in findings, normally a file name. Empty data is an
// empty object. A document that does not decode is reported and is neither
// defaulted nor validated. A key the schema does not know is an error.
func Load(source string, data []byte, target Schema, ctx Context) Report {
	return load(source, data, target, ctx, false)
}

// LoadTolerant is Load for a document where a key the schema does not know is a
// warning, not an error: the known keys are decoded, defaulted and validated, and
// each unknown one is a warning finding that names its line. Any other problem
// is an error as in Load.
func LoadTolerant(source string, data []byte, target Schema, ctx Context) Report {
	return load(source, data, target, ctx, true)
}

func load(source string, data []byte, target Schema, ctx Context, tolerant bool) Report {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		r := NewReport(source, 0, "")
		r.add(Error, "", err.Error())
		return r
	}
	return loadNode(source, 0, "", &node, target, ctx, tolerant)
}

// LoadNode is Load for a document that is already parsed. doc is its place in the
// source, from 1, and label says what it is (such as Project "rancher"); both
// appear in the findings.
func LoadNode(source string, doc int, label string, node *yaml.Node, target Schema, ctx Context) Report {
	return loadNode(source, doc, label, node, target, ctx, false)
}

// unknownField is how yaml reports a key the target has no field for.
var unknownField = regexp.MustCompile(`^line (\d+): field (\S+) not found in type `)

func loadNode(source string, doc int, label string, node *yaml.Node, target Schema, ctx Context, tolerant bool) Report {
	r := NewReport(source, doc, label)
	if node != nil && node.Kind != 0 {
		// yaml.Node.Decode cannot reject unknown fields, so the document is encoded
		// again and decoded with them rejected.
		raw, err := yaml.Marshal(node)
		if err != nil {
			r.add(Error, "", err.Error())
			return r
		}
		dec := yaml.NewDecoder(bytes.NewReader(raw))
		dec.KnownFields(true)
		if err := dec.Decode(target); err != nil && !errors.Is(err, io.EOF) {
			if !tolerant || !r.tolerate(err) {
				r.add(Error, "", err.Error())
				return r
			}
		}
	}
	target.SetDefaults()
	target.Validate(ctx, &r)
	return r
}

// tolerate turns the unknown keys of a decode error into warnings and reports
// whether that was all it held. The decoder goes on after an unknown key, so the
// known ones are already in the target.
func (r *Report) tolerate(err error) bool {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return false
	}
	for _, msg := range te.Errors {
		if !unknownField.MatchString(msg) {
			return false
		}
	}
	for _, msg := range te.Errors {
		m := unknownField.FindStringSubmatch(msg)
		r.add(Warning, "", fmt.Sprintf("line %s: unknown key %q, ignored", m[1], m[2]))
	}
	return true
}
