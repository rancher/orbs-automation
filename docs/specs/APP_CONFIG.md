# Application Config

**Status:** implemented for the loader, defaults and the checks listed under Validation. The kind and field names are not final, and everything in `config/` is a mock until the v0.x POC review ([PHASES](../PHASES.md)).

The shape of the YAML that describes the world `gero` operates on, where it lives, and what validation guarantees. How a single invocation finds it is in [CLI_CONFIG](CLI_CONFIG.md) and [WORKSPACE](WORKSPACE.md). The real files, field reference and defaults table are in [`config/README.md`](../../config/README.md); illustrative mocks are in [`config-mock/`](../../config-mock/) and [`mock-repos/`](../../mock-repos/). The types are `internal/config`, and the loader, defaults and validation are `internal/config/loader`, reachable only through `internal/workspace` ([WORKSPACE](WORKSPACE.md#raw-and-effective-values)).

Informed by [backup-restore-operator](../projects/backup-restore-operator.md) and [rancher-shell](../projects/rancher-shell.md). The backup-restore-operator style (hybrid: reusable workflows, composite actions and rendered templates) is the closer model for what the central repo offers.

## Shape

* Every document has `apiVersion`, `kind`, `name` and `spec`. It is **inspired by Kubernetes resources but not compatible with them**: there is no `metadata`, no `status` and no controller semantics. State lives in tracking issues, not in config.
* Files may hold several YAML documents. Plain block-style YAML throughout (no `{}` or `[]` flow style, except where a value is simply left out and defaulted).
* **Group fields under `spec` by purpose** (`repo`, `release`, `updateMethods`, `downstream`) so intent is obvious and omitted groups can fall back to defaults.
* **Defaults are conservative.** Only no-surprise values are defaulted, and anything that changes release behavior is written out. Stricter release verification (for example `release.requiresPrerelease`) is **off by default and opt-in**, leaving room for more checks later. A change to a default is a breaking change and bumps `apiVersion`. `gero config show --resolved` is a candidate command for seeing the defaults applied.
* One document per resource that has its own identity, owner or location (`Project`, `External`, `WorkflowsConfig`). A list inside one document for a small closed set edited together (`RancherLines`).
* Central file names use a type suffix in a flat directory (`<name>.project.yaml`, `<name>.external.yaml`, `rancher.lines.yaml`). A repo's own config is `gero.yaml`, parsed as real multi-document YAML and dispatched on `kind`.

### Kinds

| Kind | Holds |
|---|---|
| `Project` | A repo we model: `repo`, `release` (branch policy, tag pattern, line-to-branch mapping), `images` (what it publishes, and where, [IMAGES_EXIST](IMAGES_EXIST.md)) `updateMethods` (how it takes an update: what it edits and runs, and what a producer may pass) and `downstream` (the projects it ships to, and with which of their methods) |
| `RancherLines` | The Rancher lines with their lifecycle dates and Kubernetes range. The phase is derived from the dates and the current time, with an optional `phase` override. No branches: each project maps its own branches to line IDs |
| `External` | A third-party project we track (`trackOnly`, `react`) or notify |
| `WorkflowsConfig` | How workflows are rendered into a repo's branches. Lives in the target repo's `gero.yaml`, not centrally |

### Update methods and downstream edges

A bump has two halves, and each is configured by the project that knows it.

**The consumer owns the mechanics, as `updateMethods`.** A method is a named way the project takes an update, so a project with several ways has several methods, and any number of producers can use one. An entry has a fixed set of keys, the same for every strategy:

| Key | Meaning |
|---|---|
| `name` | What a producer's edge refers to |
| `strategy` | Named code in `gero` that reads and writes the value (`yamlBump`); it is how a method's value is found, compared and changed, so it is the same code for reading what the consumer holds now and for writing the new value |
| `with` | Values the consumer fixes for the strategy, such as `file: build.yaml`. A producer cannot pass these |
| `allowedParams` | The interface the method offers a producer: a list of `key`, `description`, `required` and `default`. An entry with a default, or with `required: false`, is optional. The strategy declares the params it consumes, and the method may refine them or add its own, which reach the steps as `GERO_PARAM_<KEY>`. A param that selects what is edited, such as the file, is consumer-only by the strategy's own declaration and cannot be listed here |
| `accept` | What the consumer takes: `when: stable` (the default) or `all` |
| `preEdit`, `postEdit` | Steps the consumer runs around the edit |

**The producer owns the wiring, as `downstream`.** An edge names a consumer (`to`, a project name or slug) and one of its methods, and passes the params the method allows. An edge may also set `lines` (how its releases map to the consumer's branches) and add its own `preEdit` and `postEdit` steps, which may come `from: producer` or `from: central`. The reverse view, "who consumes X" and "what does Y take", is derived from the edges, and it can be compiled into the bundle's generated section ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)).

**Validation at `config validate`.** An edge's `to` is a project and its `method` is one of that project's methods. The params it passes are a subset of the method's `allowedParams`, and every required one without a default is there. `with` and `allowedParams` use only names the strategy knows or the method adds, and they do not overlap. A step's `since` and `until` are lines in `RancherLines`, with `since` not after `until`.

**Steps.** A step is `run` (a program or script path, or an argument list; never a shell string) with optional `since` and `until`, a range of the consumer's lines for the target branch. Every step whose range holds the branch's line runs, in order, and one with no range always does. The order is a chain, outermost last: the method's `preEdit`, then the edge's `preEdit`, the edit, the edge's `postEdit`, then the method's `postEdit`. The consumer's steps surround the producer's, so the consumer's `postEdit`, which regenerates derived files, sees everything the producer's steps did. `from` is only on the edge's steps and says where the program comes from: the consumer's clone (the default and the only choice for a method's steps), the producer's repo at the release being shipped (for an owned project), or the central repo's bundle.

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
* each project-to-project edge is declared once, under the producing project's `downstream`, and names one of the consuming project's `updateMethods`
* each line's `dates` are in order (`ga <= eom <= eol`, and an `eom` needs a `ga`), and a `phase` override, when set, is one of `development`, `ga`, `eom`, `eol`. A project's `release.lines` entry with no `branch` is accepted only while the line's derived phase is `development`, so this check depends on the date it runs
* **Roles belong to relationships, not projects.** A project can be a producer on one edge and a consumer on another (for example a chart repo that consumes an operator and feeds Rancher). Project config has no `role` field; producer and consumer are derived from the edges.
* **One action per pair of projects:** `notify` (outbound) is mutually exclusive with `trackOnly` and `react` (inbound) between the same two projects, since both would imply a circular dependency. This applies to a pair, not to a project as a whole.
* **Edge kinds are explicit.** A `downstream` entry is a bump edge, an `External` `relationships[].action` names `trackOnly`, `react` or `notify`, and a QA test edge is added later, because the QA repo is related to others by something other than a bump.
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
