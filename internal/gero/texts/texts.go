// Package texts is what gero ship says when a project's config does not say it
// otherwise: the PR title, the branch prefix, the commit messages and the PR body,
// each an embedded file under defaults/ that reads as the text it makes. Every
// commit message ends with the authorship trailer (internal/gero/authorship), which
// is how a later run knows a branch is its own. The placeholders are filled by
// internal/platform/templates.
package texts

import (
	"embed"
	"strings"
)

//go:embed defaults/*
var defaults embed.FS

// text returns an embedded file without its final newline.
func text(name string) string {
	b, err := defaults.ReadFile("defaults/" + name)
	if err != nil {
		panic(err) // the files are embedded at build time, so this cannot happen
	}
	return strings.TrimSuffix(string(b), "\n")
}

// The texts gero embeds, used when a project's config does not set its own. They
// are generic: nothing in them names a project, and what varies comes from the
// placeholders (TextVars). They are plain files under defaults/ so they read as
// the text they produce.

// DefaultTitle is the PR title when pullRequests.title is not set.
func DefaultTitle() string { return text("title.txt") }

// DefaultTitleWith is the PR title when the bump also takes releases that go with
// the one asked for, such as the lasso a wrangler release requires, and
// pullRequests.title is not set. It may use {companions}, as "lasso to v0.2.10".
func DefaultTitleWith() string { return text("title_with.txt") }

// DefaultBranchPrefix is the start of the branch name when neither the user nor
// the project sets one. The rest of the name is the tag and the target branch.
func DefaultBranchPrefix() string { return text("branch_prefix.txt") }

// CommitMessage is the commit message of a bump: a subject, a short account of
// where it came from, and the trailers.
func CommitMessage() string { return text("commit.txt") + "\n" }

// CommitMessageWith is CommitMessage for a bump that also takes releases that go
// with the one asked for.
func CommitMessageWith() string { return text("commit_with.txt") + "\n" }

// TagMessage is the message of the annotated tag gero repo tag makes: the project
// and the tag, where it was tagged from, and the trailer. It may use TagVars.
func TagMessage() string { return text("tag.txt") + "\n" }

// TagVars are the variables of the tag message: the ones that say which release
// of which project it is, and the branch it is on.
//
//	project       the project's name
//	sourceRepo    the project's repository, owner/name
//	tag           the tag that is made
//	version       the tag's version, as the project writes it
//	branch        the branch without "release/"
//	targetBranch  the branch as it is named
var TagVars = []string{"project", "sourceRepo", "tag", "version", "branch", "targetBranch"}

// Body is the PR body of a bump.
func Body() string { return text("body.md") + "\n" }

// SyncTitle is the PR title of a sync: the corrections of one branch of a holder, made
// in one PR. It may use SyncVars.
func SyncTitle() string { return text("sync_title.txt") }

// SyncCommitMessage is the commit message of the write of a sync, with the trailers.
func SyncCommitMessage() string { return text("sync_commit.txt") + "\n" }

// SyncBody is the PR body of a sync.
func SyncBody() string { return text("sync_body.md") + "\n" }

// SyncStepCommitMessage is the commit message of a step of an Edit that changed files,
// in a sync.
func SyncStepCommitMessage() string { return text("sync_step_commit.txt") + "\n" }

// SyncStrategyCommitMessage is the commit message of a strategy of an Edit that changed
// files, in a sync.
func SyncStrategyCommitMessage() string { return text("sync_strategy_commit.txt") + "\n" }

// SyncVars are the variables the texts of a sync may use.
var SyncVars = []string{"holdings", "count", "line", "branch", "targetBranch", "consumer", "changes", "marker", "key", "command", "phase", "strategy", "edit"}

// StrategyCommitMessage is the commit message of a strategy that changed files:
// which strategy ran, what it changed, where it fits in the bump, and the
// trailers. It may use StrategyCommitVars.
func StrategyCommitMessage() string { return text("strategy_commit.txt") + "\n" }

// StepCommitMessage is the commit message of a step that changed files: what
// ran, where it fits in the bump, and the trailers. It may use StepCommitVars.
func StepCommitMessage() string { return text("step_commit.txt") + "\n" }

// StepCommitVars are the variables of the commit message of a step: the ones that
// say what the bump is, and which step this is.
//
//	command  the step's command, as written
//	phase    where the step runs, such as "method postEdit"
var StepCommitVars = []string{
	"project", "source", "sourceRepo", "tag", "version", "branch", "targetBranch",
	"consumer", "sourceReleaseURL", "command", "phase",
}

// StrategyCommitVars are the variables of the commit message of a strategy: the
// ones that say what the bump is, and which strategy this is.
//
//	strategy  the strategy's name, such as goBump
//	edit      the edit that names the strategy
//	changes   what the strategy changed, one bullet for each
var StrategyCommitVars = []string{
	"project", "source", "sourceRepo", "tag", "version", "branch", "targetBranch",
	"consumer", "sourceReleaseURL", "strategy", "edit", "changes",
}

// BackportTitle is the PR title of a backport: the original title, and the line it is carried
// to. It may use BackportVars.
func BackportTitle() string { return text("backport_title.txt") }

// BackportBody is the PR body of a backport.
func BackportBody() string { return text("backport_body.md") + "\n" }

// BackportVars are the variables the texts of a backport may use.
//
//	line          the Rancher line the PR is carried to, or the branch when it has none
//	branch        the target branch without "release/"
//	targetBranch  the target branch as it is named
//	consumer      the repository, owner/name
//	number        the number of the PR that is carried
//	title         its title
//	url           where to see it
//	commits       the commits that are picked, one bullet each
//	conflicts     a section naming the files resolved by hand, or nothing
//	marker        the hidden block that makes the PR known to a later run
//	key           the end of the branch name that gero builds
var BackportVars = []string{"line", "branch", "targetBranch", "consumer", "number", "title", "url", "commits", "conflicts", "marker", "key"}
