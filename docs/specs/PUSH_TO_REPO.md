# Ship (push to repo)

**Status:** draft. The state check, which writes nothing, is implemented as `gero ship`, and so is the update dispatcher it uses, and the step runner (`internal/steprun`). Preparing the branch and the commit are built, behind `--no-push` and `--dry-run`. The push and the PR are not.

`gero ship` takes a released version of a producer, brings the consumer repositories that take updates from it up to that version, and opens a PR for each branch that needs one. It is the write half of a downstream bump ([IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps)). The first user is `scc-operator` into `rancher/rancher`, which today is the bash in [scc-operator](../projects/scc-operator.md).

**`gero ship` is the workstation command.** It works in the engineer's own clone of the consumer and acts as the engineer. CI gets its own command, `gero ci:ship` (see [The CI variant](#the-ci-variant)), so that nothing CI-specific is in `ship`'s loop. The two share their backends: the state check, the dispatcher, the step runner, the templates and the PR identity and marker logic.

What a consumer edits, and how, is the consumer's update methods and the producer's downstream edges ([APP_CONFIG](APP_CONFIG.md#update-methods-and-downstream-edges)); the code that applies them is the update dispatcher (`internal/updatedispatcher`). This spec is the command around them.

## The command

`gero ship [project] [tag|line]` makes one PR for each consumer branch the release goes to. It honors `--dry-run`.

* `[project]` is the producer, found as for every command that takes a project ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags)): a config name or owner/name slug, or `.` or nothing for the project the working directory is in. A lone `v0.5.3` or `v2.15` is the release, not the project.
* The argument is a tag, used as given, or a release line, resolved to the producer's latest tag in that line. With none, the latest stable tag. A tag is found by version, not by date.
* `--prerelease` lets prereleases count: when resolving a line, and for a prerelease tag that the consumer's method would refuse (`accept.when` is `stable`).
* The routes out of the producer are its `downstream` edges. `--to <consumer>` picks one consumer when there are several, and `--method` one method when it uses several of the same consumer's.
* **Which branches.** Without `--branch`, every branch of the consumer that the release goes to is checked, in config order, newest first. The edge's `lines` decides which, and an omitted one is worked out from the producer: a **universal** edge (the default for a producer that lists no `release.lines`, since its releases belong to no line) goes to every release branch of the consumer, and one that **follows lines** (the default for a producer with lines) goes to the consumer's branch for the line the release is in. A release in no line of the producer, or a line the consumer lacks, has no branch to go to, and the error says to name one. `--branch <target>` limits the run to that one branch, which may be any branch the consumer has.
* `--allow-downgrade` ships a release older than what a branch holds. `--preview` prints the commit message and PR body as well as the branch, title and labels, and `--json` always carries them. `--yes` skips the one confirmation. `--no-push` does everything up to and including the commit, leaves the branch in your clone for review, and stops before the push and the PR; it is what the script's `--dry-run` was, and it is not `--dry-run`, which writes nothing.

## The state check

It runs first, reads from the host and not from any clone, and writes nothing. The parts that are the same for every branch (who ships to whom, the method, the release, the images) are resolved once, and each branch is then checked on its own. A branch that cannot be read fails alone and the others go on, unless it is the one named with `--branch`.

1. Resolve the release and the route, and bind the edge's params to the consumer's method.
2. For each branch, confirm it exists and read what it holds through the method's strategy (`Check`, the read side, at the branch's head commit).
3. Stop as **already up to date** when it already holds the value the release wants.
4. Stop as **blocked** on a **downgrade**: the release is older than the version the strategy finds in what the branch holds, by SemVer precedence, so a release candidate is below its release and a value with no version is not judged. `--allow-downgrade` lifts it. A blocked downgrade shows only the state and the reason, as an up to date branch does, and the same reason for several branches is one line naming them.
5. Preview what a run would write: the branch, title, labels, commit message and body.
6. Once, if any branch needs a PR, check the release's images exist ([IMAGES_EXIST](IMAGES_EXIST.md)).
7. Look for open PRs the new one would supersede.

The read half is shared with `gero repo status`, which traces what each consumer holds ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)): the branches a release goes to (`Route.BranchesFor`), a branch's head from the host's refs (`git.Refs.Branch`), and the value the strategy finds there (`Invocation.Check` over `updatedispatcher.HostTree`). Neither command reads a consumer's file by its own means.

`--dry-run` is this check, and for the write steps it runs the same path and stops short of the first write.

## Applying a bump

The update dispatcher owns the lifecycle of one branch. A **strategy** is Go code that claims a name (`yamlBump`) and is listed in an explicit registry. A method configures it once with its `with` values, which the strategy checks when the config loads, and that gives an **updater**. The edge's params, checked against the method's `allowedParams` and with their defaults filled in, are the **inputs** of each call. The updater has a read side (`Check`, never writes) and a write side (`Apply`, which returns what it changed).

The lifecycle around the edit is a chain with the consumer outermost: the method's `preEdit` steps, then the edge's, the edit, the edge's `postEdit` steps, then the method's. The consumer's `postEdit` regenerates derived files, so it sees everything a producer's step did. Only steps whose `since` and `until` range holds the branch's line run, and a branch in no line gets every step with no range. A step failure stops the run as an error naming the phase and position, and a failed post-edit step still reports what the edit changed.

**Commits follow the work.** The lifecycle is a series of units: each pre-edit step, the edit, each post-edit step. After each unit, if it changed files, exactly those changes are committed, and if it changed nothing there is no commit. So the SCC bump is two commits, the edit of `build.yaml` and then the files `generate` made, and a project with more steps between them gets a commit for each that changed something. This is the same as how an engineer would do it by hand, and it needs no per-project config: a strategy that is not `yamlBump`, or a step that is not `generate`, is just another unit. The dispatcher reports each unit as it finishes to a checkpoint callback, and `ship` supplies the callback that commits, so the dispatcher still knows nothing of git. The edit's commit has the message the preview shows, and a step's commit is "Run `<command>`", each with the trailers. A step that fails ends the run, and the branch is removed with every commit made so far.

The dispatcher decides which steps run and in what order, and builds each step's environment (`GERO_TAG`, `GERO_VERSION`, `GERO_TARGET_BRANCH`, `GERO_SOURCE_REPO`, and `GERO_PARAM_<KEY>` for each input). A **step runner**, which `ship` supplies, runs the command: the process, its scrubbed environment through `execx`, and where the program comes from (`from`: the consumer's clone, the producer's repo at the release for an owned project, or the central bundle). The dispatcher starts no process, so its lifecycle is tested with a runner that records calls.

## What the scripts do today

A faithful account of `push-to-rancher`, so the differences below are visible. For each target branch: fetch and check out the base, create `bot/scc-operator-<tag>-<branch>-<epoch>`, set `defaultSccOperatorImage` in `build.yaml`, run the first `//go:generate` line of `generate.go`, commit with `Automation:` and `Created-by:` trailers, push, and `gh pr create` with the label `status/auto-created`. A target whose `build.yaml` already matches is skipped. A failing target is skipped and listed at the end. Before anything, the image must exist in Docker Hub, Prime staging and, for stable tags, Prime production. CI runs it from a workflow that mints a token scoped to `rancher` and clones the target with persisted credentials; a local run uses `gh auth token`, the engineer's own clone and `--remote`.

| Today | Here | Why |
|---|---|---|
| One remote fetches the base and receives the push | Fetch from `upstream`, push to the push target | A fork as the only remote means a stale base, or pushing to the upstream by necessity |
| `gh pr create --head <branch>` | Head is `<githubUser>:<branch>` for a fork, the bare branch for the upstream | A head without an owner cannot resolve to a fork |
| Edits in the engineer's clone (`checkout -B` resets their branches, `checkout -f` discards uncommitted work, a dry run leaves branches behind) | A work branch in a clean clone, else a temporary worktree, always a worktree for a dry run | A tool must not destroy local work or leave debris |
| `bot/` prefix and an epoch suffix everywhere | The engineer's prefix locally (`bot/` in CI), deterministic suffix | Ownership is visible, and a rerun finds its own branch |
| `eval` of a line read from the target's `generate.go` | Steps declared in config, run as an argument list | Nothing executable is read from a downstream repo or run through a shell |
| `yq`, with the key and image written in the script | A strategy that edits the value in place, configured by the consumer | The script's file, key and wording become config |
| `docker manifest inspect` | An anonymous registry check inside `gero` | No `docker` needed on a runner or a laptop |
| A PR per run, never reused | A marker identifies the bump, and a newer version supersedes the older PR | A rerun does not pile up PRs |

## The sequence

Per target branch, newest first:

1. **Preconditions.** The state check above. A missing artifact stops the whole fan-out before any write, and a branch problem skips only that branch.
2. **Prepare the branch** in the engineer's clone ([below](#preparing-the-branch)).
3. **Edit and commit.** The lifecycle above, in that branch's directory, with a commit after each unit that changed files, made through `git` so the engineer's hooks, identity and signing settings apply ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). No unit changing anything ends the branch as **already up to date**: no commit and no PR.
5. **Push** to the push target, never forced. Fetching is from `upstream`.
6. **PR.** Find open PRs with the same key. The same version is **already open** and the run stops. An older version is **superseded**: the new PR is created, then the old one gets a comment linking it and is closed.
7. **Report.** One line per branch: `updated`, `already up to date`, `already open`, `superseded`, `skipped (reason)` or `failed (reason)`.

`--dry-run` runs steps 1 to 4 in a temporary worktree and prints what 5 and 6 would do. It never pushes a branch or opens a PR, and it never moves the engineer's checkout. The worktree and its branch are removed afterwards. The report says it was a dry run.

`--no-push` runs steps 1 to 4 for real and stops, so the engineer can review the commit and push it later. The branch is kept, whichever place it was prepared in, and the report names it and the command to see the change (`git diff <upstream>/<target>...<branch>`). A dirty clone still gets a worktree, which is removed while the branch stays. `--no-push` with `--dry-run` is a dry run.

## Preparing the branch

The consumer's clone is found as for every command ([WORKSPACE](WORKSPACE.md#resolving-repo-paths)). With none, the command fails and says to clone it or set `repos.paths.<slug>`; it never clones for itself. Each target branch is prepared the same way:

1. **Fetch** the one upstream branch the state check read: objects only, so no local branch and nothing in the working tree moves. The new branch starts at the commit the state check read. If the branch has moved since, the check is read again before anything is written.
2. **Pick where to work**, for each target from the state the clone is in (an untracked file can be in the way of one branch and not another), and report why when it is a worktree:
   * **In place**, when the clone is clean and this is not a dry run: `git switch -c <branch> <commit>` in the engineer's own checkout.
   * **A temporary worktree** otherwise: `git worktree add -b <branch> <dir> <commit>` under the temp directory. The engineer's checkout is not touched at all. A dry run always works here, and removes the branch with it.
3. **Clean** means: HEAD is a branch (not detached), no merge, rebase, cherry-pick or bisect is in progress, and no tracked file is staged, modified or deleted. Untracked files are allowed, unless one would be overwritten by the target branch's tree, which sends the run to a worktree instead.
4. **The commit holds only what the run changed.** Staging is every tracked file the edit and the steps changed, plus untracked files that appeared during the run, never the engineer's own untracked files.
5. **Afterwards, in place:** the branch the engineer started on is checked out again, and the new branch stays in the repository, since it is what gets pushed. A worktree is removed whatever happened.
6. **On a failure** only what the run changed is undone: its edits, the files it created, and the new branch when it holds no commit.

**A rerun** finds the branch name already in use, because the name is deterministic. A branch identical to the commit it would start from is reused. One with commits of its own is an error that names it, since those are the engineer's to keep or delete.

go-git cannot create linked worktrees or switch a checkout the way `git` does, so this part goes through `internal/git/gitcli`, with the reason recorded next to the code.

## The CI variant

`gero ci:ship` is the same bump from a workflow, as the bot. Its shape is not decided on purpose. Either `gero` does the work itself (makes a shallow temporary clone with the workflow's token, resolves the bot identity, pushes to the upstream), or workflow steps do the setup first and hand `gero` the environment and config to work in. Both use the same backends, and only the loop and the setup differ. The decision waits until the local command works, and the differences are listed in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md).

## What a run reports

After the state check, each target that needs a PR gets a result, shown under the targets and carried in `--json`:

| Result | Meaning |
|---|---|
| `committed` | The commits are on the new branch in your clone (`--no-push`), listed one per line. The report names the branch, whether it was made in place or in a worktree and why, and the command to review it |
| `would commit` | A dry run made the commit in a temporary worktree, and removed it and the branch. The report shows the same detail |
| `already up to date` | The edit and the steps changed nothing, so there is no commit and no branch |
| `failed` | Preparing the branch, a step or the commit failed, with the reason and the step's phase and position. The work is aborted and the clone is as it was found |

A failure in one target does not stop the others, and the exit code is 1 when any target failed or was blocked. A branch that moved since the check is a failure that says to run again.

Until the push is built, a run with neither flag stops after the state check and says so. The one confirmation of the write-command contract comes with the push, since until then nothing leaves the machine.

## Branches and PR identity

* **Name.** `<prefix><tag>-<target>`, for example `bot/scc-operator-v0.5.3-v2.15`, with `release/` stripped from the target and no timestamp. The prefix is a template: the user's `branchPrefix` replaces the consumer's `pullRequests.branchPrefix` whole, which replaces the default `bot/{source}-`. The tag and target are always appended, so the name is unique for the bump whatever the prefix.
* **Finding a PR does not use the name**, because the prefix differs by actor. Every PR body carries a hidden, versioned marker (additive fields) naming `source`, `consumer`, `target` and `version`. The **overlap key** is source, consumer and target: two PRs with the same key are one bump at two versions. The lookup lists the consumer's open PRs for the base branch and reads the markers. Labels are the configured ones and are not used for lookup.
* **A newer version supersedes.** A new version is a new branch and a new PR, and the old one is commented and closed. The branch is never updated or force-pushed. A closed, unmerged PR for the same marker is not reopened and does not block a new one, and a merged one ends the branch as up to date.

## Commits and the audit trail

* **Templates are compiled in.** The commit message, body and trailers are generic templates in `internal/ship/templates`, written with `{name}` placeholders from a closed set (`templates.TextVars`). The body's `{changes}` is generated from what the strategy and the steps did, so it never claims a step that did not run. An unknown placeholder is a config error. Project-specific and bundle-shipped templates ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)) are not designed until a project needs one.
* **The run record** is one line in the body: the engineer's login locally, or the workflow run URL in CI.
* **Labels, title and branch prefix** come from the consumer's `pullRequests` block. A label the repository lacks is a warning, and the PR is still created. How labels are applied, and which are ever created, is in [METADATA](METADATA.md).
* **Identity and signing** are the actor rules in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md).

## Errors and exit codes

Outcomes are typed, with a closed status and the step they happened in, so wording is generic and carries config values and no project's own phrasing. The command exits 0 when every branch is `updated`, `already up to date`, `already open` or `superseded`, and 1 when any branch is blocked, failed, or the preconditions stopped the run. A usage or config error is 2.

## Testing

Table tests with a fake GitHub client, a local bare repository as the remote, and a step runner that records calls ([TESTING](TESTING.md)). The cases that matter: each outcome including partial failure; the same scenario as the workstation actor and as the bot; a local run then a CI run finding the first run's PR; a dry run changing nothing remote and leaving the engineer's clone byte for byte as found; and the engineer's local `main` and uncommitted files surviving a run.

## Not built yet

* The push, the PR and superseding an older one, with `--yes` and the confirmation, and a plain run doing all of it.
* The preview shows the edit's commit message and not the commits of the steps, which are only known once the steps have run (`TODO(step-commits)` in the preview). A way to name a step's commit, or fold it into the previous one, is the first knob to add if a project needs it.
* The `signCommits` key (the commit's `Sign` option exists), and the files a commit changed in the report.
* `gero ci:ship`.
* The check for open PRs the new one would supersede.
* `from: producer` and `from: central` for steps, which need the producer's checkout and the bundle.

## Open questions

* Whether engineers push to a fork or straight to the upstream today. The design supports both through `fork` and `upstream`; the default for a target is a config decision.
* How steps such as `go generate` get a toolchain that matches the target branch. A workstation without it fails with a `gero doctor` hint.
* The shape of `gero ci:ship` (above).
* Whether a clean clone with untracked files should be worked in place when none collide, as designed here, or sent to a worktree whenever there are any.
* How a strategy that is not `yamlBump` reads its value for the downgrade check. Each defines it, and `chartsPackageBump` is not built.
