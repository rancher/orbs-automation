package git

import "errors"

// ErrNotRemoteURL is returned when a remote URL has no owner/name path.
var ErrNotRemoteURL = errors.New("remote URL has no owner/name path")

// SlugFromURL returns the owner/name slug of a remote URL: the last two path
// segments, without a .git suffix. The host is not checked.
func SlugFromURL(url string) (string, error) {
	return "", errors.New("not implemented")
}
