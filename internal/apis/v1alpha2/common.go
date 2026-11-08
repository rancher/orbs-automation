package v1alpha2

// Range limits something to the Rancher lines from Since to Until, either of
// which may be left out. The zero Range is every line.
type Range struct {
	Since string `yaml:"since"` // a line id: from this line on
	Until string `yaml:"until"` // a line id: up to and including this line
}

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
