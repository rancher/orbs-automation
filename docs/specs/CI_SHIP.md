# CI_SHIP

**Status:** agreed. What is not built is under [Not built yet](#not-built-yet). The backend already acts as the bot when `Settings.CI` is set.

`gero ci:ship` is the same bump as [`gero ship`](PUSH_TO_REPO.md), run by a workflow as the bot, for one release event, as the final step of the producer's release workflow. It replaces the bash that the producer's workflow runs today (`push-to-rancher.yml` calling `run-gha.sh`: check the image, set the bot identity in a fresh checkout of the consumer, open a PR for each branch). The context rules it follows are in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md), and this spec only adds what is specific to shipping.

## One backend, two loops

`gero ci:ship` calls the same domain code as `gero ship` (`internal/ship`): resolving the release for each line, the state check, the preview, the one decision per target, publishing, refreshing and superseding, and the events that every report is made from. Nothing in that code asks whether it runs in CI. What differs is the thin loop around it in `cmd/` and `internal/cli/`, and the few inputs the context supplies:

* **The place to work.** A workflow has no engineer's clone. `ci:ship` works in the checkouts the workflow provides, found as every command finds a repo, through `repos.search`; it makes no clone of its own. The workflow's checkout action clones the producer and the consumers into one directory, and a step before `ci:ship` seeds a user file that names it, for example `GERO_CONFIG=$RUNNER_TEMP/gero.yaml gero init --search <dir>`, with `GERO_CONFIG` set to the same path for the run. It is the same user file as on a workstation, the one of whoever the job runs as, and not a config of the bot: `GERO_CONFIG` is how CI is made to read it. `repos.search` is the only user-level key a run needs. A job makes its config before it calls `gero`, so `ci:ship` stays free of any flag that says where the repos are. `internal/ship` already works in a clean clone without touching anything else: a new branch in place, removed after the run. The `Workspace` interface is the seam: `gero ship` answers `CloneDir` with the engineer's clone found by `gero cd`, and `ci:ship` answers it with the checkout of the consumer.
* **The fan-out.** One run is for one release event, as the `push-to-*` scripts are, and it runs the bump loop of `gero ship` on every active consumer branch of that release, as `gero ship` does.
* **The config.** From orbs-automation a job uses the local `config/`. From any other repo it loads the bundle of the `gero` version it runs, not the newest release ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)). The default bundle is embedded in the binary and is also a release asset, so a job needs only the binary.
* **The actor, the push and the labels** are the CI column of the table in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#what-differs-by-context): the bot's commit identity from `GERO_BOT_USER`, a push to the upstream with the workflow's token, the `bot/` branch prefix, and the project's `botLabels` besides its `labels`. The push target in CI is always the upstream ([PUSH_TO_REPO](PUSH_TO_REPO.md#where-the-branch-is-pushed)).
* **No prompt, and no flag for it.** `ci:ship` never asks, so it has no `--yes`, `--non-interactive` or `--no-push`, and no `--push-to`: it always pushes, to the upstream. It takes the rest of the flags of `gero ship` (`--to`, `--line`, `--prerelease`, `--allow-downgrade`, `--preview`, `--refresh`) and its arguments, so a workflow gives it the tag its build produced and the PRs go to the consumers of that tag's line.
* **Output.** The log handler is JSON and `--json` works as it does for `gero ship` ([CLI_OUTPUT](CLI_OUTPUT.md)). The report has one model with two renderings: a fuller one for the job log and `$GITHUB_STEP_SUMMARY`, which is what the bash's `summary` function does today, and a shorter one for comments.

## Rules that do not change in CI

* **One PR per bump, whoever opens it.** The hidden marker identifies a bump, so a run from CI supersedes the bot's own older PRs for it and never touches anyone else's, and it finds a PR an engineer opened as theirs. Ownership is the author login, which is the bot's.
* **`--refresh` replaces only what is ours.** In CI "ours" means every commit on the branch has the `Automation: gero ship` trailer and the bot's author email. A branch with a commit by a person is never replaced.
* **`gero/hold` is read before any work** on a PR, as [METADATA](METADATA.md#control-labels) says, and a held PR gets a note in the report and no new work.
* **A per-target result, partial failure tolerated,** and the same exit codes as `gero ship`.
* **A rejected push fails the run.** The branch name is deterministic and a push replaces only a branch it saw, so when the branch moved since, the push is rejected, the run reports it and exits non-zero, and nothing is retried.
* **Nothing is signed.** The setting is a user-file key and CI ignores the file ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#push-credentials-and-signing)).

## What CI adds

* **The run record.** The PR body and the refresh comment record the workflow run URL, where `gero ship` records the engineer's login ([PUSH_TO_REPO](PUSH_TO_REPO.md)).
* **Steps `from: producer` and `from: central`.** A workflow has the producer's own checkout and the central bundle at hand, so steps that run a script of the producer's, or one shipped in the bundle, are possible here. They are not for a workstation run, which has neither ([APP_CONFIG](APP_CONFIG.md), [CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)). `steprun` refuses both today.
* **The bot's commit identity.** The fresh clone has no git configuration, so the commits are made as the bot explicitly: `internal/ship` passes the identity resolved from `GERO_BOT_USER` (`identity.ResolveIn`, which looks up the bot's numeric id) to every commit when it runs in CI, and a run in CI without `GERO_BOT_USER` fails before it writes ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#the-actor)).
* **Tags are pushed with a token that starts workflows.** A push made with the workflow's default `GITHUB_TOKEN` does not start other workflows, so the token passed to `gero` in CI is a person's or an App's, and a tag pushed by `gero repo tag --push` (or its CI form) starts the release workflows that run on tags, as a branch push starts the ones that run on pushes ([TASK_COMMANDS](TASK_COMMANDS.md#gero-repo-tag-project-line-tag)).
* **The token check.** `cigate.RequireCrossRepoToken` refuses the default `GITHUB_TOKEN` for a write to another repository, before any write ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#the-actor)).
* **Inputs from the trigger.** A workflow runs it after the producer's tag is pushed ([ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md)). The arguments are those of `gero ship`: the producer, a tag or line, and `--line`.

## Calling it from a workflow

The checkout action clones the producer and each consumer as siblings under `$GITHUB_WORKSPACE`, so one `repos.search` entry finds them all. The scan lists the directories inside each search directory, to a depth limit, and matches a clone to a project by its remote URL, never by its directory name. The working directory is not a clone, so the project is always named.

```yaml
- uses: actions/checkout@<sha>
  with:
    path: ${{ github.workspace }}/scc-operator
- uses: actions/checkout@<sha>
  with:
    repository: rancher/rancher
    path: ${{ github.workspace }}/rancher-repo
- name: Push updates to rancher/rancher
  env:
    GH_TOKEN: ${{ steps.generate_token.outputs.token }}
    GERO_BOT_USER: "${{ steps.generate_token.outputs.app-slug }}[bot]"
    GERO_CONFIG: ${{ runner.temp }}/gero.yaml
    GERO_UPSTREAM: origin
  run: |
    gero init --search "$GITHUB_WORKSPACE"
    gero ci:ship scc-operator "$TAG"
```

What the push scripts took as environment becomes this:

| Script input | `gero ci:ship` |
|---|---|
| The tag (`TAG`) | The second argument |
| The source repository (`SOURCE_REPO`) | The project, the first argument |
| The token (`GH_TOKEN`) | `GH_TOKEN`, the same. It must differ from `GITHUB_TOKEN` |
| The bot's login (`APP_USER`) | `GERO_BOT_USER`, the same value, `<app-slug>[bot]` |
| The producer's and the consumer's directories (`SCC_DIR`, `RANCHER_DIR`) | None: `repos.search` in the user file `GERO_CONFIG` names |
| A list of branches to process (`RANCHER_BRANCHES_OVERRIDE`) | None: `--line` names a Rancher line, not a branch |

`GERO_CONFIG` and `GERO_UPSTREAM` are what `gero` adds. CI reads a user file only when `GERO_CONFIG` names it, so `gero init` and `ci:ship` are given the same path. `GERO_UPSTREAM` is the name of the remote that is the real repository, which has no default, and in a checkout it is `origin`. `GITHUB_ACTIONS=true`, which the runner sets, is what makes the context CI. The variables `GERO_TAG`, `GERO_VERSION`, `GERO_TARGET_BRANCH` and `GERO_SOURCE_REPO` are not inputs: the bump engine builds them for the steps it runs.

## Not built yet

* The step summary, and the workflow run URL in the PR body.
* `from: producer` and `from: central` for steps.
* The preflight line that names workstation signs under a forced CI context ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md)). It only warns.
* The workflow in `shell`, and a pinned, signed `gero` installed in the workflows of both repos, with the bash scripts removed.

## Open questions

None yet.
