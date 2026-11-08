package repl

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
)

// The placeholders of a template. The set grows when a user asks for something it
// cannot say, and each is cheap: it comes from the working directory and the
// config already loaded, with no command run, no network and no git read
// (docs/specs/REPL.md#the-prompt).
const (
	placeholderProject = "project"
	placeholderDir     = "dir"
	placeholderPath    = "path"
)

func known(name string) bool {
	return name == placeholderProject || name == placeholderDir || name == placeholderPath
}

// Template is a parsed prompt template: plain text and placeholders, checked
// once so that nothing in it can fail while the REPL runs
// (docs/specs/REPL.md#the-prompt).
type Template struct{ parts []part }

// part is plain text, or a placeholder with the text it writes when the value is
// not empty.
type part struct {
	literal string
	name    string
	text    string
	hasText bool
}

// Values are what the placeholders of a template stand for.
type Values struct {
	// Project is the config name of the project the working directory is in, or
	// empty outside one.
	Project string
	// Dir is the working directory, and Home the home directory, which {path}
	// writes as ~.
	Dir, Home string
}

// ParseTemplate checks a template and prepares it to be drawn. {name} is the
// value of a placeholder, {name:text} is text, with %s in it standing for the
// value, written only when the value is not empty, and {{ and }} are literal
// braces. The placeholders are project, dir and path. An unknown one, an
// unbalanced brace or a control character is an error: a prompt is plain text,
// and a raw escape in it is how a shell prompt comes to be redrawn in the wrong
// place.
func ParseTemplate(text string) (*Template, error) {
	for _, r := range text {
		if unicode.IsControl(r) {
			return nil, fmt.Errorf("control character %q in the template: a prompt is plain text and placeholders", r)
		}
	}
	rs := []rune(text)
	var parts []part
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			parts = append(parts, part{literal: literal.String()})
			literal.Reset()
		}
	}
	for i := 0; i < len(rs); {
		doubled := i+1 < len(rs) && rs[i+1] == rs[i]
		switch {
		case rs[i] == '{' && doubled:
			literal.WriteRune('{')
			i += 2
		case rs[i] == '}' && doubled:
			literal.WriteRune('}')
			i += 2
		case rs[i] == '}':
			return nil, fmt.Errorf("unbalanced } at position %d: write }} for a brace", i+1)
		case rs[i] == '{':
			p, next, err := placeholder(rs, i)
			if err != nil {
				return nil, err
			}
			flush()
			parts = append(parts, p)
			i = next
		default:
			literal.WriteRune(rs[i])
			i++
		}
	}
	flush()
	return &Template{parts: parts}, nil
}

// placeholder reads the placeholder that opens at rs[open], and returns it with
// the index after its closing brace.
func placeholder(rs []rune, open int) (part, int, error) {
	end := open + 1
	for end < len(rs) && rs[end] != '}' && rs[end] != ':' {
		end++
	}
	if end == len(rs) {
		return part{}, 0, fmt.Errorf("unterminated { at position %d: write {{ for a brace", open+1)
	}
	name := string(rs[open+1 : end])
	switch {
	case name == "":
		return part{}, 0, fmt.Errorf("empty placeholder at position %d: name one of {project}, {dir} or {path}", open+1)
	case !known(name):
		return part{}, 0, fmt.Errorf("unknown placeholder {%s}: the placeholders are {project}, {dir} and {path}", name)
	}
	p := part{name: name}
	if rs[end] == '}' {
		return p, end + 1, nil
	}
	var text strings.Builder
	for k := end + 1; k < len(rs); {
		doubled := k+1 < len(rs) && rs[k+1] == rs[k]
		switch {
		case rs[k] == '}' && !doubled:
			p.text, p.hasText = text.String(), true
			return p, k + 1, nil
		case rs[k] == '{' && !doubled:
			return part{}, 0, fmt.Errorf("unbalanced { in the text of {%s}: write {{ for a brace", name)
		case rs[k] == '{' || rs[k] == '}':
			text.WriteRune(rs[k]) // a doubled brace is one literal brace
			k += 2
		default:
			text.WriteRune(rs[k])
			k++
		}
	}
	return part{}, 0, fmt.Errorf("unterminated {%s: at position %d: close it with }", name, open+1)
}

// Render draws the template for v.
func (t *Template) Render(v Values) string {
	var b strings.Builder
	for _, p := range t.parts {
		if p.name == "" {
			b.WriteString(p.literal)
			continue
		}
		value := valueOf(p.name, v)
		switch {
		case !p.hasText:
			b.WriteString(value)
		case value != "":
			b.WriteString(strings.ReplaceAll(p.text, "%s", value))
		}
	}
	return b.String()
}

func valueOf(name string, v Values) string {
	switch name {
	case placeholderProject:
		return v.Project
	case placeholderDir:
		if v.Dir == "" {
			return ""
		}
		return filepath.Base(v.Dir)
	case placeholderPath:
		return tilde(v.Dir, v.Home)
	}
	return ""
}

// tilde writes dir with home as ~, when dir is home or under it.
func tilde(dir, home string) string {
	if home == "" {
		return dir
	}
	home = filepath.Clean(home)
	if dir == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(dir, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return dir
}

// PromptTemplate parses the template of the settings. The error names the key and
// where its value came from, as one in the user file does.
func PromptTemplate(s clicfg.Settings) (*Template, error) {
	t, err := ParseTemplate(s.ReplPrompt)
	if err != nil {
		return nil, fmt.Errorf("repl.prompt (%s): %w", promptSource(s), err)
	}
	return t, nil
}

func promptSource(s clicfg.Settings) string {
	switch s.Source("repl.prompt") {
	case clicfg.SourceFile:
		return s.UserFile
	case clicfg.SourceEnv:
		return clicfg.EnvName("repl.prompt")
	}
	return "the default"
}
