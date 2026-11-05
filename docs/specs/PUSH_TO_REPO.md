# Push to repo

**Status:** draft. Nothing here is built. It is written ahead of the bump engine ([PHASES](../PHASES.md)) because the design decides what the first Flow A run looks like, and it is revised as that settles.

The write half of a downstream bump ([IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps)): take a released version of a producer, edit a consumer repo on each target branch, push a branch and open or reuse a PR. The first user is `scc-operator` into `rancher/rancher`, which is today the bash in [scc-operator](../projects/scc-operator.md). The command runs in both contexts and acts as the engineer locally and as the bot in CI ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). Branch resolution, edit strategies and the config that drives them belong to the bump engine and [APP_CONFIG](APP_CONFIG.md), not here.

## What the scripts do today

A faithful account of `push-to-rancher`, so the differences below are visible. For each target branch: fetch and check out the base, create `bot/scc-operator-<tag>-<branch>-<epoch>`, set `defaultSccOperatorImage` in `build.yaml`, run the first `//go:generate` line of `generate.go`, commit with `Automation:` and `Created-by:` trailers, push, and `gh pr create` with the label `status/auto-created`. A target whose `build.yaml` already matches is skipped. A failing target is skipped and listed at the end. Before anything, the image must exist in Docker Hub, Prime staging and, for stable tags, Prime production.

CI runs it from a workflow that reads the App credentials from Vault, mints a token scoped to `rancher`, clones the target with persisted credentials and passes `GH_TOKEN` and `APP_USER`. A local run (`run-local.sh`) uses `gh auth token`, the engineer's own clone and git config, and `--remote` to name one remote.

## What changes

| Today | Here | Why |
|---|---|---|
| One remote fetches the base and receives the push | Fetch from `upstream`, push to the push target | A fork as the only remote means a stale base, or pushing to the upstream by necessity |
| `gh pr create --head <branch>` | Head is `<githubUser>:<branch>` for a fork, the bare branch for the upstream | A head without an owner cannot resolve to a fork |
| Edits in the engineer's clone (`checkout -B` resets their local branches, `checkout -f` discards uncommitted work, a dry run leaves branches behind) | A throwaway clone | A tool must not destroy local work |
| The workflow pre-clones the target and persists credentials | `gero` clones with the token | One credential path in both contexts |
| `bot/` prefix and an epoch suffix everywhere | Prefix follows the actor, deterministic suffix | Ownership is visible, and a rerun finds its own branch |
| `eval` of a line read from the target's `generate.go` | A post-edit command declared in config | Nothing executable is read from a downstream repo |
| `docker manifest inspect` | An anonymous registry check inside `gero` | No `docker` needed on a runner or a laptop |
| A fork guard on every step (`github.repository == ...`) | One guard, checked once | Scattered guards drift |

## The sequence

Per target branch, in the order the bump engine resolves them, newest first:

1. **Preconditions.** The artifact exists in every required registry (the set depends on whether the version is stable, [IMAGES_EXIST](IMAGES_EXIST.md)), and the target branch exists and is not frozen. A failure stops the whole fan-out before any write when it concerns the artifact, and skips only that target when it concerns the branch.
2. **Clone.** One throwaway clone of the target repo per run, shared across branches. Objects come from a local reference to the engineer's clone where one exists, else a shallow clone. Each branch gets its own worktree.
3. **Edit.** The named strategy, then the post-edit commands. A strategy that finds nothing to change ends the target as **already up to date**: not an error, no branch, no PR.
4. **Commit.** Through go-git, or through `git` when the engineer has signing on ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). Commit message, trailers and identity below.
5. **Push.** To the push target, over the credentials in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md). Fetching is from `upstream`.
6. **PR.** Find an open PR for the target, reuse it or open one. A reused PR is reported as **already open** and its branch is updated only when the new commit differs.
7. **Report.** One line per target: `updated`, `already up to date`, `already open`, `skipped (reason)` or `failed (reason)`.

`--dry-run` runs steps 1 to 4 and prints what 5 and 6 would do ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). Nothing it created is left behind.

## Branches and PR identity

* **Name.** `<prefix><producer>-<version>-<target>`, for example `dan/scc-operator-v0.5.3-v2.15` or `bot/scc-operator-v0.5.3-v2.15`. `<target>` is the branch with `release/` stripped, as today. No timestamp.
* **Prefix follows the actor.** The engineer's `branchPrefix` ([CLI_CONFIG](CLI_CONFIG.md)) locally, `bot/` in CI, so a branch's owner shows in its name.
* **Finding an existing PR does not use the name.** The prefix differs by actor, so a name match would miss the other actor's PR. The lookup is the consumer's stable PR key ([IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps)) plus a hidden marker in the PR body naming producer, version and target branch **(leaning)**. A local run then a CI run, or the reverse, finds the open PR and reuses it.
* **Closed PRs.** A closed, unmerged PR for the same marker is not reopened and does not block a new one, since closing it was a decision. A merged one ends the target as already up to date.

## Commits and the audit trail

* **Message and trailers** are as today (`Update SCC Operator to <tag>`, `Automation: <command>`, `Created-by: <integration>`) in both contexts.
* **The run record** is one line in the PR body: the engineer's login locally, or the workflow run URL in CI (`GITHUB_SERVER_URL`, `GITHUB_REPOSITORY`, `GITHUB_RUN_ID`). Reading the PR is enough to see which path made it.
* **Labels and title** come from the consumer's `pullRequests` block, not from code.
* **Identity and signing** are the actor rules in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md).

## Errors and exit codes

Per-target outcomes follow the report above. The command exits 0 when every target is `updated`, `already up to date` or `already open`, and 1 when any target `failed` or the preconditions stopped the run. A skipped target (frozen or missing branch) is not a failure unless the consumer config marks the branch required **(leaning)**. A usage or config error is 2 ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)).

## Testing

Table tests with a fake GitHub client and a local bare repository as the "remote" ([TESTING](TESTING.md), [GITHUB_ACCESS](GITHUB_ACCESS.md)). The cases that matter:

* Each outcome in the report, including partial failure that leaves the other targets done.
* The same scenario as the workstation actor and as the bot: commit identity, push target, PR head and author in the fake.
* A local run then a CI run, and the reverse: the second finds the first's PR and does not open another.
* A dry run changes nothing in the remote and leaves the engineer's clone's refs, index and working tree byte-for-byte as found.
* The engineer's local `main` and uncommitted files survive a run.

## Not built yet

* Everything. The command name is not chosen, and it depends on the bump engine's command shape.

## Open questions

* Whether engineers push to a fork or straight to the upstream today (the local script can only do the second). The design supports both through `fork` and `upstream`; which one is the default for a target is a config decision.
* How post-edit commands such as `go generate` get a toolchain that matches the target branch in both contexts. A workstation without it fails with a `gero doctor` hint.
* Whether the throwaway clone should be shallow in CI when a strategy needs history.
* Whether a reused PR's branch is force-updated or left alone when the new commit differs.
* How a branch name is built from the consumer's `pullRequests.branchPrefix` template and the user's `branchPrefix` setting, which replaces it whole ([WORKSPACE](WORKSPACE.md#raw-and-effective-values)). Whether the template covers the whole name or only a prefix before a fixed `<version>-<target>` tail, which placeholders exist, and how the reuse lookup stays independent of the name.
* The command's name and where it sits in the tree ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)).
