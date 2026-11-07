package repl

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

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

// replOnly are the commands only the REPL has, beside the tree's own.
var replOnly = []string{"pwd", "ls", "exit"}

// Complete returns the words that may complete the word at the end of line, all
// of which start with what is typed of it, sorted and each once.
func (c Completion) Complete(line string) []string {
	words, toComplete := splitForCompletion(line)
	var out []string
	switch {
	case len(words) == 0:
		out = append(c.tree(nil, toComplete), withPrefix(replOnly, toComplete)...)
	case words[0] == "pwd", words[0] == "exit", words[0] == "shell":
		// They take no argument.
	case words[0] == "cd":
		if len(words) == 1 {
			out = withPrefix(c.projects(), toComplete)
		}
	case words[0] == "ls":
		if len(words) == 1 {
			out = c.ls(toComplete)
		}
	default:
		out = c.tree(words, toComplete)
	}
	sort.Strings(out)
	return slices.Compact(out)
}

func (c Completion) tree(words []string, toComplete string) []string {
	if c.Tree == nil {
		return nil
	}
	return c.Tree(words, toComplete)
}

func (c Completion) projects() []string {
	if c.Projects == nil {
		return nil
	}
	return c.Projects()
}

// ls completes what ls takes: a directory under the working directory, written
// as far as it is typed, or a project written with a leading @.
func (c Completion) ls(toComplete string) []string {
	var out []string
	for _, p := range c.projects() {
		out = append(out, "@"+p)
	}
	if c.Dir == nil {
		return withPrefix(out, toComplete)
	}
	dirPart := toComplete[:strings.LastIndex(toComplete, "/")+1]
	if filepath.IsAbs(dirPart) {
		return withPrefix(out, toComplete)
	}
	dir := filepath.Join(c.Dir(), dirPart)
	if rel, err := filepath.Rel(c.Dir(), dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return withPrefix(out, toComplete)
	}
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, dirPart+e.Name()+"/")
		}
	}
	return withPrefix(out, toComplete)
}

// splitForCompletion cuts a line into the words that are complete and the one
// being typed, which is empty after a blank.
func splitForCompletion(line string) (words []string, toComplete string) {
	fields := strings.Fields(line)
	if line == "" || strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
		return fields, ""
	}
	return fields[:len(fields)-1], fields[len(fields)-1]
}

// withPrefix is the words that start with prefix.
func withPrefix(words []string, prefix string) []string {
	var out []string
	for _, w := range words {
		if strings.HasPrefix(w, prefix) {
			out = append(out, w)
		}
	}
	return out
}
