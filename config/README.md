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

`RancherLines` gives each line a `phase`, following the product dates (GA, EOM, EOL):

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
| `repo.owned` | `false` |
| `repo.defaultBranch` | `main` |
| `release.branches` | `single` |
| `release.tagPattern` | `"v*"` |
| `release.requiresPrerelease` | off (opt-in) |
| `release.lines` | `universal` |
| `sources[].accept.when` | `stable` |
| `sources[].accept.lines` | `all` |
| `sources[].update.postEdit` | none |
| `pullRequests.labels` | none |

`release.requiresPrerelease` is off unless a project opts in. Stricter release verification is opt-in, which also leaves room for more checks later.

Documents use `apiVersion: gero/v1alpha1` until v1.0.0 of `gero`, when the shapes are agreed and become `gero/v1`. A change to a default after that is a breaking change and bumps `apiVersion`. A future `gero config show --resolved` would print every project with defaults filled in.

## Resolution rules

* A `release.lines` entry with no `branch` resolves to the project's default branch only while that line's `phase` is `development`. Any other line without a branch is an error.
* A source with `accept.lines: all` maps, per Rancher line, the source's release for that line to this project's branch for that line (e.g. kuberlr-kubectl `8.x` is line v2.15, which feeds BRO `release/v11.x`). A `universal` source feeds every line.
* A repo's `gero.yaml` may only hold `WorkflowsConfig` documents for now; any other kind is an error. The reconciler reads central config, and fetches a repo's `gero.yaml` only when it is about to act on that repo.
