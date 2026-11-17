package reposync

import (
	"context"
	"fmt"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
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
	Head   string `json:"head,omitempty"` // the commit at the head of the branch on the host
	File   string `json:"file,omitempty"`
	// Current is what the branch holds, and Want what the stream's series for the
	// line would be written as.
	Current  string `json:"current,omitempty"`
	Want     string `json:"want,omitempty"`
	UpToDate bool   `json:"upToDate"`
	// Change says in words what a write would change, for a line that differs.
	Change string `json:"change,omitempty"`
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
	var mine []*chain.Holding
	for _, h := range holder.Held() {
		if h.Kind() == chain.HoldsSeries && !h.Virtual() {
			mine = append(mine, h)
		}
	}
	if len(mine) == 0 {
		return nil, nil
	}
	refs, err := ws.UpstreamRefs(ctx, holder.Slug())
	if err != nil {
		return nil, fmt.Errorf("read the branches of %s: %w", holder.Slug(), err)
	}
	var out []SeriesHolding
	for _, h := range mine {
		held := SeriesHolding{Name: h.Name(), Producer: h.Producer().Slug(), Stream: h.Stream().Name()}
		for _, ed := range h.Edits() {
			held.Edits = append(held.Edits, ed.Name())
		}
		for _, line := range seriesLinesOf(holder, h) {
			held.Lines = append(held.Lines, seriesLine(ctx, ws, holder, refs, h, line))
		}
		out = append(out, held)
	}
	return out, nil
}

// seriesLinesOf is the lines a series holding is for: those the holder lists, or every
// line of the stream for a holder with one branch, that the stream has a series for
// and the holding's range holds.
func seriesLinesOf(holder *chain.Project, h *chain.Holding) []string {
	candidates := holder.Lines()
	if len(candidates) == 0 && h.Stream() != nil {
		candidates = h.Stream().Lines()
	}
	var out []string
	for _, line := range candidates {
		if h.AppliesTo(line) {
			out = append(out, line)
		}
	}
	return out
}

// seriesLine reads one line of a series holding at the head of the line's branch.
func seriesLine(ctx context.Context, ws Reader, holder *chain.Project, refs git.Refs, h *chain.Holding, line string) SeriesLine {
	out := SeriesLine{Line: line}
	if h.Kind() != chain.HoldsSeries {
		out.Error = "a release holding is not brought in line yet"
		return out
	}
	branch, ok := holder.BranchOfLine(line, "")
	if !ok {
		out.Error = fmt.Sprintf("%s has no branch for %s", holder.Slug(), line)
		return out
	}
	out.Branch = branch
	head, ok := refs.Branch(branch)
	out.Head = head.Commit
	if !ok {
		out.Error = fmt.Sprintf("%s has no branch %s", holder.Slug(), branch)
		return out
	}
	check, err := bump.CheckHolding(ctx, bump.HostTree(ws, holder.Slug(), head.Commit), h, line, branch, bump.Release{})
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Want, out.Current, out.UpToDate = check.Want, check.Current, check.UpToDate
	if len(check.Locations) > 0 {
		out.File = check.Locations[0].Place.File
	}
	if out.UpToDate {
		return out
	}
	out.Change = check.Change.Description
	out.Acknowledged, _ = h.Acknowledgement(chain.RuleSeriesDiffers, line)
	if edits := h.Edits(); len(edits) > 0 && out.Acknowledged == "" {
		out.Suggest = fmt.Sprintf("run `gero repo sync %s --line %s` to open a pull request on %s that writes %s, with Edit %s", holder.Name(), line, branch, out.Want, edits[0].Name())
	}
	return out
}
