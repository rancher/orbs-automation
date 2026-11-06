# CI_SHIP

**Status:** draft. The command, what it shares with `gero ship` and what differs are decided in outline; the shape of its setup is open on purpose ([Open questions](#open-questions)). Nothing here is built.

`gero ci:ship` is the same bump as [`gero ship`](PUSH_TO_REPO.md), run by a workflow as the bot. It replaces the bash that the producer's workflow runs today (`push-to-rancher.yml` calling `run-gha.sh`: check the image, set the bot identity in a fresh checkout of the consumer, open a PR for each branch). The context rules it follows are in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md), and this spec only adds what is specific to shipping.

## One backend, two loops

`gero ci:ship` calls the same domain code as `gero ship` (`internal/ship`): resolving the release for each line, the state check, the preview, the one decision per target, publishing, refreshing and superseding, and the events that every report is made from. Nothing in that code asks whether it runs in CI. What differs is the thin loop around it in `cmd/` and `internal/cli/`, and the few inputs the context supplies:

* **The place to work.** A workflow has no engineer's clone. `ci:ship` is given a clean clone of the consumer, which `internal/ship` already works in without touching anything else: a new branch in place, removed after the run. The `Workspace` interface is the seam. `gero ship` answers `CloneDir` with the engineer's clone found by `gero cd`, and `ci:ship` answers it with a temporary one ([Open questions](#open-questions) says how it gets there).
* **The actor, the push and the labels** are the CI column of the table in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#what-differs-by-context): the bot's commit identity from `GERO_BOT_USER`, a push to the upstream with the workflow's token, the `bot/` branch prefix, and the project's `botLabels` besides its `labels`. The push target in CI is always the upstream ([PUSH_TO_REPO](PUSH_TO_REPO.md#where-the-branch-is-pushed)).
* **No prompt.** The workflow passes `--non-interactive` (or sets `GERO_NON_INTERACTIVE`), and without it the command fails before writing, naming the flag.
* **Output.** The log handler is JSON, `--json` and `--output env` work as for every command ([CLI_OUTPUT](CLI_OUTPUT.md)), and the report is also written as markdown to `$GITHUB_STEP_SUMMARY` **(leaning)**, which is what the bash's `summary` function does today.

## Rules that do not change in CI

* **One PR per bump, whoever opens it.** The hidden marker identifies a bump, so a run from CI supersedes the bot's own older PRs for it and never touches anyone else's, and it finds a PR an engineer opened as theirs. Ownership is the author login, which is the bot's.
* **`--refresh` replaces only what is ours.** In CI "ours" means every commit on the branch has the `Automation: gero ship` trailer and the bot's author email. A branch with a commit by a person is never replaced.
* **`gero/hold` is read before any work** on a PR, as [METADATA](METADATA.md#control-labels) says, and a held PR gets a note in the report and no new work.
* **A per-target result, partial failure tolerated,** and the same exit codes as `gero ship`.
* **Nothing is signed.** The setting is a user-file key and CI ignores the file ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#push-credentials-and-signing)).

## What CI adds

* **The run record.** The PR body and the refresh comment record the workflow run URL, where `gero ship` records the engineer's login ([PUSH_TO_REPO](PUSH_TO_REPO.md)).
* **Steps `from: producer` and `from: central`.** A workflow has the producer's own checkout and the central bundle at hand, so steps that run a script of the producer's, or one shipped in the bundle, are possible here. They are not for a workstation run, which has neither ([APP_CONFIG](APP_CONFIG.md), [CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)).
* **The bot's commit identity.** The fresh clone has no git configuration, so the commits are made as the bot explicitly: `internal/ship` passes the identity resolved from `GERO_BOT_USER` (`identity.ResolveIn`, which looks up the bot's numeric id) to every commit when it runs in CI, and a run in CI without `GERO_BOT_USER` fails before it writes ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#the-actor)).
* **The token check.** A write to another repository refuses the default `GITHUB_TOKEN` before any write ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#the-actor)).
* **Inputs from the trigger.** A workflow runs it after the producer's tag is pushed, or from a dispatch of the tracker or the reconciler ([ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md)). The arguments are those of `gero ship`: the producer, a tag or line, and `--line`.

## Not built yet

* The command, its `cmd/` registration (CI-only, [CLI_FRAMEWORK](CLI_FRAMEWORK.md)) and its loop.
* Resolving the bot's identity in a real workflow: the domain passes it to the commits (tested with a fake), and `GERO_BOT_USER` with the id lookup is exercised only by tests of `identity`.
* The temporary clone as a `Workspace`, and the token as a one-shot credential for the push (`gitcli.Push` takes the engineer's own today).
* The markdown step summary, and the workflow run URL in the PR body.
* `from: producer` and `from: central` for steps.
* A real workflow that calls it, beside the bash and then in place of it.

## Open questions

* **Who makes the clone.** Either `gero` does its own setup (a shallow temporary clone with the workflow's token, the bot identity resolved, a push to the upstream), or workflow steps check out the consumer and hand `gero` the directory. Both are one more `Workspace`, and only the setup differs. The first keeps the workflow to one step and tests the same code in both contexts. The second keeps checkout and authentication in the actions the workflow already uses, and needs no clone code. Decide when the first workflow is written.
* Whether `ci:ship` fans out across lines in one run, as `gero ship` does, or a workflow runs it once for each line it is given.
* How two runs of the same bump at once are made safe. The branch name is deterministic and a push replaces only a branch it saw, so the second push is rejected, but what that run reports and whether it retries is not decided.
* Whether the step summary and the PR comments are the same text, or the summary is longer.
