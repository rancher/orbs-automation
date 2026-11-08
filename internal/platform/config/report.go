package config

import (
	"errors"
	"fmt"
	"strings"
)

// Severity says whether a finding makes the object invalid.
type Severity int

// The severities. A warning never makes an object invalid.
const (
	Error Severity = iota
	Warning
)

// Finding is one problem, or one thing worth a warning, in a document.
type Finding struct {
	Severity Severity
	Source   string // the file or stream the document came from
	Doc      int    // the document's place in the source, from 1; 0 when there is one
	Label    string // what the document is, such as Project "rancher"
	Path     string // where in the document, such as spec.layout
	Msg      string
}

// Error returns the finding as one line naming the source, the document, the path
// and the message, leaving out what is not set.
func (f Finding) Error() string {
	parts := make([]string, 0, 5)
	if f.Source != "" {
		parts = append(parts, f.Source)
	}
	if f.Doc > 0 {
		parts = append(parts, fmt.Sprintf("document %d", f.Doc))
	}
	if f.Label != "" {
		parts = append(parts, f.Label)
	}
	if f.Path != "" {
		parts = append(parts, f.Path)
	}
	parts = append(parts, f.Msg)
	return strings.Join(parts, ": ")
}

// Report collects the findings of one document.
type Report struct {
	Findings []Finding

	source string
	doc    int
	label  string
}

func newReport(source string, doc int, label string) Report {
	return Report{source: source, doc: doc, label: label}
}

func (r *Report) add(sev Severity, path, msg string) {
	r.Findings = append(r.Findings, Finding{
		Severity: sev, Source: r.source, Doc: r.doc, Label: r.label, Path: path, Msg: msg,
	})
}

// Errorf reports an error at path.
func (r *Report) Errorf(path, format string, args ...any) {
	r.add(Error, path, fmt.Sprintf(format, args...))
}

// Warnf reports a warning at path.
func (r *Report) Warnf(path, format string, args ...any) {
	r.add(Warning, path, fmt.Sprintf(format, args...))
}

// Err returns the errors as one error that joins them, or nil when there are
// none. It is always a join, even for one finding, so a caller that walks to a
// joined error to count the problems keeps working. Warnings are not in it.
func (r Report) Err() error {
	var errs []error
	for _, f := range r.Findings {
		if f.Severity == Error {
			errs = append(errs, f)
		}
	}
	return errors.Join(errs...)
}

// Warnings returns the findings that are warnings, in the order reported.
func (r Report) Warnings() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity == Warning {
			out = append(out, f)
		}
	}
	return out
}
