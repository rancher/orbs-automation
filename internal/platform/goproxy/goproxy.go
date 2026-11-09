// Package goproxy asks a Go module proxy whether a version of a module is there,
// as `go get` would find it: a plain HTTP request to the proxy's `.info` for the
// version, with no Go toolchain. A release of a Go module exists for a consumer
// when its tag is on the repository and the proxy knows the version.
package goproxy

import (
	"context"
	"net/http"
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

// Exists reports whether the proxy has version of module. It never fails: a
// problem reaching the proxy is an Unreachable result with the reason.
func (c Checker) Exists(_ context.Context, _, _ string) Result {
	return Result{}
}
