package v1alpha2

import (
	"fmt"
	"regexp"
	"slices"
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
