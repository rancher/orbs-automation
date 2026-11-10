# TODO

What is open, in pieces you can pick up. Each item is checked against the code and the specs' **Not built yet** lists, and is written to become one issue: it starts with a verb, names one result, and says what makes it done. An item that is too big for one issue says so and is split before it is taken. The plan by milestone is [PHASES](PHASES.md), the detail of each part is in its [spec](specs/README.md), and [contributing](contributing/README.md) says how to work on the code and docs.

When an item is built, delete it here and delete its line from the spec's **Not built yet** list, in the same change.

Each item has a size: **S** is a few hours with the spec, **M** a day or two, **L** is design as well as code, and starts with a spec if there is none. *(no code)* needs knowledge of the repos or the release process, not Go. *(needs a real run)* cannot be shown by a test. To take one, open an issue naming it so we do not collide.

## Needed for v1.0.0

### Ship from CI

Replace the `push-to-rancher` scripts of `scc-operator` and `shell` with `gero`. [CI_SHIP](specs/CI_SHIP.md)

* **L: Build the `gero ci:ship` command.** Register it as CI-only and run the bump loop of `gero ship` as the bot: the bot identity from `GERO_BOT_USER`, and `cigate.RequireCrossRepoToken` called before work. Done when a test drives it end to end against fakes.
* **M: Push with the workflow's token from a temporary clone.** The clone is a `Workspace` and the token is a one-shot credential for `gitcli.Push` and `PushTag`. Done when `ci:ship` pushes without the engineer's own credentials.
* **S: Write the markdown step summary and put the workflow run URL in the PR body** of a `ci:ship` run.
* **M: Run steps `from: producer` and `from: central`.** `steprun` refuses both. Done when a step can run a script of the producer's checkout or one from the bundle, in `ci:ship` only. [APP_CONFIG](specs/APP_CONFIG.md)
* **M: Let `ci:*` commands run without a checkout of the central config.** Both CI workflows point `GERO_CENTRAL_CONFIG_DIR` at a checkout. Decide whether a job may use the bundle of the installed release, and build it. [CENTRAL_BUNDLE](specs/CENTRAL_BUNDLE.md)
* **M: Add a workflow to `scc-operator` and `shell` that installs a pinned, signed `gero` and runs `ci:ship`,** beside the bash script, then in place of it. Needs `ci:ship`.
* **S (needs a real run): Verify the `auth status` 403 path against a real installation token.** It is tested with fakes only. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

### The chain

`kuberlr-kubectl` to BRO to `rancher/charts` to `rancher/rancher`, one `gero ship` for each hop.

* **S (no code): Answer the open questions of [CHAIN_TRACING](specs/CHAIN_TRACING.md)** with the release team: the Rancher key for the charts branch before v2.16, how to read the commit pinned in `lock.yaml`, and how a BRO tag maps to a chart version. Done when each answer is in the spec. The two items below need them.
* **M: Read a commit or a branch that a repo holds at a ref.** The config models both kinds and validates them; `resolveHolding` handles only releases. Includes the `lock.yaml` read. Done when `gero repo status` and `gero lines resolve` report what such a holding holds. [CHAIN_TRACING](specs/CHAIN_TRACING.md), [UPDATE_ENGINE](design/UPDATE_ENGINE.md)
* **S: Let the update engine read and write the `dockerfile` format.** It returns `ErrFormatNotBuilt`. Done when the refusal test is replaced by a read and write test. [UPDATE_ENGINE](design/UPDATE_ENGINE.md)
* **L: Walk several hops in `gero lines resolve`,** with the tree and `--depth`. Needs the reader above. [LINES_RESOLVE](specs/LINES_RESOLVE.md)
* **L: Build the `chartsPackageBump` strategy,** so BRO's tag becomes a charts bump. It is registered as a stub that stops a run before it writes. Done when the stub test is replaced by a test of a real bump. [CHAIN_TRACING](specs/CHAIN_TRACING.md)
* **M: Check a held tag's stage against the release team's rule.** Done when a mismatch is reported by `gero repo status` or the walk. [CHAIN_TRACING](specs/CHAIN_TRACING.md)
* **S: Make `gero config validate` and `gero doctor` build the update engine and report its errors,** so a strategy that cannot run fails there and not at `ship`. [APP_CONFIG](specs/APP_CONFIG.md)
* **S (no code): Check the config against the repos.** Confirm charts' stable branch names (`release-v2.X`) and find the package of `ob-team-charts`, which is left out until it is known.

### Confidence in the foundation

* **M (no code): Get the process docs reviewed by the release team.** All of them are still drafts. Done when each says reviewed, with a date and the reviewer. [process](process/README.md)
* **M: Rename the config kind from `gero/v1alpha2` to `gero/v1`** once the field names are settled. Done when the schemas, `config/`, the tests and [APP_CONFIG](specs/APP_CONFIG.md) agree.
* **S: Add the exit-code helper,** so a usage or validation error exits 2. Done when a typed error maps to 2 in `Execute` and each command class has a test. [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md), [DOCTOR](specs/DOCTOR.md)
* **M: Add the integration test tier,** a `test/integration` module that runs the built binary, and a CI job for it. Done when one black-box test runs and the module adds nothing to the root `go.mod`. [TESTING](specs/TESTING.md)
* **M: Add an integration test that pushes to a scratch repository.** Needs the tier. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)
* **S: Run `go test ./...` once in CI with a hostile environment** (a junk `HOME`, `GERO_*`, `GITHUB_TOKEN`, `XDG_*`). Done when a test that reads the machine fails the job. [TESTING](specs/TESTING.md)
* **S: Add a Go test that validates every file of `config/` against the JSON Schemas.** Needs a validator module. [APP_CONFIG](specs/APP_CONFIG.md#json-schemas)
* **S (needs a real run): Confirm a release end to end:** `cosign` runs in the `ci-image/go1.27` container, the bundles land on the release, and `gero doctor self-update` installs it. [SELF_UPDATE](specs/SELF_UPDATE.md)
* **S (needs a real run): Confirm a pushed tag starts the release workflow** of a real repo, for `gero repo tag --push`. [TASK_COMMANDS](specs/TASK_COMMANDS.md)
* **S (no code): Correct the specs that no longer match the code.** `REPO_STATUS` names a `TODO(releasetag)` marker that does not exist. `BRANCH_TAGS` lists the default short commit as open, and the code settles it. `PROJECT_LAYOUT` asks where generated code goes, and `generated/` and `tools/` answer it. `CENTRAL_BUNDLE` asks about rate limits, and `bundlestore` reports `ErrRateLimited`.

## Good places to start

Small and bounded. None blocks anything else.

### Commands

* **S: Make `gero doctor --online` warn when a newer `gero` release exists.** Done when a stand-in releases source drives a test. [SELF_UPDATE](specs/SELF_UPDATE.md)
* **S: Add `--json` to `gero config validate`,** a list of problems with the same exit code as text. [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md)
* **S: Make `gero config resolve repos -v` log the scanned directories, the remotes read and why each candidate matched or not.** [WORKSPACE](specs/WORKSPACE.md)
* **S: Complete the project argument of `gero cd` from the config.** [SHELL_SETUP](specs/SHELL_SETUP.md)
* **S: Add a `gero doctor` check that the shell block is installed and current.** [SHELL_SETUP](specs/SHELL_SETUP.md)

### `ship` and `repo status`

* **S: Find an existing bump PR by its marker in `gero ship`,** not by listing the newest 100 open PRs. On a busy base such as `main` of `rancher/rancher` an older PR of the bump is missed, and `ship` only warns. Done when a test with more than 100 open PRs finds it. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)
* **S: Print the preflight line that names workstation signs under a forced CI context** in `gero ship`. `clicfg.WorkstationSigns` has no caller. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)
* **S: Print the preflight line in `gero repo branch` and `gero repo tag`,** which follow the other rules and do not print it. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)
* **M: Show in `gero repo status` how many releases a consumer is behind.** Needs the tags between two releases (`TODO(release-state)`). [REPO_STATUS](specs/REPO_STATUS.md)
* **M: Show in `gero repo status` the open bump PR of a consumer that is behind,** as a tier of its own using the marker lookup. [REPO_STATUS](specs/REPO_STATUS.md)
* **S: Show in `gero repo status` the images a release publishes,** using `internal/registry.CheckProject`. [REPO_STATUS](specs/REPO_STATUS.md)

### Config

* **S: Raise the `lifecycle-date` warning** when a release is shipped to a line past a lifecycle date. Done when the rule is in `rules.go` and the [CONFIG_SCHEMA](design/CONFIG_SCHEMA.md#warnings) table says so.
* **S: Warn on holding cycles longer than two projects.** Done when a three-project cycle raises `circular-holding`. [APP_CONFIG](specs/APP_CONFIG.md)
* **L: Design the opt-in flags of `gero semver check` and `gero repo build-tags`,** such as a SemVer grammar mode and literal prefix matching, then build them with parity tests. The open questions of both specs come first. [CHECK_SEMVER](specs/CHECK_SEMVER.md), [BRANCH_TAGS](specs/BRANCH_TAGS.md)

### Internals

* **S: Extract one `ui.Confirm`** for `ship`, `repo tag`, `shell install` and `doctor self-update`, which each draw their own `[y/N]`. Done when the four use it. [CLI_OUTPUT](specs/CLI_OUTPUT.md)
* **M: Hand the logger to every command through the context.** `ship` builds its own in `cmd/`, and the others do not log. [CLI_OUTPUT](specs/CLI_OUTPUT.md)
* **M: Narrow `ship.Git` and `ship.Remote` to what the tasks call.** Both carry every method any task needs. [PROJECT_LAYOUT](specs/PROJECT_LAYOUT.md)

## Later (v1.x and beyond)

Each is a draft or not designed yet. See [PHASES](PHASES.md).

### Tracking issues and the reconciler

[METADATA](specs/METADATA.md), [GITHUB_ACCESS](specs/GITHUB_ACCESS.md), [ARCHITECTURE_DESIGN](ARCHITECTURE_DESIGN.md)

* **L: Write and render the state block of a tracking issue,** the part a machine reads and the view a person reads.
* **M: Create the `gero/stale` and `gero/superseded` labels on demand and reconcile them.**
* **M: Honor `gero/hold`:** check for it before work, so a held line is skipped.
* **L: Build `gero ci:reconcile`,** which brings the tracking issues in line with what the repos hold. It is a stub. Needs the items above and the reconciler's reads and writes of issues, check runs and releases.

### Checks over the chain

[TOOLING_GOALS](design/TOOLING_GOALS.md)

* **L: Check stable readiness:** would a stable Rancher build be valid with what Rancher holds? Needs the multi-hop walk.
* **M: Check line consistency:** does each hop hold a version of its own line?

### The central bundle and workflows

[CENTRAL_BUNDLE](specs/CENTRAL_BUNDLE.md)

* **M: Add the `WorkflowsConfig` type and schema,** and validate a repo's `gero.yaml` against the allowed kinds. [APP_CONFIG](specs/APP_CONFIG.md)
* **L: Build `gero workflows sync` and `gero workflows verify`.** Both are stubs. Sync brings the templates and generated chunks into a repo, and verify fails when a copy has drifted. Needs the type above.
* **M: Generate `VERSION.md` from config.**
* **S: Send conditional requests when refreshing the bundle,** so an unchanged bundle is not downloaded again.
* **S: Add a setting for the bundle refresh interval.** `bundlestore.Resolver` takes one; no setting supplies it.
* **S: Decide whether `auto` should ever prefer a newer bundle to a local copy,** and record the answer in the spec.

### Lines and lifecycle

* **M: Build `gero lines sync`,** which fetches the SUSE lifecycle page and updates the dates in the lines file, showing the diff first. `internal/lifecycle` already parses the page; nothing calls it. [IMPLEMENTATION](IMPLEMENTATION.md)
* **S: Make `gero doctor` flag drift between the lifecycle dates and the page.** Needs `lines sync`.
* **M: Enforce `requiresPrerelease` for tags made by hand or by a workflow.** `gero repo tag` enforces it only for its own tags. [IMPLEMENTATION](IMPLEMENTATION.md)

### Doctor

Each is one issue. [DOCTOR](specs/DOCTOR.md)

* **S: Check that the token can open pull requests.**
* **S: Check access to the tracker repo.**
* **S: Check push access to each fork.**
* **S: Warn of a clone far behind its upstream.**

### Other

* **M: Decide which steps of `gero ship` are used alone, and name them.** The candidates are goals, not command names: send a branch, open a PR for a pushed branch, apply a release to one branch and commit it, and close superseded PRs. Decide whether each is needed when one is built. [TASK_COMMANDS](specs/TASK_COMMANDS.md#use-cases-not-yet-commands)
* **S: Add `--from <ref>` to `gero repo branch`.** [TASK_COMMANDS](specs/TASK_COMMANDS.md)
* **M: Add golden files under `testdata/` for command output,** which tests compare as strings. Optional. [TESTING](specs/TESTING.md)
* **L: Write the spec for the QA bridge.** [PHASES](PHASES.md)
* **L: Write the spec for the dashboard.** [PHASES](PHASES.md)

The REPL items ([REPL](specs/REPL.md): `help`, the unknown-name message, a failed-line prompt marker, `!<command>`) wait for the team's use and are taken up when someone asks.

## Wishlist

Not goals today; each could help a use case later.

* **L: Infer the upstream, fork, `githubUser` and release line from the working directory,** with an `inferred` source in `gero config show`. [WORKSPACE](specs/WORKSPACE.md#wishlist)
* **L: Add a go-git client** for the git work that runs the `git` binary. The seam is there (`TODO(go-git)`). [GITHUB_ACCESS](specs/GITHUB_ACCESS.md)
* **S: Publish the JSON Schemas at a stable URL.** [APP_CONFIG](specs/APP_CONFIG.md#json-schemas)
* **S (no code): Wire the published schemas into the team's editors.** Needs the stable URL.
* **S: Add `gero doctor self-update --version`** to install a named release or roll back. [SELF_UPDATE](specs/SELF_UPDATE.md)
* **S: Add a way to give `gero doctor self-update` a `trusted_root.json` by file** for offline use. `selfupdate.Config.TrustedRoot` takes one; no flag or setting supplies it. [SELF_UPDATE](specs/SELF_UPDATE.md)
* **M: Add a `host` setting (`GERO_HOST`, `--host`) and read it in `gero repo clone`,** so the SSH URL is not github.com only (`TODO(host)`). [CLI_CONFIG](specs/CLI_CONFIG.md), [GITHUB_ACCESS](specs/GITHUB_ACCESS.md)
* **S: Let a `repo clone`'s fork have a repository name other than the upstream's** under `githubUser`. [CLI_CONFIG](specs/CLI_CONFIG.md)
