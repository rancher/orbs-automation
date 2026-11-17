package reposync

import (
	"context"
	"errors"
	"fmt"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/platform/git"
	"github.com/rancher/orbs-automation/internal/tagscheme"
)

// SeriesHolding is what a project holds of the series of one stream, line by line: a file that
// another tool reads to pick the newest release of a series. It names no release, so it is read
// against the series the stream has for the line, and a difference is the series-differs warning.
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
	// Left is why a difference is not one to correct: the holding holds a newer release than
	// the newest stable one, or it pins another Artifact, which ship writes.
	Left string `json:"left,omitempty"`
}

// SeriesHoldings reads each series holding of a project, for each line it holds, at the head
// of the line's branch on the host, and says how it stands against the series its stream has
// for the line. A branch that cannot be read has its Error set and does not stop the others.
// A project with no series holding has none. The release holdings of a project are what
// `repo status` shows as what it takes from each producer; sync reads them again to correct them.
func SeriesHoldings(ctx context.Context, ws Reader, holder *chain.Project) ([]SeriesHolding, error) {
	all, err := pulledHoldingsOf(holder, nil)
	if err != nil {
		return nil, err
	}
	var mine []*chain.Holding
	for _, h := range all {
		if h.Kind() == chain.HoldsSeries {
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
	tags := newTagSource(ctx, ws)
	var out []SeriesHolding
	for _, h := range mine {
		held := SeriesHolding{Name: h.Name(), Producer: h.Producer().Slug(), Stream: streamOf(h).Name()}
		for _, ed := range h.Edits() {
			held.Edits = append(held.Edits, ed.Name())
		}
		for _, line := range seriesLinesOf(holder, h) {
			got := heldLine(ctx, ws, holder, refs, tags, h, line)
			held.Lines = append(held.Lines, got.SeriesLine)
		}
		out = append(out, held)
	}
	return out, nil
}

// streamOf is the stream a holding holds something of: the stream of a series, or the stream of
// the artifact of a release.
func streamOf(h *chain.Holding) *chain.Stream {
	if s := h.Stream(); s != nil {
		return s
	}
	if a := h.Artifact(); a != nil {
		return a.Stream()
	}
	return nil
}

// tagSource reads the tags of a producer from the host, once for each repository in a run.
type tagSource func(slug string) ([]git.Tag, error)

func newTagSource(ctx context.Context, ws Reader) tagSource {
	type result struct {
		tags []git.Tag
		err  error
	}
	seen := map[string]result{}
	return func(slug string) ([]git.Tag, error) {
		if r, ok := seen[slug]; ok {
			return r.tags, r.err
		}
		refs, err := ws.UpstreamRefs(ctx, slug)
		seen[slug] = result{refs.Tags, err}
		return refs.Tags, err
	}
}

// seriesLinesOf is the lines a pulled holding is for: those the holder lists, or every line of
// the stream for a holder with one branch, that the stream has something for and the
// holding's range holds.
func seriesLinesOf(holder *chain.Project, h *chain.Holding) []string {
	candidates := holder.Lines()
	if stream := streamOf(h); len(candidates) == 0 && stream != nil {
		candidates = stream.Lines()
	}
	var out []string
	for _, line := range candidates {
		if h.AppliesTo(line) {
			out = append(out, line)
		}
	}
	return out
}

// held is what a holding holds on one line, and, for a release, the release it should hold.
type held struct {
	SeriesLine
	release *bump.Release
}

// heldLine reads one line of a pulled holding at the head of the line's branch.
func heldLine(ctx context.Context, ws Reader, holder *chain.Project, refs git.Refs, tags tagSource, h *chain.Holding, line string) held {
	out := held{SeriesLine: SeriesLine{Line: line}}
	branch, ok := holder.BranchOfLine(line, h.Stage())
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
	var rel bump.Release
	rule := chain.RuleSeriesDiffers
	if h.Kind() == chain.HoldsRelease {
		rule = chain.RuleReleaseDiffers
		if len(h.Pinned()) > 0 {
			out.Left = "it pins another Artifact, whose release comes from a go.mod: ship writes it"
			return out
		}
		want, err := wantedRelease(h, line, tags)
		if err != nil {
			out.Error = err.Error()
			return out
		}
		rel = want
		out.release = &want
	}
	check, err := bump.CheckHolding(ctx, bump.HostTree(ws, holder.Slug(), head.Commit), h, line, branch, rel)
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
	if out.release != nil && check.Relation == bump.RelationOlder {
		out.Left = "it holds a newer release than the newest stable one, and a sync never moves a holding backwards"
		return out
	}
	out.Acknowledged, _ = h.Acknowledgement(rule, line)
	if edits := h.Edits(); len(edits) > 0 && out.Acknowledged == "" {
		out.Suggest = fmt.Sprintf("run `gero repo sync %s --line %s` to open a pull request on %s that writes %s, with Edit %s", holder.Name(), line, branch, out.Want, edits[0].Name())
	}
	return out
}

// wantedRelease is the newest stable release of the stream of a release holding for a line:
// of the series the stream has for the line, or of every tag it accepts when the stream is
// universal. A holding that accepts all takes a prerelease too.
func wantedRelease(h *chain.Holding, line string, tags tagSource) (bump.Release, error) {
	stream := h.Artifact().Stream()
	slug := h.Producer().Slug()
	scheme, err := tagscheme.ForStream(stream)
	if err != nil {
		return bump.Release{}, err
	}
	all, err := tags(slug)
	if err != nil {
		return bump.Release{}, fmt.Errorf("read the tags of %s: %w", slug, err)
	}
	arg := ""
	if !stream.Universal() {
		arg = line
	}
	rel, err := tagscheme.ResolveRelease(scheme, slug, all, arg, h.AcceptsAll(), h.AcceptsAll())
	if errors.Is(err, tagscheme.ErrNoRelease) {
		return bump.Release{}, fmt.Errorf("%s has no stable release of %s to hold for %s", slug, stream.Name(), line)
	}
	if err != nil {
		return bump.Release{}, err
	}
	return bump.ReleaseOf(h, rel.Tag, rel.Commit)
}
