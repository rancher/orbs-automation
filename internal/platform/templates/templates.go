// Package templates fills the {name} placeholders of the texts gero ship writes:
// the commit message, the PR title, the branch name and the value a strategy
// sets. The syntax is deliberately small, as the placeholders already written
// in config are: a name in braces, with {{ and }} for a literal brace. There
// are no conditions, loops or functions, so a text cannot grow into a program
// that differs per project. The variables a text may use are a closed set that
// the caller supplies, and an unknown one is an error, both when the config is
// validated and when the text is filled.
//
// A placeholder may carry modifiers after a pipe, applied left to right, to spell
// a value the way a file does: {line|trimprefix:v} turns v2.16 into 2.16 and
// {stage|replace:stable=prod} turns stable into prod. The set is closed (upper,
// lower, title, trimprefix, trimsuffix and replace of a whole value), and an
// argument cannot contain } or |.
package templates

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// name is what a placeholder may be called.
var name = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// part is a run of literal text, or one placeholder.
type part struct {
	text          string // the literal, or the placeholder's name
	isPlaceholder bool
	mods          []modifier // a placeholder's modifiers, in order
}

// modifier respells a value: a name and, for some, an argument.
type modifier struct{ name, arg string }

// modifierNames are the modifiers there are.
var modifierNames = []string{"upper", "lower", "title", "trimprefix", "trimsuffix", "replace"}

// reversible are the modifiers that lose nothing, so a text that is read as well
// as written may use them.
var reversible = []string{"trimprefix", "trimsuffix", "replace"}

// parseModifier reads one modifier such as trimprefix:v and checks its argument.
func parseModifier(f, placeholder string) (modifier, error) {
	if f == "" {
		return modifier{}, fmt.Errorf("empty modifier in {%s}", placeholder)
	}
	mod, arg, hasArg := strings.Cut(f, ":")
	switch mod {
	case "upper", "lower", "title":
		if hasArg {
			return modifier{}, fmt.Errorf("%s takes no argument, in {%s}", mod, placeholder)
		}
	case "trimprefix", "trimsuffix":
		if arg == "" {
			return modifier{}, fmt.Errorf("%s needs an argument, in {%s}", mod, placeholder)
		}
	case "replace":
		if from, _, ok := strings.Cut(arg, "="); !ok || from == "" {
			return modifier{}, fmt.Errorf("replace needs FROM=TO with a FROM, in {%s}", placeholder)
		}
	default:
		return modifier{}, fmt.Errorf("unknown modifier %q in {%s}: use one of %s", mod, placeholder, strings.Join(modifierNames, ", "))
	}
	return modifier{name: mod, arg: arg}, nil
}

// apply respells value by the modifier.
func (m modifier) apply(value string) string {
	switch m.name {
	case "upper":
		return strings.ToUpper(value)
	case "lower":
		return strings.ToLower(value)
	case "title":
		r, size := utf8.DecodeRuneInString(value)
		if size == 0 {
			return value
		}
		return string(unicode.ToUpper(r)) + value[size:]
	case "trimprefix":
		return strings.TrimPrefix(value, m.arg)
	case "trimsuffix":
		return strings.TrimSuffix(value, m.arg)
	case "replace":
		from, to, _ := strings.Cut(m.arg, "=")
		if value == from {
			return to
		}
	}
	return value
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
			if n == "" {
				return nil, fmt.Errorf("empty placeholder {} at position %d", i)
			}
			fields := strings.Split(n, "|")
			if !name.MatchString(fields[0]) {
				return nil, fmt.Errorf("{%s} is not a placeholder name: use letters and digits", n)
			}
			var mods []modifier
			for _, f := range fields[1:] {
				m, err := parseModifier(f, n)
				if err != nil {
					return nil, err
				}
				mods = append(mods, m)
			}
			flush()
			parts = append(parts, part{text: fields[0], isPlaceholder: true, mods: mods})
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

// TextVars are the variables the texts of a PR may use: the pullRequests title
// and branchPrefix of a project, and the commit message and body that gero
// embeds. A strategy's value template has its own, smaller set (ship.ValueVars).
//
//	project           the producer's config name
//	source            the same, as the title says it ("Update {source} to ...")
//	sourceRepo        the producer's owner/name
//	tag               the release tag as published, such as v0.5.3
//	version           the tag without a leading v
//	branch            the consumer's target branch without "release/"
//	targetBranch      the consumer's target branch as it is named
//	consumer          the consumer's owner/name
//	sourceReleaseURL  where the producer's release is published
//	file, from, to    what the strategy changed: the file, its old and new value
//	companions        the releases that go with this one, such as "lasso to v0.2.10": empty when none
//	changes           a list of what was changed, generated from the edits
//	marker            the hidden block that identifies the PR (PR body only)
var TextVars = []string{
	"project", "source", "sourceRepo", "tag", "version", "branch", "targetBranch",
	"consumer", "sourceReleaseURL", "file", "from", "to", "companions", "changes", "marker",
}

// StrategyVars are the extra variables a strategy supplies to the texts of a PR
// of a project that uses it, by the strategy's name. They are listed here, and
// not only in the strategy, so config validation can accept them without
// importing the domain, and so a strategy that is not implemented yet can have
// its variables written in config.
var StrategyVars = map[string][]string{
	"chartsPackageBump": {"package", "chartVersion", "action"},
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
		for _, m := range p.mods {
			v = m.apply(v)
		}
		out.WriteString(v)
	}
	return out.String(), nil
}

// StepCommitVars are the variables of the commit message of a step: the ones that
// say what the bump is, and which step this is.
//
//	command  the step's command, as written
//	phase    where the step runs, such as "method postEdit"
var StepCommitVars = []string{
	"project", "source", "sourceRepo", "tag", "version", "branch", "targetBranch",
	"consumer", "sourceReleaseURL", "command", "phase",
}

// StrategyCommitVars are the variables of the commit message of a strategy: the
// ones that say what the bump is, and which strategy this is.
//
//	strategy  the strategy's name, such as goBump
//	edit      the edit that names the strategy
//	changes   what the strategy changed, one bullet for each
var StrategyCommitVars = []string{
	"project", "source", "sourceRepo", "tag", "version", "branch", "targetBranch",
	"consumer", "sourceReleaseURL", "strategy", "edit", "changes",
}

// CheckReversible returns an error when the text uses a modifier that loses
// information. A text that is read as well as written, such as the form of a
// holding, may use only modifiers that can be undone.
func CheckReversible(text string) error {
	parts, err := parse(text)
	if err != nil {
		return err
	}
	for _, p := range parts {
		for _, m := range p.mods {
			if !slices.Contains(reversible, m.name) {
				return fmt.Errorf("modifier %s of {%s} loses information, so it cannot be used where the text is also read: use %s",
					m.name, p.text, strings.Join(reversible, ", "))
			}
		}
	}
	return nil
}
