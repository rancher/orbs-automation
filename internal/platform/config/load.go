package config

import (
	"bytes"
	"errors"
	"io"

	"go.yaml.in/yaml/v3"
)

// Load decodes one YAML document into target, sets its defaults and validates it.
// source names the document in findings, normally a file name. Empty data is an
// empty object. A document that does not decode is reported and is neither
// defaulted nor validated.
func Load(source string, data []byte, target Schema, ctx Context) Report {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		r := newReport(source, 0, "")
		r.add(Error, "", err.Error())
		return r
	}
	return LoadNode(source, 0, "", &node, target, ctx)
}

// LoadNode is Load for a document that is already parsed. doc is its place in the
// source, from 1, and label says what it is (such as Project "rancher"); both
// appear in the findings.
func LoadNode(source string, doc int, label string, node *yaml.Node, target Schema, ctx Context) Report {
	r := newReport(source, doc, label)
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
			r.add(Error, "", err.Error())
			return r
		}
	}
	target.SetDefaults()
	target.Validate(ctx, &r)
	return r
}
