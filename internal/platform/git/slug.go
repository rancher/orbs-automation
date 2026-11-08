package git

import (
	"errors"
	"net/url"
	"strings"
)

// ErrNotRemoteURL is returned when a remote URL has no owner/name path.
var ErrNotRemoteURL = errors.New("remote URL has no owner/name path")

// SlugFromURL returns the owner/name slug of a remote URL: the last two path
// segments, without a .git suffix. The host is not checked. It accepts https,
// ssh, git and scp-like (git@host:owner/name) URLs, and rejects local paths and
// file URLs, which name no remote.
func SlugFromURL(raw string) (string, error) {
	path, ok := remotePath(strings.TrimSpace(raw))
	if !ok {
		return "", ErrNotRemoteURL
	}
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	if len(segs) < 2 {
		return "", ErrNotRemoteURL
	}
	owner, name := segs[len(segs)-2], strings.TrimSuffix(segs[len(segs)-1], ".git")
	if name == "" {
		return "", ErrNotRemoteURL
	}
	return owner + "/" + name, nil
}

// remotePath returns the path part of a URL that points at a remote host.
func remotePath(raw string) (string, bool) {
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "file" || u.Host == "" {
			return "", false
		}
		return u.Path, true
	}
	// scp-like: [user@]host:path, where the colon comes before any slash.
	host, path, found := strings.Cut(raw, ":")
	if !found || host == "" || strings.Contains(host, "/") {
		return "", false
	}
	return path, true
}
