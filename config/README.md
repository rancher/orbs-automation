# Config

The configs `gero` runs on. Shapes are `gero/v1alpha1` and may still change before v1.0.0. Illustrative configs for projects we do not model yet live in [`../config-mock/`](../config-mock/).

Every file is YAML with `apiVersion`, `kind`, `name` and `spec`, inspired by Kubernetes resources but not compatible with them (no `metadata` or `status`). Files may hold several documents.

| Kind | Meaning | One document per |
|---|---|---|
| `Project` | A repo we model: identity, release policy, and what it takes updates from | project |
| `RancherLines` | The set of Rancher lines and facts about each | ecosystem (single doc, list inside) |
| `External` | A third-party project we track or notify | external project |
| `WorkflowsConfig` | How workflows are rendered into a repo's branches (lives in the repo) | repo |

File names in this repo: `<name>.project.yaml`, `<name>.external.yaml`, `rancher.lines.yaml`. In a target repo: `gero.yaml`.

## Line phases

`RancherLines` records each line's lifecycle dates under `dates` (`ga`, `eom`, `eol`, as `YYYY-MM-DD`), copied from the [SUSE lifecycle page](https://www.suse.com/lifecycle/#suse-rancher-prime). The phase is not stored: it is worked out from the dates and the current time, and each phase begins at 00:00 UTC on its date. A line with no `ga` date, or one in the future, is in `development`. The dates must be in order (`ga <= eom <= eol`).

An optional `phase` on a line overrides the dates, for a line the page does not cover or to force a phase early.

| Phase | Meaning |
|---|---|
| `development` | Before GA. Served from a repo's default branch. |
| `ga` | Generally available, fully supported. |
| `eom` | End of maintenance reached: still supported, but only for fixes. |
| `eol` | End of life. Remove the line from config. |

## Defaults (applied when a field is omitted)

Only conservative or no-surprise defaults. Anything that changes release behavior must be written out.

| Field | Default |
|---|---|
| `finalDestination` | `false`; only `rancher/rancher` sets it |
| `repo.owned` | `false`: `gero` does not ship from the repository |
| `repo.managedBy` | `orbs` for an owned project; none for an unowned one, which is shown as unspecified |
| `repo.defaultBranch` | `main` |
| `release.branching` | `single` |
| `release.tagPattern` | `"v*"` |
| `release.tagScheme` | `semver`; `perLineTag` must set another, and no other branching may |
| `release.requiresPrerelease` | off (opt-in) |
| `release.lines` | `universal` |
| `images` | one image named `repo.name`, in every registry (`dockerhub`, `prime-staging`, `prime-production`) |
| `images[].registries` | every registry |
| `images[].since` | none: the image is expected for every line |
| `updateMethods[].accept.when` | `stable` |
| `downstream[].stage` | none: an edge from an unowned project follows whatever stage the consumer holds |
| `downstream[].lines` | `universal` when the producer lists no `release.lines`, else `all` (worked out when read, not filled in) |
| `updateMethods[].preEdit`, `postEdit` | none |
| `updateMethods[].allowedParams[].required` | true, unless the entry has a `default` |
| `pullRequests.labels` | none |
| `pullRequests.pushTo` | none: `fork` or `upstream`, for a repository that is only ever pushed to one way; the flag and the user's settings come first |
| `pullRequests.botLabels` | none: applied only to a PR the bot opens, such as `origin/bot` |

`release.requiresPrerelease` is off unless a project opts in. Stricter release verification is opt-in, which also leaves room for more checks later. `gero repo tag` and `gero repo status` read it: with it set a stable tag follows a stable one only through a prerelease, and a stable tag is only made on a commit that already has a prerelease of its version ([TASK_COMMANDS](../docs/specs/TASK_COMMANDS.md#gero-repo-tag-project-line-tag)). `gero ship` does not use it, and picking the next tag from a workflow is not built.

`finalDestination: true` marks the one project that releases end in, `rancher/rancher`: every other project ships to it, directly or through another project, and it ships to none. Only that project sets it and at most one project may, which the loader checks. It lets a view such as `gero lines resolve` start from the end of the line without knowing a project by name, and that view shows nothing as shipping on from it.

`release.branching` is the layout of the project's own release branches, not a list of them: `single` keeps every release on the default branch, whatever lines it lists, `perLine` keeps each Rancher line on the branch its `release.lines` entry names (the default branch for a line with none), and `perLineTag` keeps every release on the default branch and finds the lines in the tags (below). It is separate from `release.lines`, which says which version series goes with which Rancher line.

`repo.owned` and `repo.managedBy` work together. `owned` says whether `gero` ships from the repository: it is true when the repository's workflows run `gero`. `managedBy` says which team is responsible for the project, as a lowercase slug (`orbs`, `release-team`). An owned project with no `managedBy` is `orbs`'s; an unowned project with none is shown as unspecified, and never assumed to be `orbs`'s. Every edge from an unowned project is observed: `gero` reads where it stands and `gero ship` never opens a PR for it. An edge into an unowned project is written as usual. To adopt `gero` for a repository another team manages, set `owned: true` and change its workflows to call `gero`; `managedBy` keeps naming their team ([CHAIN_TRACING](../docs/specs/CHAIN_TRACING.md)).

`release.branching: perLineTag` is for a project whose releases are on one branch and whose Rancher lines are only in its tags, such as `rancher/rancher-assets`. It lists its `release.lines`, each with only its `line` (no `branch` or `version`, which the loader refuses): the lines it is used for, such as `v2.16` for Assets, and the edge to a consumer reaches only the lines both list. `release.tagScheme` names how those tags spell a line (`semver` by default, or `calverLineStage` for `v2.16-20261030T1111Z` and `v2.16-20261030T1111Z-dev`). A `downstream` edge's `stage` is optional and only for an edge from an unowned project: it names a stage the producer's scheme gives (`dev` or `stable` for `calverLineStage`), to follow only that stream.

Documents use `apiVersion: gero/v1alpha1` until v1.0.0 of `gero`, when the shapes are agreed and become `gero/v1`. A change to a default after that is a breaking change and bumps `apiVersion`. A future `gero config show --resolved` would print every project with defaults filled in.

## Resolution rules

* A `release.lines` entry with no `branch` resolves to the project's default branch only while that line's phase is `development`. Any other line without a branch is an error.
* A `downstream` edge with `lines: all` maps, per Rancher line, the producer's release for that line to the consumer's branch for that line (e.g. kuberlr-kubectl `8.x` is line v2.15, which feeds BRO `release/v11.x`). A `universal` edge feeds every branch of the consumer, and it is the default for a producer that lists no lines of its own.
* A repo's `gero.yaml` may only hold `WorkflowsConfig` documents for now; any other kind is an error. The reconciler reads central config, and fetches a repo's `gero.yaml` only when it is about to act on that repo.
