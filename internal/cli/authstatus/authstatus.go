// Package authstatus holds the logic of gero auth status: it resolves the
// GitHub host and token the way every other command will, asks the host who
// the token belongs to, and reports the result without ever printing the
// token. It is framework-free: cmd/ owns the cobra wiring.
package authstatus

// Status is what gero auth status reports.
type Status struct {
	Host        string `json:"host"`
	HostSource  string `json:"hostSource"`
	TokenSource string `json:"tokenSource"`
	Valid       bool   `json:"valid"`
	Login       string `json:"login,omitempty"` // empty when the token is rejected
}

// Render formats st as clicfg.OutputText or clicfg.OutputJSON. The token is
// not part of Status, so it cannot appear.
func Render(_ Status, _ string) (string, error) {
	return "", nil
}
