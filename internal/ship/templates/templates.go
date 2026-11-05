// Package templates fills the {name} placeholders of the texts gero ship writes:
// the commit message, the PR title, the branch name and the value a strategy
// sets. The syntax is deliberately small, as the placeholders already written
// in config are: a name in braces, with {{ and }} for a literal brace. There
// are no conditions, loops or functions, so a text cannot grow into a program
// that differs per project. The variables a text may use are a closed set that
// the caller supplies, and an unknown one is an error, both when the config is
// validated and when the text is filled.
package templates

// Placeholders returns the names used by the text, in order of first use and
// without repeats. It returns an error for a brace that is not part of a
// {name} or an escaped {{ or }}.
func Placeholders(text string) ([]string, error) {
	return nil, nil
}

// Validate returns an error for a malformed text or a placeholder that is not in
// allowed.
func Validate(text string, allowed []string) error {
	return nil
}

// Render fills the placeholders of text from vars. It returns an error for a
// malformed text or a placeholder that vars does not have; a variable whose
// value is empty is filled with nothing.
func Render(text string, vars map[string]string) (string, error) {
	return "", nil
}
