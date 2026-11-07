package repl

// Completion gives the completions of a line typed at the REPL: the words of the
// command tree, the commands only the REPL has, the projects cd and ls take, and
// the directories ls takes. The tree is the one gero shell install completes in a
// shell, so a command completes the same in both places (docs/specs/REPL.md).
type Completion struct {
	// Tree returns the completions of the command tree for the words before the
	// one being completed and that word, as cobra gives them. Nil gives none.
	Tree func(words []string, toComplete string) []string
	// Projects lists the projects by config name. Nil gives none.
	Projects func() []string
	// Dir is the working directory. Nil is none.
	Dir func() string
}

// Complete returns the words that may complete the word at the end of line, all
// of which start with what is typed of it.
func (c Completion) Complete(line string) []string { return nil }
