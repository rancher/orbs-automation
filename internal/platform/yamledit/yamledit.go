// Package yamledit reads, and later changes, one value of a YAML file without
// disturbing the rest of it. A downstream file such as rancher's build.yaml is
// edited by a bump and then reviewed as a diff, so comments, quoting and
// indentation must survive; re-encoding the document would reflow all of them.
// It is the third importer of the YAML library, next to the two config loaders
// (docs/specs/DEPENDENCY_POLICY.md).
package yamledit

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

var (
	// ErrKeyNotFound is returned when the path names no key in the document.
	ErrKeyNotFound = errors.New("key not found")
	// ErrInvalidYAML is returned when the file is not YAML, and the message says
	// where and shows the line.
	ErrInvalidYAML = errors.New("not valid YAML")
	// ErrInvalidPath is returned for a path that is not written in the path syntax:
	// an empty step, a quote left open, or an index that is not a number.
	ErrInvalidPath = errors.New("invalid path")
	// ErrNotScalar is returned when the path names a mapping or a list, which
	// has no single value to read or replace.
	ErrNotScalar = errors.New("value is not a scalar")
	// ErrAmbiguous is returned when a key appears twice in the same mapping, so
	// the path does not name one value.
	ErrAmbiguous = errors.New("key appears more than once")
	// ErrUnsupported is returned for a value Set cannot replace without
	// guessing: a block scalar, a tagged or anchored value, or one that spans
	// lines. It refuses and leaves the file alone.
	ErrUnsupported = errors.New("value cannot be replaced in place")
)

// Get returns the scalar at path in the first document of src. A path is the keys
// of nested mappings joined with dots, such as global.kubectl.tag, in the syntax
// of yq: a key with a dot in it is in double quotes ("2.16"), [n] is the nth item
// of a list, and a leading dot is optional. A quoted value comes back without its
// quotes, and a number or a boolean as its text.
func Get(src []byte, path string) (string, error) {
	node, err := find(src, path)
	if err != nil {
		return "", err
	}
	return node.Value, nil
}

// find returns the scalar node at path in the first document of src.
func find(src []byte, path string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("read YAML: %w", err)
	}
	if path == "" || len(doc.Content) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrKeyNotFound, path)
	}
	steps, err := parsePath(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, path)
	}
	node := doc.Content[0]
	for _, st := range steps {
		next, err := st.walk(node)
		if err != nil {
			return nil, fmt.Errorf("%w: %q", err, path)
		}
		if next == nil {
			return nil, fmt.Errorf("%w: %q", ErrKeyNotFound, path)
		}
		node = next
	}
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("%w: %q", ErrNotScalar, path)
	}
	return node, nil
}

// step is one move down a path: a key of a mapping, or an item of a list.
type step struct {
	key     string
	index   int
	isIndex bool
}

// walk moves from node to the child the step names, nil when there is none.
func (st step) walk(node *yaml.Node) (*yaml.Node, error) {
	if !st.isIndex {
		return child(node, st.key)
	}
	if node.Kind != yaml.SequenceNode || st.index >= len(node.Content) {
		return nil, nil
	}
	return node.Content[st.index], nil
}

// parsePath reads a path: keys joined by dots, a key in double quotes when it has
// a dot in it, and [n] for the nth item of a list, as yq writes them. A leading
// dot is optional, so global.kubectl.tag, .global.kubectl.tag and
// .items[0].name are all paths, and ."a.b".c is the key c in the key a.b.
func parsePath(path string) ([]step, error) {
	var steps []step
	i := 0
	if strings.HasPrefix(path, ".") {
		i++
	}
	for i < len(path) {
		switch {
		case path[i] == '[':
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("%w: a [ is not closed", ErrInvalidPath)
			}
			n, err := strconv.Atoi(path[i+1 : i+end])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("%w: %q is not a list index", ErrInvalidPath, path[i+1:i+end])
			}
			steps = append(steps, step{index: n, isIndex: true})
			i += end + 1
		case path[i] == '"':
			key, next, err := quotedKey(path, i)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step{key: key})
			i = next
		default:
			end := strings.IndexAny(path[i:], ".[")
			if end < 0 {
				end = len(path) - i
			}
			if end == 0 {
				return nil, fmt.Errorf("%w: a key is empty", ErrInvalidPath)
			}
			steps = append(steps, step{key: path[i : i+end]})
			i += end
		}
		if i < len(path) && path[i] == '.' {
			i++
			if i == len(path) || path[i] == '.' || path[i] == '[' {
				return nil, fmt.Errorf("%w: a key is empty", ErrInvalidPath)
			}
		} else if i < len(path) && path[i] != '[' {
			return nil, fmt.Errorf("%w: unexpected %q after a key", ErrInvalidPath, path[i:i+1])
		}
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("%w: it names nothing", ErrInvalidPath)
	}
	return steps, nil
}

// quotedKey reads the key in double quotes that starts at path[i], and returns it
// with the position after the closing quote. A backslash makes the next character
// the key's own.
func quotedKey(path string, i int) (key string, next int, err error) {
	var b strings.Builder
	for j := i + 1; j < len(path); j++ {
		switch path[j] {
		case '\\':
			j++
			if j == len(path) {
				return "", 0, fmt.Errorf("%w: a quote is not closed", ErrInvalidPath)
			}
			b.WriteByte(path[j])
		case '"':
			return b.String(), j + 1, nil
		default:
			b.WriteByte(path[j])
		}
	}
	return "", 0, fmt.Errorf("%w: a quote is not closed", ErrInvalidPath)
}

// child returns the value of key in a mapping node, nil when node is not a
// mapping or has no such key, and ErrAmbiguous when the key appears twice.
func child(node *yaml.Node, key string) (*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, nil
	}
	var found *yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value != key {
			continue
		}
		if found != nil {
			return nil, ErrAmbiguous
		}
		found = node.Content[i+1]
	}
	return found, nil
}

// Set replaces the scalar at path in the first document of src with value and
// returns the new file and the value it had. Only the bytes of the value change:
// comments, indentation, the quoting style the value had, line endings and every
// other line stay as they were, so the diff of the file is that one value. A
// plain value stays plain unless the new one needs quoting, and a quoted one
// stays quoted. A new value equal to the old one returns src unchanged. The new
// value must be one line. It returns ErrUnsupported rather than guess for a
// block scalar, a tag, an anchor or a value that spans lines.
func Set(src []byte, path, value string) (out []byte, old string, err error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, "", fmt.Errorf("%w: the new value must be one line", ErrUnsupported)
	}
	node, err := find(src, path)
	if err != nil {
		return nil, "", err
	}
	if node.Anchor != "" || node.Style&(yaml.TaggedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return nil, "", fmt.Errorf("%w: %q is a block scalar, or has a tag or an anchor", ErrUnsupported, path)
	}
	if node.Value == value {
		return bytes.Clone(src), node.Value, nil
	}
	start := offset(src, node.Line, node.Column)
	end, style, ok := valueEnd(src, start, node)
	if !ok {
		return nil, "", fmt.Errorf("%w: %q is not a plain, single-quoted or double-quoted value on one line", ErrUnsupported, path)
	}
	text, err := encode(value, style)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	out = make([]byte, 0, len(src)-(end-start)+len(text))
	out = append(out, src[:start]...)
	out = append(out, text...)
	out = append(out, src[end:]...)
	return out, node.Value, nil
}

// offset returns the byte offset of a 1-based line and column, where a column
// counts characters, as the YAML parser reports them.
func offset(src []byte, line, column int) int {
	pos := 0
	for l := 1; l < line; l++ {
		i := bytes.IndexByte(src[pos:], '\n')
		if i < 0 {
			return len(src)
		}
		pos += i + 1
	}
	for c := 1; c < column && pos < len(src); c++ {
		_, size := utf8.DecodeRune(src[pos:])
		pos += size
	}
	return pos
}

// valueEnd finds where the scalar that starts at start ends, and the style it
// is written in. ok is false when its text in the file is not exactly the value
// the parser read, which is a value with escapes, folded across lines or
// otherwise written in a way that is not a plain replacement of its bytes.
func valueEnd(src []byte, start int, node *yaml.Node) (end int, style yaml.Style, ok bool) {
	if start >= len(src) {
		return 0, 0, false
	}
	switch {
	case node.Style&yaml.DoubleQuotedStyle != 0:
		i := bytes.IndexByte(src[start+1:], '"')
		if src[start] != '"' || i < 0 {
			return 0, 0, false
		}
		inner := src[start+1 : start+1+i]
		if bytes.ContainsAny(inner, "\\\n") || string(inner) != node.Value {
			return 0, 0, false
		}
		return start + 1 + i + 1, yaml.DoubleQuotedStyle, true
	case node.Style&yaml.SingleQuotedStyle != 0:
		if src[start] != '\'' {
			return 0, 0, false
		}
		for i := start + 1; i < len(src); i++ {
			switch {
			case src[i] == '\n':
				return 0, 0, false
			case src[i] == '\'' && i+1 < len(src) && src[i+1] == '\'':
				i++
			case src[i] == '\'':
				if strings.ReplaceAll(string(src[start+1:i]), "''", "'") != node.Value {
					return 0, 0, false
				}
				return i + 1, yaml.SingleQuotedStyle, true
			}
		}
		return 0, 0, false
	default:
		end = start + len(node.Value)
		if end > len(src) || string(src[start:end]) != node.Value || strings.Contains(node.Value, "\n") {
			return 0, 0, false
		}
		return end, 0, true
	}
}

// encode writes value as a YAML scalar in the style the old value had. A plain
// value stays plain when it can, and is quoted when it would otherwise be read
// as something other than the string it is (a number, a boolean, text with a
// colon or a hash).
func encode(value string, style yaml.Style) ([]byte, error) {
	out, err := yaml.Marshal(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: style, Value: value})
	if err != nil {
		return nil, err
	}
	out = bytes.TrimSuffix(out, []byte("\n"))
	if bytes.Contains(out, []byte("\n")) {
		return nil, errors.New("the new value does not fit on one line")
	}
	return out, nil
}
