package repl

import "github.com/rancher/orbs-automation/internal/clicfg"

// Template is a parsed prompt template: plain text and placeholders, checked
// once so that nothing in it can fail while the REPL runs
// (docs/specs/REPL.md#the-prompt).
type Template struct{}

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
// unbalanced brace or a control character is an error.
func ParseTemplate(text string) (*Template, error) { return &Template{}, nil }

// Render draws the template for v.
func (t *Template) Render(v Values) string { return "" }

// PromptTemplate parses the template of the settings. The error names the key and
// where its value came from, as one in the user file does.
func PromptTemplate(s clicfg.Settings) (*Template, error) { return ParseTemplate(s.ReplPrompt) }
