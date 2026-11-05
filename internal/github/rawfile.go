package github

import (
	"context"
	"errors"
	"net/http"
)

// ErrFileNotFound is returned when the repository, the ref or the path does not
// exist, or the repository is private and no token was given: the host answers
// the same 404 for all of them.
var ErrFileNotFound = errors.New("file not found")

// maxRawFile bounds what RawFile reads; the files read this way are small
// configuration files.
const maxRawFile = 1 << 20

// RawFile reads one file of the repository slug (owner/name) at ref, a commit or
// a branch, from the host's raw content service. It is a plain HTTP read: it
// needs no API quota and no token for a public repository. It asks anonymously
// first and sends token only when the host answers 404, which is how a private
// repository answers an anonymous read, so a token is never sent to a
// repository that does not need it. An empty token means none is available.
// Only github.com is supported; other hosts return ErrUnsupportedHost.
func RawFile(ctx context.Context, client *http.Client, host, token, slug, ref, path string) ([]byte, error) {
	return nil, nil
}
