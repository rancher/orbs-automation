package ship

import (
	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui"
)

// humanSink shows the events of a run as lines and tables on an Output, as they
// arrive: the title, each branch read, the images, what the PR would be, each
// commit as it is made, and a summary at the end.
type humanSink struct {
	out  ui.Output
	full bool // show the commit message and body of each PR too
}

var _ domain.Sink = (*humanSink)(nil)

func newHumanSink(out ui.Output, full bool) *humanSink { return &humanSink{out: out, full: full} }

// Event shows one event.
//
// STUB: shows nothing until a later commit.
func (h *humanSink) Event(domain.Event) {}
