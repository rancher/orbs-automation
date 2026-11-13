# TODO

The work that is accepted, by milestone. Nothing is here that has not been checked against the code and decided. The milestones are those of [PHASES](PHASES.md); the detail of each part is in its [spec](specs/README.md).

**Reading an item.** It starts with a verb, names one result, and says what makes it done. The letter is its size: **S** is hours, **M** a day or two, **L** includes design and may need a spec first. *(no code)* needs knowledge of the repos or the release process, and *(needs a real run)* cannot be shown by a test. To take one, open an issue that names it so we do not collide.

**Keeping it true.** When an item is built, delete it here and delete its line from the spec's **Not built yet** list, in the same change.

| Milestone | Means |
|---|---|
| [Before V1](#before-v1) | Needed for v1.0.0 |
| [After V1](#after-v1) | The v1.x releases, soon after |
| [Before V2](#before-v2) | Needed for the v2.0.0 gate |
| [Moonshot and wishlist](#moonshot-and-wishlist) | Wanted, with no date |
| [On request](#on-request) | Built only if someone asks |

---

## Before V1

### Ship from CI

`gero ci:ship` replaces the push scripts of `scc-operator` and `shell`. It runs for one release event, as the final step of that repo's release workflow, over the checkouts the workflow provides. [CI_SHIP](specs/CI_SHIP.md)

* **L: Finish `gero ci:ship`.** It is registered, CI-only, and refuses the workflow's own token. Given the tag a build produced, it runs the bump loop of `gero ship` as the bot on every active consumer branch of that tag's line. A push that is rejected because the branch moved fails the run, with no retry. Done when a test drives a release event through it against fakes, including the rejected push.
* **M: Make `ci:ship` work in the checkouts the workflow gives it.** The push with the `GH_TOKEN` the workflow passes is built and tested for the header git sends, not yet against GitHub. The workflow's checkout action clones the producer and the consumers into one directory, and a step before `ci:ship` seeds a user file whose `repos.search` names it (`GERO_CONFIG=... gero init --search <dir>`, then the same `GERO_CONFIG` for the run), so `ci:ship` has no flag for it. Check that seeding in CI with a test. A checkout has only an `origin` remote, so the job sets `GERO_UPSTREAM=origin`. The run owns its checkout, so it sets `StageUntracked` on the `createbranch` input ([PUSH_TO_REPO](specs/PUSH_TO_REPO.md)); a file an earlier workflow step left untracked is then committed. Done when a test workflow pushes a bump branch with no credentials of an engineer.
* **S: Write the step summary and put the workflow run URL in the PR body.** One report model with two renderings: a fuller one for the job log and a shorter one for comments. Done when a run writes `GITHUB_STEP_SUMMARY` and the PR body names the run.
* **M: Run steps `from: producer` and `from: central`.** `steprun` refuses both. Done when a step runs a script from the producer's checkout or from the bundle, in `ci:ship` only. [APP_CONFIG](specs/APP_CONFIG.md)
* **S: Print the preflight line that names workstation signs under a forced CI context,** in `gero ship` and `ci:ship`. It only warns. `clicfg.WorkstationSigns` has no caller. Done when forcing the CI context on a workstation prints the line. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)
* **M: Add a workflow to `scc-operator` and `shell` that installs a pinned, signed `gero` and runs `ci:ship`,** beside the bash script, then in place of it. Needs `ci:ship`.
* **S (needs a real run): Check the `auth status` 403 path against a real installation token.** It is tested with fakes only. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

### Tracing the chain

One chain runs `kuberlr-kubectl`, BRO, `rancher/charts`, `rancher/rancher`. Chains differ by line: before v2.16 Rancher holds a charts branch directly, from v2.16 the commit comes through the Assets image, and some components touch neither. Every item below follows each project's own holdings by line and assumes no fixed path. [CHAIN_TRACING](specs/CHAIN_TRACING.md)

* **M: Read a commit or a branch that a repo holds, at a ref.** The config models and validates both kinds; the engine skips them. Done when `gero repo status` and `gero lines resolve` report what such a holding holds. [UPDATE_ENGINE](design/UPDATE_ENGINE.md)
* **M: Read the commit pinned in `lock.yaml` at a ref and compare it with the head of the stage's branch.** Done when `repo status` says whether the newest Assets tag was built from today's charts branch.
* **S: Confirm where Rancher holds the charts branch.** Before v2.16 it is `CHART_DEFAULT_BRANCH` in `package/Dockerfile`; find the file and key for v2.16 and later. Done when each line's holding names the right file and key and a test reads it.
* **L: Walk several hops in `gero lines resolve` and `gero repo status`.** The walk is safe on cycles, prints a tree, and takes `--depth`. Done when `ob-team-charts`, `charts` and `rancher` appear as one connected path for a line. [LINES_RESOLVE](specs/LINES_RESOLVE.md)
* **M: Check a held tag's stage against the release team's rule.** The rule comes from the release team. Done when a mismatch is reported by `repo status` or the walk.

### Charts automation

BRO's release becomes a PR in `rancher/charts`. The team has bash tooling for this today, and the strategy is an abstraction of it. [CHAIN_TRACING](specs/CHAIN_TRACING.md)

* **M: Add the `chartVersion` scheme to `internal/tagscheme`.** It reads the part after `+up` and answers Valid, LineOf, Latest, IsStable and Compare. Done when the `rancher-charts` stream resolves its newest tag instead of failing with "scheme chartVersion is not built".
* **S (no code): Read the team's bash tooling for charts PRs and write down what it does, by line.** The maintainer provides it when this starts. Done when each behaviour it has is in a spec section or a test case.
* **L: Build the `chartsPackageBump` strategy.** It replicates the team's process, takes its settings from `with:` so others can use it, turns BRO's tag into a charts bump, and reports what it wrote through `Result.Changes` like the other strategies. Done when the stub test is replaced by a test of a real bump and a dry run beside the bash tooling agrees with it.

### The config contract and the foundation

* **M: Freeze the field names of the kinds that exist, and rename `gero/v1alpha2` to `gero/v1`.** Done when the schemas, `config/`, the tests and [APP_CONFIG](specs/APP_CONFIG.md) agree.
* **S: Validate every file of `config/` against the JSON Schemas in a Go test.** Needs a validator module, which needs an entry in [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md). Done when an invalid file in `config/` fails `go test`.
* **S: Make `gero config validate` and `gero doctor` build the update engine and report its errors,** so a strategy that cannot run fails there and not at `ship`. Done when an unbuilt strategy named in config is reported by both.
* **S: Add the exit-code helper.** A usage or validation error exits 2. Done when a typed error maps to 2 in `Execute` and each command class has a test. [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md), [DOCTOR](specs/DOCTOR.md)

---

## After V1

### Tracking and the reconciler

Tracking issues show where each release stands, and `gero ci:reconcile` keeps them true. It runs on a cron, which catches periodic work and drift from people editing issues and PRs across repos, and on `repository_dispatch`, which is nearer to real time. Both are planned together. [METADATA](specs/METADATA.md)

* **L: Write and render the state block of a tracking issue:** the part a machine reads and the view a person reads. Done when a fixture issue round-trips and renders.
* **L: Build `gero ci:reconcile`.** It brings the tracking issues in line with what the repos hold, with its reads and writes of issues, check runs and releases behind an interface of its own, and runs from both triggers. It is a stub today. Done when a fixture-driven test reconciles drift in either direction.
* **M: Create the `gero/stale` and `gero/superseded` labels on demand and reconcile them.** Done when a superseded PR gains the label and a current one loses it.
* **M: Honor `gero/hold`.** Check for it before work, so a held line is skipped, with a warning on workstation commands. Done when a held line is skipped and says why.
* **S: Define the `gero/*` label names once,** as constants the whole tool shares. Done when no label name is written twice.

### Workflow rendering and `VERSION.md`

A repo provides workflow templates and configs, and `gero` renders them with the RancherLines variables injected, replacing the `automation-core` branch tooling. [APP_CONFIG](specs/APP_CONFIG.md), [CENTRAL_BUNDLE](specs/CENTRAL_BUNDLE.md)

* **M: Add the `WorkflowsConfig` type and schema, and a loader for a repo's `gero.yaml`.** It rejects every other kind, since `WorkflowsConfig` is the only one a target repo holds. Done when a repo's `gero.yaml` loads, and any other kind in it is an error.
* **L: Build `gero workflows sync` and `gero workflows verify`.** Sync renders a repo's templates and configs with the RancherLines variables, from template and generated chunks in the bundle; verify exits 3 when a copy has drifted. When templates ship, settle the name of `centralConfigDir`. Both are stubs today. Done when sync writes a repo's workflows and verify fails on a changed copy.
* **M: Generate `VERSION.md` from `branches.lines` and `RancherLines`.** The columns and format are chosen per repo: a `Project` config opts into the ones it wants. Done when a config with two different choices produces two different files, and `verify` detects drift.

### Lines and lifecycle

* **M: Build `gero lines sync`.** It fetches the SUSE lifecycle page, updates the dates in the lines file and shows the diff first. `internal/lifecycle` parses the page and nothing calls it. A new line is still added by hand. Done when a changed date on a fixture page produces a reviewed diff.
* **S: Make `gero doctor` flag drift between the lifecycle dates and the page.** Needs `lines sync`.
* **S: Raise the `lifecycle-date` warning** when a release is shipped to a line past a lifecycle date. Done when the rule is in `rules.go` and the [CONFIG_SCHEMA](design/CONFIG_SCHEMA.md#warnings) table says so.

### Release status and the task commands

* **M: Show in `gero repo status` how many releases a consumer is behind.** Count the tags in the line's series newer than the version held. The per-series index belongs in `internal/tagscheme`, and the engine has to hand back the version it found (`TODO(release-state)`). [REPO_STATUS](specs/REPO_STATUS.md)
* **M: Show in `gero repo status` the open bump PR of a consumer that is behind,** as a tier of its own, reusing `pulls.Service` and the marker lookup. Done when a behind consumer with an open bump PR shows it.
* **S: Show in `gero repo status` the images a release publishes,** from `registry.CheckProject`, behind an option.
* **M: Read a pinned holding against what its pin requires in `gero repo status`,** and show a pin as an edge in `gero lines resolve`. Today the lasso BRO holds through wrangler is compared with the latest lasso. Done when `repo status lasso` says BRO's lasso is below what its wrangler requires. [REPO_STATUS](specs/REPO_STATUS.md), [TOOLING_GOALS](design/TOOLING_GOALS.md)
* **S: Warn in `gero ship lasso` when the tag is below what the consumer's wrangler requires.** Needs the go.mod of the wrangler the consumer holds. Done when shipping an older lasso than that wrangler needs says so on the target. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)
* **M: Let `gero ship --line` take lines held on different streams,** such as `wrangler --line v2.15,v2.16 --to bro`, as one plan for each stream. It is refused today. Done when each line gets the release of its own stream in one run. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)
* **S: Find an existing bump PR by its marker in `gero ship`,** not by listing the newest 100 open PRs (`pulls.DefaultMax`). On a busy base an older PR of the bump is missed. Done when a test with more than 100 open PRs finds it. [PUSH_TO_REPO](specs/PUSH_TO_REPO.md)
* **M: Enforce `requiresPrerelease` for tags made by hand or by a workflow.** `gero repo tag` enforces it only for its own tags. Done when a stable tag without a prior prerelease is reported wherever it was made. [IMPLEMENTATION](IMPLEMENTATION.md)
* **M: Move closing superseded PRs, the plan and the publish steps out of `internal/ship` into `internal/task`.** Behaviour does not change. Done when `ship` calls tasks for each step and its tests still pass. [PROJECT_LAYOUT](specs/PROJECT_LAYOUT.md)
* **S: Print the preflight line in `gero repo branch` and `gero repo tag`,** which do not print one. Done when both name the acting identity and how it signs. [EXECUTION_CONTEXT](specs/EXECUTION_CONTEXT.md)

### Checks over the chain

Read-only, on the walk. [TOOLING_GOALS](design/TOOLING_GOALS.md)

* **L: Check stable readiness:** would a stable Rancher build be valid with what Rancher holds? Needs the multi-hop walk. Done when the answer names what blocks it and how many projects were traced.
* **M: Check line consistency:** does each hop hold a version of its own line? Done when a hop holding another line's version is reported with its path.

### Config and CLI

* **M: Add `gero config show --resolved`,** which prints the graph with defaults applied. [APP_CONFIG](specs/APP_CONFIG.md)
* **S: Warn of holding cycles longer than two projects.** Done when a three-project cycle raises `circular-holding`.
* **M: Add one `ui.Confirm`** for `ship`, `repo tag`, `shell install` and `doctor self-update`, which each draw their own `[y/N]`. Done when all four use it. [CLI_OUTPUT](specs/CLI_OUTPUT.md)
* **M: Give every command the logger through the context.** `ship` builds its own in `cmd/` and no other command logs. Done when one logger is built and handed down. [CLI_OUTPUT](specs/CLI_OUTPUT.md)
* **M: Narrow `ship.Git` and `ship.Remote` to what each task calls.** Both carry every method any task needs. Done when each task takes an interface of its own.
* **S: Add `--json` to `gero config validate` and `--output env` to `gero auth status`.** Done when each renders and exits as the text form does. [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md)

### Doctor and the workstation

* **M: Add the online checks to `gero doctor`:** the token can open PRs, access to the tracker repo, push access to each fork, and a clone far behind its upstream. [DOCTOR](specs/DOCTOR.md)
* **S: Make `gero doctor --online` warn when a newer `gero` release exists.** Done when a stand-in releases source drives a test. [SELF_UPDATE](specs/SELF_UPDATE.md)
* **S: Add a `gero doctor` check that the shell block is installed and current.** [SHELL_SETUP](specs/SHELL_SETUP.md)
* **M: Make `gero config resolve repos -v` log the scanned directories, the remotes read and why each candidate matched or not.** [WORKSPACE](specs/WORKSPACE.md)
* **S: Complete the project argument of `gero cd` from the config.** [SHELL_SETUP](specs/SHELL_SETUP.md)
* **M: Verify a real signed release bundle in a test or CI step.** Today the self-update tests use a stand-in verifier. Done when a captured `.sigstore.json` is verified by the real verifier against the real signer identity. [SELF_UPDATE](specs/SELF_UPDATE.md)

---

## Before V2

* **L: Automate "branch for X".** Compose `gero repo branch`, the update of `branches.lines`, `VERSION.md` and the workflow render, in an order settled first. Done when a real Rancher line cut runs through `gero`, which is a gate of v2.0.0. Needs the rendering work above. [IMPLEMENTATION](IMPLEMENTATION.md)

---

## Moonshot and wishlist

**Moonshot**

* **L: Automate adding a new Rancher line and its branches,** end to end. A new line is added by hand until then, since its Kubernetes range is not on the lifecycle page.
* **L: Write the spec for the dashboard** (its command is `dashboard` or `triage`, still to name), then build it. [PHASES](PHASES.md)
* **L: Write the spec for the QA bridge.** [PHASES](PHASES.md)

**Wishlist**

* **S: Offer a few Actions that make using `gero` easier in CI,** such as one that calls `gero ci:ship` from a shipping repo. Where they live and what kind they are is decided with the first repo that uses one.
* **S: Publish the JSON Schemas at a stable URL,** then wire them into the team's editors (no code). [APP_CONFIG](specs/APP_CONFIG.md#json-schemas)
* **S: Add a paste-safe mode to `gero doctor`** that leaves out local paths. [DOCTOR](specs/DOCTOR.md)
* **M: Improve the REPL:** a short `help` of its own commands, a one-line hint for an unknown name, `!<command>` for a shell line, and an opt-in prompt prefix when `GERO_REPL` is set. [REPL](specs/REPL.md)
* **M: Support more shells and test the scripts for real:** Windows PowerShell 5, nushell and elvish, and tests that run the generated scripts in real fish and `pwsh`. [SHELL_SETUP](specs/SHELL_SETUP.md)
* **M: Add golden files under `testdata/` for command output,** which tests compare as strings today. [TESTING](specs/TESTING.md)
* **M: Add a pty harness** to test the REPL and prompts in a terminal. Needs a dependency decision in [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md).
* **L: Infer the upstream, fork, `githubUser` and release line from the working directory,** with an `inferred` source in `gero config show`. [WORKSPACE](specs/WORKSPACE.md)
* **S: Let a clone's fork have a repository name other than the upstream's** under `githubUser`. [CLI_CONFIG](specs/CLI_CONFIG.md)
* **L: Add a go-git client** for the git work that runs the `git` binary. The seam is there (`TODO(go-git)`). [GITHUB_ACCESS](specs/GITHUB_ACCESS.md)

---

## On request

Not planned. Each is built if someone asks, and not before.

* **GitHub hosts beyond github.com:** a `host` setting (`GERO_HOST`, `--host`) used by credentials and by the SSH URL of `gero repo clone`, and API base URLs for `*.ghe.com` and `github.localhost`. [GITHUB_ACCESS](specs/GITHUB_ACCESS.md)
* **Commands for the steps of `ship` used alone:** push a branch, open a PR, make a bump commit, close superseded PRs. Wait for a second caller. [TASK_COMMANDS](specs/TASK_COMMANDS.md)
* **`gero repo branch --from <ref>`.** [TASK_COMMANDS](specs/TASK_COMMANDS.md)
* **A branch shorthand with `@`, sharing `resolvePushTarget`, and a config setting for a step's commit.**
* **`gero doctor self-update --version`** to install a named release or roll back. [SELF_UPDATE](specs/SELF_UPDATE.md)
* **A `trusted_root.json` given to `gero doctor self-update` by file,** for offline use. [SELF_UPDATE](specs/SELF_UPDATE.md)
* **A Windows build.** [SELF_UPDATE](specs/SELF_UPDATE.md)
* **More REPL prompt placeholders,** such as the status of the last line. [REPL](specs/REPL.md)
* **Running the checks of `gero doctor` in parallel.** [DOCTOR](specs/DOCTOR.md)
