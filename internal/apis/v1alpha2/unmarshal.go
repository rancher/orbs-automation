package v1alpha2

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// UnmarshalYAML reads each ref as one name or a list of names.
func (r *Refs) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("refs must be a mapping of a key to a name or a list of names")
	}
	refs := make(Refs, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		names, err := namesOf(value)
		if err != nil {
			return fmt.Errorf("refs.%s must be a name or a list of names", key)
		}
		refs[key] = names
	}
	*r = refs
	return nil
}

func namesOf(node *yaml.Node) ([]string, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return []string{node.Value}, nil
	case yaml.SequenceNode:
		names := make([]string, 0, len(node.Content))
		for _, n := range node.Content {
			if n.Kind != yaml.ScalarNode {
				return nil, errors.New("not a name")
			}
			names = append(names, n.Value)
		}
		return names, nil
	}
	return nil, errors.New("not a name or a list")
}

// UnmarshalYAML reads a string as a one-element command and a list as it is.
func (c *Command) UnmarshalYAML(node *yaml.Node) error {
	names, err := namesOf(node)
	if err != nil {
		return errors.New("run must be a string or a list of strings")
	}
	*c = names
	return nil
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
