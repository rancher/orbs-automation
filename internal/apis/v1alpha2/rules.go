package v1alpha2

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/config"
)

// slug is what a lowercase slug looks like, as in a team name.
var slug = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// lineID is what a Rancher line looks like.
var lineID = regexp.MustCompile(`^v[0-9]+\.[0-9]+$`)

// oneOf reports an error at path unless value is one of the allowed, and returns
// whether it was.
func oneOf(r *config.Report, path, value string, allowed ...string) bool {
	if slices.Contains(allowed, value) {
		return true
	}
	r.Errorf(path, "must be %s, got %q", orList(allowed), value)
	return false
}

// orList writes a, b or c.
func orList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

// checkLineID reports an error at path unless the value looks like a Rancher line.
func checkLineID(r *config.Report, path, value string) {
	if !lineID.MatchString(value) {
		r.Errorf(path, "%q is not a Rancher line: write it like v2.16", value)
	}
}

// quoteAll quotes each item, for a message that lists them.
func quoteAll(items []string) string {
	q := make([]string, len(items))
	for i, it := range items {
		q[i] = fmt.Sprintf("%q", it)
	}
	return strings.Join(q, ", ")
}

// checkRange reports an error at path unless the range's ends are Rancher lines
// and the since is not after the until.
func checkRange(r *config.Report, path string, rng Range) {
	if rng.Since != "" && !lineID.MatchString(rng.Since) {
		r.Errorf(path+".since", "%q is not a Rancher line: write it like v2.16", rng.Since)
	}
	if rng.Until != "" && !lineID.MatchString(rng.Until) {
		r.Errorf(path+".until", "%q is not a Rancher line: write it like v2.16", rng.Until)
	}
	if !lineID.MatchString(rng.Since) || !lineID.MatchString(rng.Until) {
		return
	}
	since, until := lineNumbers(rng.Since), lineNumbers(rng.Until)
	if since[0] > until[0] || (since[0] == until[0] && since[1] > until[1]) {
		r.Errorf(path, "since %s is after until %s", rng.Since, rng.Until)
	}
}

// lineNumbers reads a line id, or a Kubernetes minor, that already matched as its
// major and minor.
func lineNumbers(id string) [2]int {
	major, minor, _ := strings.Cut(strings.TrimPrefix(id, "v"), ".")
	var n [2]int
	n[0], _ = strconv.Atoi(major)
	n[1], _ = strconv.Atoi(minor)
	return n
}
