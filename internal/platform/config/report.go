package config

import (
	"errors"
	"fmt"
	"strings"
)

// RuleUnknownKey is the rule of the warning for a key a user file does not have.
const RuleUnknownKey = "unknown-key"

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
	Line     string // the Rancher line a warning is about, when it is about one
	Reason   string // why a warning is expected, when the document acknowledges it
	Rule     string // the stable id of the rule a warning is of, such as circular-holding; none for an error
	Msg      string
}

// Error returns the finding as one line naming the source, the document, the path
// and the message, leaving out what is not set, and the rule of a warning at the
// end in brackets.
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
	text := strings.Join(parts, ": ")
	if f.Rule != "" {
		text += " [" + f.Rule + "]"
	}
	return text
}

// Report collects the findings of one document.
type Report struct {
	Findings []Finding

	source string
	doc    int
	label  string
}

// NewReport returns an empty Report whose findings name the document: a finding
// reported on it carries the source, the place of the document in it, and the
// label. A check across documents uses it to report on the one at fault.
func NewReport(source string, doc int, label string) Report {
	return Report{source: source, doc: doc, label: label}
}

func (r *Report) add(sev Severity, rule, path, msg string) {
	r.Findings = append(r.Findings, Finding{
		Severity: sev, Source: r.source, Doc: r.doc, Label: r.label, Path: path, Rule: rule, Msg: msg,
	})
}

// Errorf reports an error at path.
func (r *Report) Errorf(path, format string, args ...any) {
	r.add(Error, "", path, fmt.Sprintf(format, args...))
}

// Warnf reports a warning of a rule at path. The rule is the stable id that
// names the warning, which a document can acknowledge.
func (r *Report) Warnf(rule, path, format string, args ...any) {
	r.add(Warning, rule, path, fmt.Sprintf(format, args...))
}

// WarnLinef is Warnf for a warning about one Rancher line, which an
// acknowledgement may then be limited to.
func (r *Report) WarnLinef(rule, _, path, format string, args ...any) {
	r.Warnf(rule, path, format, args...)
}

// Acknowledged returns the warnings that a document says are expected, in the
// order reported. They are not in Warnings.
func (r Report) Acknowledged() []Finding { return nil }

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
