# Application Config

**Status:** implemented for the loader, defaults and the checks listed under Validation. The kind and field names are not final, and everything in `config/` is a mock until the v0.x POC review ([PHASES](../PHASES.md)).

The shape of the YAML that describes the world `gero` operates on, where it lives, and what validation guarantees. How a single invocation finds it is in [CLI_CONFIG](CLI_CONFIG.md) and [WORKSPACE](WORKSPACE.md). The real files, field reference and defaults table are in [`config/README.md`](../../config/README.md); illustrative mocks are in [`config-mock/`](../../config-mock/) and [`mock-repos/`](../../mock-repos/). The types are `internal/config`, and the loader, defaults and validation are `internal/config/loader`, reachable only through `internal/workspace` ([WORKSPACE](WORKSPACE.md#raw-and-effective-values)).

Informed by [backup-restore-operator](../projects/backup-restore-operator.md) and [rancher-shell](../projects/rancher-shell.md). The backup-restore-operator style (hybrid: reusable workflows, composite actions and rendered templates) is the closer model for what the central repo offers.

## Shape

* Every document has `apiVersion`, `kind`, `name` and `spec`. It is **inspired by Kubernetes resources but not compatible with them**: there is no `metadata`, no `status` and no controller semantics. State lives in tracking issues, not in config.
* Files may hold several YAML documents. Plain block-style YAML throughout (no `{}` or `[]` flow style, except where a value is simply left out and defaulted).
* **Group fields under `spec` by purpose** (`repo`, `release`, `sources[].accept`, `sources[].update`) so intent is obvious and omitted groups can fall back to defaults.
* **Defaults are conservative.** Only no-surprise values are defaulted, and anything that changes release behavior is written out. Stricter release verification (for example `release.requiresPrerelease`) is **off by default and opt-in**, leaving room for more checks later. A change to a default is a breaking change and bumps `apiVersion`. `gero config show --resolved` is a candidate command for seeing the defaults applied.
* One document per resource that has its own identity, owner or location (`Project`, `External`, `WorkflowsConfig`). A list inside one document for a small closed set edited together (`RancherLines`).
* Central file names use a type suffix in a flat directory (`<name>.project.yaml`, `<name>.external.yaml`, `rancher.lines.yaml`). A repo's own config is `gero.yaml`, parsed as real multi-document YAML and dispatched on `kind`.

### Kinds

| Kind | Holds |
|---|---|
| `Project` | A repo we model: `repo`, `release` (branch policy, tag pattern, line-to-branch mapping) and `sources` (what it takes updates from, with `accept` and `update`) |
| `RancherLines` | The Rancher lines with their lifecycle dates and Kubernetes range. The phase is derived from the dates and the current time, with an optional `phase` override. No branches: each project maps its own branches to line IDs |
| `External` | A third-party project we track (`trackOnly`, `react`) or notify |
| `WorkflowsConfig` | How workflows are rendered into a repo's branches. Lives in the target repo's `gero.yaml`, not centrally |

Repo shapes, branch policy, line mapping and the bump model these fields express are product decisions, in [IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps).

## Placement

* **Release workflows are rendered into the real branch.** Keyless signing and attestation depend on the OIDC token's workflow source, so the release workflow has to live on the tagged branch itself. (Confirmed by team experience from debugging.)
* **Central repo:** the project list, topology, where things flow between repos, the Rancher line facts, and the configs of projects we do not own (`rancher/charts`, `rancher/rancher`).
* **Target repo:** `gero.yaml` holds **only `WorkflowsConfig`** for now (toolchain, test matrix, enabled workflows, `gero` version pin). Any other kind in a repo's `gero.yaml` is an error until we decide to allow more. Moving owned `Project` config into repos is a possible later step.
* **Per-repo lookup:** the repo's `automation-core` branch if it exists, otherwise the default branch.
* **The reconciler reads central config**, and fetches a repo's `gero.yaml` only when it is about to act on that repo. Facts needed just to observe (tag patterns, whether stable requires an RC) stay central.
* **Standardize shared workflows across repos** where inputs can cover the differences (for example CI test workflows as published reusable workflows or actions in the central repo). Workflows still hand-copied per branch today (backport, stale, renovate, unrc) are candidates later.
* **Rendered files carry a generated header** with template and CLI version and a content hash. A `verify` command detects drift, and a dry-run diff is shown before any deploy PR is opened.
* **Avoid:** runtime fetching of config from another branch, branch-to-branch maps hardcoded in shell scripts, and per-branch copies of workflows that could be shared.

## Validation

`gero config validate` runs without credentials and reports every problem at once, each naming the document and field. It checks:

* dangling project references, and that each `release.lines` entry names a known Rancher line
* each project-to-project edge is declared once, under the consuming project's `sources`
* each line's `dates` are in order (`ga <= eom <= eol`, and an `eom` needs a `ga`), and a `phase` override, when set, is one of `development`, `ga`, `eom`, `eol`. A project's `release.lines` entry with no `branch` is accepted only while the line's derived phase is `development`, so this check depends on the date it runs
* **Roles belong to relationships, not projects.** A project can be a producer on one edge and a consumer on another (for example a chart repo that consumes an operator and feeds Rancher). Project config has no `role` field; producer and consumer are derived from the edges.
* **One action per pair of projects:** `notify` (outbound) is mutually exclusive with `trackOnly` and `react` (inbound) between the same two projects, since both would imply a circular dependency. This applies to a pair, not to a project as a whole.
* **Edge kinds are explicit.** A `sources` entry is a bump edge, an `External` `relationships[].action` names `trackOnly`, `react` or `notify`, and a QA test edge is added later, because the QA repo is related to others by something other than a bump.
* a `gero.yaml` in a repo contains only allowed kinds
* repo slugs are unique

**No hard limit on graph shape beyond the pair rule.** Real workflows are complex, and artificial limits (such as banning every cycle through three or more projects) could block legitimate setups. Cycles across several projects are warnings at most. Graph traversal must be cycle-safe regardless (track visited projects).

## Not built yet

* Validation of a repo's `gero.yaml` (the allowed-kinds check) and of `WorkflowsConfig`.
* Cycle warnings across several projects.
* `gero config show --resolved`.

## Open questions

* Final kind and field names.
* Whether any config other than `WorkflowsConfig` ever moves into target repos.
