# PROPOSAL

**Status:** implemented. The conveyor in `internal/proposal`, which every command that writes a pull request runs, and the sources of proposals built on it. Today those are `gero ship` ([PUSH_TO_REPO](PUSH_TO_REPO.md)), `gero ci:ship` ([CI_SHIP](CI_SHIP.md)) and `gero repo sync` ([REPO_SYNC](REPO_SYNC.md)).

## What a proposal is

A **proposal** is a change to one base branch of one repo, to be made as one pull request:

* **Where:** the repo, the base branch, and the commit read at its head on the host.
* **What:** a write in a work directory, with the commits it makes: a release into the holdings of a consumer, or the corrected holdings of a project, and then the steps of the Edits that follow them.
* **How it reads:** the branch, the title, the body, the commit messages and the labels (`Preview`).
* **Who it is:** a marker in the body (source, repo, base branch and a version), so a later run finds its own pull request. How two versions of a marker are ordered is the source's: semver for a release, equality of a fingerprint for a sync, where any other fingerprint is stale.

A source supplies those four things and nothing else. It decides *what to propose*: `internal/ship` resolves a release and the branches it goes to, and `internal/reposync` reads what a project holds against config. It never decides how a pull request is made.

## Life cycle

One run proposes to one repo, over several branches, and has one session:

1. **Begin** sets the run up: the clone and its upstream remote, the publisher (where branches are pushed, and who is running so that the actor's own PRs are known), and who the commits are made as (the bot's in CI).
2. **Decide**, for each proposal, looks at the open PRs of its base branch, reading only: one of the actor's own that is the same is already open, an older one is stale and will be closed, a newer one blocks, and the PRs of others are reported and never touched. A failed lookup fails the proposal before any work.
3. **Confirm** asks the one question, when the run publishes and something is to be written. A dry run, `--no-push` and a run with nothing to write are not asked.
4. **Enact**, for each proposal, does the work: a proposal that needs none is its own result; one whose PR is already open only has its older PRs closed (and may be refreshed); otherwise **Commit** makes the branch in the clone or a worktree, runs the source's write and ends the work whatever happened, and the publisher pushes it, opens the PR and closes what it replaces.
5. The source reports. A run reports to its own events and text; the conveyor reports through a small `Event` and `Sink` that a source may pass on as its own.

## Packages

```
platform/github/pulls   the PR service: find, create, comment on, close and label
task/*                  one action each: createbranch, bumpcommit, pushbranch, createpr, createtag
proposal                the conveyor above: Begin, Decide, Commit, Enact, the Publisher, the Executor, the Marker, the results
ship, reposync          sources of proposals: what to propose, the write, the texts, the order of two versions
cli/proposer            the wiring from Settings: the executor, the GitHub remote, the push context, the question, the clone adapter
cli/ship, cli/reposync  one command each: flags, report and exit rules
```

`internal/proposal` calls the tasks and the PR service; it does not replace them. It imports no source, and lint keeps `ship` and `reposync` from importing each other, so what both need goes in the conveyor ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)). The helpers the tests of every source share (real git repositories on disk, the fake GitHub) are in `internal/proposal/proposaltest`.

## Not built yet

* Refreshing an open PR (rebuild it and replace its branch when it differs) is a hook of the conveyor, and its logic is still `ship`'s, so `repo sync` has no `--refresh`.
* Closing the PRs a proposal replaces, and the decision itself, as tasks: both callers are in the conveyor, so a task is made when a caller outside it needs one.

## Open questions

None yet.
