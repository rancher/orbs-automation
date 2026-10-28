# Implementation Notes

Working notes on implementation choices: decisions, leanings and open questions. Evidence about specific repos lives in [`projects/`](projects/README.md) and [`references/`](references/frameworks-automation.md); this doc links to it instead of repeating it.

Status markers: unmarked items are decided. **(leaning)** is a proposal not yet agreed. **(unverified)** was decided from memory and needs confirmation (e.g. a small spike) before we rely on it.

## 1. CLI Packaging

* The binary is named **`gero`** (Dr. Gero). It does not need to match the repo name.
* **Standalone-first.** The CLI must be usable by humans without being a `gh` extension, and CI downloads a pinned release binary.
* **Decided: standalone only, using the GitHub APIs directly** (`go-gh` and `go-github`, section 3). `gh` is not a hard dependency: it is only needed for the keyring token fallback and interactive or passthrough steps.
* There is no published `gh` extension, so the central repo keeps its name and does not need a `gh-` prefix. A developer who wants `gh gero` can install the binary as an extension locally (e.g. `gh extension install .` from a checkout); that is a personal convenience and is not supported or documented as an install path.
* Name collisions for the `gero` binary (existing Go CLIs on `PATH`) have not been checked.

## 2. Execution Contexts

* **CI-only commands** (e.g. the reconciler) require a GHA environment and the GHA-provided token, and fail fast with a clear error otherwise (e.g. check `GITHUB_ACTIONS=true` and that a token is present).
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

* `requires_prerelease` means a stable tag must come from a commit that already has an RC. It is a safety check that some repos do not need at all.
* Even when the rule is on, it can be **overridden per release** (e.g. a single-commit CVE dependency bump that needs a new patch, not an RC retest).
  * Mechanism: an explicit CLI flag or issue label, with the reason recorded in the tracking issue. **(leaning)**
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

## 7. Config Placement and Workflow Distribution

Informed by [backup-restore-operator](projects/backup-restore-operator.md) and [rancher-shell](projects/rancher-shell.md). The backup-restore-operator style (hybrid: reusable workflows, composite actions and rendered templates) is the closer model for what the central repo offers.

* **Release workflows are rendered into the real branch.** Keyless signing and attestation depend on the OIDC token's workflow source, so the release workflow has to live on the tagged branch itself. (Confirmed by team experience from debugging.)
* **Principle:** split config by where the information is used. Config belongs in the central repo only if the tool needs it without touching the target repo. When the tool is already working in a target repo, it has that repo's contents and reads the config from there.
* **Central repo owns:** the project list, topology, where things flow between repos, and the shared line map (section 8).
* **Each repo owns:** config that tells the tool how to render workflows into its branches (toolchain, test matrix, enabled workflows, `gero` version pin).
* **Per-repo config lookup:** the repo's `automation-core` branch if it exists, otherwise the default branch (`main`).
* **Read cost:** the tool reads a target repo's config only when it is also about to act on that repo, so extra API calls are limited to those cases (some will be no-ops). Anything the reconciler needs just to observe (e.g. tag patterns, whether stable requires an RC) belongs in central config. The alternative is to let the reconciler read current per-repo config on every run so results always reflect the target repo. Not decided; likely affordable with a modest number of repos, especially with conditional requests (ETags). **(unverified)**
* **Standardize shared workflows across repos** where inputs can cover the differences (e.g. CI test workflows as published reusable workflows or actions in the central repo).
* **Rendered files carry a generated header** with template and CLI version and a content hash. A `verify` command detects drift, and a dry-run diff is shown before any deploy PR is opened.
* **Avoid:** runtime fetching of config from another branch, branch-to-branch maps hardcoded in shell scripts, and per-branch copies of workflows that could be shared.
* Workflows still hand-copied per branch today (backport, stale, renovate, unrc) are candidates to standardize later.

## 8. Repo Shapes and Downstream Bumps

Evidence: [scc-operator](projects/scc-operator.md), [rancher-charts](projects/rancher-charts.md), [rancher-rancher](projects/rancher-rancher.md) and the cross-project table in [projects/README](projects/README.md). Proposals unless noted.

* **Release branches are optional.** A repo declares its release-branch policy (e.g. `single` or `per-minor`), and the tool must not assume `release/vX.Y`.
* **A repo's version line is separate from its consumers' branches.** A repo has one or more release lines and N consumers. Producers do not carry downstream branch names.
* **One shared line map.** The mapping from a line to each repo's branch is encoded in many places today, with different logic per place. It belongs in central config once, and every consumer resolves from it.
  * The development line comes from a repo's default branch (`main`). A branch cut is an event that creates a release branch and moves `main` to the next line. The map must express both.
  * Falling back to `main` for a line with no branch yet is an explicit rule in the map. Any other unmapped line is a loud failure, never a silent skip.
* **A downstream bump has four parts:**
  1. **Target:** repo, token reference, optional fork.
  2. **Branch resolution:** (producer version, producer line) to downstream branch, from the shared map.
  3. **Edit:** a named, idempotent strategy with parameters (file, key, package name). Semantic strategies (chart version arithmetic, `go get` plus `go generate`, index lookups) are code in the CLI. Names, keys, labels and title templates are config. One implementation per strategy, replacing the divergent per-repo copies.
  4. **Post-edit commands:** an ordered list, with the toolchain and any required refs declared.
* **PR conventions** come from a per-consumer template (title, branch prefix, labels, body) plus a stable key for reusing an open PR. Use deterministic branch names, not epoch suffixes. Producers are standardized on the consumer's own conventions (e.g. the `rancher/charts` PR title format and labels).
* `rancher/charts`' own `auto-bump` workflows do not fit most of our use cases and are out of scope.
* **Preconditions and fan-out.** Check that the image or release asset exists and that the target branch is not frozen before opening a PR. A fan-out to several branches reports per-target state and tolerates partial failure.
* **Avoid:** `eval` of commands read from a downstream repo, tag triggers of `*` with no format check, fork guards scattered through workflows, and undefined variables feeding checkouts.

## 9. Topology Config Validation

When implementation starts, build a validator for the central topology config that checks:

* dangling project references and cycles
* `upstream` / `downstream` consistency (or declare each edge once and derive the other direction)
* one action per external dependency: `notify` (outbound) is mutually exclusive with `react` / `track-only` (inbound) for the same pair, since both would imply a circular dependency

## 10. QA Handshake (V2, undecided)

Evidence: [observability-e2e](projects/observability-e2e.md). Nothing here is decided.

* **Leaning (to agree with the QA owners):** dispatch a thin QA workflow with a correlation ID, milestone and Rancher version, observe the run's conclusion, and record it on the tracking issue as evidence. The stable-tag gate stays a human decision.
* **Avoid initially:** polling Qase directly, having QA write status into engineering repos, and per-RC full-matrix runs.
* **Prerequisite:** decide what artifact a QA run proves it tested (Rancher version, chart version or image).

## 11. Documentation Policy

* Docs that become redundant once an implementation exists are removed. The code carries the documentation through good comments.
* Core docs (`VISION`, `ARCHITECTURE_DESIGN`, this file) hold decisions and design. Facts about a specific repo go in `projects/`, and inspirations in `references/`.

## 12. Open Questions

* Scope of V1: which repos, and what release cadence?
* Reconciler triggering: cron only, or `repository_dispatch` where needed.
* Topology file shape: one file or a few resource files for the central config, how release lines and branches are modeled, and how a single-line, many-consumer repo is expressed (including per-consumer rules such as stable-only or RC-only).
* Exact split of fields between central config and the per-repo config.
* Whether the reconciler reads per-repo config on every run or only central config (section 7).
* Which existing workflows to standardize first, and whether the central repo publishes them as reusable workflows or composite actions.
* Which parts of the charts and Rancher bump logic ship in V1, and whether the CLI replaces the existing "push to X" scripts or wraps them at first.
* How new Rancher lines are cut and registered in the shared map (manual edit, or a command that updates it).
* How downstream repos pin and call reusable workflows; resolve when we review a repo that consumes the existing core branches.
