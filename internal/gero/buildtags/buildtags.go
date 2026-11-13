// Package buildtags names the tags gero repo build-tags prints for a branch: the
// rolling branch tag, the static tag for one commit, and the previous static tag.
// The names follow Rancher's branches (main, and release/<version>) and the build
// tags its CI publishes, so they are a rule of gero's and not of git. It does no
// I/O besides the Tags call.
package buildtags

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	mainRef    = "refs/heads/main"
	releaseRef = "refs/heads/release/"
)

// ErrUnsupportedRef is returned for any ref other than refs/heads/main and
// refs/heads/release/<version>.
var ErrUnsupportedRef = errors.New("unsupported branch pattern")

// BranchTags holds the three values the script prints.
type BranchTags struct {
	BranchTag       string
	BranchStaticTag string
	PrevStaticTag   string
}

// Tags lists tag names, newest first by creator date.
type Tags interface {
	NewestFirst() ([]string, error)
}

// ResolveBranchTags derives the branch tag, the static tag and the previous static tag
// for ref. commit is used as given, so the caller decides how it is
// abbreviated. The newest tag starting with the static-tag prefix is the
// previous one, even when it points at the current commit.
func ResolveBranchTags(ref, commit string, tags Tags) (BranchTags, error) {
	var res BranchTags
	var prefix string
	switch {
	case ref == mainRef:
		res.BranchTag = "head"
		prefix = "main-"
	case strings.HasPrefix(ref, releaseRef):
		version := strings.TrimPrefix(ref, releaseRef)
		res.BranchTag = version + "-head"
		prefix = version + "-head-"
	default:
		return BranchTags{}, fmt.Errorf("%w: %q, expected 'main' or 'release/*'", ErrUnsupportedRef, ref)
	}
	res.BranchStaticTag = prefix + commit

	names, err := tags.NewestFirst()
	if err != nil {
		return BranchTags{}, fmt.Errorf("list tags: %w", err)
	}
	res.PrevStaticTag = newestWithPrefix(names, prefix)
	return res, nil
}

// newestWithPrefix returns the first name matching prefix. The script hands the
// prefix to grep as a regex, so the "." in "v2.x-head-" matches any character;
// that is kept. A prefix that is not a valid regex matches nothing, as a failed
// grep would.
func newestWithPrefix(names []string, prefix string) string {
	re, err := regexp.Compile("^" + prefix)
	if err != nil {
		return ""
	}
	for _, name := range names {
		if re.MatchString(name) {
			return name
		}
	}
	return ""
}
