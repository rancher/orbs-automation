// Package bundle is the format of the central config bundle: the config/ directory
// of the central repository as one multi-document YAML file, which a gero release
// carries beside its binaries (docs/specs/CENTRAL_BUNDLE.md). Compile makes it,
// for tools/compilebundle, and the reader of the CLI takes it apart again here,
// so the two cannot drift.
package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"testing/fstest"
	"time"
	"unicode"

	"github.com/rancher/orbs-automation/internal/chainload"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// Header is what the first document of a bundle says about it.
type Header struct {
	GeroVersion  string    // the gero release the bundle is made for, such as v0.5.0
	SourceCommit string    // the commit of the central repo that config was read from
	CompiledAt   time.Time // when, as the release job says: never the clock, so a build repeats
	// FormatVersion is the version of the bundle format, set by Parse and ignored by
	// Compile, which writes the one it knows.
	FormatVersion int
	// InputsDigest and CompilerRevision say what the bundle was made from: the hash of
	// the config files that went in (Digest), and the revision of Compile that made it.
	// Parse sets them, empty and zero for a bundle made before they were written, and
	// Compile ignores them, writing the ones it works out.
	InputsDigest     string
	CompilerRevision int
}

// CompilerRevision is the revision of Compile. It is raised whenever Compile makes
// different bytes for the same files, so that the same digest and revision always mean
// the same bundle. The golden test fails until it is raised.
const CompilerRevision = 1

// Digest is the hash of the config files under fsys that Compile would bundle: the
// YAML files in the order of their names, each by its name and the hash of its bytes.
// It is the same for the same files whoever lists them, and changes when one byte of
// one file, or a file's name, changes.
func Digest(fsys fs.FS) (string, error) {
	names, err := inputFiles(fsys)
	if err != nil {
		return "", err
	}
	var manifest strings.Builder
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(data)
		manifest.WriteString(name + "\x00" + hex.EncodeToString(sum[:]) + "\n")
	}
	sum := sha256.Sum256([]byte(manifest.String()))
	return hex.EncodeToString(sum[:]), nil
}

// inputFiles are the config files Compile bundles and Digest hashes: the YAML files
// under fsys, in the order fs.WalkDir lists them.
func inputFiles(fsys fs.FS) ([]string, error) {
	var names []string
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ext := path.Ext(name); ext == ".yaml" || ext == ".yml" {
			names = append(names, name)
		}
		return nil
	})
	return names, err
}

// ID is what identifies the bundle in a store: a hash of the revision of Compile and
// the digest of the files it was made from. A bundle that does not carry a digest
// has no ID.
func (h Header) ID() string {
	if h.InputsDigest == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("compiler revision " + strconv.Itoa(h.CompilerRevision) + "\ninputs " + h.InputsDigest + "\n"))
	return hex.EncodeToString(sum[:])
}

// Name is the file name of the bundle, in the output directory and as a release
// asset.
const Name = "gero-config-bundle.yaml"

const (
	// bundleAPIVersion is not a config kind: the header is read by the reader of
	// the bundle, before the config, and is not part of gero/v1alpha2.
	bundleAPIVersion = "gero-bundle/v1"
	formatVersion    = 1

	// schemaPrefix starts the comment that points an editor at a schema. It only
	// works as the first line of a file, so it is dropped from the bundle.
	schemaPrefix = "# yaml-language-server:"
)

// Compile validates the config under fsys as every command does and returns the
// bundle: the header document, then each YAML file's documents as written, in the
// order of the file names. The one change to a file is that the comment pointing
// an editor at a schema, and a document marker, are dropped from its top. It
// returns nothing when the config does not validate or is empty.
func Compile(fsys fs.FS, h Header) ([]byte, error) {
	data, _, err := CompileReport(fsys, h)
	return data, err
}

// CompileReport is Compile, and the warnings the config had, which Compile leaves
// unsaid: the tool prints them, and a binary that compiles its own config does not.
func CompileReport(fsys fs.FS, h Header) ([]byte, []config.Finding, error) {
	digest, err := Digest(fsys)
	if err != nil {
		return nil, nil, err
	}
	h.InputsDigest, h.CompilerRevision = digest, CompilerRevision
	head, err := h.document()
	if err != nil {
		return nil, nil, err
	}
	ctx := config.Context{Now: h.CompiledAt}
	source, rep := chainload.Load(fsys, ctx)
	if err := rep.Err(); err != nil {
		return nil, nil, fmt.Errorf("the config does not validate: %w", err)
	}
	if source.Empty() {
		return nil, nil, errors.New("there is no config to bundle: no YAML file holds a document")
	}

	names, err := inputFiles(fsys)
	if err != nil {
		return nil, nil, err
	}
	var body strings.Builder
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, nil, err
		}
		body.WriteString("---\n# source: " + name + "\n" + trimTop(string(data)))
	}

	// What a reader does with the body is checked here: it has to load to the
	// config that went in.
	_, bodyFS, err := Parse([]byte(head + body.String()))
	if err != nil {
		return nil, nil, fmt.Errorf("the bundle does not parse back: %w", err)
	}
	back, rep := chainload.Load(bodyFS, ctx)
	if err := rep.Err(); err != nil {
		return nil, nil, fmt.Errorf("the bundle does not load back: %w", err)
	}
	if len(back.Projects()) != len(source.Projects()) || len(back.Holdings()) != len(source.Holdings()) ||
		len(back.Lines()) != len(source.Lines()) {
		return nil, nil, errors.New("the bundle does not load back to the config that went in")
	}
	return []byte(head + body.String()), source.Warnings(), nil
}

// document returns the header as the first document of a bundle. It is written
// as text and not as YAML, so the values must be one plain word each.
func (h Header) document() (string, error) {
	for _, f := range []struct{ name, value string }{
		{"version", h.GeroVersion}, {"commit", h.SourceCommit}, {"inputs digest", h.InputsDigest},
	} {
		if f.value == "" {
			return "", fmt.Errorf("the %s of the header is empty", f.name)
		}
		if strings.IndexFunc(f.value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return "", fmt.Errorf("the %s of the header, %q, must not contain spaces or control characters", f.name, f.value)
		}
	}
	return fmt.Sprintf("apiVersion: %s\nkind: Bundle\nformatVersion: %d\ngeroVersion: %s\nsourceCommit: %s\ncompiledAt: %s\ninputsDigest: %s\ncompilerRevision: %d\n",
		bundleAPIVersion, formatVersion, h.GeroVersion, h.SourceCommit, h.CompiledAt.UTC().Format(time.RFC3339), h.InputsDigest, h.CompilerRevision), nil
}

// trimTop drops from the top of a file the schema comment and the document
// marker, whichever come first, and makes sure it ends in a newline.
func trimTop(s string) string {
	for {
		line, rest, _ := strings.Cut(s, "\n")
		if !strings.HasPrefix(line, schemaPrefix) && line != "---" {
			break
		}
		s = rest
	}
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

var (
	// ErrMalformed is returned by Parse for data that is not a bundle: no header,
	// or a header that is missing a field or has a bad one.
	ErrMalformed = errors.New("not a config bundle")
	// ErrUnsupported is returned by Parse for a bundle of a format this gero cannot
	// read. A gero older than its newest bundle is expected, so this is a normal
	// outcome and the message names both versions.
	ErrUnsupported = errors.New("the config bundle is in a format this gero cannot read")
)

// Parse reads a bundle: it checks the header, which is the first document, and
// returns it with the rest, the config chunk, as a file system that holds one file,
// Name, for the config loader. It is what Compile makes the bundle for.
func Parse(data []byte) (Header, fs.FS, error) {
	text := string(data)
	head, body := text, ""
	if i := strings.Index(text, "\n---\n"); i >= 0 {
		head, body = text[:i+1], text[i+1:]
	}
	fields := map[string]string{}
	for _, line := range strings.Split(head, "\n") {
		if key, value, ok := strings.Cut(line, ": "); ok && !strings.HasPrefix(line, "#") {
			fields[key] = strings.TrimSpace(value)
		}
	}

	api := fields["apiVersion"]
	if !strings.HasPrefix(api, "gero-bundle/") {
		return Header{}, nil, fmt.Errorf("%w: the first document has no apiVersion of %s", ErrMalformed, "gero-bundle/...")
	}
	if api != bundleAPIVersion {
		return Header{}, nil, fmt.Errorf("%w: the bundle is %s, and this gero reads %s", ErrUnsupported, api, bundleAPIVersion)
	}
	if fields["kind"] != "Bundle" {
		return Header{}, nil, fmt.Errorf("%w: the kind of the first document is %q, not Bundle", ErrMalformed, fields["kind"])
	}
	format, err := strconv.Atoi(fields["formatVersion"])
	if err != nil {
		return Header{}, nil, fmt.Errorf("%w: formatVersion is %q, not a number", ErrMalformed, fields["formatVersion"])
	}
	if format != formatVersion {
		return Header{}, nil, fmt.Errorf("%w: the bundle is format version %d, and this gero reads %d", ErrUnsupported, format, formatVersion)
	}
	h := Header{GeroVersion: fields["geroVersion"], SourceCommit: fields["sourceCommit"], FormatVersion: format}
	for name, value := range map[string]string{"geroVersion": h.GeroVersion, "sourceCommit": h.SourceCommit} {
		if value == "" {
			return Header{}, nil, fmt.Errorf("%w: the header has no %s", ErrMalformed, name)
		}
	}
	if h.CompiledAt, err = time.Parse(time.RFC3339, fields["compiledAt"]); err != nil {
		return Header{}, nil, fmt.Errorf("%w: compiledAt is %q, not a time", ErrMalformed, fields["compiledAt"])
	}
	// A bundle made before the digest was written has neither field, and has no ID.
	h.InputsDigest = fields["inputsDigest"]
	if rev, ok := fields["compilerRevision"]; ok {
		if h.CompilerRevision, err = strconv.Atoi(rev); err != nil {
			return Header{}, nil, fmt.Errorf("%w: compilerRevision is %q, not a number", ErrMalformed, rev)
		}
	}
	return h, fstest.MapFS{Name: {Data: []byte(body)}}, nil
}
