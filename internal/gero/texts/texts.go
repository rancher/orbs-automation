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

// Body is the PR body of a bump.
func Body() string { return text("body.md") + "\n" }

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
