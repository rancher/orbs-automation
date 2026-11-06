# Ship (push to repo)

**Status:** draft, built for the workstation. `gero ship` checks the state, asks once, makes the commits (one per unit of work), pushes to the fork or the upstream, opens the PR with the labels that exist and closes the actor's own older PRs. It is tested against real git and a fake GitHub, and **not yet against real GitHub**: the push spike (`TestPushSpikeToARealRemote`) and a trial on a scratch repository are still to be run. The CI variant is not built.

`gero ship` takes a released version of a producer, brings the consumer repositories that take updates from it up to that version, and opens a PR for each branch that needs one. It is the write half of a downstream bump ([IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps)). The first user is `scc-operator` into `rancher/rancher`, which today is the bash in [scc-operator](../projects/scc-operator.md).

**`gero ship` is the workstation command.** It works in the engineer's own clone of the consumer and acts as the engineer. CI gets its own command, `gero ci:ship` (see [The CI variant](#the-ci-variant)), so that nothing CI-specific is in `ship`'s loop. The two share their backends: the state check, the dispatcher, the step runner, the templates and the PR identity and marker logic.

What a consumer edits, and how, is the consumer's update methods and the producer's downstream edges ([APP_CONFIG](APP_CONFIG.md#update-methods-and-downstream-edges)); the code that applies them is the update dispatcher (`internal/updatedispatcher`). This spec is the command around them.

## The command

`gero ship [project] [tag|line]` makes one PR for each consumer branch the release goes to. It honors `--dry-run`.

* `[project]` is the producer, found as for every command that takes a project ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags)): a config name or owner/name slug, or `.` or nothing for the project the working directory is in. A lone `v0.5.3` or `v2.15` is the release, not the project.
* The argument is a tag, used as given, or a release line, resolved to the producer's latest tag in that line. With none, the latest stable tag, except for a producer whose releases follow release lines (below). A tag is found by version, not by date.
* **A release for each line.** A producer that lists `release.lines` and whose edge follows them (the default) keeps a version series for each Rancher line, such as shell's `0.9.x` for `v2.16` and `0.8.x` for `v2.15`. Given no argument, `ship` gives each line the latest tag of that line's own series, for every line both the producer and the consumer have, in the producer's order, so the releases differ from target to target. A line the producer has no release for is left out and the report says so, unless it was named with `--line`, which is then an error. A tag, or a line given as the argument, goes only to the line it is in: `--line` for another line is refused (`v0.8.3 is in v2.15, not v2.16`), since putting a `0.9` release on the `0.8` line is the mistake the series exist to prevent. A producer with no lines, or an edge that says `lines: universal`, has one release for every target.
* `--prerelease` lets prereleases count: when resolving a line, and for a prerelease tag that the consumer's method would refuse (`accept.when` is `stable`).
* The routes out of the producer are its `downstream` edges. `--to <consumer>` picks one consumer when there are several, and `--method` one method when it uses several of the same consumer's.
* **Which branches.** Without `--line`, every branch of the consumer that the release goes to is checked, in config order, newest first. The edge's `lines` decides which, and an omitted one is worked out from the producer: a **universal** edge (the default for a producer that lists no `release.lines`, since its releases belong to no line) goes to every release branch of the consumer, and one that **follows lines** (the default for a producer with lines) goes to the consumer's branch for the line the release is in. A release in no line of the producer, or a line the consumer lacks, has no branch to go to, and the error says to name a line of the consumer. `--line <line>` limits the run to that one Rancher line of the consumer (`v2.15`, or `2.15`), on the branch the consumer keeps it on: its own, or the default branch for the line in development. The branch is never named on the command line, so the flag means the same for consumers whose branch names differ. A line the consumer does not have is an error that lists the ones it does, and a consumer with no release lines cannot be given one.
* `--allow-downgrade` ships a release older than what a branch holds. `--preview` prints the commit message and PR body as well as the branch, title and labels, and `--json` always carries them. `--yes` skips the one confirmation. `--no-push` does everything up to and including the commit, leaves the branch in your clone for review, and stops before the push and the PR; it is what the script's `--dry-run` was, and it is not `--dry-run`, which writes nothing.

## The state check

It runs first, reads from the host and not from any clone, and writes nothing. The parts that are the same for every branch (who ships to whom, the method, the release, the images) are resolved once, and each branch is then checked on its own. A branch that cannot be read fails alone and the others go on, unless it is the one named with `--line`.

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
2. **Confirm, once** for the whole run, before anything is written and only when something will be pushed or closed: a plain yes/no listing the PRs to open, close and refresh (`--yes` skips it). A dry run and `--no-push` ask nothing, since nothing leaves the machine.
3. **Prepare the branch** in the engineer's clone ([below](#preparing-the-branch)).
4. **Edit and commit.** The lifecycle above, in that branch's directory, with a commit after each unit that changed files, made through `git` so the engineer's hooks, identity and signing settings apply ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). No unit changing anything ends the branch as **already up to date**: no commit and no PR.
5. **Push** to the push target, never forced. Fetching is from `upstream`.
6. **PR.** Find open PRs with the same key. The same version is **already open** and the run stops. An older version is **superseded**: the new PR is created, then the old one gets a comment linking it and is closed.
7. **Report.** One line per branch: `updated`, `already up to date`, `already open`, `superseded`, `skipped (reason)` or `failed (reason)`.

`--dry-run` runs steps 1, 3 and 4 in a temporary worktree and prints what 5 and 6 would do. It never pushes a branch or opens a PR, and it never moves the engineer's checkout. The worktree and its branch are removed afterwards. The report says it was a dry run.

`--no-push` runs steps 1, 3 and 4 for real and stops, so the engineer can review the commit and push it later. The branch is kept, whichever place it was prepared in, and the report names it and the command to see the change (`git diff <upstream>/<target>...<branch>`). A dirty clone still gets a worktree, which is removed while the branch stays. `--no-push` with `--dry-run` is a dry run.

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

**A rerun** finds the branch name already in use, because the name is deterministic and a branch is kept after it is published. What to do depends on whose commits are on it ([Existing branches and PRs](#existing-branches-and-prs)).

go-git cannot create linked worktrees or switch a checkout the way `git` does, so this part goes through `internal/git/gitcli`, with the reason recorded next to the code.

## The CI variant

`gero ci:ship` is the same bump from a workflow, as the bot. Its shape is not decided on purpose. Either `gero` does the work itself (makes a shallow temporary clone with the workflow's token, resolves the bot identity, pushes to the upstream), or workflow steps do the setup first and hand `gero` the environment and config to work in. Both use the same backends, and only the loop and the setup differ. The decision waits until the local command works, and the differences are listed in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md).

## What a run reports

A target is named in the report by its **Rancher line** (`v2.15`), which means the same in every project, and not by the consumer's branch (`release/v2.15`, or `main` for the development line), which does not. The branch is shown beside it in the tables, in the closing list of PRs as its base, and wherever it is a command to copy; a branch that is in no line is named by its branch. The same line is on every event and in the JSON (`line`). A push is reported with the branch it pushed, as `v2.15: pushed bot/scc-operator-v0.5.3-v2.15 to origin`.

After the state check, each target that needs a PR gets a result, shown under the targets and carried in `--json`:

| Result | Meaning |
|---|---|
| `committed` | The commits are on the new branch in your clone (`--no-push`), listed one per line. The report names the branch, whether it was made in place or in a worktree and why, and the command to review it |
| `would commit` | A dry run made the commit in a temporary worktree, and removed it and the branch. The report shows the same detail |
| `already up to date` | The edit and the steps changed nothing, so there is no commit and no branch |
| `failed` | Preparing the branch, a step or the commit failed, with the reason and the step's phase and position. The work is aborted and the clone is as it was found |

A failure in one target does not stop the others, and the exit code is 1 when any target failed or was blocked. A branch that moved since the check is a failure that says to run again.

## Branches and PR identity

* **Name.** `<prefix><tag>-<target>`, for example `bot/scc-operator-v0.5.3-v2.15`, with `release/` stripped from the target and no timestamp. The prefix is a template: the user's `branchPrefix` replaces the consumer's `pullRequests.branchPrefix` whole, which replaces the default `bot/{source}-`. The tag and target are always appended, so the name is unique for the bump whatever the prefix.
* **Finding a PR does not use the name**, because the prefix differs by actor. Every PR body carries a hidden, versioned marker (additive fields) naming `source`, `consumer`, `target` and `version`. The **overlap key** is source, consumer and target: two PRs with the same key are one bump at two versions. The lookup lists the consumer's open PRs for the base branch and reads the markers. Labels are the configured ones and are not used for lookup.
* **Only your own work is touched.** A PR is *eligible* to be superseded when it has this bump's overlap key **and** was opened by the same login that is running (the engineer, or the bot in CI). A PR with the key from anyone else, a person or the CI bot, is not eligible: it is never closed, commented on or reused, and does not block a new PR. If it is the same version or newer, the report warns that it exists (`also open: #99 by rancher-bot`), since two PRs for one bump is worth knowing about.
* **One open at a time, per actor.** A new version is a new branch and a new PR, and every eligible older PR is commented and closed, not only the latest: if several of the actor's own PRs overlap they are all stale. The branch is never updated or force-pushed. A closed, unmerged PR for the same marker is not reopened and does not block a new one, and a merged one ends the branch as up to date.
* **The decision** for one target, from the open PRs found for its base branch:
  * An eligible PR at the same version: **already open**. No push, no PR. Eligible PRs at older versions are still closed, each with a comment naming the open one.
  * An eligible PR at a newer version: **blocked** for that target, naming it, since this release is the older one. Other targets go on.
  * Eligible PRs at older versions only: **superseded**. The PR is created first, then each older PR gets a comment linking it, then is closed. The comment is made only when the new PR was created, so a failure leaves the old PRs as they were.

## Existing branches and PRs

A branch or a PR for the bump may already exist, from an earlier run of this machine's or from a half-finished one. The rules are the same for the local branch and the remote one, and they turn on one question: are all of its commits **ours**?

* **Ours** means a commit has the `Automation: gero ship` trailer **and** an author email equal to the actor's (the commit identity the clone gives, or the bot's in CI). Every commit between the branch and its merge-base with the target must be ours. A branch with a commit that is not ours is never replaced, rewritten or force-pushed, and the run says whose commit it found.
* **A local branch that is ours** no longer fails a rerun. If it is based on the current head of the target it is reused, which also resumes a run whose push failed. If the target has moved on, it is rebuilt from the new head and replaces the old one. Only a local branch with commits that are not ours is an error, as before.
* **An open PR of the actor's at this version** is still "already open" and nothing is rebuilt by default. The report says whether it is behind its base, which GitHub tells with one read and no build: `already open #104 (behind release/v2.15; --refresh to rebuild)`.
* **`--refresh`** rebuilds it, and only when it is eligible: the PR is open, is the actor's, has its head in the repository the branch is pushed to, and every commit on the branch is ours. The branch is built again from the current head of the target in a scratch branch, running the edit and the steps as usual, and the result is compared with the existing branch. They are compared by the change each makes and not by their trees, since the trees differ wherever the target has moved: a fingerprint of each side's diff against its own base, with the hunk positions and the blob ids left out.
  * **Identical and not behind:** the branch is left alone and nothing is pushed. The report says `already open, identical`.
  * **Different, or behind:** the new branch is pushed over the old with `--force-with-lease` naming the exact commit that was seen, so a push made since is never overwritten. The PR is the same PR. A branch that is behind is replaced even when the change is the same, since "behind" means the base has moved and the PR is out of date; the comment says by how much. The description is brought up to date, and the refresh is recorded.
  * **Not eligible:** the run says which condition failed, and changes nothing.
* **The description follows the build.** Whatever the branch did, `--refresh` compares the PR's title and body with the ones a new PR would get, and when they differ it updates them. The body is merged and not overwritten: the visible text and the `gero:ship` marker come from the new build, and every other hidden `<!-- gero:<kind> v<N> ... -->` block already on the PR, such as the one the CI tracker will keep there, is kept byte for byte after them. A block of a kind the new body also writes is the new one's. Other HTML comments are not gero's and are not carried over. Merging the result again changes nothing, and bodies that differ only in line endings or in the whitespace at their ends are the same body, so a rerun writes nothing. A dry run says `would update the description` and writes nothing. Without `--refresh` the description is never touched.
* **The refresh is recorded in one comment**, which gero edits in place and finds by a hidden marker, and not a new comment each time. Each refresh that changed something adds a line to it: what the branch was rebuilt on, when, and by how much it was behind, or that the description was updated. A refresh that found nothing to change leaves it alone.
* `--refresh` takes no value. It applies to every eligible target of the run, and `--line` already says which targets there are.

## Where the branch is pushed

A push goes to one of two places, named by role and not by remote: the **fork** (the remote in `fork`, where the engineer has their own copy) or the **upstream** (the real repository). Which is a choice that varies with the environment, so it is layered, the nearest to the run winning:

1. the `--push-to fork|upstream` flag;
2. the user's per-repo setting, `repos.overrides.<slug>.pushTo`;
3. the consumer Project's `pullRequests.pushTo`, for a repository that is only ever pushed to one way;
4. the user's own default, `pushTo` in the user file;
5. else the fork when one is set, and the upstream when it is not.

So an engineer with a fork set pushes to it without saying so, and a repository that does not take PRs from forks can say so once in its Project. The role is resolved to a remote name from the settings (`fork`, and `upstream` with its per-repo override, [WORKSPACE](WORKSPACE.md)), and a role with no remote set is an error that says which setting to make. The PR's head follows the role: `<githubUser>:<branch>` for the fork and the bare branch for the upstream. In CI the role is always the upstream, with the workflow's token.

## The PR service

Creating, finding and closing PRs, and commenting and labelling them, is one service in `internal/github`, and `ship` is its first user and not its only one: the reconciler's tracking issues need the same calls (a comment, a label, closing) on issues, which GitHub treats as the same kind of item as a PR. So the service speaks in plain types of its own (go-github's stay inside), and each user declares the narrow interface it needs, as with every other consumer.

* **Package.** `internal/github/pulls`, inside the one place go-github may be imported, with its own plain types (`PR`, `NewPR`, `Open`).
* **Finding, without listing everything.** A repository can have hundreds of open PRs, so a lookup reads as few as it can. `OpenForHead` asks for one head with one request, which finds the PR of a branch whose name is known. `OpenForBase` lists the open PRs of a base branch newest first and stops at a limit (one page of 100 by default), saying `Truncated` when there were more; a bump's PR is recent, so the newest are the ones to read, and a run that was cut short says so in its report. `ship` reads the marker of each ([Branches and PR identity](#branches-and-pr-identity)); the service does not know what a marker is.
* **Creating.** A PR from a head to a base with a title and body. Labels are not part of creating it.
* **Labels are a separate step** ([METADATA](METADATA.md#applying-labels)): the repository's labels are read once per run, the configured labels that exist are applied after the PR exists, and one that is missing is a warning and not a failure. A label failure never fails the PR.
* **Commenting and closing** work on a PR by number, so the same calls close a superseded PR and, later, a tracking issue.
* **Outcomes.** A PR with the same marker and version is **already open** and the run stops. An older version is **superseded**: the new PR is created first, and then the old one gets a comment linking it and is closed, so a failure leaves the old PR open and not nothing. A newer version already open is a blocker that names it, since this release is the older one.

## Commits and the audit trail

* **Templates are compiled in.** The commit message, body and trailers are generic templates in `internal/ship/templates`, written with `{name}` placeholders from a closed set (`templates.TextVars`). The body's `{changes}` is generated from what the strategy and the steps did, so it never claims a step that did not run. An unknown placeholder is a config error. Project-specific and bundle-shipped templates ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)) are not designed until a project needs one.
* **The run record** is one line in the body: the engineer's login locally, or the workflow run URL in CI.
* **Labels, title and branch prefix** come from the consumer's `pullRequests` block. `labels` go on every PR, and `botLabels` only on a PR the bot opens, so a label that says a bot made it, such as `origin/bot`, is never on a PR an engineer opened from a workstation. A label the repository lacks is a warning, and the PR is still created. How labels are applied, and which are ever created, is in [METADATA](METADATA.md).
* **Identity and signing** are the actor rules in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md).

## Errors and exit codes

Outcomes are typed, with a closed status and the step they happened in, so wording is generic and carries config values and no project's own phrasing. The command exits 0 when every branch is `updated`, `already up to date`, `already open` or `superseded`, and 1 when any branch is blocked, failed, or the preconditions stopped the run. A usage or config error is 2.

## Testing

Table tests with a fake GitHub client, a local bare repository as the remote, and a step runner that records calls ([TESTING](TESTING.md)). The cases that matter: each outcome including partial failure; the same scenario as the workstation actor and as the bot; a local run then a CI run finding the first run's PR; a dry run changing nothing remote and leaving the engineer's clone byte for byte as found; and the engineer's local `main` and uncommitted files surviving a run.

## Not built yet

* A run against real GitHub: the push spike, and a first PR on a scratch repository. The same goes for `--refresh`, whose forced push has only been tried against local repositories.
* The preview shows the edit's commit message and not the commits of the steps, which are only known once the steps have run (`TODO(step-commits)` in the preview). A way to name a step's commit, or fold it into the previous one, is the first knob to add if a project needs it.
* The files a commit changed in the report.
* `gero ci:ship`.
* The sequence above as chained tasks. Making the branch is the `createbranch` task, the edit and its commits are `bumpcommit`, pushing is `pushbranch` and opening and labelling the PR is `createpr`; the other steps work today inside `internal/ship`, and moving each to `internal/task/` changes no behavior and none of this spec, apart from this list ([PROJECT_LAYOUT](PROJECT_LAYOUT.md#tasks-single-actions-that-compose)).
* `from: producer` and `from: central` for steps, which need the producer's checkout and the bundle.

## Open questions

* Whether the default PR title should carry something that identifies the tooling. Identity is the marker, the author and, in shared repositories, a label such as `origin/bot`; a title is a project's own convention, and matching on it is fragile.
* How steps such as `go generate` get a toolchain that matches the target branch. A workstation without it fails with a `gero doctor` hint.
* The shape of `gero ci:ship` (above).
* Whether a clean clone with untracked files should be worked in place when none collide, as designed here, or sent to a worktree whenever there are any.
* How a strategy that is not `yamlBump` reads its value for the downgrade check. Each defines it, and `chartsPackageBump` is not built.
