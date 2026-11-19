# BACKPORT

**Status:** implemented, except what is under [Not built yet](#not-built-yet). `gero repo backport [project] <line>... <pr>` carries a merged pull request down to the release branches of its own repo, one pull request for each line, and stops for a person when a cherry-pick conflicts.

## Direction

Changes are merged to the default branch and carried down to each release branch from there. `backport` does what a person does by hand: it cherry-picks what the merged PR put on the base branch onto each target branch, and opens one PR for each. It sits beside `repo sync` under `repo`: both write pull requests into the project's own repo, where `ship` writes into other projects' repos. It is a source of proposals on the conveyor ([PROPOSAL](PROPOSAL.md)): it supplies what to propose, the write, the texts and the identity, and the conveyor decides, commits and publishes.

## Command

`gero repo backport [project] <line>[,<line>...] <pr>`

* **`<pr>`** is the last argument: a number, `#123` or a PR URL. A URL carries its repo, so the project may be left out; a project and a URL that name different repos is an error.
* **`<line>`** is a Rancher line (with or without the `v`) or one of the project's release branches, as `repo tag` and `repo branch` take it (`Project.ResolveLineOrBranch`). Several are separated by commas or repeated. There is no `all`, no range, and no tag or series input. An input that is neither a line nor a branch of the project is an error that says what the project has.
* **`project`** is the config name, an `owner/name` slug, or, left out, the repo the current directory is in, as `ship` finds it.
* **Flags** are those of the other writers: `--yes` (`--non-interactive`), `--no-push`, `--dry-run`, `--push-to`, `--preview`. There is no `--refresh`.

Targets are worked newest line first. A target that is the PR's own base branch is refused.

## What a run does

1. **Check the PR on the host, never from a clone.** It exists in the project's repo and is merged, not just closed. Its base must not be one of the targets.
2. **Resolve the targets** and refuse the ones that do not resolve, before anything is written.
3. **Decide each target** with the conveyor. An open backport PR of the actor's own for the same source PR and target, with the same fingerprint, is already open. One with another fingerprint is stale and is closed, with a comment, once the new one is open. A PR of someone else's is reported and never touched. A target whose history already holds the picked commit, found by its `(cherry picked from commit <sha>)` line, is reported as already backported and gets no PR.
4. **Pick the commits** by the rule below. Every pick is `git cherry-pick -x`, so the origin is in the message; the author of each commit is kept and the actor is the committer.
5. **Ask once**, then for each target in order: a branch from the target's head, the picks, the push and the PR. A dry run and `--no-push` do not ask.

### Which commits

Read from the host: the merge commit `M` the PR records, how many parents it has, how many commits `N` the PR has, and the message and author of each.

| What is found | What a person would do | What it does |
|---|---|---|
| `M` has one parent, `N` is 1 | pick the one commit | `cherry-pick -x M` |
| `M` has one parent, `N` > 1, and the `N` commits ending at `M` along first parents have the PR commits' messages and authors, in order | a rebase merge: pick each commit | `cherry-pick -x` of those `N`, oldest first |
| `M` has one parent, `N` > 1, and they do not match | a squash: pick the result | `cherry-pick -x M` |
| `M` has two parents | a merge commit | `cherry-pick -x -m 1 M`, one commit |

What fits none of these (no `M`, commits that cannot be read) is an error that names what was read. It is never a guess. The same PR on the same host always gives the same picks.

## Conflicts

A cherry-pick that conflicts stops everything. Nothing else runs while one is open, and no later line is tried.

* **At a terminal,** the pick is left in progress in the work directory. gero prints the path and the conflicted files, and asks which editor or IDE to use the first time, then keeps the answer in gero's state directory, as it does the REPL's history ([CLI_CONFIG](CLI_CONFIG.md)), since an answer is not a setting. Choosing to edit the files yourself opens nothing and keeps nothing, and a command typed in place of a choice is used and kept. It opens the editor on the path and waits at a prompt: **continue**, **abort**, or open the editor again.
* **Continue** checks that no path is unmerged and no conflict marker is left in what was changed, and says what remains if there is any. When it is clean gero finishes the pick, notes the files that were resolved by hand in the PR body, and goes on to publish and then to the next line.
* **Abort** abandons the pick, removes the branch and the work directory so the clone is as it was found, and ends the run. The line is reported `aborted`, the lines not tried are `not attempted`, and PRs already published stay.
* **With no terminal,** a conflict aborts the pick and fails the run with the conflicted files. There is no resumable state and no mode that waits.
* **A dry run never waits.** It reports a conflict as one it would meet, naming the files, abandons the pick, and goes on to the next line, since nobody is held up by it.
* **If the editor cannot be asked or started,** the prompt is still there: the directory can be opened by hand. A resolver that fails ends the run as an abort does.

A run that is started again after any of this is safe: the decision in step 3 finds what is already open or already backported.

## Texts and identity

* **Texts.** Its own title, commit message and body (`backport_*` in `internal/gero/texts/defaults`), as `sync` has; a project's `pullRequests.title` does not apply. The title is `[Backport <line>] <Original title>`. The body links the source PR with a plain reference and no `@` mention, lists the picked commits, and names any files resolved by hand.
* **Identity.** The branch is built from `backport`, the source PR number and the target branch. The marker names `backport`, the repo, the target branch and a fingerprint of the picked commits.

## Not built yet

* A branch an earlier run left in the clone (a `--no-push` run, or a push that failed) is not reused. The conveyor knows a branch as its own by the author of its commits, and a pick keeps the author of the commit it picks, so a second run stops at "not ours to change" until the branch is deleted.

## Open questions

None yet.
