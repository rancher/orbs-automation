# Implementation Notes

Working notes on implementation choices: decisions, leanings and open questions. Evidence about specific repos lives in [`projects/`](projects/README.md) and [`references/`](references/frameworks-automation.md); this doc links to it instead of repeating it.

Status markers: unmarked items are decisions or working agreements. **(leaning)** is a proposal not yet agreed. **(unverified)** was decided from memory and needs confirmation (e.g. a small spike) before we rely on it.

How the CLI itself is built (framework, CLI config and its precedence, output, logging, exit codes, dependency policy) is in the [specs](specs/README.md). This doc covers what the tool does.

## 1. CLI Packaging

* The binary is named **`gero`** (Dr. Gero). It does not need to match the repo name.
* **Standalone only, using the GitHub APIs directly** (`go-gh` and `go-github`, section 3). `gh` is not a hard dependency: it is only needed for the keyring token fallback and interactive or passthrough steps.
* There is no published `gh` extension, so the central repo keeps its name and does not need a `gh-` prefix. A developer who wants `gh gero` can install the binary as an extension locally (e.g. `gh extension install .` from a checkout); that is a personal convenience and is not supported or documented as an install path.
* Name collisions for the `gero` binary on `PATH` are assumed not to be a problem (not a common binary name, and none expected on the team).

## 2. Execution Contexts

* **CI-only commands** (e.g. the reconciler) require a GHA environment and the GHA-provided token, and fail fast with a clear error otherwise (check `GITHUB_ACTIONS=true` and that a token is present). A command declares its context once, as an annotation enforced centrally ([CLI_FRAMEWORK §3](specs/CLI_FRAMEWORK.md)).
* **Either-context commands** work on a developer workstation or in CI, with auth resolved from the environment.
* Reconcile only ever runs in CI.

## 3. GitHub Access

Use two Go libraries together, and shell out to `gh` only where it adds something.

* **`cli/go-gh`:** auth and host resolution, the authenticated HTTP client, GraphQL, current-repo detection. It is an ordinary Go module and does not require running as a `gh` extension.
  * Token resolution: `GH_TOKEN` / `GITHUB_TOKEN`, then `gh`'s config; keyring-stored tokens fall back to running `gh auth token`, which requires `gh` on `PATH`. **(unverified)**
* **`google/go-github`:** typed REST calls (issues, tags, PRs, releases, check runs).
  * Build it on the `*http.Client` from `go-gh` (`api.NewHTTPClient`) so both share one token and transport. **(unverified)**
  * It bumps major versions often. Pin it and keep usage behind our own interfaces.
  * Enterprise hosts need the base URL set explicitly. Open question: do we ever target non-github.com hosts?
* **`gh` via subprocess:** only for interactive or human-facing steps (e.g. `gh auth login`, browser handoffs) and pure passthrough. Not used for logic that must be tested or that runs in CI.
* **`git` via `os/exec`:** local operations such as cloning and committing rendered workflows. PR creation goes through the API.
* **GraphQL** via the `go-gh` client, only if REST proves insufficient (e.g. batched state queries).

### Interfaces

* Define **narrow interfaces on the consumer side**, not one large `GitHub` interface mirroring the API.
* The reconciler gets one (list tags, get PR state, list check runs, read/update an issue body, post a comment) so it can be tested against a fake.
* Do not wrap `gh` subprocess calls in the same interface.
* Skip interfaces for one-shot commands (e.g. `workflows sync`) until a second implementation or a test needs one.
* The reconciler needs its own backoff policy for rate limits and retries.

## 4. Release Rules

* `release.requiresPrerelease` means a stable tag must come from a commit that already has an RC. It is a safety check that some repos do not need at all, so it is off unless a project opts in. The config accepts it from the start; enforcement ships later (see [PHASES](PHASES.md)).
* Even when the rule is on, it can be **overridden per release** (e.g. a single-commit CVE dependency bump that needs a new patch, not an RC retest).
  * Mechanism: an explicit CLI flag or issue label, with the reason recorded in the tracking issue. **(leaning)** A flag follows the conventions in [CLI_FRAMEWORK §2](specs/CLI_FRAMEWORK.md).
* **Tag classification** (semver, prerelease, build metadata) is one shared library function. Existing repos reimplement it in shell (see [projects/](projects/README.md)).
* **Tag creation works both ways.** A human can tag and push in the target repo and the tracker is notified (e.g. via `repository_dispatch`), or a workflow run from the tracker creates the requested tag in the target repo. The reconciler observes either path. The dispatch path is added as needed.
* Today humans create tags with git and verify QA sign-off before creating a stable tag. The tool fits that process first.

## 5. Versioning

* Actions are **sub-versioned by the CLI**: one version and one tag stream. An actions-only fix may produce a no-op binary release. Separate `actions/vX.Y.Z` tags can be added later if this proves painful.
* Rendered stubs record when, and from which template/CLI version, they were generated, so drift can be detected.
* A stub rendered by CLI version X should pin workflows compatible with X.
* Shared workflows are pinned by CLI version tag or SHA, never a floating branch ref, so a bad push cannot hit every branch at once.

## 6. Tracking Issue State

Informed by [frameworks-automation](references/frameworks-automation.md) (an inspiration, not a template).

* Machine state lives in a **versioned hidden block** in the issue body (e.g. an HTML comment holding YAML). The visible markdown is rendered from it.
* The block is additive-only so older runs can read newer state. A missing or corrupt block is a loud error, never silently treated as empty state.
* Only the block and generated sections are rewritten. Text outside them is left alone.
* **Checkboxes are read-only.** They display state and are never parsed. Any user interaction must feel obvious (e.g. through explicit commands, labels, or workflow dispatch), not through editing the issue body. Humans are not expected to edit tracking issues.
* Issue identity does not rely on the title. The version and other identifying fields are stored in the block. Labels are for lookup and filtering.
* **Stale state** is marked with labels, using a threshold on the last reconciled time.
* **Locking is per operation**, not a global lock across all workflows.
* Idempotency uses deterministic branch names and reuses an existing open PR.
* Cron minutes are staggered off the round hour. Actions are pinned by SHA.
* Listing calls (releases, PRs) must paginate. Decide up front how prereleases are treated, since `/releases/latest` excludes them.
* Render everything from one state object, with a zero- vs one-based stage convention documented in code, so rendering cannot contradict state.
* The reconciler is tested with fixture-driven cases (state in, expected state out), and config validation runs without credentials.
* **Audience:** engineers, QA and managers (the future GUI uses the same state), so each issue must be understandable on its own.

## 7. Config Shape and Placement

Informed by [backup-restore-operator](projects/backup-restore-operator.md) and [rancher-shell](projects/rancher-shell.md). The backup-restore-operator style (hybrid: reusable workflows, composite actions and rendered templates) is the closer model for what the central repo offers. Real configs live in [`../config/`](../config/README.md) and mock configs in [`../config-mock/`](../config-mock/) and [`../mock-repos/`](../mock-repos/); they are illustrative and not final.

### Shape

* Every config document has `apiVersion`, `kind`, `name` and `spec`. It is **inspired by Kubernetes resources but not compatible with them**: there is no `metadata`, no `status` and no controller semantics. State lives in tracking issues, not in config.
* Files may hold several YAML documents. Plain block-style YAML throughout (no `{}` or `[]` flow style, except where a value is simply left out and defaulted).
* **Group fields under `spec` by purpose** (e.g. `repo`, `release`, `sources[].accept`, `sources[].update`) so intent is obvious and omitted groups can fall back to defaults.
* **Defaults are conservative.** Only no-surprise values are defaulted, and anything that changes release behavior is written out. Stricter release verification (e.g. `release.requiresPrerelease`) is **off by default and opt-in**, leaving room for more checks later. A change to a default is a breaking change and bumps `apiVersion`. A resolved view (`gero config show --resolved`) is a candidate command. The defaults table lives in `config/README.md`.
* Use one document per resource that has its own identity, owner or location (`Project`, `External`, `WorkflowsConfig`). Use a list inside one document for a small closed set edited together (`RancherLines`).
* **Kinds so far:** `Project`, `RancherLines`, `External`, and `WorkflowsConfig` (render settings, lives in the target repo). Names are not final.
* Central file names use a type suffix in a flat directory (`<name>.project.yaml`, `<name>.external.yaml`, `rancher.lines.yaml`). A repo's own config is `gero.yaml`, parsed as real multi-document YAML and dispatched on `kind`.

### Placement

* **Release workflows are rendered into the real branch.** Keyless signing and attestation depend on the OIDC token's workflow source, so the release workflow has to live on the tagged branch itself. (Confirmed by team experience from debugging.)
* **Central repo:** the project list, topology, where things flow between repos, the Rancher line facts, and the configs of projects we do not own (`rancher/charts`, `rancher/rancher`).
* **Target repo:** `gero.yaml` holds **only `WorkflowsConfig`** for now (toolchain, test matrix, enabled workflows, `gero` version pin). Any other kind in a repo's `gero.yaml` is an error until we decide to allow more. Moving owned `Project` config into repos is a possible later step.
* **Per-repo lookup:** the repo's `automation-core` branch if it exists, otherwise the default branch.
* **Reconciler reads central config**, and fetches a repo's `gero.yaml` only when it is about to act on that repo. Facts needed just to observe (e.g. tag patterns, whether stable requires an RC) stay central.
* **Standardize shared workflows across repos** where inputs can cover the differences (e.g. CI test workflows as published reusable workflows or actions in the central repo).
* **Rendered files carry a generated header** with template and CLI version and a content hash. A `verify` command detects drift, and a dry-run diff is shown before any deploy PR is opened.
* **Avoid:** runtime fetching of config from another branch, branch-to-branch maps hardcoded in shell scripts, and per-branch copies of workflows that could be shared.
* Workflows still hand-copied per branch today (backport, stale, renovate, unrc) are candidates to standardize later.

## 8. Repo Shapes and Downstream Bumps

Evidence: [scc-operator](projects/scc-operator.md), [rancher-charts](projects/rancher-charts.md), [rancher-rancher](projects/rancher-rancher.md), [kuberlr-kubectl and the other mock projects](../config-mock/) and the cross-project table in [projects/README](projects/README.md).

* **Pull-style config.** Each project lists its **sources** (the projects it takes updates from) with `accept` (which releases and lines it takes) and `update` (strategy, params, post-edit commands). Producers do not describe their consumers. Push style exists only for `notify` toward external projects we do not model. Roles (producer/consumer) are per edge, and the reverse view ("who consumes X") is derived.
* **Every project is tied to a Rancher line for our team's repos**, except "universal" projects (e.g. `ob-team-charts`, `scc-operator`) that only become line-specific when they land in a branch of a line-specific repo such as `rancher/charts`. These leave `release.lines` out, because `universal` is the default.
* **Release branches are optional.** A repo declares its release-branch policy (`single` or `perLine`), and the tool must not assume `release/vX.Y`.
* **A repo's version line is separate from its consumers' branches.** Producers do not carry downstream branch names.
* **One shared line map.** The mapping from a line to each repo's branch was encoded in many places with different logic. The `RancherLines` document holds line facts only (phase, Kubernetes range), with no branches. Phase follows the product dates: `development`, `ga`, `eom`, `eol`. Each project maps its own series and branches to line IDs under `release.lines`, including `rancher/rancher` itself.
  * The development line comes from a repo's default branch. A branch cut moves `main` to the next line. A `release.lines` entry with no `branch` resolves to the default branch only while the line's `phase` is `development`; any other missing branch is an error, never a silent skip.
  * A source with `accept.lines: all` maps, per Rancher line, the source's release for that line to the consumer's branch for that line.
* **A downstream bump has four parts:**
  1. **Target:** repo, token reference, optional fork.
  2. **Branch resolution:** (producer version, producer line) to downstream branch, from the line map.
  3. **Edit:** a named, idempotent strategy with parameters (file, key, package name). Semantic strategies (chart version arithmetic, `go get` plus `go generate`, index lookups) are code in the CLI. Names, keys, labels and title templates are config. One implementation per strategy, replacing the divergent per-repo copies.
  4. **Post-edit commands:** an ordered list, with the toolchain and any required refs declared.
* **PR conventions** come from a per-consumer `pullRequests` block (title, branch prefix, labels) plus a stable key for reusing an open PR. Use deterministic branch names, not epoch suffixes. Producers are standardized on the consumer's own conventions (e.g. the `rancher/charts` PR title format).
* `rancher/charts`' own `auto-bump` workflows do not fit most of our use cases and are out of scope.
* **Preconditions and fan-out.** Check that the image or release asset exists and that the target branch is not frozen before opening a PR. A fan-out to several branches reports per-target state and tolerates partial failure.
* **Whether the tracker builds the full chain up front or reads hop by hop** depends on the question being asked. It is an implementation choice driven by requirements and performance.
* **Avoid:** `eval` of commands read from a downstream repo, tag triggers of `*` with no format check, fork guards scattered through workflows, and undefined variables feeding checkouts.

## 9. Config Validation

When implementation starts, build a validator (runnable without credentials) that checks:

* dangling project references, and that each `release.lines` entry names a known Rancher line
* each project-to-project edge declared once, under the consuming project's `sources`
* **Roles belong to relationships, not projects.** A project can be a producer on one edge and a consumer on another (e.g. a chart repo that consumes an operator and feeds Rancher). Project config has no `role` field; producer and consumer are derived from the edges.
* **One action per pair of projects, per edge:** `notify` (outbound) is mutually exclusive with `trackOnly` / `react` (inbound) between the same two projects, since both would imply a circular dependency. This applies to a pair, not to a project as a whole.
* edge kinds are explicit: a `sources` entry is a bump edge, `External` `relationships[].action` names `trackOnly`, `react` or `notify`, and a QA test edge is added later, because a project like the QA repo is related to others by something other than a bump
* a `gero.yaml` in a repo contains only allowed kinds

**No hard limit on graph shape beyond the pair rule.** Real workflows are complex, and artificial limits (such as banning every cycle through three or more projects) could block legitimate setups. Cycles across several projects are warnings at most. Graph traversal must be cycle-safe regardless (track visited projects).

## 10. QA Handshake (v3, undecided)

Evidence: [observability-e2e](projects/observability-e2e.md). Nothing here is decided.

* **Leaning (to agree with the QA owners):** dispatch a thin QA workflow with a correlation ID, milestone and Rancher version, observe the run's conclusion, and record it on the tracking issue as evidence. The stable-tag gate stays a human decision.
* **Avoid initially:** polling Qase directly, having QA write status into engineering repos, and per-RC full-matrix runs.
* **Prerequisite:** decide what artifact a QA run proves it tested (Rancher version, chart version or image).

## 11. Standards for Target Repos

Conventions the tool wants repos to follow, so automation does not fight them.

* **Helm chart unit tests and dynamic values.** Tests for a value that automation bumps (for example a default image tag) should set the input explicitly and assert the same value, not depend on the chart's default. That way the value can change without breaking tests, and the tests still validate the template mechanics. For example, `backup-restore-operator`'s `kubectl-image_test.yaml` asserts the default `kuberlr-kubectl` tag, so a default bump would break it. The tool does not edit test files as part of a bump.
* **`VERSION.md` is the standard place for branch and version mapping, and it is generated from config.** Repos document their line mapping today in different places and formats (a `VERSION.md`, a README table). The standard is a `VERSION.md` in each owned repo, generated by `gero` from the project's `release.lines` and the `RancherLines` facts (e.g. the Kubernetes range), and kept in sync on the default branch with a PR, like rendered workflows. It carries a generated header, and `verify` detects drift. This also moves toward automating today's manual "branch for X" workflow. Release branches keep whatever copy they had when cut.

## 12. Documentation Policy

* Docs that become redundant once an implementation exists are removed. The code carries the documentation through good comments.
* Core docs (`VISION`, `ARCHITECTURE_DESIGN`, this file) hold decisions and design. Facts about a specific repo go in `projects/`, and inspirations in `references/`.

## 13. Open Questions

* Scope and order of the milestones: see [PHASES](PHASES.md).
* CLI-layer questions (YAML library, dry-run default, exit codes): see the Open questions in [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md) and [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md).
* Reconciler triggering: cron only, or `repository_dispatch` where needed.
* Whether any config other than `WorkflowsConfig` ever moves into target repos.
* Final kind names and field names (everything in `config/` is a mock).
* Which existing workflows to standardize first, and whether the central repo publishes them as reusable workflows or composite actions.
* Whether the CLI replaces the existing "push to X" scripts or wraps them at first (decided in v0.x, see [PHASES](PHASES.md)).
* How new Rancher lines are cut and registered in the line map (manual edit, or a command that updates it).
* How downstream repos pin and call reusable workflows; resolve when we review a repo that consumes the existing core branches.
* The `VERSION.md` format: one standard layout across repos, which fields it shows (branch, tag, Rancher line, Kubernetes range, EOL section), and whether other tools that read `VERSION.md` today (e.g. frameworks-automation reads per-repo `VERSION.md` at runtime) need the old formats to keep working.
* The "branch for X" workflow itself: which steps `gero` automates (create branches, update `release.lines`, regenerate `VERSION.md`, re-render workflows) and in what order.
