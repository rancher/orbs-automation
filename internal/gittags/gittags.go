// Package gittags derives the branch tag names the branch-tags script prints.
// It does no I/O besides the Tags call and never reads the environment.
package gittags

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	mainRef     = "refs/heads/main"
	releaseRef  = "refs/heads/release/"
	shortLength = 7
)

// ErrUnsupportedRef is returned for any ref other than refs/heads/main and
// refs/heads/release/<version>.
var ErrUnsupportedRef = errors.New("unsupported branch pattern")

// Result holds the three values the script prints.
type Result struct {
	BranchTag       string
	BranchStaticTag string
	PrevStaticTag   string
}

// Tags lists tag names, newest first by creator date.
type Tags interface {
	NewestFirst() ([]string, error)
}

// Short returns the first 7 characters of sha, or all of it when shorter. It
// is how a CI commit is abbreviated.
func Short(sha string) string {
	if len(sha) > shortLength {
		return sha[:shortLength]
	}
	return sha
}

// Resolve derives the branch tag, the static tag and the previous static tag
// for ref. commit is used as given, so the caller decides how it is
// abbreviated. The newest tag starting with the static-tag prefix is the
// previous one, even when it points at the current commit.
func Resolve(ref, commit string, tags Tags) (Result, error) {
	var res Result
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
		return Result{}, fmt.Errorf("%w: %q, expected 'main' or 'release/*'", ErrUnsupportedRef, ref)
	}
	res.BranchStaticTag = prefix + commit

	names, err := tags.NewestFirst()
	if err != nil {
		return Result{}, fmt.Errorf("list tags: %w", err)
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
