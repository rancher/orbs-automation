// Package templates fills the {name} placeholders of the texts gero ship writes:
// the commit message, the PR title, the branch name and the value a strategy
// sets. The syntax is deliberately small, as the placeholders already written
// in config are: a name in braces, with {{ and }} for a literal brace. There
// are no conditions, loops or functions, so a text cannot grow into a program
// that differs per project. The variables a text may use are a closed set that
// the caller supplies, and an unknown one is an error, both when the config is
// validated and when the text is filled.
package templates

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// name is what a placeholder may be called.
var name = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// part is a run of literal text, or one placeholder.
type part struct {
	text          string // the literal, or the placeholder's name
	isPlaceholder bool
}

// parse splits text into literals and placeholders, turning {{ and }} into
// literal braces.
func parse(text string) ([]part, error) {
	var parts []part
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, part{text: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(text); {
		switch c := text[i]; {
		case c == '{' && i+1 < len(text) && text[i+1] == '{':
			lit.WriteByte('{')
			i += 2
		case c == '}' && i+1 < len(text) && text[i+1] == '}':
			lit.WriteByte('}')
			i += 2
		case c == '{':
			end := strings.IndexByte(text[i+1:], '}')
			if end < 0 {
				return nil, fmt.Errorf("unclosed { at position %d", i)
			}
			n := text[i+1 : i+1+end]
			switch {
			case n == "":
				return nil, fmt.Errorf("empty placeholder {} at position %d", i)
			case !name.MatchString(n):
				return nil, fmt.Errorf("{%s} is not a placeholder name: use letters and digits", n)
			}
			flush()
			parts = append(parts, part{text: n, isPlaceholder: true})
			i += end + 2
		case c == '}':
			return nil, fmt.Errorf("unmatched } at position %d: write }} for a literal brace", i)
		default:
			lit.WriteByte(c)
			i++
		}
	}
	flush()
	return parts, nil
}

// Placeholders returns the names used by the text, in order of first use and
// without repeats. It returns an error for a brace that is not part of a
// {name} or an escaped {{ or }}.
func Placeholders(text string) ([]string, error) {
	parts, err := parse(text)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, p := range parts {
		if p.isPlaceholder && !slices.Contains(names, p.text) {
			names = append(names, p.text)
		}
	}
	return names, nil
}

// Validate returns an error for a malformed text or a placeholder that is not in
// allowed.
func Validate(text string, allowed []string) error {
	names, err := Placeholders(text)
	if err != nil {
		return err
	}
	for _, n := range names {
		if !slices.Contains(allowed, n) {
			return fmt.Errorf("unknown placeholder {%s}: use one of %s", n, strings.Join(allowed, ", "))
		}
	}
	return nil
}

// Render fills the placeholders of text from vars. It returns an error for a
// malformed text or a placeholder that vars does not have; a variable whose
// value is empty is filled with nothing.
func Render(text string, vars map[string]string) (string, error) {
	parts, err := parse(text)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, p := range parts {
		if !p.isPlaceholder {
			out.WriteString(p.text)
			continue
		}
		v, ok := vars[p.text]
		if !ok {
			return "", fmt.Errorf("unknown placeholder {%s}", p.text)
		}
		out.WriteString(v)
	}
	return out.String(), nil
}
