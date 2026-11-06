# Task commands

**Status:** draft. Nothing here is built. The tasks they run are ([PROJECT_LAYOUT](PROJECT_LAYOUT.md#tasks-single-actions-that-compose)).

The steps of [`gero ship`](PUSH_TO_REPO.md) as commands of their own, for an engineer who wants one part of the process and not the whole. Each runs one task and `ship` chains the same tasks, so a step behaves the same alone and inside `ship`. They are workstation commands: they act as the engineer, in the engineer's own clone, and read the same settings as `ship`.

| Command | Task | Does |
|---|---|---|
| `gero repo branch [project] <line> <name>` | `createbranch` | Makes a branch off the upstream branch of a release line |
| `gero repo push [project] [--branch <name>]` | `pushbranch` | Pushes a branch, never forced |
| `gero pr create [project] [--head <branch>] [--base <line\|branch>]` | `createpr` | Opens a PR for a branch that is already pushed, and labels it |
| `gero ship bump …` **(leaning)** | `bumpcommit` | Applies one change set to a branch and commits it |

## Rules they share

* **Project arguments** are as everywhere ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags)): the config name or `owner/name`, `.` for the project the working directory is in, and the working directory's project when none is named. In `gero repl` that is the project last changed into.
* **The branch is never a silent guess.** Where a command acts on a branch of the engineer's, it takes the checked-out branch unless one is named (`--head` for `pr create`, `--branch` for `repo push`), and says which in the question, the dry run and the report. A detached HEAD with none named is an error that says to name one.
* **Writes ask once, and honor `--dry-run`, `--yes` and `--json`**, as `ship` does ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)). A dry run says what would be done and sends nothing. `--json` prints the task's output.
* **Where a branch is pushed** is `--push-to fork|upstream`, decided as in `ship`: the flag, then the settings, then the fork when one is set. It sets the `owner:branch` head of a PR and the remote of a push.
* **Never forced.** The one replacement is of an earlier attempt's branch whose every commit is ours, and only with `--refresh`, as in `ship`.

## `gero repo branch [project] <line> <name>`

Makes the branch `<name>` at the head of the upstream branch of the release line `<line>` (`v2.15` is `release/v2.15`, found as `gero repo info` finds it), after fetching it. With two arguments they are the line and the name, with three the project comes first.

By default it **leaves your checkout where it is**, as `git branch <name> <upstream>/<branch>` does, and ends by saying `git switch <name>`. `--switch` moves the checkout onto the new branch, and only when the clone is clean; otherwise it fails and says why, since a temporary worktree is no use to someone who wants to work in the branch. A branch of the name that is ours and already on the current head is kept as it is. One with commits that are not ours is an error, and one of ours that the line has moved on from is made again, as `createbranch` does for `ship`.

```
$ gero repo branch v2.15 my-fix
Created my-fix at upstream/release/v2.15 (1b3c9e07) in rancher/rancher
  git switch my-fix

$ gero repo branch scc-operator main bump-deps --switch
Created bump-deps at upstream/main (a41f02cc) in rancher/scc-operator, and switched to it
```

An optional `--from <ref>` for cutting the branch off something other than the head of the line, a branch or a tag, is **(leaning)** and not decided.

## `gero repo push [project] [--branch <name>]`

Pushes a branch of the engineer's to the push target, with the checks `ship` makes: nothing on the remote is the ordinary push, the same commit sends nothing, a branch that grew from the remote's is a fast forward, and a branch that has gone another way is an earlier attempt's, which is replaced only when every commit on it is ours and `--refresh` was given. The error for one that is not ours says whose commit it is.

```
$ gero repo push
Push bot/scc-operator-v0.5.4-v2.15 to origin?  [y/N] y
Pushed bot/scc-operator-v0.5.4-v2.15 to origin
```

## `gero pr create [project] [--head <branch>] [--base <line|branch>]`

Opens a PR from `--head`, or the checked-out branch, into `--base`, or the repository's default branch (a line such as `v2.15` means `release/v2.15`). The head is `owner:branch` for a branch on the fork and the bare branch for the upstream. It does not push: a branch that is not on the remote is an error that says to run `gero repo push`. A head equal to the base is refused.

The title and body default to the subject and the rest of the message of the branch's tip commit; `--title`, `--body` and `--body-file` replace them. `--label` (repeatable) names the labels to apply. A label the repository does not have, and a failure to read or apply labels, is a warning and never fails the PR. It does not close PRs it replaces; that is `ship`'s.

```
$ gero pr create --base v2.15 --label status/auto-created
Open PR from danpock:bot/scc-operator-v0.5.4-v2.15 into release/v2.15 on rancher/rancher?  [y/N] y
Opened rancher/rancher#104  https://github.com/rancher/rancher/pull/104
  warning: label status/auto-created is not in rancher/rancher, so it was not applied

$ gero pr create scc-operator --head my-fix --push-to upstream --dry-run
Would open PR from my-fix into main on rancher/scc-operator   (nothing was sent)
```

## `gero ship bump` **(leaning)**

`gero ship` keeps its meaning: a bare call with a project and a release or line runs the whole process, and its flags are its own. `bump` is a subcommand of it for the one step that applies the consumer's update method for a release to a branch and commits it (the edit and its commits are never separated), so `ship` is the command that does everything and `ship bump` the part of it that changes files. Cobra reads `bump` as the subcommand when it is the first argument, so no project may be named `bump`; none is.

What it needs is not decided: which branch it commits onto (the checked-out one, which must not be the target it is for), how the consumer and the update method are chosen (`--to` and `--method` as in `ship`), and whether it may be run in a temporary worktree.

## Where the code goes

Each command is a `cmd/` file and a package `internal/cli/<name>/` that builds the dependencies, calls the task and renders its output, as for every command ([PROJECT_LAYOUT](PROJECT_LAYOUT.md#2-layout)); none imports another command's package. The name is the task's: `repobranch`, `repopush`, `prcreate`, `shipbump`.

## Not built yet

Everything.

## Open questions

* What `ship bump` commits onto, and how it picks the consumer and method (above).
* Whether `repo branch` gets `--from <ref>`.
* Whether a later token such as `@` should stand for the checked-out branch where a branch is a positional, which none is now.
* Whether `repo push` and `pr create` share one `--push-to` resolution call or each repeats it. A shared one belongs in `internal/ship` with `resolvePushTarget` until a task needs it, then in a domain package.
