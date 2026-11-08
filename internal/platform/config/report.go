package config

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
// and the message.
func (f Finding) Error() string { return "" }

// Report collects the findings of one document.
type Report struct {
	Findings []Finding
}

// Errorf reports an error at path.
func (r *Report) Errorf(_, _ string, _ ...any) {}

// Warnf reports a warning at path.
func (r *Report) Warnf(_, _ string, _ ...any) {}

// Err returns the errors as one error that joins them, or nil when there are
// none. It is always a join, even for one finding, so a caller that walks to a
// joined error to count the problems keeps working.
func (r Report) Err() error { return nil }

// Warnings returns the findings that are warnings, in the order reported.
func (r Report) Warnings() []Finding { return nil }
