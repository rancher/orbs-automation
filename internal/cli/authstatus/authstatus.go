// Package authstatus holds the logic of gero auth status: it resolves the
// GitHub host and token the way every other command will, asks the host who
// the token belongs to, and reports the result without ever printing the
// token. It is framework-free: cmd/ owns the cobra wiring.
package authstatus

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/github"
)

// Status is what gero auth status reports.
type Status struct {
	Host        string `json:"host"`
	HostSource  string `json:"hostSource"`
	TokenSource string `json:"tokenSource"`
	Valid       bool   `json:"valid"`
	Login       string `json:"login,omitempty"` // empty when the token is rejected
}

// hostSources and tokenSources turn the raw source names that go-gh reports
// into words for the text output. JSON keeps the raw names, which scripts can
// match on. A source not listed is shown as it is.
var (
	hostSources = map[string]string{
		github.SourceExplicit: "--hostname",
		"hosts":               "gh config",
	}
	tokenSources = map[string]string{
		"GH_TOKEN":                "environment variable GH_TOKEN",
		"GITHUB_TOKEN":            "environment variable GITHUB_TOKEN",
		"GH_ENTERPRISE_TOKEN":     "environment variable GH_ENTERPRISE_TOKEN",
		"GITHUB_ENTERPRISE_TOKEN": "environment variable GITHUB_ENTERPRISE_TOKEN",
		"oauth_token":             "gh config file",
		"gh":                      "gh auth token (keyring)",
	}
)

// Render formats st as clicfg.OutputText or clicfg.OutputJSON. The token is
// not part of Status, so it cannot appear.
func Render(st Status, format string) (string, error) {
	switch format {
	case clicfg.OutputText:
		return renderText(st), nil
	case clicfg.OutputJSON:
		return renderJSON(st)
	}
	return "", unsupported(format)
}

func unsupported(format string) error {
	return fmt.Errorf("gero auth status does not support output format %q (use text or json)", format)
}

func renderText(st Status) string {
	login := st.Login
	if !st.Valid {
		login = "token rejected by GitHub"
	}
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Host:\t%s (%s)\n", st.Host, label(hostSources, st.HostSource))
	fmt.Fprintf(w, "Token:\t%s\n", label(tokenSources, st.TokenSource))
	fmt.Fprintf(w, "Login:\t%s\n", login)
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	return b.String()
}

func label(names map[string]string, source string) string {
	if s, ok := names[source]; ok {
		return s
	}
	return source
}

func renderJSON(st Status) (string, error) {
	out, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}
