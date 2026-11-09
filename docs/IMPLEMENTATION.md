# Implementation Notes

What the tool does, and the decisions, leanings and open questions behind it. How the CLI itself is built (framework, config, output, logging, exit codes, dependencies, GitHub access) is in the [specs](specs/README.md). Evidence about specific repos lives in [`projects/`](projects/README.md) and [`references/`](references/frameworks-automation.md), and this doc links to it instead of repeating it.

Unmarked items are decisions or working agreements. **(leaning)** is a proposal not yet agreed. **(unverified)** was decided from memory and needs confirmation (for example a small spike) before we rely on it.

## CLI Packaging

* The binary is named **`gero`** (Dr. Gero). It does not need to match the repo name.
* **Standalone only, using the GitHub APIs directly** ([GITHUB_ACCESS](specs/GITHUB_ACCESS.md)). `gh` is not a hard dependency: it is only needed for the keyring token fallback and interactive or passthrough steps.
* There is no published `gh` extension, so the central repo keeps its name and does not need a `gh-` prefix. A developer who wants `gh gero` can install the binary as an extension locally (for example `gh extension install .` from a checkout); that is a personal convenience and is not supported or documented as an install path.
* Name collisions for the `gero` binary on `PATH` are assumed not to be a problem (not a common binary name, and none expected on the team).

## Execution Contexts

* **CI-only commands** (for example the reconciler) require a GHA environment and the GHA-provided token, and fail fast with a clear error otherwise (check `GITHUB_ACTIONS=true` and that a token is present). A command declares its context once, as an annotation enforced centrally ([CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md)).
* **Either-context commands** work on a developer workstation or in CI, with auth resolved from the environment.
* Reconcile only ever runs in CI.

## GitHub and Git Access

`gero` uses `go-gh` (token and host only) and `go-github` (typed REST) directly. It runs the `git` binary for anything that touches a working tree, so an engineer's own SSH agent, credentials and signing apply, and reads refs and tags with `go-git`. A go-git client for the rest could replace the binary later. The details and the reconciler's consumer-side interfaces are in [GITHUB_ACCESS](specs/GITHUB_ACCESS.md).

## Release Rules

* A stream's `requiresPrerelease` means a stable tag must come from a commit that already has an RC. It is a safety check that some repos do not need at all, so it is off unless a project opts in. `gero repo tag` enforces it for the tags it makes, and `gero repo status` shows what it allows; enforcing it for tags made by hand or by a workflow comes later (see [PHASES](PHASES.md)).
* Even when the rule is on, it can be **overridden per release** (for example a single-commit CVE dependency bump that needs a new patch, not an RC retest).
  * Mechanism: an explicit CLI flag or issue label, with the reason recorded in the tracking issue. **(leaning)** A flag follows the conventions in [CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md).
* **Tag classification** (semver, prerelease, build metadata) is one shared library function, `internal/platform/semver` ([CHECK_SEMVER](specs/CHECK_SEMVER.md)). Existing repos reimplement it in shell (see [projects/](projects/README.md)).
* **Tag creation works both ways.** A human can tag and push in the target repo and the tracker is notified (for example via `repository_dispatch`), or a workflow run from the tracker creates the requested tag in the target repo. The reconciler observes either path. The dispatch path is added as needed.
* Today humans create tags with git and verify QA sign-off before creating a stable tag. The tool fits that process first.

## Versioning

* Actions are **sub-versioned by the CLI**: one version and one tag stream. An actions-only fix may produce a no-op binary release. Separate `actions/vX.Y.Z` tags can be added later if this proves painful.
* Rendered stubs record when, and from which template and CLI version, they were generated, so drift can be detected.
* A stub rendered by CLI version X pins workflows compatible with X.
* Shared workflows are pinned by CLI version tag or SHA, never a floating branch ref, so a bad push cannot hit every branch at once.

## Tracking Issue State

Informed by [frameworks-automation](references/frameworks-automation.md) (an inspiration, not a template).

* Machine state lives in a **versioned hidden block** in the issue body (for example an HTML comment holding YAML). The visible markdown is rendered from it.
* The block is additive-only so older runs can read newer state. A missing or corrupt block is a loud error, never silently treated as empty state.
* Only the block and generated sections are rewritten. Text outside them is left alone.
* **Checkboxes are read-only.** They display state and are never parsed. Any user interaction must feel obvious (explicit commands, labels, or workflow dispatch), not through editing the issue body. Humans are not expected to edit tracking issues.
* Issue identity does not rely on the title. The version and other identifying fields are stored in the block. Labels are for lookup and filtering.
* **Stale state** is marked with labels, using a threshold on the last reconciled time.
* **Locking is per operation**, not a global lock across all workflows.
* Idempotency uses deterministic branch names and reuses an existing open PR.
* Cron minutes are staggered off the round hour. Actions are pinned by SHA.
* Listing calls (releases, PRs) must paginate. Decide up front how prereleases are treated, since `/releases/latest` excludes them.
* Render everything from one state object, with a zero- vs one-based stage convention documented in code, so rendering cannot contradict state.
* The reconciler is tested with fixture-driven cases (state in, expected state out), and config validation runs without credentials.
* **Audience:** engineers, QA and managers (the future GUI uses the same state), so each issue must be understandable on its own.

## Config

The shape of the config, where it lives and what validation checks are in [APP_CONFIG](specs/APP_CONFIG.md). The product decisions it expresses are in the next section.

## Repo Shapes and Downstream Bumps

Evidence: [scc-operator](projects/scc-operator.md), [rancher-charts](projects/rancher-charts.md), [rancher-rancher](projects/rancher-rancher.md), [kuberlr-kubectl and the other projects of the chain](../config/) and the cross-project table in [projects/README](projects/README.md).

* **The consumer owns the mechanics, the producer owns the wiring.** A consumer lists its **holdings**: where each piece of a producer's release lives in its repo (a file, a key, a format) and what form it is written in, with the **edits** that run around the write, and the producer lists the **artifacts** its releases make, which the holdings refer to. The consumer knows its own layout and conventions, so a layout change touches the consumer only, and a holding that many producers could use is written once. Roles (producer/consumer) are per holding, and the reverse view ("who holds X") is derived.
* **Every project is tied to a Rancher line for our team's repos**, except "universal" projects (for example `ob-team-charts`, `scc-operator`) that only become line-specific when they land in a branch of a line-specific repo such as `rancher/charts`. These give their stream `universal: true` and list no lines.
* **Release branches are optional.** A repo declares its branch layout (`single` or `perLine`), and the tool must not assume `release/vX.Y`.
* **A repo's version line is separate from its consumers' branches.** Producers do not carry downstream branch names.
* **One shared line map.** The mapping from a line to each repo's branch was encoded in many places with different logic. The `RancherLines` document holds line facts only (lifecycle dates, Kubernetes range), with no branches. The dates (GA, EOM, EOL) are copied from the [SUSE lifecycle page](https://www.suse.com/lifecycle/#suse-rancher-prime). Phase is derived from them and the current UTC time, never stored: `development` (no GA date yet, or GA is in the future), `ga`, `eom`, `eol`, each beginning at 00:00 UTC on its date. An optional `phase` on a line overrides the dates. The page is the source of truth; a `gero lines sync` command (planned) updates the dates from it, and `gero doctor` will flag drift. Each project maps its own series (in its stream) and branches (in `branches.lines`) to line IDs, including `rancher/rancher` itself.
  * The development line comes from a repo's default branch. A branch cut moves `main` to the next line. A `branches.lines` entry with no `branch` resolves to the default branch only while the line's derived phase is `development`; any other missing branch is an error, never a silent skip.
  * A holding maps, per Rancher line, the producer's release for that line to the consumer's branch for that line. A producer whose stream is `universal` goes to every release branch of the consumer.
* **A downstream bump has four parts:**
  1. **Target:** the consumer and the branch each release goes to, from the line map. A route is a producer and a consumer it has release holdings in, and one run ships down one route, so every holding of a release is written together, in one pull request for each target branch.
  2. **Branch resolution:** (producer tag, producer line, holding stage) to the consumer's branch, from the line map.
  3. **Write:** each holding renders the release in its form and replaces the value at its file and key, in place. A rule that needs code (chart version arithmetic, index lookups) is a named strategy of an Edit, claimed by name in one registry; there is one implementation for each, replacing the divergent per-repo copies. Names, keys, labels and title templates are config.
  4. **Steps around the write:** an Edit's ordered `preEdit` and `postEdit` lists, run as argument lists (never a shell string), each limited to the lines it applies to.
* **PR conventions** come from a per-consumer `pullRequests` block (title, branch prefix, labels) plus a stable key for reusing an open PR. Use deterministic branch names, not epoch suffixes. Producers are standardized on the consumer's own conventions (for example the `rancher/charts` PR title format).
* `rancher/charts`' own `auto-bump` workflows do not fit most of our use cases and are out of scope.
* **Preconditions and fan-out.** Check that the image or release asset exists and that the target branch is not frozen before opening a PR. A fan-out to several branches reports per-target state and tolerates partial failure.
* **Whether the tracker builds the full chain up front or reads hop by hop** depends on the question being asked. It is an implementation choice driven by requirements and performance.
* **Avoid:** `eval` of commands read from a downstream repo, tag triggers of `*` with no format check, fork guards scattered through workflows, and undefined variables feeding checkouts.

## QA Handshake (v3, undecided)

Evidence: [observability-e2e](projects/observability-e2e.md). Nothing here is decided.

* **Leaning (to agree with the QA owners):** dispatch a thin QA workflow with a correlation ID, milestone and Rancher version, observe the run's conclusion, and record it on the tracking issue as evidence. The stable-tag gate stays a human decision.
* **Avoid initially:** polling Qase directly, having QA write status into engineering repos, and per-RC full-matrix runs.
* **Prerequisite:** decide what artifact a QA run proves it tested (Rancher version, chart version or image).

## Standards for Target Repos

Conventions the tool wants repos to follow, so automation does not fight them.

* **Helm chart unit tests and dynamic values.** Tests for a value that automation bumps (for example a default image tag) should set the input explicitly and assert the same value, not depend on the chart's default. That way the value can change without breaking tests, and the tests still validate the template mechanics. For example, `backup-restore-operator`'s `kubectl-image_test.yaml` asserts the default `kuberlr-kubectl` tag, so a default bump would break it. The tool does not edit test files as part of a bump.
* **`VERSION.md` is the standard place for branch and version mapping, and it is generated from config.** Repos document their line mapping today in different places and formats (a `VERSION.md`, a README table). The standard is a `VERSION.md` in each owned repo, generated by `gero` from the project's `branches.lines` and the `RancherLines` facts (for example the Kubernetes range), and kept in sync on the default branch with a PR, like rendered workflows. It carries a generated header, and `verify` detects drift. This also moves toward automating today's manual "branch for X" workflow. Release branches keep whatever copy they had when cut.

## Open Questions

* Scope and order of the milestones: see [PHASES](PHASES.md).
* Config-shape questions (final names, whether config moves into target repos): see [APP_CONFIG](specs/APP_CONFIG.md). CLI-layer questions are in the spec each belongs to ([CLI_FRAMEWORK](specs/CLI_FRAMEWORK.md), [DEPENDENCY_POLICY](specs/DEPENDENCY_POLICY.md)).
* Reconciler triggering: cron only, or `repository_dispatch` where needed.
* Which existing workflows to standardize first, and whether the central repo publishes them as reusable workflows or composite actions.
* Whether the CLI replaces the existing "push to X" scripts or wraps them at first (decided in v0.x, see [PHASES](PHASES.md)).
* How new Rancher lines are cut and registered in the line map (manual edit, or a command that updates it). Updating the dates of lines already in the map is planned as `gero lines sync`; a new line also needs a Kubernetes range, which the lifecycle page does not give.
* How downstream repos pin and call reusable workflows; resolve when we review a repo that consumes the existing core branches.
* The `VERSION.md` format: one standard layout across repos, which fields it shows (branch, tag, Rancher line, Kubernetes range, EOL section), and whether other tools that read `VERSION.md` today (for example frameworks-automation reads per-repo `VERSION.md` at runtime) need the old formats to keep working.
* The "branch for X" workflow itself: which steps `gero` automates (create branches, update `branches.lines`, regenerate `VERSION.md`, re-render workflows) and in what order.
