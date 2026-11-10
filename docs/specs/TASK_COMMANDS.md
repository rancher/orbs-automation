# Task commands

**Status:** implemented for `gero repo branch` and `gero repo tag`. The other steps of `ship` are listed as use cases, not as commands, until one is built. The tasks they would all run are built ([PROJECT_LAYOUT](PROJECT_LAYOUT.md#tasks-single-actions-that-compose)).

The steps of [`gero ship`](PUSH_TO_REPO.md) as commands of their own, for an engineer who wants one part of the process and not the whole. Each runs one task and `ship` chains the same tasks, so a step behaves the same alone and inside `ship`. They are workstation commands: they act as the engineer, in the engineer's own clone, and read the same settings as `ship`.

| Command | Task | Does |
|---|---|---|
| `gero repo branch [project] <line> <name>` | `createbranch` | Makes a branch off the upstream branch of a release line |
| `gero repo tag [project] <line> [tag]` | `createtag` | Tags the head of a release line's upstream branch in the clone, and with `--push` sends the tag |

The other steps of `ship` are [use cases](#use-cases-not-yet-commands), not named commands.

## Rules they share

* **Project arguments** are as everywhere ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags)): the config name or `owner/name`, `.` for the project the working directory is in, and the working directory's project when none is named. In `gero repl` that is the project last changed into.
* **The branch is never a silent guess.** Where a command acts on a branch of the engineer's, it takes the checked-out branch unless one is named, and says which in the question, the dry run and the report. A detached HEAD with none named is an error that says to name one.
* **A command that sends something off the machine asks once, and honors `--dry-run`, `--yes` and `--json`**, as `ship` does ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)). A dry run says what would be done and sends nothing. `--json` prints the task's result. A command that changes only the local clone, `repo branch`, asks nothing, as `ship --no-push` asks nothing, and still honors `--dry-run` and `--json`.
* **Where a branch is pushed** is `--push-to fork|upstream`, decided as in `ship`: the flag, then the settings, then the fork when one is set. It sets the `owner:branch` head of a PR and the remote of a push.
* **Never forced.** The one replacement is of an earlier attempt's branch whose every commit is ours, and only with `--refresh`, as in `ship`.

## `gero repo branch [project] <line> <name>`

Makes the branch `<name>` at the head of the upstream branch of the release line `<line>` (`v2.15` is `release/v2.15`, found as `gero repo info` finds it), after fetching it. With two arguments they are the line and the name, with three the project comes first.

By default it **leaves your checkout where it is**, as `git branch <name> <upstream>/<branch>` does, and ends by saying `git switch <name>`. `--switch` moves the checkout onto the new branch afterwards, with git's own rules for uncommitted changes, so it fails as `git switch` would. A clone with changes in it is never disturbed otherwise: the branch is made in a temporary worktree and the worktree is removed. A branch of the name that holds commits of ours on the current head is kept as it is and the report says so. One with commits that are not ours is an error that says whose, and one of ours that the line has moved on from is made again, as `createbranch` does for `ship`. With `--dry-run` the report says `Would create` and no branch is left behind. `--json` prints the project, the branch, where it was cut from and the commit.

```
$ gero repo branch v2.15 my-fix
Created my-fix at upstream/release/v2.15 (1b3c9e07) in rancher/rancher
  git switch my-fix

$ gero repo branch scc-operator main bump-deps --switch
Created bump-deps at upstream/main (a41f02cc) in rancher/scc-operator, and switched to it

$ gero repo branch v2.15 my-fix --dry-run
Would create my-fix at upstream/release/v2.15 (1b3c9e07) in rancher/rancher
```

## `gero repo tag [project] <line> [tag]`

Makes the tag `<tag>` **in the engineer's clone**, on the head of the upstream branch of the release line `<line>`, found as `repo branch` finds it: a line (`v2.15` or `2.15`), or a branch of the project (`release/v2.15`, or `main` for a project with no lines), which has the series of the line it is the branch of. The head is read from the host when the command runs and fetched, so the tag is on what is live there and not on what the clone last saw, and the checkout is left where it is. A tag is a full version and a project or a line never is, which tells two arguments apart: with a version second they are the line and the tag, else the project and the line.

**The tag is a local, lightweight one, and sending it is a push.** The consequence of `gero repo tag` is a tag in the clone, which the engineer pushes (`git push upstream v0.8.4-rc.1`, which the command prints), so GitHub sees the engineer push it and a workflow that runs on `push: tags` starts as them. `--push` does that push as well, with the engineer's own git credentials, to the upstream: a release tag is never pushed to a fork. The tags of these repositories are lightweight, bar a few made by hand with `git tag -a`, so there is no tagger, message or signature to keep. In CI the same push is made with a token that is not the default `GITHUB_TOKEN`, which is how it starts workflows ([CI_SHIP](CI_SHIP.md)).

**A given tag is checked, not trusted.** It matches the project's `tagPattern`, is a version in the line's series (`0.8.x` for shell's `v2.15`), is newer than the line's newest tag, and does not exist. The host's tags and the clone's own are both looked at. A tag the host has on the same commit is "already tagged" and the command succeeds, so a rerun is safe; a tag only the clone has, on that commit, is not made again and is pushed with `--push`; a tag on another commit, in either place, is an error. A tag is never moved, deleted or forced.

**An omitted tag is chosen** (`--stable`, `--minor` and `--major` only apply then, and a tag with one of them is an error) from the newest tag of the line's series, whichever kind it is, as `tagscheme.Newest` finds it for `repo status` too, so a tag the stream excludes is not the one followed, and a commit that already has that tag gets no new one. With no tag in the line yet, the first is the start of its series (`v0.8.0-rc.1` for `0.8.x`), and a project with no series has to name it. The version is written with the `v` the line's tags have, else the project's pattern says, by `Version.NextPrerelease` and `NextStable` ([`internal/platform/semver`](../../internal/platform/semver/next.go)), which know nothing of projects or lines. What a line may have next is the policy of [`internal/releasetag`](../../internal/releasetag/releasetag.go), which is pure and is asked by this command, by the `createtag` task (which only makes the tag) and by `repo status`, so what the status says is what the command does. It passes semver the version, the bump and the label:

| Newest tag | Default | `--stable` |
|---|---|---|
| stable `v0.8.3` | prerelease `v0.8.4-rc.1`, or `--minor` or `--major` for a higher version | the next stable `v0.8.4`, unless the project sets `requiresPrerelease`, which refuses it and says an RC comes first |
| prerelease `v0.8.4-rc.1` | `v0.8.4-rc.2` | `v0.8.4`, the promotion of that RC |

* **Prereleases are written with a dot, `rc.1`,** as SemVer has it and as the team's repositories already do. A legacy prerelease with no dot (`rc1`) is continued in its own style (`rc2`), since `rc.2` sorts below `rc1` and would be a tag older than the one before it. A new prerelease is always dotted.
* **A promotion, the "unRC", is never the default.** It needs `--stable`, and says what it is: `v0.8.4 on release/v2.15 at 1b3c9e07, which is v0.8.4-rc.2 with no commits since`. When the head is no longer on that RC's commit it refuses and names both commits, since the promotion is the same code made stable, and the way on is another RC. A project with `requiresPrerelease` also refuses a stable tag after a stable one, and a stable tag that is named is only made on a commit that has a prerelease of its version ([IMPLEMENTATION](../IMPLEMENTATION.md)).
* **Making the tag asks nothing**, as `repo branch` asks nothing: it changes only the clone and is undone with `git tag -d`. **`--push` asks once**, before anything is made, since a push is the step that leaves the machine, and `--yes` and `GERO_NON_INTERACTIVE` skip it. `--json` is for a program and never asks, so a push with it needs `--yes`. `--dry-run` says what would be done and changes nothing. What is asked is settled first, so a tag the line cannot have is an error and not a question.
* **No GitHub token is needed.** The head and the tags are read from the host without one, and a push is git's.

```
$ gero repo tag shell v2.15
Tagged v0.8.4-rc.1 on release/v2.15 at 8c61398 in rancher/shell
  to send it: git push upstream v0.8.4-rc.1

$ gero repo tag shell v2.15 --stable --push
Push the tag v0.8.4 on release/v2.15 at 8c61398, which is v0.8.4-rc.1 with no commits since, in rancher/shell to upstream? [y/N] y
Tagged and pushed v0.8.4 on release/v2.15 at 8c61398 ... to upstream
  https://github.com/rancher/shell/releases/tag/v0.8.4
```

**In CI** the same task runs as the bot, with a push made with a token that starts workflows. The tracking issue's sign-off, a check before a stable tag is made, is a gate above this command and not part of it, and is not decided ([METADATA](METADATA.md#control-labels)).

## Use cases not yet commands

Each is a goal an engineer may have of one step of `ship`, with the task that already does it. They have no command names on purpose: the name, and whether the goal is wanted as a command at all, wait until a second caller needs one. Each takes the project as every command does (the working directory's, or named), and the rules above apply.

* **Send a branch.** Push a branch of the engineer's to the push target, the checked-out one unless one is named, with the checks `ship` makes: nothing on the remote is the ordinary push, the same commit sends nothing, a branch that grew from the remote's is a fast forward, and one that has gone another way is an earlier attempt's, which is replaced only when every commit on it is ours and `--refresh` was given. The error for one that is not ours says whose commit it is. Task: `pushbranch`. It leaves the machine, so it asks once.
* **Open a PR for a pushed branch.** From a head branch (the checked-out one unless one is named) into a base (a line such as `v2.15` means `release/v2.15`, else the repository's default branch). The head is `owner:branch` for a branch on the fork and the bare branch for the upstream. It does not push: a branch that is not on the remote is an error that says to send it first. A head equal to the base is refused. The title and body default to the subject and the rest of the message of the branch's tip commit, and may be given. The labels to apply may be named; a label the repository does not have, or a failure to read or apply labels, is a warning and never fails the PR. It does not close the PRs it replaces; that is `ship`'s. Task: `createpr`.
* **Apply a release to a branch and commit it.** Write the consumer's holdings of a release into a branch and commit it, with the edit and its commits never separated, and no push and no PR. Task: `bumpcommit`. `gero ship --no-push` already does this for every target of a release and leaves each branch in the clone for review, so this is only for putting the change on one branch the engineer has already made. Not decided: which branch it commits onto (the checked-out one, which must not be the target it is for), how the consumer is chosen when the project ships to several, and whether it may run in a temporary worktree.
* **Close the PRs a newer bump supersedes.** `ship` does this after it opens a PR, and when one is already open. It is not a task yet.

## Where the code goes

Each command is a `cmd/` file and a package `internal/cli/<name>/` that builds the dependencies, calls the task and renders its output, as for every command ([PROJECT_LAYOUT](PROJECT_LAYOUT.md#2-layout)); none imports another command's package. The name is the command's: `repobranch` and `repotag` today.

## Not built yet

Each is built only if someone asks.

* The use cases above as commands, under names not chosen, when a second caller needs one.
* `gero repo branch --from <ref>`, for cutting the branch off something other than the head of the line, a branch or a tag.
* A branch shorthand with `@`, sharing `resolvePushTarget` between the push and PR use cases, and a config setting for a step's commit.

## Open questions

None yet.
