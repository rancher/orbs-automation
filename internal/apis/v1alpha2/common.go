package v1alpha2

// Command is a program or script path with its arguments, written as one string
// or a list. It is never split like a shell command line.
type Command []string

// Step is a command run around an edit, optionally only for a range of lines.
type Step struct {
	Run   Command `yaml:"run"`
	Since string  `yaml:"since"`
	Until string  `yaml:"until"`
	From  string  `yaml:"from"` // where the program comes from
}
