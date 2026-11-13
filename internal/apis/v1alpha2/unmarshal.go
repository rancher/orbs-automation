package v1alpha2

import (
	"bytes"
	"errors"

	"go.yaml.in/yaml/v4"
)

// UnmarshalYAML reads a string as a one-element command and a list as it is.
func (c *Command) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*c = Command{node.Value}
		return nil
	case yaml.SequenceNode:
		cmd := make(Command, 0, len(node.Content))
		for _, n := range node.Content {
			if n.Kind != yaml.ScalarNode {
				return errors.New("run must be a string or a list of strings")
			}
			cmd = append(cmd, n.Value)
		}
		*c = cmd
		return nil
	}
	return errors.New("run must be a string or a list of strings")
}

// UnmarshalYAML reads a string as one branch, and a mapping as a branch for each
// stage.
func (b *Branch) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*b = Branch{Name: node.Value}
		return nil
	case yaml.MappingNode:
		var stages map[string]string
		if err := node.Decode(&stages); err != nil {
			return errors.New("branch must be a string or a map of stage to branch")
		}
		*b = Branch{Stages: stages}
		return nil
	}
	return errors.New("branch must be a string or a map of stage to branch")
}

// UnmarshalYAML reads a mapping as one location and a list as a list of them. A
// location with a field it does not have is an error, as it is anywhere else.
func (l *Locations) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.MappingNode:
		at, err := decodeLocation(node)
		if err != nil {
			return err
		}
		*l = Locations{at}
		return nil
	case yaml.SequenceNode:
		locations := make(Locations, 0, len(node.Content))
		for _, n := range node.Content {
			at, err := decodeLocation(n)
			if err != nil {
				return err
			}
			locations = append(locations, at)
		}
		*l = locations
		return nil
	}
	return errors.New("at must be a location or a list of locations")
}

// decodeLocation reads one location, refusing a field it does not have. A node
// decodes without the decoder's strictness, so it is written out and read again
// with it.
func decodeLocation(node *yaml.Node) (At, error) {
	if node.Kind != yaml.MappingNode {
		return At{}, errors.New("a location must be a mapping of file, key, format and lines")
	}
	text, err := yaml.Marshal(node)
	if err != nil {
		return At{}, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(text))
	dec.KnownFields(true)
	var at At
	if err := dec.Decode(&at); err != nil {
		return At{}, err
	}
	return at, nil
}
