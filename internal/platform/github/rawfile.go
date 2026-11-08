package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	if host != "github.com" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedHost, host)
	}
	u := url.URL{Scheme: "https", Host: "raw.githubusercontent.com", Path: "/" + slug + "/" + ref + "/" + path}
	body, status, err := rawGet(ctx, client, u.String(), "")
	if err == nil && status == http.StatusNotFound && token != "" {
		body, status, err = rawGet(ctx, client, u.String(), token)
	}
	switch {
	case err != nil:
		return nil, err
	case status == http.StatusOK:
		return body, nil
	case status == http.StatusNotFound:
		return nil, fmt.Errorf("%w: %s at %s in %s", ErrFileNotFound, path, ref, slug)
	}
	return nil, fmt.Errorf("read %s of %s: %d %s", path, slug, status, http.StatusText(status))
}

// rawGet fetches target, with token when there is one, and returns the body of
// a 200 and the status of any answer.
func rawGet(ctx context.Context, client *http.Client, target, token string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", UserAgent())
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRawFile+1))
	if err != nil {
		return nil, 0, err
	}
	if len(body) > maxRawFile {
		return nil, 0, fmt.Errorf("the file is larger than %d bytes", maxRawFile)
	}
	return body, resp.StatusCode, nil
}
