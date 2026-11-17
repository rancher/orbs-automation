package proposal

import (
	"regexp"
	"strings"

	"github.com/rancher/orbs-automation/internal/bump"
)

// blankLines is two or more blank lines in a row.
var blankLines = regexp.MustCompile(`\n{3,}`)

// CollapseBlankLines leaves one blank line where a text has several in a row, as it does
// where a list that was left out had been.
func CollapseBlankLines(text string) string { return blankLines.ReplaceAllString(text, "\n\n") }

// Bullets lists changes, one bullet each.
func Bullets(changes []bump.Change) string {
	lines := make([]string, len(changes))
	for i, c := range changes {
		lines[i] = "- " + c.Description
	}
	return strings.Join(lines, "\n")
}

// FirstNonEmpty is the first of the values that is not empty, and empty for none.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
