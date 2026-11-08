package v1alpha2

import (
	"errors"

	"go.yaml.in/yaml/v3"
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
