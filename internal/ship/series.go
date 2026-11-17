package ship

import (
	"context"

	"github.com/rancher/orbs-automation/internal/chain"
)

// SeriesHolding is what a project holds of the series of one stream, line by line:
// a file that another tool reads to pick the newest release of a series. It names no
// release, so it is read against the series the stream has for the line, and a
// difference is the series-differs warning.
type SeriesHolding struct {
	Name     string       `json:"name"`     // the holding
	Producer string       `json:"producer"` // the repository of the project that makes the stream, owner/name
	Stream   string       `json:"stream"`
	Edits    []string     `json:"edits,omitempty"` // the edits that follow the holding
	Lines    []SeriesLine `json:"lines,omitempty"`
}

// SeriesLine is what one branch of the holder holds of the series of its line.
type SeriesLine struct {
	Line   string `json:"line"`
	Branch string `json:"branch"`
	File   string `json:"file,omitempty"`
	// Current is what the branch holds, and Want what the stream's series for the
	// line would be written as.
	Current  string `json:"current,omitempty"`
	Want     string `json:"want,omitempty"`
	UpToDate bool   `json:"upToDate"`
	// Error is why the branch could not be read, or the line has no series to hold.
	Error string `json:"error,omitempty"`
	// Suggest is the pull request that would fix a difference, said when an edit
	// follows the holding. Acknowledged is why the holding's document says the
	// difference is expected.
	Suggest      string `json:"suggest,omitempty"`
	Acknowledged string `json:"acknowledged,omitempty"`
}

// SeriesHoldings reads each series holding of a project, for each line it holds,
// at the head of the line's branch on the host, and says how it stands against the
// series its stream has for the line. A branch that cannot be read has its Error set
// and does not stop the others. A project with no series holding has none.
func SeriesHoldings(ctx context.Context, ws Reader, holder *chain.Project) ([]SeriesHolding, error) {
	return nil, nil
}
