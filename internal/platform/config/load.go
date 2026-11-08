package config

import "go.yaml.in/yaml/v3"

// Load decodes one YAML document into target, sets its defaults and validates it.
// source names the document in findings, normally a file name.
func Load(_ string, _ []byte, _ Schema, _ Context) Report {
	return Report{}
}

// LoadNode is Load for a document that is already parsed. doc is its place in the
// source, from 1, and label says what it is (such as Project "rancher"); both
// appear in the findings.
func LoadNode(_ string, _ int, _ string, _ *yaml.Node, _ Schema, _ Context) Report {
	return Report{}
}
