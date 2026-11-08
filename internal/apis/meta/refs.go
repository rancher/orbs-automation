package meta

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
