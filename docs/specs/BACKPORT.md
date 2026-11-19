# BACKPORT

**Status:** implemented, except what is under [Not built yet](#not-built-yet). `gero repo backport [project] <line>[,<line>...] <pr>[,<pr>...]` carries merged pull requests down to the release branches of its own repo, one pull request for each line, whether it is one PR or a bundle of several, and stops for a person when a cherry-pick conflicts.

## Direction

Changes are merged to the default branch and carried down to each release branch from there. `backport` does what a person does by hand: it cherry-picks what the merged PR put on the base branch onto each target branch, and opens one PR for each. It sits beside `repo sync` under `repo`: both write pull requests into the project's own repo, where `ship` writes into other projects' repos. It is a source of proposals on the conveyor ([PROPOSAL](PROPOSAL.md)): it supplies what to propose, the write, the texts and the identity, and the conveyor decides, commits and publishes.

## Command

`gero repo backport [project] <line>[,<line>...] <pr>[,<pr>...]`

* **`<pr>`** is the last argument: one or more merged PRs, separated by commas, each a number, `#123` or a PR URL. A URL carries its repo, so the project may be left out; a project and a URL that name different repos, or URLs of different repos, is an error. Several PRs make a [bundle](#bundles). A PR named twice counts once, and a bundle has at most 10.
* **`<line>`** is a Rancher line (with or without the `v`) or one of the project's release branches, as `repo tag` and `repo branch` take it (`Project.ResolveLineOrBranch`). Several are separated by commas or repeated. There is no `all`, no range, and no tag or series input. An input that is neither a line nor a branch of the project is an error that says what the project has.
* **`project`** is the config name, an `owner/name` slug, or, left out, the repo the current directory is in, as `ship` finds it.
* **Flags** are those of the other writers: `--yes` (`--non-interactive`), `--no-push`, `--dry-run`, `--push-to`, `--preview`. There is no `--refresh`.

Targets are worked newest line first. A target that is the PR's own base branch is refused.

## What a run does

1. **Check the PRs on the host, never from a clone.** Each exists in the project's repo and is merged, not just closed. Its base must not be one of the targets.
2. **Resolve the targets** and refuse the ones that do not resolve, before anything is written.
3. **Decide each target** with the conveyor. An open backport PR of the actor's own for the same source PR and target, with the same fingerprint, is already open. One with another fingerprint is stale and is closed, with a comment, once the new one is open. A PR of someone else's is reported and never touched. A target that already has what is to be picked, found as under [Bundles](#bundles), is reported as already backported and gets no PR.
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

## Bundles

Several PRs that belong together go to each target as one pull request.

* **Order.** The PRs are picked in the order their merge commits sit on the base branch, oldest first: one comes before another when it is an ancestor of it. How they were typed never matters. All of them must be merged to the same base, or the run refuses and names the one that differs. Each PR gets the rule above on its own, so a squash, a rebase and a merge commit can be in one bundle; the commits are joined in PR order.
* **What is already there.** For each target and each PR, a PR is left out when every one of its picks is already on the target, whoever put it there. A pick is found there by the `(cherry picked from commit <sha>)` line, which a `cherry-pick -x` leaves, by hand or by gero, or by a commit with the same patch-id (`git cherry`), which catches a pick made without `-x` and a backport that was squash-merged without the line. A merge-commit pick has no patch-id and is found by the line only. This is read from the host before any pick, because what remains decides the identity of the PR. A pick made by hand with its conflicts resolved differently has another patch-id and no line, so it is not recognised: its pick comes out empty or conflicts, and the target fails naming the PR; running again without that PR is the way past it.
* **What remains.** Two or more PRs are a bundle PR. One is a single-PR backport, as if it alone had been named, and its body says which PRs were already on the branch. None is "already backported", with no PR.
* **Texts.** A bundle's title is `[Backport <line>] Backport of: #12, #15`, and its body lists each source PR, linked by its title, then the picked commits, each naming its PR. A single PR keeps `[Backport <line>] <Original title>`.
* **Identity.** The marker's source is `backport-` and the numbers of the PRs that remain, sorted and joined by `+` (`backport-12+15`); a single PR is `backport-12`. The branch is built from it, the fingerprint and the target branch.
* **Overlap.** When the PRs of a new backport include every PR of an open backport PR of the actor's own on the same target, that PR is closed with a comment once the new one is open. A PR of someone else's is never closed. Where the sets only partly overlap, or the new one does not cover the open one, the report names the open PR and nothing is closed.
* **Conflicts** are as below, and the prompt and the PR body name the PR whose pick stopped. There is no choice to leave one PR out of a bundle in midway, because the identity is fixed before the picks.

## Conflicts

A cherry-pick that conflicts stops everything. Nothing else runs while one is open, and no later line is tried.

* **At a terminal,** the pick is left in progress in the work directory. gero prints the path and the conflicted files, and asks which editor or IDE to use the first time, then keeps the answer in gero's state directory, as it does the REPL's history ([CLI_CONFIG](CLI_CONFIG.md)), since an answer is not a setting. Choosing to edit the files yourself opens nothing and keeps nothing, and a command typed in place of a choice is used and kept. It opens the editor on the path and waits at a prompt: **continue**, **abort**, or open the editor again.
* **Continue** checks that no path is unmerged and no conflict marker is left in what was changed, and says what remains if there is any. When it is clean gero finishes the pick, notes the files that were resolved by hand in the PR body, and goes on to publish and then to the next line.
* **Abort** abandons the pick, removes the branch and the work directory so the clone is as it was found, and ends the run. The line is reported `aborted`, the lines not tried are `not attempted`, and PRs already published stay.
* **With no terminal,** a conflict aborts the pick and fails the run with the conflicted files. There is no resumable state and no mode that waits.
* **A dry run never waits.** It reports a conflict as one it would meet, naming the files, abandons the pick, and goes on to the next line, since nobody is held up by it.
* **If the editor cannot be asked or started,** the prompt is still there: the directory can be opened by hand. A resolver that fails ends the run as an abort does.

A run that is started again after any of this is safe: the decision in step 3 finds what is already open or already backported.

## Report

The report, as text or as `--json`, mirrors the input: the PRs are a list, `prs`, of one or more, each with its number, title, URL, how it landed (`shape`) and the commits it picks. Each target carries the PRs it includes, the ones it left out because they were already there, the commits it picks, and its result, which is one pull request.

## Texts and identity

* **Texts.** Its own title, commit message and body (`backport_*` in `internal/gero/texts/defaults`), as `sync` has; a project's `pullRequests.title` does not apply. The title of a single PR is `[Backport <line>] <Original title>`. The body links the source PR with a plain reference and no `@` mention, lists the picked commits, and names any files resolved by hand.
* **Identity.** The branch is built from `backport`, the source PR numbers and the target branch. The marker names `backport`, the repo, the target branch and a fingerprint of the picked commits.

## Not built yet

* A branch an earlier run left in the clone (a `--no-push` run, or a push that failed) is not reused. The conveyor knows a branch as its own by the author of its commits, and a pick keeps the author of the commit it picks, so a second run stops at "not ours to change" until the branch is deleted.

## Open questions

None yet.
