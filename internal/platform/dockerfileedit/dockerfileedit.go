// Package dockerfileedit reads and replaces the value of one ARG or ENV in a
// Dockerfile, leaving every other byte of the file as it was.
package dockerfileedit

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNotFound is returned when no ARG or ENV of the name has a value in the file.
	ErrNotFound = errors.New("no ARG or ENV with a value")
	// ErrAmbiguous is returned when the name is given a value in more than one place,
	// so it does not name one value.
	ErrAmbiguous = errors.New("given a value more than once")
	// ErrInvalidValue is returned for a new value that cannot be written on one
	// instruction line as it is.
	ErrInvalidValue = errors.New("invalid value")
)

// Get returns the value that an ARG or ENV of name is given in src, without its
// quotes. An ARG with no default gives no value, so a stage may redeclare an ARG
// to use it. The name must be given a value in exactly one place: a Dockerfile
// that sets it twice is an error, since a holding names one value.
func Get(src []byte, name string) (string, error) {
	found, err := find(src, name)
	if err != nil {
		return "", err
	}
	return string(src[found.start:found.end]), nil
}

// Set replaces the value that an ARG or ENV of name is given in src with value,
// and returns the new file and the value it replaced. Only those bytes change, and
// the value keeps the quotes it was in. The value must be one line, and must not
// hold the quote it is written in, a backslash, or, when it is not quoted, a
// space or a quote. The name is found as Get finds it.
func Set(src []byte, name, value string) (out []byte, old string, err error) {
	found, err := find(src, name)
	if err != nil {
		return nil, "", err
	}
	if err := checkValue(found, value); err != nil {
		return nil, "", err
	}
	old = string(src[found.start:found.end])
	out = make([]byte, 0, len(src)-len(old)+len(value))
	out = append(out, src[:found.start]...)
	out = append(out, value...)
	out = append(out, src[found.end:]...)
	return out, old, nil
}

// entry is one name given a value, and where the value is: its bytes, without the
// quotes around them.
type entry struct {
	name       string
	start, end int
	quote      byte // the quote the value is in, or 0
	legacy     bool // the old `ENV name value` form, where the value is the rest of the line
}

func find(src []byte, name string) (entry, error) {
	var found []entry
	for _, e := range entries(src) {
		if e.name == name {
			found = append(found, e)
		}
	}
	switch {
	case len(found) == 0:
		return entry{}, fmt.Errorf("%w: %s is not given a value by an ARG or ENV in the file", ErrNotFound, name)
	case len(found) > 1:
		return entry{}, fmt.Errorf("%w: %s is given a value in %d places in the file", ErrAmbiguous, name, len(found))
	}
	return found[0], nil
}

func checkValue(e entry, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%w: the new value of %s must be one line", ErrInvalidValue, e.name)
	}
	if strings.Contains(value, `\`) {
		return fmt.Errorf("%w: the new value of %s has a backslash", ErrInvalidValue, e.name)
	}
	switch {
	case e.quote != 0 && strings.IndexByte(value, e.quote) >= 0:
		return fmt.Errorf("%w: the new value of %s has the %c it is quoted in", ErrInvalidValue, e.name, e.quote)
	case e.quote == 0 && !e.legacy && strings.ContainsAny(value, " \t\"'"):
		return fmt.Errorf("%w: the new value of %s has a space or a quote and the value is not quoted", ErrInvalidValue, e.name)
	case e.quote == 0 && e.legacy && (value == "" || strings.ContainsAny(value, "\"'")):
		return fmt.Errorf("%w: the new value of %s is empty or has a quote", ErrInvalidValue, e.name)
	}
	return nil
}

// entries lists every name an ARG or ENV in src gives a value, in the order they
// are written.
func entries(src []byte) []entry {
	var all []entry
	for pos := 0; pos < len(src); {
		start, end := pos, logicalEnd(src, pos)
		pos = end
		if pos < len(src) {
			pos++ // the newline that ends the instruction
		}
		i := skipBlank(src, start, end)
		if i >= end || src[i] == '#' {
			continue
		}
		j := i
		for j < end && src[j] != ' ' && src[j] != '\t' {
			j++
		}
		switch strings.ToUpper(string(src[i:j])) {
		case "ARG":
			all = append(all, pairs(src, j, end, false)...)
		case "ENV":
			all = append(all, pairs(src, j, end, true)...)
		}
	}
	return all
}

// logicalEnd is where the instruction that starts at pos ends: the end of its
// line, or of the line after it when a line ends in a backslash. A comment has no
// continuation.
func logicalEnd(src []byte, pos int) int {
	if i := skipBlank(src, pos, len(src)); i < len(src) && src[i] == '#' {
		return lineEnd(src, pos)
	}
	end := lineEnd(src, pos)
	for end < len(src) && continues(src, end) {
		end = lineEnd(src, end+1)
	}
	return end
}

func lineEnd(src []byte, pos int) int {
	if n := bytes.IndexByte(src[pos:], '\n'); n >= 0 {
		return pos + n
	}
	return len(src)
}

// continues says whether the line that ends at end ends in a backslash.
func continues(src []byte, end int) bool {
	if end > 0 && src[end-1] == '\r' {
		end--
	}
	return end > 0 && src[end-1] == '\\'
}

// continuation is the length of the backslash and the line break at i, or 0.
func continuation(src []byte, i, end int) int {
	if i >= end || src[i] != '\\' {
		return 0
	}
	if i+1 < end && src[i+1] == '\n' {
		return 2
	}
	if i+2 < end && src[i+1] == '\r' && src[i+2] == '\n' {
		return 3
	}
	return 0
}

// skipBlank moves past spaces and line continuations.
func skipBlank(src []byte, i, end int) int {
	for i < end {
		if n := continuation(src, i, end); n > 0 {
			i += n
		} else if c := src[i]; c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			i++
		} else {
			break
		}
	}
	return i
}

// pairs reads the `name=value` pairs of an instruction body, and for an ENV the
// old `name value` form. A name with no value, as in a bare ARG, gives none.
func pairs(src []byte, i, end int, isEnv bool) []entry {
	var out []entry
	for first := true; ; first = false {
		i = skipBlank(src, i, end)
		if i >= end {
			return out
		}
		j := i
		for j < end && !strings.ContainsRune(" \t\r\n=", rune(src[j])) && continuation(src, j, end) == 0 {
			j++
		}
		name := string(src[i:j])
		if j >= end || src[j] != '=' {
			if first && isEnv && name != "" {
				if e, ok := legacy(src, name, j, end); ok {
					out = append(out, e)
				}
				return out
			}
			if j == i {
				j++ // a stray character: step over it
			}
			i = j
			continue
		}
		e, next, ok := value(src, name, j+1, end)
		if !ok {
			return out
		}
		if name != "" {
			out = append(out, e)
		}
		i = next
	}
}

// value reads the value that starts at i, after the `=`, and where the next pair
// starts. It is not ok for a quote that is never closed.
func value(src []byte, name string, i, end int) (e entry, next int, ok bool) {
	if i < end && (src[i] == '"' || src[i] == '\'') {
		q := src[i]
		k := i + 1
		for k < end && src[k] != q {
			if q == '"' && src[k] == '\\' {
				k++
			}
			k++
		}
		if k >= end {
			return entry{}, 0, false
		}
		return entry{name: name, start: i + 1, end: k, quote: q}, k + 1, true
	}
	k := i
	for k < end && !strings.ContainsRune(" \t\r\n", rune(src[k])) && continuation(src, k, end) == 0 {
		if src[k] == '\\' && k+1 < end {
			k++
		}
		k++
	}
	return entry{name: name, start: i, end: k}, k, true
}

// legacy reads the value of the old `ENV name value` form: the rest of the line,
// without a pair of quotes around all of it.
func legacy(src []byte, name string, i, end int) (entry, bool) {
	start := skipBlank(src, i, end)
	stop := end
	for stop > start && strings.ContainsRune(" \t\r", rune(src[stop-1])) {
		stop--
	}
	if start >= stop {
		return entry{}, false
	}
	e := entry{name: name, start: start, end: stop, legacy: true}
	if q := src[start]; (q == '"' || q == '\'') && stop-start >= 2 && src[stop-1] == q {
		e.start, e.end, e.quote = start+1, stop-1, q
	}
	return e, true
}
