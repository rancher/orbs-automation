# Application Config

**Status:** implemented: the provider, the graph, the update engine and the checks under Validation. The kind and field names are not final until the POC review at the end of v0.x ([PHASES](../PHASES.md)). The files in `config/` are the real configs.

The shape of the YAML that describes the world `gero` operates on, where it lives, and what validation guarantees. How a single invocation finds it is in [CLI_CONFIG](CLI_CONFIG.md) and [WORKSPACE](WORKSPACE.md). The field reference is [CONFIG_SCHEMA](../design/CONFIG_SCHEMA.md), the reasons for the kinds are in [CONFIG_UNITS](../design/CONFIG_UNITS.md), and the real files and defaults are in [`config/README.md`](../../config/README.md); illustrative mocks are in [`mock-repos/`](../../mock-repos/). The types and their rules are `internal/apis/v1alpha2`, the provider that reads a directory is `internal/appconfig`, the graph commands read is `internal/chain` ([CHAIN_MODEL](../design/CHAIN_MODEL.md)), and the engine that ships into the holdings is `internal/bump` ([UPDATE_ENGINE](../design/UPDATE_ENGINE.md)). Config is reachable only through `internal/workspace`.

Informed by [backup-restore-operator](../projects/backup-restore-operator.md) and [rancher-shell](../projects/rancher-shell.md). The backup-restore-operator style (hybrid: reusable workflows, composite actions and rendered templates) is the closer model for what the central repo offers.

## Shape

* Every document has `apiVersion` (`gero/v1alpha2`), `kind`, `name` and `spec`, and may have `refs`. It is **inspired by Kubernetes resources but not compatible with them**: there is no `metadata`, no `status` and no controller semantics. State lives in tracking issues, not in config.
* `name` is a lowercase slug, unique within its kind, and is how logs, errors and `refs` name the document. `refs` holds names only, and the key says which kind each name is of.
* Files may hold several YAML documents, and **nothing depends on which file a document is in** or its place in it.
* **Defaults are conservative.** Only no-surprise values are defaulted, and anything that changes release behavior is written out. Stricter release verification (`requiresPrerelease`) is **off by default and opt-in**. A change to a default is a breaking change and bumps `apiVersion`.
* A repo's own config is `gero.yaml`, parsed as real multi-document YAML and dispatched on `kind`.

### Kinds

| Kind | Holds |
|---|---|
| `Project` | A repo as a place: `repo` (who owns it, whether it is outside the organization), `pullRequests`, and `branches` (the layout, the stages, and the branch of each Rancher line) |
| `Stream` | What a project releases: the scheme and tag pattern, the stages, the lines and the version series of each, and whether any release serves every line |
| `Artifact` | One thing each release of a stream produces: an image (checked in its registries, [IMAGES_EXIST](IMAGES_EXIST.md)) or a file of the GitHub release |
| `Holding` | Where a piece of a producer's release lives in a repo, and how it is written: a file, a key and a format, and a release, a commit or a branch |
| `Edit` | Extra steps when shipping, around the write of the holdings, and a named rule that needs code |
| `RancherLines` | The Rancher lines with their lifecycle dates and Kubernetes range. The phase is derived from the dates and the current time, with an optional `phase` override. No branches: each project maps its own to line ids |
| `WorkflowsConfig` | How workflows are rendered into a repo's branches. Lives in the target repo's `gero.yaml`, not centrally |

There is no `External` kind: a third-party repo is a Project with `repo.external: true`, which gero never ships into.

### Holdings and edits

A bump has two halves, and each is configured by the project that knows it. The consumer knows its own layout, so it owns the **Holding**: the file and key it keeps a producer's release in, and the form it is written in (`{image}:{tag}`, a URL, a bare tag). The producer knows what it makes, so it owns the **Artifact** the holding refers to.

* A **holding** has `at` (a file and a key, both templates over `{line}`, `{stage}` and `{branch}`, and a format, `yaml` today) and `holds` (a `kind` of release, commit or branch, the `form`, the `lines` it is for, a `stage`, and `accept`: `stable`, the default, or `all`). A consumer that wants both has two holdings.
* **A route** is a producer and a consumer that holds releases of it. One run of `gero ship` writes every release holding of the route, in one pull request for each target branch ([UPDATE_ENGINE](../design/UPDATE_ENGINE.md)), so a consumer that holds two files of one release is never left half updated. The reverse view, "who holds X" and "what does Y take", is derived from the holdings and can be compiled into the bundle ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)).
* **An edit** follows one or more holdings. It has `preEdit` and `postEdit` steps (`run` is a program or script path, or an argument list; never a shell string) with optional `since` and `until`, a range of the consumer's lines for the target branch, and `from`, where the program comes from (the consumer's clone, the producer's repo for an owned project, or the central bundle). It may name a `strategy`, a rule in code with its own `with` values. A strategy is checked when the config loads, so `config validate` and `gero ship` agree; `chartsPackageBump` is named in the config and not built, and a run that needs it stops before anything is written.
* The order of a run is the edits' `preEdit` steps, the write of every holding, the strategies, and the edits' `postEdit` steps. Only steps whose range holds the branch's line run, and one with no range always does.

**One project is the final destination.** A project may set `finalDestination: true`: the project that releases end in, `rancher/rancher`, which the others ship to and which ships to none. At most one project sets it, and it is what a view of how the lines connect starts from ([LINES_RESOLVE](LINES_RESOLVE.md)).

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

`gero config validate` runs without credentials and reports every problem at once, each naming the file, the document and the field, and a line for each warning. Validation has three parts, and a config is valid when the first two pass.

**One document alone, in `apis/v1alpha2`.** The envelope (a supported `apiVersion`, a known `kind`, a name that is a slug), strict fields, and each kind's own rules: a repo that is `owner/name` with no space and not both owned and external; pull request texts that use only the variables they may; a layout, stages and branch forms that agree (a line with a branch for each stage needs `branches.stages`, and at most one line is on the default branch); a stream with a known scheme, tag patterns that are globs, series in the grammar of the scheme that do not overlap, and `lines` or `universal`; an artifact that is an image or a file with a name of the right kind; a holding with a file and a key that use only the variables they may, a listed format, a form that uses only modifiers that can be undone, stages only where they mean something, and `since` not after `until`; an edit that does something, with steps that have a program, a range that is in order and a place the program comes from; and `RancherLines` with ids that are lines and listed once, dates in order, a known `phase` and a Kubernetes range.

**The set, in `ValidateSet`.** Names are unique within a kind; every ref is given, is one the kind has, takes one name or a list as the kind says, and names a document of the right kind; repo slugs are unique whatever their case and there is one `finalDestination`; image names are unique within a project; there is one `RancherLines` document and every line a document names is in it; a calver stream is on a project with the `single` layout; a holding of a release refs an artifact and one of a commit or branch refs the project it is taken from, and a project holds nothing of itself; a holding's stage is a stage of its holder and its stages are stages its producer's streams have; and a step run from the producer has a producer that is owned.

**Warnings, never errors.** A lifecycle date ([CONSTRAINTS](../design/CONSTRAINTS.md)), a line with no `branch` once it is past development (an omitted branch means the default branch, which is for the line in development), and two projects that hold something of each other (the dependency is circular).

**What depends on a strategy or a format, in the update engine.** Building the engine finds an edit whose strategy does not exist or whose `with` it rejects, and a release holding in a format gero cannot write yet. It is the same construction a run uses; `gero doctor` does not run it yet.

* **Roles belong to relationships, not projects.** A project can be a producer on one route and a consumer on another (for example a chart repo that consumes an operator and feeds Rancher). Project config has no `role` field; producer and consumer are derived from the holdings.
* **Holdings are explicit.** A release holding is written by a run, and a commit or branch holding is only observed. A QA test edge is added later, because the QA repo is related to others by something other than a holding.

**No hard limit on graph shape.** Real workflows are complex, and artificial limits (such as banning every cycle) could block legitimate setups. A circular dependency between two projects is a warning. Graph traversal must be cycle-safe regardless (track visited projects).

## JSON Schemas

The kinds are described as JSON Schema (draft 2020-12) so an editor can validate and complete a config file. They are in [`generated/jsonschema/`](../../generated/jsonschema/): one for each kind, in `v1alpha2/`, and `gero.v1alpha2.schema.json`, which holds them all and picks the one for a document by its `kind`, so it fits a file of several documents. The schema of the user file ([CLI_CONFIG](CLI_CONFIG.md)) is `user-config.schema.json`.

* They are derived from the Go types by `tools/genschema`, which reads the same `yaml` tags the decoder does, so a key and a nesting cannot differ. Descriptions are the Go comments. The allowed values of a field are built from the constants the rules check, in the tool's `hints.go`.
* A schema checks the shape of one document: its keys, types, allowed values and the envelope. It does not check what only the set can, such as a ref that resolves or a unique name, nor the rules that depend on other fields; `gero config validate` does.
* Regenerate with `go run ./tools/genschema` from the repository root. A test fails when the committed files are stale.
* To use one, point the editor's YAML support at the file. The files in `config/` start with a comment the YAML language server reads, for the combined schema on the `main` branch:

  ```yaml
  # yaml-language-server: $schema=https://raw.githubusercontent.com/rancher/orbs-automation/main/generated/jsonschema/gero.v1alpha2.schema.json
  ```

  A user file takes `user-config.schema.json` the same way. Other editors are wired in their own settings, and whoever wants that does it.

## Not built yet

* A schema for `WorkflowsConfig`, which has no Go type yet.
* Validation of a repo's `gero.yaml` (the allowed-kinds check) and of `WorkflowsConfig`.
* Cycle warnings across more than two projects.
* `gero config show --resolved`.
* `gero doctor` checking the engine's findings as `config validate` does not yet either.
* The `chartsPackageBump` strategy, which the real config names, and the `dockerfile` format.

## Open questions

* Final kind and field names.
* Whether any config other than `WorkflowsConfig` ever moves into target repos.
