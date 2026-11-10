// Package gomodedit reads and replaces the version of a module in a go.mod,
// leaving the rest of the file as it was: the other lines, the comments and
// `// indirect`. It is the one place the go.mod parser is used
// (docs/specs/DEPENDENCY_POLICY.md).
package gomodedit

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

var (
	// ErrNotRequired is returned when the go.mod has no require line for the module.
	ErrNotRequired = errors.New("module not required")
	// ErrInvalidVersion is returned for a version that is empty or has a space in it.
	ErrInvalidVersion = errors.New("invalid version")
)

// Get returns the version of module that src requires, and an error when it does
// not. A module that is replaced or excluded and not required is not required.
func Get(src []byte, module string) (string, error) {
	reqs, err := requires(src, module)
	if err != nil {
		return "", err
	}
	return reqs[0].Mod.Version, nil
}

// Set replaces the version of module that src requires with version, and returns
// the new text and the version it replaced. Nothing else in src changes: it
// replaces the version text where the parser found it, so the comments, `//
// indirect` and the layout stay, and a replace directive is not touched. A module
// that is required more than once is changed on each line. It is an error when
// src does not require module, or the version is empty or has a space in it.
func Set(src []byte, module, version string) (out []byte, old string, err error) {
	if version == "" || strings.ContainsAny(version, " \t\r\n") {
		return nil, "", fmt.Errorf("%w: %q", ErrInvalidVersion, version)
	}
	reqs, err := requires(src, module)
	if err != nil {
		return nil, "", err
	}
	old = reqs[0].Mod.Version
	type edit struct{ at, length int }
	var edits []edit
	for _, req := range reqs {
		if req.Mod.Version == version {
			continue
		}
		at, err := versionOffset(src, req, module)
		if err != nil {
			return nil, "", err
		}
		edits = append(edits, edit{at, len(req.Mod.Version)})
	}
	// From the last to the first, so an edit does not move the ones before it.
	sort.Slice(edits, func(i, j int) bool { return edits[i].at > edits[j].at })
	out = append([]byte(nil), src...)
	for _, e := range edits {
		out = append(out[:e.at], append([]byte(version), out[e.at+e.length:]...)...)
	}
	return out, old, nil
}

// requires parses src and returns the require lines of module, in file order, or
// ErrNotRequired when there are none.
func requires(src []byte, module string) ([]*modfile.Require, error) {
	f, err := modfile.ParseLax("go.mod", src, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}
	var reqs []*modfile.Require
	for _, req := range f.Require {
		if req.Mod.Path == module {
			reqs = append(reqs, req)
		}
	}
	if len(reqs) == 0 {
		return nil, fmt.Errorf("%s: %w", module, ErrNotRequired)
	}
	return reqs, nil
}

// versionOffset is where the version of a require line starts in src: the first
// place the version text is, on the line, after the module's path.
func versionOffset(src []byte, req *modfile.Require, module string) (int, error) {
	start, end := req.Syntax.Start.Byte, req.Syntax.End.Byte
	if start < 0 || end > len(src) || start > end {
		return 0, fmt.Errorf("go.mod: the line of %s is outside the file", module)
	}
	line := src[start:end]
	path := bytes.Index(line, []byte(module))
	if path < 0 {
		return 0, fmt.Errorf("go.mod: the path of %s is not on its line", module)
	}
	rest := path + len(module)
	version := bytes.Index(line[rest:], []byte(req.Mod.Version))
	if version < 0 {
		return 0, fmt.Errorf("go.mod: the version of %s is not on its line", module)
	}
	return start + rest + version, nil
}

// Directives returns the Go version a go.mod asks for in its go line, and the
// toolchain it names in its toolchain line, each empty when the file has none. The
// toolchain is as written, such as go1.26.10, and the version without its go, such
// as 1.26 or 1.26.3.
func Directives(src []byte) (goVersion, toolchain string, err error) {
	// The strict parser: the lax one, which Get and Set use so a require line it does
	// not like is no obstacle, leaves the toolchain line out.
	f, err := modfile.Parse("go.mod", src, nil)
	if err != nil {
		return "", "", fmt.Errorf("parse go.mod: %w", err)
	}
	if f.Go != nil {
		goVersion = f.Go.Version
	}
	if f.Toolchain != nil {
		toolchain = f.Toolchain.Name
	}
	return goVersion, toolchain, nil
}
