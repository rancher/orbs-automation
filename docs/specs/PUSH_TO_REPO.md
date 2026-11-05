# Ship (push to repo)

**Status:** draft. The state check, which writes nothing, is implemented as `gero ship`; the rest is not built. It is written ahead of the bump engine ([PHASES](../PHASES.md)) because the design decides what the first Flow A run looks like, and it is revised as that settles.

The write half of a downstream bump ([IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps)): take a released version of a producer, edit a consumer repo on each target branch, push a branch and open or reuse a PR. The first user is `scc-operator` into `rancher/rancher`, which is today the bash in [scc-operator](../projects/scc-operator.md). The command runs in both contexts and acts as the engineer locally and as the bot in CI ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). Branch resolution, edit strategies and the config that drives them belong to the bump engine and [APP_CONFIG](APP_CONFIG.md), not here.

## The command

`gero ship <project> [tag|line]`, in both contexts, writes, honors `--dry-run`. It makes **one PR** for one consumer and one target branch; doing many is a loop over it later.

* `<project>` is the producer being shipped, found as for `gero repo info` (config name or owner/name slug).
* The argument is a tag, used as given, or a release line, resolved to the project's latest tag in that line. With no argument, the project's latest stable tag.
* `--prerelease` lets prereleases count: when resolving a line, and for a prerelease tag the consumer's `accept.when` would refuse. Without it a line resolves to its latest stable tag.
* The consumers are the projects whose `sources` name the producer (the reverse view the config already implies). `--to <consumer>` picks one when there are several, and `--branch <target>` names the target branch. Finding the target from the tag's line waits for the `accept.lines` decision ([APP_CONFIG](APP_CONFIG.md)), so `--branch` is required for now.
* Inputs of the script map as: the tag is the argument, the source repo comes from the project, the clone is made by `gero` (no `RANCHER_DIR`), the remote is the workspace's upstream, and `DRY_RUN` is `--dry-run`.
* `--preview` prints the commit message and PR body as well as the branch, title and labels, and `--json` always carries them. `--yes` (`--non-interactive`) skips the one confirmation.

**The state check comes first and writes nothing.** Before any clone or PR: resolve the tag, find the consumer and confirm the target branch exists upstream, read the consumer's current value through the strategy's reader and stop as `already up to date` when it already equals the target, check the images exist ([IMAGES_EXIST](IMAGES_EXIST.md)), and look for open PRs the new one would supersede. `--dry-run` is this check plus a rendering of the branch, title, commit message and body.

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
3. **Edit.** The named strategy (a reader and a writer over the same params. YAML keys are edited in place by `internal/yamledit`; a patch layer for other file types, and a strategy that runs a config-defined script with inputs, come when a second kind is needed), then the post-edit commands. A strategy that finds nothing to change ends the target as **already up to date**: not an error, no branch, no PR.
4. **Commit.** Through go-git, or through `git` when the engineer has signing on ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). Commit message, trailers and identity below.
5. **Push.** To the push target, over the credentials in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md). Fetching is from `upstream`.
6. **PR.** Find open PRs with the same key. The same version is **already open**, and the run stops. An older version is **superseded**: the new PR is created, then the old one gets a comment linking the new one and is closed.
7. **Report.** One line: `updated`, `already up to date`, `already open`, `superseded`, `skipped (reason)` or `failed (reason)`.

`--dry-run` runs steps 1 to 4 and prints what 5 and 6 would do ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). Nothing it created is left behind.

## Branches and PR identity

* **Name.** `<prefix><tag>-<target>`, for example `bot/scc-operator-v0.5.3-v2.15`. `<target>` is the branch with `release/` stripped, as today. No timestamp. The prefix is a template: the user's `branchPrefix` replaces the project's `pullRequests.branchPrefix` whole, which replaces the default `bot/{source}-`. The tag and target are always appended, so the name stays unique for the bump whatever prefix is chosen.
* **Prefix follows the actor.** The engineer's `branchPrefix` ([CLI_CONFIG](CLI_CONFIG.md)) locally, `bot/` in CI, so a branch's owner shows in its name.
* **Finding an existing PR does not use the name.** The prefix differs by actor, so a name match would miss the other actor's PR. Every PR body carries a hidden, versioned marker (additive-only fields, like the tracking issue's state block) naming `source`, `consumer`, `target` and `version`. The **overlap key** is source, consumer and target: two PRs with the same key are the same bump at two versions. The lookup lists the consumer's open PRs filtered by base branch, which keeps it small, and reads the markers. Labels are the configured ones and are not used for lookup; a label can become a search index if the list ever gets large.
* **A newer version supersedes.** The branch is never updated or force-pushed: a new version is a new branch and a new PR, and the old open PR gets a comment linking the new one and is closed. A local run then a CI run, or the reverse, finds the same marker and does not open a second PR.
* **Closed PRs.** A closed, unmerged PR for the same marker is not reopened and does not block a new one, since closing it was a decision. A merged one ends the target as already up to date.

## Commits and the audit trail

* **Templates are compiled in.** The commit message, PR body and trailers are generic templates embedded in `gero` (under `internal/ship/templates`), written with `{name}` placeholders from a closed set (`templates.TextVars`: project, source, sourceRepo, tag, version without its `v`, branch, targetBranch, consumer, sourceReleaseURL, file, from, to, marker), a strategy's own extra names where it has them, and `{changes}`, a list generated from the edits and post-edit commands that were actually made, so a body never claims a step that did not run. An unknown placeholder is a config error. Project-specific templates, and runtime templates shipped in the central bundle ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)), are not designed until a project needs one.
* **The run record** is one line in the PR body: the engineer's login locally, or the workflow run URL in CI (`GITHUB_SERVER_URL`, `GITHUB_REPOSITORY`, `GITHUB_RUN_ID`). Reading the PR is enough to see which path made it.
* **Labels, title and branch prefix** come from the consumer's `pullRequests` block (the title and prefix use the same placeholders), not from code. A label the repository lacks is a warning on the report, and the PR is still created.
* **Identity and signing** are the actor rules in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md).

## Errors and exit codes

Outcomes are typed (a closed status and reason with the step it happened in), not templated text, so wording is generic and carries config values and no project's own phrasing. The command exits 0 when the target is `updated`, `already up to date`, `already open` or `superseded`, and 1 when any target `failed` or the preconditions stopped the run. A skipped target (frozen or missing branch) is not a failure unless the consumer config marks the branch required **(leaning)**. A usage or config error is 2 ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)).

## Testing

Table tests with a fake GitHub client and a local bare repository as the "remote" ([TESTING](TESTING.md), [GITHUB_ACCESS](GITHUB_ACCESS.md)). The cases that matter:

* Each outcome in the report, including partial failure that leaves the other targets done.
* The same scenario as the workstation actor and as the bot: commit identity, push target, PR head and author in the fake.
* A local run then a CI run, and the reverse: the second finds the first's PR and does not open another.
* A dry run changes nothing in the remote and leaves the engineer's clone's refs, index and working tree byte-for-byte as found.
* The engineer's local `main` and uncommitted files survive a run.

## Not built yet

* The write steps: the throwaway clone, the post-edit commands, the commit, the push, the PR and superseding an older one. The state check, the strategies' reader and in-place writer, the embedded templates and the preview they render are built, as is the marker's format.
* The check for open PRs the new one would supersede, `--dry-run`, `--yes` and the confirmation.
* `--branch` is always required. Finding it from the tag's line waits for `accept.lines`.
* A shipped value that is older than what the consumer holds (a downgrade) is not detected: the state check compares the two as text.

## Open questions

* Whether engineers push to a fork or straight to the upstream today (the local script can only do the second). The design supports both through `fork` and `upstream`; which one is the default for a target is a config decision.
* How post-edit commands such as `go generate` get a toolchain that matches the target branch in both contexts. A workstation without it fails with a `gero doctor` hint.
* Whether the throwaway clone should be shallow in CI when a strategy needs history.
* The name of `--prerelease`, and whether an explicit prerelease tag needs it when `accept.when` is `stable`.
* How the target branch is found from the tag's line, which is the `accept.lines` question.
