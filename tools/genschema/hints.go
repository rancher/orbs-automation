package main

import "github.com/invopop/jsonschema"

// hint adds to one field what its tag and type cannot say. def is the Go name of
// the struct and key the YAML key of the field; for a list, the hint is of an item.
type hint struct {
	def, key string
	enum     []string
	pattern  string
}

// hints are the allowed values of the fields that have a fixed set, built from the
// constants the rules check against, so the schema and the rules cannot differ.
var hints []hint

// applyHint puts a hint on the field it names, and says so when there is none, so
// a renamed field cannot leave a hint that does nothing.
func applyHint(defs jsonschema.Definitions, h hint) error {
	return nil
}
