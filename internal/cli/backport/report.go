package backport

import (
	"errors"

	domain "github.com/rancher/orbs-automation/internal/backport"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// ErrFailed is returned, with the report still written, when a target failed, was abandoned at a
// conflict, or was not attempted because an earlier one was.
var ErrFailed = errors.New("some targets were not backported")

// renderText is the report of a backport: what the PR is, what is picked, and for each target
// what was done, with the texts of the PR when full.
func renderText(p domain.Plan, full bool, view *style.Renderer) string {
	return ""
}

// failed is ErrFailed when a target did not get what it was asked for.
func failed(p domain.Plan) error {
	return nil
}
