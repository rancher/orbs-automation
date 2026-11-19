// Package backport holds the logic of gero repo backport: what the command asks of the backport
// domain, the questions it asks a person at a conflict, and how it reports what came of it. It is
// framework-free: cmd/ owns the cobra wiring.
package backport

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Args is what the positional arguments of gero repo backport say.
type Args struct {
	Project string   // the repo, by config name or slug; empty for the one the working directory is in
	Lines   []string // the targets, as typed
	PR      int      // the number of the merged PR, when PRs is not set
	// PRs are the numbers of the merged PRs, one or several, as typed. Several are a bundle.
	PRs []int
	// Repo is the owner/name a PR URL named, or empty when the PR was a number. It must be the
	// repo of the project.
	Repo string
}

// ParseArgs reads [project] <line>[,<line>...] <pr>. The PR is always the last argument: a
// number, #number or the URL of the PR. The targets are the one before it, separated by commas.
func ParseArgs(args []string) (Args, error) {
	if len(args) < 2 {
		return Args{}, errors.New("name the release line or branch to backport to, and the PR: gero repo backport [project] <line>[,<line>...] <pr>")
	}
	if len(args) > 3 {
		return Args{}, errors.New("too many arguments: separate several lines with commas, as v2.15,v2.14")
	}
	var out Args
	last := args[len(args)-1]
	repo, number, ok := parsePR(last)
	if !ok {
		return Args{}, fmt.Errorf("%q is not a PR: the last argument is its number, #number or its URL", last)
	}
	out.PR, out.Repo = number, repo
	if len(args) == 3 {
		out.Project = args[0]
	}
	for _, line := range strings.Split(args[len(args)-2], ",") {
		line = strings.TrimSpace(line)
		if line == "" {
			return Args{}, fmt.Errorf("%q has an empty line in it", args[len(args)-2])
		}
		out.Lines = append(out.Lines, line)
	}
	return out, nil
}

// prURL matches the URL of a pull request on a host.
var prURL = regexp.MustCompile(`^https?://[^/]+/([^/]+/[^/]+)/pull/([0-9]+)/?$`)

// parsePR reads a PR given as 123, #123 or its URL, which also says the repo.
func parsePR(text string) (repo string, number int, ok bool) {
	if m := prURL.FindStringSubmatch(text); m != nil {
		n, _ := strconv.Atoi(m[2])
		return m[1], n, n > 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(text, "#"))
	return "", n, err == nil && n > 0
}
