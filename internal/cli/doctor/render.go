package doctor

// Build says which gero produced a report, for the header and the JSON.
type Build struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// Report is everything the renderers need.
type Report struct {
	Build    Build
	Outcomes []Outcome
}

// Render formats r as clicfg.OutputText or clicfg.OutputJSON. A check's message
// is whatever the check chose to say, and no check is given a token to say it
// with.
func Render(_ Report, _ string) (string, error) {
	return "", nil
}

// RenderChecks formats the check list for --list-checks as clicfg.OutputText or
// clicfg.OutputJSON.
func RenderChecks(_ []Check, _ string) (string, error) {
	return "", nil
}
