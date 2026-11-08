# Application Config

**Status:** implemented for the loader, defaults, the update dispatcher's checks and the checks listed under Validation. The kind and field names are not final until the POC review at the end of v0.x ([PHASES](../PHASES.md)). The files in `config/` are the real configs.

The shape of the YAML that describes the world `gero` operates on, where it lives, and what validation guarantees. How a single invocation finds it is in [CLI_CONFIG](CLI_CONFIG.md) and [WORKSPACE](WORKSPACE.md). The real files, field reference and defaults table are in [`config/README.md`](../../config/README.md); illustrative mocks are in [`config-mock/`](../../config-mock/) and [`mock-repos/`](../../mock-repos/). The types are `internal/config`, and the loader, defaults and structural validation are `internal/config/loader`, reachable only through `internal/workspace` ([WORKSPACE](WORKSPACE.md#raw-and-effective-values)). What depends on a strategy is checked by `internal/updatedispatcher`.

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
| `Project` | A repo we model: `repo`, `release` (branch policy, tag pattern, line-to-branch mapping), `images` (what it publishes, and where, [IMAGES_EXIST](IMAGES_EXIST.md)), `updateMethods` (how it takes an update) and `downstream` (the projects it ships to, and with which of their methods) |
| `RancherLines` | The Rancher lines with their lifecycle dates and Kubernetes range. The phase is derived from the dates and the current time, with an optional `phase` override. No branches: each project maps its own branches to line IDs |
| `External` | A third-party project we track (`trackOnly`, `react`) or notify |
| `WorkflowsConfig` | How workflows are rendered into a repo's branches. Lives in the target repo's `gero.yaml`, not centrally |

### Update methods and downstream edges

A bump has two halves, and each is configured by the project that knows it. The consumer knows its own layout, so it owns the **method**. The producer knows what it ships and where, so it owns the **edge**.

**A method is a strategy configured for one consumer.** A project lists `updateMethods`, each a named way it takes an update, and any number of producers can use one. An entry has the same keys whatever its strategy:

| Key | Meaning |
|---|---|
| `name` | What a producer's edge refers to |
| `strategy` | The name of a strategy, which is Go code that claims it (`yamlBump`). The name is defined in the code and nowhere else |
| `with` | Values the consumer fixes for the strategy, such as `file: build.yaml`. The strategy checks them once, when the config loads. A producer cannot pass these |
| `allowedParams` | What a producer's edge may pass: a list of `key`, `description`, `default` and `required`. **This is the only declaration of a param.** A param is required unless it has a `default` or says `required: false`. A strategy reads its inputs by name and has no declaration of them. For `yamlBump` these are `updateKey` and an optional `value`, a template over `{image}`, `{tag}`, `{version}` and `{sourceRepo}` (default `{tag}`), so an edge can write `{image}:{tag}` where a file holds an image reference |
| `accept` | Which releases the consumer takes: `when: stable` (the default) or `all`. A consumer that wants both has two methods |
| `preEdit`, `postEdit` | Steps the consumer runs around the edit |

**One project is the final destination.** A project may set `finalDestination: true`: the project that releases end in, `rancher/rancher`, which the others ship to and which ships to none. At most one project sets it, and it is what a view of how the lines connect starts from ([LINES_RESOLVE](LINES_RESOLVE.md)).

**An edge is a producer's call into a method.** A project lists `downstream` edges. Each names a consumer (`to`, by project name or slug) and one of its methods, and passes the params the method allows. An edge may set `lines` (how its releases map to the consumer's branches; omitted, it is worked out from the producer) and add its own `preEdit` and `postEdit` steps, which may come from the consumer's clone, from the producer's repo (for an owned project) or from the central bundle (`from`). The reverse view, "who consumes X" and "what does Y take", is derived from the edges and can be compiled into the bundle ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)).

**Three stages, three inputs.** A strategy is code. A method **configures** it once with `with`, and that gives an updater that is checked early, by `config validate`. An edge **invokes** it with params, which become the inputs of each call. The strategy never sees how a param was declared, only the values, with defaults filled in. The updater has a read side that never writes (used by the state check) and a write side ([PUSH_TO_REPO](PUSH_TO_REPO.md#applying-a-bump)).

**Steps.** A step is `run` (a program or script path, or an argument list; never a shell string) with optional `since` and `until`, a range of the consumer's lines for the target branch. Only steps whose range holds the branch's line run, and one with no range always does. The order is a chain with the consumer outermost: the method's `preEdit`, then the edge's, the edit, the edge's `postEdit`, then the method's. The consumer's `postEdit`, which regenerates derived files, sees everything a producer's steps did.

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

`gero config validate` runs without credentials and reports every problem at once, each naming the document and field. Validation has two parts, and a config is valid when both pass.

**Structure, in the config loader.**

* dangling project references, and that each `release.lines` entry names a known Rancher line
* each project-to-project edge is declared once, under the producing project's `downstream`, and names a consumer that exists and one of its `updateMethods`
* a method has a name and a strategy, its params have unique keys, and a key is not both fixed under `with` and offered under `allowedParams`
* a step has a command, its `since` and `until` are lines in `RancherLines` with `since` not after `until`, and `from` is only on an edge's step (the producer's own files only for an owned project)
* an edge's `lines` is `all` or `universal` when set
* `repo.managedBy`, when set, is a lowercase slug; `release.branching` is `single`, `perLine` or `perLineTag`; a `perLineTag` project lists at least one `release.lines` entry, each with only its `line`; `release.tagScheme` is a known scheme, and a scheme other than `semver` is only for `perLineTag`; and an edge's `stage`, when set, is on an edge from an unowned project and is a stage its producer's scheme gives ([CHAIN_TRACING](CHAIN_TRACING.md))
* each line's `dates` are in order (`ga <= eom <= eol`, and an `eom` needs a `ga`), and a `phase` override, when set, is one of `development`, `ga`, `eom`, `eol`. A project's `release.lines` entry with no `branch` is accepted only while the line's derived phase is `development`, so this check depends on the date it runs
* a `gero.yaml` in a repo contains only allowed kinds
* repo slugs are unique

**What depends on a strategy, in the update dispatcher.** Building its engine from the loaded config finds a method whose strategy does not exist, `with` values the strategy rejects (a typo, a missing file), a method that does not offer an input its strategy requires, and an edge whose params do not fit the method (an unknown param, or a required one left out). It is the same construction a run uses, so `config validate` and `gero ship` agree; `gero doctor` does not run it yet.

* **Roles belong to relationships, not projects.** A project can be a producer on one edge and a consumer on another (for example a chart repo that consumes an operator and feeds Rancher). Project config has no `role` field; producer and consumer are derived from the edges.
* **One action per pair of projects:** `notify` (outbound) is mutually exclusive with `trackOnly` and `react` (inbound) between the same two projects, since both would imply a circular dependency. This applies to a pair, not to a project as a whole.
* **Edge kinds are explicit.** A `downstream` entry is a bump edge, an `External` `relationships[].action` names `trackOnly`, `react` or `notify`, and a QA test edge is added later, because the QA repo is related to others by something other than a bump.

**No hard limit on graph shape beyond the pair rule.** Real workflows are complex, and artificial limits (such as banning every cycle through three or more projects) could block legitimate setups. Cycles across several projects are warnings at most. Graph traversal must be cycle-safe regardless (track visited projects).

## Not built yet

* Validation of a repo's `gero.yaml` (the allowed-kinds check) and of `WorkflowsConfig`.
* Cycle warnings across several projects.
* `gero config show --resolved`.
* `gero doctor` checking the dispatcher's findings as `config validate` does.
* The `chartsPackageBump` strategy, which the mock configs name.

## Open questions

* Final kind and field names.
* Whether any config other than `WorkflowsConfig` ever moves into target repos.
* How `yamlBump` finds its key: today the fixed input name `updateKey`.
