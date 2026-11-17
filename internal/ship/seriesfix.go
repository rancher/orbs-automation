package ship

import (
	"context"
)

// SeriesInput is what the user asked to fix: the series holdings of one project.
type SeriesInput struct {
	Project string   // the holder, by config name or slug; empty for the one the working directory is in
	Holding string   // limit the run to one holding, by name
	Lines   []string // limit the run to these Rancher lines
	// BranchPrefix is the user's branchPrefix setting, which replaces the project's.
	BranchPrefix string
	DryRun       bool   // compute and report, and create no branch or PR
	NoPush       bool   // stop after the commit, leaving the branch in the clone
	PushTo       string // fork or upstream, or empty to let the settings decide
	CI           bool   // the actor is the bot
}

// SeriesPlan is what a fix of the series holdings of a project would do, and what
// a run did.
type SeriesPlan struct {
	Holder  string         `json:"holder"` // the repository of the project, owner/name
	Clone   string         `json:"clone,omitempty"`
	Remote  string         `json:"remote,omitempty"`
	Push    *PushTarget    `json:"push,omitempty"`
	Actor   *Actor         `json:"actor,omitempty"`
	Targets []SeriesTarget `json:"targets,omitempty"`
}

// SeriesTarget is one line of one series holding: what its branch holds, what it
// should, and whether a PR is needed.
type SeriesTarget struct {
	SeriesLine
	Holding string `json:"holding"`
	Status  Status `json:"status"`
	// Reason says why a target is not ready: no Edit follows the holding, so nothing
	// writes it, or the branch could not be read.
	Reason  string        `json:"reason,omitempty"`
	Preview *Preview      `json:"preview,omitempty"` // what a run would write
	Result  *TargetResult `json:"result,omitempty"`  // what a run did, for a target that was ready
}

// FixSeries finds each line of the series holdings of a project whose branch holds a
// series other than the one its stream has, and, when ex is not nil, makes the commit
// for each that an Edit follows, as Ship does for a release: in the engineer's clone
// or a worktree of it, and with a publisher pushed and opened as a PR. A holding no Edit
// follows is only reported, since nothing writes a holding until an Edit says what a
// write of it takes. A target that fails is reported and the others go on.
func FixSeries(ctx context.Context, ws Workspace, in SeriesInput, ex *Executor) (SeriesPlan, error) {
	return SeriesPlan{}, nil
}
