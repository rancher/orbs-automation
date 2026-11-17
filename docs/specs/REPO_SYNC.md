# REPO_SYNC

**Status:** implemented for series holdings; see [Not built yet](#not-built-yet). `gero repo sync [project]` brings what a project holds in line with config, and opens one pull request for each of its branches that has something to correct.

## Direction

`gero ship` pushes: the project is the producer of a release, and the writes go into the downstream projects that hold it. `repo sync` pulls: the project is the holder, and the writes go into its own files, from what config says the held values should be. Both end in a pull request on the holder's branch, and both use the same work branch, push, PR identity and commit audit trail ([PUSH_TO_REPO](PUSH_TO_REPO.md)). What differs is where the wanted value comes from: a release the user named, for `ship`; config, for `sync`.

`sync` is not for series holdings alone. A holding is brought in line when its kind has a way to say what its value should be for a line; a series is the first ([CONFIG_SCHEMA](../design/CONFIG_SCHEMA.md#holding)): the series its stream has for the line, written in the holding's form. Any holding a holder keeps, of a release, a commit or a branch, can later say the same, and then one `sync` brings in line everything a project holds.

## What a run does

* **Rows.** For each pulled holding of the project and each line it is for, the value held is read at the head of the line's branch on the host, and compared with the wanted value. The result is a row: current, differs, or unreadable. `gero repo status` shows the same rows under Series, read the same way.
* **One pull request for each branch.** A pull request has one base. A project with a branch for each Rancher line gets one for each branch that differs, and a branch that holds several corrections gets them all in one pull request: the write of every holding in one commit, then each step of the Edits that follow them, run once, each as a commit when it changed files. Lines of one holding that want different values of one place on one branch (a project with a single branch) are left alone and the report says why.
* **Only through an Edit.** A holding is written only when an Edit follows it. A holding with none, and a difference its document acknowledges (`series-differs`), are reported and left alone, and the rest of the branch is still written.
* **Identity.** The branch is the prefix and a key made of `sync`, a fingerprint of what the pull request writes (each holding and the value) and the branch without `release/`. The PR's marker names `sync`, the holder, the branch and the fingerprint. The same fingerprint is the same fix and is already open; any other fingerprint of the actor's own on that branch is an earlier sync that is now stale, so it is commented on and closed once the new one is open. A PR of someone else's is never touched, and one with the same fingerprint is only reported.
* **The rest as for a release.** The clone or worktree, asking once, `--yes`, `--no-push`, `--dry-run`, `--push-to`, the commit identity and signing. There is no `--refresh`. `--holding` and `--line` limit the run.
* **Texts.** Its own title, commit message and body (`sync_*` in `internal/gero/texts/defaults`); a project's `pullRequests.title` does not apply.

## Not built yet

* Holdings of a release, a commit or a branch brought in line by `sync`: what the wanted value is for each (the newest release a line accepts, the head of a branch), and the PR text for it.
* One command run across every project of a team.

## Open questions

None yet.
