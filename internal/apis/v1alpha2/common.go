package v1alpha2

import "strings"

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

// Contains reports whether a line is in the range, its ends included. A line may
// be written with or without its v, in any case. A line that does not read as a
// Rancher line is counted as in the range, as an end that does not read is as open.
func (r Range) Contains(line string) bool {
	n, ok := readLine(line)
	if !ok {
		return true
	}
	if since, ok := readLine(r.Since); ok && lineBefore(n, since) {
		return false
	}
	if until, ok := readLine(r.Until); ok && lineBefore(until, n) {
		return false
	}
	return true
}

// readLine reads a line id written with or without its v, in any case, as its
// major and minor. It is false for text that is not a line.
func readLine(s string) ([2]int, bool) {
	s = strings.ToLower(s)
	if !strings.HasPrefix(s, "v") {
		s = "v" + s
	}
	if !lineID.MatchString(s) {
		return [2]int{}, false
	}
	return lineNumbers(s), true
}

// lineBefore reports whether line a, as its major and minor, is before line b.
func lineBefore(a, b [2]int) bool { return a[0] < b[0] || (a[0] == b[0] && a[1] < b[1]) }
