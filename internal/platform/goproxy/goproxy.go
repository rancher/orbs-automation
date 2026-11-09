// Package goproxy asks a Go module proxy whether a version of a module is there,
// as `go get` would find it: a plain HTTP request to the proxy's `.info` for the
// version, with no Go toolchain. A release of a Go module exists for a consumer
// when its tag is on the repository and the proxy knows the version.
package goproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the public Go module proxy.
const DefaultBaseURL = "https://proxy.golang.org"

// Status is the outcome of looking for one version of a module.
type Status string

// The outcomes. NotFound means the proxy answered and has no such version, which
// is a tag it cannot serve yet, or a release that was not published. Unreachable
// means nothing is known: the proxy could not be asked, or answered with
// something else.
const (
	Found       Status = "found"
	NotFound    Status = "not found"
	Unreachable Status = "unreachable"
)

// Result is a Status and, when it is Unreachable, why.
type Result struct {
	Status Status
	Detail string
}

// Checker asks a Go module proxy whether a version of a module exists. The zero
// value works: it uses the public proxy and a client with a timeout.
type Checker struct {
	Client  *http.Client // nil means a client with a 20 second timeout
	BaseURL string       // "" means DefaultBaseURL; another proxy, or a test server
}

// Exists reports whether the proxy has version of module. It asks for the info of
// the version with GET: a 200 whose JSON is for that version is Found, a 404 or a
// 410 is NotFound, and anything else, including a 200 that is not the version,
// is Unreachable with the reason. The proxy fetches a tag it has not seen when it
// is first asked, so asking is also what makes it available. It never fails: a
// problem reaching the proxy is an Unreachable result with the reason.
func (c Checker) Exists(ctx context.Context, module, version string) Result {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+escape(module)+"/@v/"+escape(version)+".info", nil)
	if err != nil {
		return Result{Status: Unreachable, Detail: err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gero")
	resp, err := client.Do(req)
	if err != nil {
		return Result{Status: Unreachable, Detail: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return foundVersion(resp.Body, version)
	case http.StatusNotFound, http.StatusGone:
		return Result{Status: NotFound}
	}
	return Result{Status: Unreachable, Detail: fmt.Sprintf("the proxy answered %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))}
}

// foundVersion reads the info the proxy sent, and says Found only when it is for
// the version that was asked for.
func foundVersion(body io.Reader, version string) Result {
	var info struct{ Version string }
	if err := json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&info); err != nil {
		return Result{Status: Unreachable, Detail: "the proxy's answer is not the info of a version: " + err.Error()}
	}
	switch info.Version {
	case version:
		return Result{Status: Found}
	case "":
		return Result{Status: Unreachable, Detail: "the proxy's answer names no version"}
	}
	return Result{Status: Unreachable, Detail: fmt.Sprintf("the proxy answered for %s, not %s", info.Version, version)}
}

// escape writes a module path or a version as the proxy protocol wants it: each
// capital letter is a ! and the lowercase letter, so the path is the same on a
// file system that ignores case.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}
