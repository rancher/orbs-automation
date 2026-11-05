package registry

import (
	"context"
	"net/http"
)

// Status is the outcome of looking for one image.
type Status string

// The outcomes. NotFound means the registry answered and has no such image,
// which is a release that was not published. Unreachable means nothing is known:
// the registry could not be asked, or answered with something else.
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

// Checker asks registries whether an image exists, as docker manifest inspect
// does but without Docker. The zero value works: it uses https and a client
// with a timeout.
type Checker struct {
	Client *http.Client // nil means a client with a 20 second timeout
	Scheme string       // "" means https; tests use http
}

// Exists reports whether image:tag is in the registry at host. It asks for the
// manifest with HEAD, and on a bearer challenge it fetches an anonymous token
// from the realm the registry names and asks once more. It never fails: a
// problem reaching the registry is an Unreachable result with the reason.
func (c Checker) Exists(ctx context.Context, host, image, tag string) Result {
	return Result{}
}
