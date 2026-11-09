# TODO

What is open, in pieces you can pick up. Done work is not listed: git has it, and `gero --help` and the tests say what exists. The plan by milestone is [PHASES](docs/PHASES.md), the detail of each part is in its [spec](docs/specs/README.md), and [contributing](docs/contributing/README.md) says how to work on the code and docs.

Each item has a size: **S** is a few hours with the spec, **M** a day or two, **L** is design as well as code. Items marked *(no code)* need knowledge of the repos or the release process, not Go. To take one, open an issue naming it so we do not collide.

## Needed for v1.0.0

### Ship from CI

Replace the `push-to-rancher` scripts of `scc-operator` and `shell` with `gero`.

* **L: `gero ci:ship`.** The same bump as `gero ship`, run by a workflow as the bot: a temporary clone, the bot identity from `GERO_BOT_USER`, a push with a token that starts workflows, a step summary, the workflow run URL in the PR body. [CI_SHIP](docs/specs/CI_SHIP.md)
* **M: config for `ci:*` with no checkout.** Today CI points `GERO_CENTRAL_CONFIG_DIR` at a checkout. Decide whether a job may use the bundle of the installed release instead, and build it. [CENTRAL_BUNDLE](docs/specs/CENTRAL_BUNDLE.md)
* **M: a workflow that installs a pinned, signed `gero` and runs `ci:ship`,** beside the bash script, then in place of it, in `scc-operator` and `shell`. Needs `ci:ship`.
* **S: the `auth status` fallback for an installation token,** which is unverified against a real one. [EXECUTION_CONTEXT](docs/specs/EXECUTION_CONTEXT.md)

### The chain

`kuberlr-kubectl` to BRO to `rancher/charts` to `rancher/rancher`, one `gero ship` for each hop.

* **M: read a commit or branch holding at a ref.** Assets pins a charts commit in `lock.yaml`, and Rancher before v2.16 names a charts branch. The config is written; the reader is not. [CHAIN_TRACING](docs/specs/CHAIN_TRACING.md), [UPDATE_ENGINE](docs/design/UPDATE_ENGINE.md)
* **S: the `dockerfile` format,** accepted by the config and refused by the engine. [UPDATE_ENGINE](docs/design/UPDATE_ENGINE.md)
* **L: the walk through several hops,** the tree in `gero lines resolve` and `--depth`. Needs the reader above. [LINES_RESOLVE](docs/specs/LINES_RESOLVE.md)
* **L: `chartsPackageBump` and BRO's tag into a charts bump.** The strategy is named in the config and stops a run before it writes. [CHAIN_TRACING](docs/specs/CHAIN_TRACING.md)
* **S (no code): check the config against the repos.** Charts' stable branch names (`release-v2.X`), and the package of `ob-team-charts`, which is left out until it is known.

### Confidence in the foundation

* **M (no code): review the process docs.** They were written from a read of the repos and corrected by the team lead, and the release team has not reviewed them. Settle them and mark each reviewed. [process](docs/process/README.md)
* **M: settle the config kind and field names** so `gero/v1alpha2` becomes `gero/v1`. [APP_CONFIG](docs/specs/APP_CONFIG.md)
* **S: the exit-code helper,** so a usage or validation error exits 2 and not 1. [CLI_FRAMEWORK](docs/specs/CLI_FRAMEWORK.md)
* **M: the integration test tier,** a separate module that runs the built binary. [TESTING](docs/specs/TESTING.md)
* **S: run `go test ./...` once in CI with a hostile environment,** to catch a test that reads the machine. [TESTING](docs/specs/TESTING.md)
* **S: a Go test that validates `config/` against the JSON Schemas.** Needs a validator module. [APP_CONFIG](docs/specs/APP_CONFIG.md#json-schemas)
* **S: confirm a release end to end:** `cosign` runs in the `ci-image/go1.27` container, the bundles land on the release, and `gero doctor self-update` installs it. [SELF_UPDATE](docs/specs/SELF_UPDATE.md)
* **S: confirm a pushed tag starts the release workflow** of a real repo, for `gero repo tag --push`. [TASK_COMMANDS](docs/specs/TASK_COMMANDS.md)

## Good places to start

Small and bounded. None blocks anything else.

* **S: `gero doctor` checks for `repos.cloneRoot` and the `git` binary.** [DOCTOR](docs/specs/DOCTOR.md)
* **S: `gero doctor --online` reporting that a newer release exists.** [SELF_UPDATE](docs/specs/SELF_UPDATE.md)
* **S: `gero doctor self-update --version`** to install a named release or roll back, and a `trusted_root.json` by file for offline use. [SELF_UPDATE](docs/specs/SELF_UPDATE.md)
* **S: GitHub Enterprise Server hosts in `gero repo clone`.** The SSH URL is github.com only (`TODO(host)`). [TOOLING_GOALS](docs/design/TOOLING_GOALS.md)
* **S: the newest-100 open PRs warning in `gero ship`** for a busy base such as `main` of `rancher/rancher`: look the bump up by its marker, or warn only when a PR would be closed. [PUSH_TO_REPO](docs/specs/PUSH_TO_REPO.md)
* **S: say that the workstation signs under a forced CI context** in the preflight line of `gero ship`. The detection exists and nothing calls it. [EXECUTION_CONTEXT](docs/specs/EXECUTION_CONTEXT.md)
* **M: `gero repo status` extras:** how many releases a consumer is behind, the open bump PR, and the images a release publishes. [REPO_STATUS](docs/specs/REPO_STATUS.md)
* **S: fold `ship`'s `ResolveRelease` into `internal/releasetag`** so "latest" has one definition (the `TODO(release-state)` markers). [REPO_STATUS](docs/specs/REPO_STATUS.md)
* **S: the `lifecycle-date` warning** for a release shipped to a line past a lifecycle date. [CONFIG_SCHEMA](docs/design/CONFIG_SCHEMA.md#warnings)
* **M: the opt-in improvements of the two script ports,** such as the SemVer grammar mode for `gero semver check` and literal prefix matching for `gero repo build-tags`, with parity tests. [CHECK_SEMVER](docs/specs/CHECK_SEMVER.md), [BRANCH_TAGS](docs/specs/BRANCH_TAGS.md)
* **S: a `gero config resolve repos` that says why a source won,** and the checks `gero repo list` leaves out. [CLI_FRAMEWORK](docs/specs/CLI_FRAMEWORK.md)
* **S: the fork of a clone is assumed to have the upstream's repository name** under `githubUser`; allow another. [CLI_CONFIG](docs/specs/CLI_CONFIG.md)

## Later (v1.x and beyond)

Each is a draft or not designed yet. See [PHASES](docs/PHASES.md).

* **Tracking issues and the reconciler** ([METADATA](docs/specs/METADATA.md), [ARCHITECTURE_DESIGN](docs/ARCHITECTURE_DESIGN.md)): the state block, the rendered view, the `gero/` labels and `gero/hold`, `gero ci:reconcile`.
* **Checks over the chain:** stable readiness (would a stable Rancher build be valid with what Rancher holds?) and line consistency (does each hop hold a version of its own line?). [TOOLING_GOALS](docs/design/TOOLING_GOALS.md)
* **`gero workflows sync` and `verify`,** which bring the workflow templates into the bundle, and `VERSION.md` generated from config. [CENTRAL_BUNDLE](docs/specs/CENTRAL_BUNDLE.md)
* **`gero lines sync`:** update the lifecycle dates from the SUSE lifecycle page, and flag drift in `gero doctor`. [IMPLEMENTATION](docs/IMPLEMENTATION.md)
* **Steps of `gero ship` for use alone,** as goals and not command names: send a branch, open a PR for a pushed branch, apply a release to one branch and commit it, and close superseded PRs. Decide the names, and whether each is needed, when one is built. [TASK_COMMANDS](docs/specs/TASK_COMMANDS.md#use-cases-not-yet-commands)
* **The central bundle:** conditional requests, a way to name a release, a refresh interval setting, the templates and generated chunks, and whether `auto` should ever prefer a newer bundle to a local copy. [CENTRAL_BUNDLE](docs/specs/CENTRAL_BUNDLE.md)
* **QA bridge** and a **dashboard.** [PHASES](docs/PHASES.md)

## Wishlist

Not goals today; each could help a use case later.

* **Inference** of the upstream, fork, `githubUser` and release line from the working directory, with an `inferred` source in `gero config show`. The user file sets them today. [WORKSPACE](docs/specs/WORKSPACE.md#wishlist)
* **A go-git client** for the git work that runs the `git` binary today. The seam is there. [GITHUB_ACCESS](docs/specs/GITHUB_ACCESS.md)
* **A stable URL for the JSON Schemas,** and wiring them into the team's editors. [APP_CONFIG](docs/specs/APP_CONFIG.md#json-schemas)
