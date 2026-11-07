// Package linesresolve holds the logic of gero lines resolve: the release-line map
// of the whole config, from the Rancher lines (newest first) through the project
// that is the end of the line for most things (rancher/rancher, the hub) to the
// projects that feed it and the ones it feeds. It reads the central config and the
// clock and never the network (gero repo status does). It is framework-free: cmd/
// owns the cobra wiring.
package linesresolve

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
)

// ErrNoHub is returned when no project can be taken as the hub of the map, and
// none was named.
var ErrNoHub = errors.New("no hub project")

// Workspace is what the command needs from the invocation's workspace. It is
// defined here, where it is consumed, so tests can use a fake.
type Workspace interface {
	// Config is the central application config, with defaults applied and
	// validated.
	Config() (config.Config, error)
	// Updates builds the engine of the update methods, which routes the edges.
	Updates() (*updatedispatcher.Engine, error)
}

// Options is what the user asked for.
type Options struct {
	Hub    string   // the project the lines are connected through, by config name or slug; "" for the default
	Lines  []string // only these lines, such as v2.15 or 2.15; none for all
	Phases []string // only lines in these phases (development, ga, eom, eol); none for all
}

// Result is the map: one entry for each Rancher line, newest first.
type Result struct {
	Hub     string `json:"hub"`     // the hub's repository, owner/name
	HubName string `json:"hubName"` // its config name
	Lines   []Line `json:"lines"`
	// Gaps is every gap of every line, as the sentences each line carries, so a
	// reader can see at once whether the map has any.
	Gaps []string `json:"gaps,omitempty"`
}

// Line is one Rancher line and what is connected to it.
type Line struct {
	ID         string     `json:"id"`
	Phase      string     `json:"phase"`
	Dates      Dates      `json:"dates"`
	Kubernetes Kubernetes `json:"kubernetes"`
	// Hub is the hub's own branch and series for the line, nil when it has none.
	Hub *Node `json:"hub,omitempty"`
	// TakesFrom are the projects whose releases are shipped to the hub's branch for
	// the line, and ShipsTo the projects that the hub's releases for the line go to.
	TakesFrom []Link `json:"takesFrom,omitempty"`
	ShipsTo   []Link `json:"shipsTo,omitempty"`
	// Also are the projects that have the line but are not connected to the hub on
	// it directly, with their own connections, which is where a chain through
	// another project shows.
	Also []Also `json:"also,omitempty"`
	// Gaps are what does not connect: a project that ships to the hub but has no
	// such line, say.
	Gaps []string `json:"gaps,omitempty"`
}

// Dates are the lifecycle dates of a line, as YYYY-MM-DD, empty when not set.
type Dates struct {
	GA  string `json:"ga,omitempty"`
	EOM string `json:"eom,omitempty"`
	EOL string `json:"eol,omitempty"`
}

// Kubernetes is the supported Kubernetes minor range of a line.
type Kubernetes struct {
	Min string `json:"min,omitempty"`
	Max string `json:"max,omitempty"`
}

// Node is a project's own place on a line: its branch and the series of versions.
type Node struct {
	Project string `json:"project"` // the repository, owner/name
	Name    string `json:"name"`    // its config name
	Branch  string `json:"branch"`
	// IsDefaultBranch is set when the line has no branch of its own, so Branch is the
	// project's default branch.
	IsDefaultBranch bool   `json:"isDefaultBranch,omitempty"`
	Series          string `json:"series,omitempty"`
}

// Link is a connection to another project on a line: the project, how the update is
// made, and what it holds of the line.
type Link struct {
	Project string `json:"project"`
	Name    string `json:"name"`
	Method  string `json:"method"`
	// Series and Branch are the other project's own series and branch for the line,
	// when it ships (a producer) and has them.
	Series string `json:"series,omitempty"`
	Branch string `json:"branch,omitempty"`
	// Universal says the producer has no lines of its own: its releases go to every
	// release branch of the consumer, whatever the line.
	Universal bool `json:"universal,omitempty"`
	// Branches are the consumer's branches a release goes to, for a link to a
	// consumer.
	Branches []string `json:"branches,omitempty"`
}

// Also is a project on a line that is not connected to the hub on it directly.
type Also struct {
	Node
	TakesFrom []Link `json:"takesFrom,omitempty"`
	ShipsTo   []Link `json:"shipsTo,omitempty"`
}

// Resolve builds the map from the config and the engine, at the time now.
//
// STUB: builds nothing until a later commit.
func Resolve(cfg config.Config, engine *updatedispatcher.Engine, opts Options, now time.Time) (Result, error) {
	return Result{}, nil
}

// Run writes the map to w in the format the settings ask for.
//
// STUB: does nothing until a later commit.
func Run(ctx context.Context, w io.Writer, s clicfg.Settings, cwd string, opts Options) error {
	return nil
}
