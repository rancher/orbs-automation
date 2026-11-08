# Config

The configs `gero` runs on, in `gero/v1alpha2`: the release chain of the projects we model, as documents. The shape of each kind is in [CONFIG_SCHEMA](../docs/design/CONFIG_SCHEMA.md); how a project is described and why is in [CONFIG_UNITS](../docs/design/CONFIG_UNITS.md), and what the tools read from it is in [APP_CONFIG](../docs/specs/APP_CONFIG.md).

Every file is YAML. A document has `apiVersion`, `kind`, `name`, optional `refs` and a `spec`, inspired by Kubernetes resources but not compatible with them (no `metadata` or `status`). Files may hold several documents, and nothing depends on a file's name or place: a document is what it says it is.

| Kind | Meaning |
|---|---|
| `Project` | A repo as a place: who owns it, its pull requests, its branch for each Rancher line |
| `Stream` | What a project releases: the tags, the scheme, the lines and their version series |
| `Artifact` | One thing each release of a stream produces: an image or a file |
| `Holding` | Where a piece of a producer's release lives in a repo, and how it is written |
| `Edit` | Extra steps when shipping, and a named rule that needs code |
| `RancherLines` | The set of Rancher lines and facts about each |

File names here: one file for each project (`<name>.yaml`), with its stream, artifacts and holdings, and `rancher.lines.yaml`.

## Line phases

`RancherLines` records each line's lifecycle dates under `dates` (`ga`, `eom`, `eol`, as `YYYY-MM-DD`), copied from the [SUSE lifecycle page](https://www.suse.com/lifecycle/#suse-rancher-prime). The phase is not stored: it is worked out from the dates and the current time, and each phase begins at 00:00 UTC on its date. A line with no `ga` date, or one in the future, is in `development`. The dates must be in order (`ga <= eom <= eol`).

An optional `phase` on a line overrides the dates, for a line the page does not cover or to force a phase early.

| Phase | Meaning |
|---|---|
| `development` | Before GA. Served from a repo's default branch. |
| `ga` | Generally available, fully supported. |
| `eom` | End of maintenance reached: still supported, but only for fixes. |
| `eol` | End of life. Remove the line from config. |

A lifecycle date raises a warning, never an error, so a release exception is never made harder.

## Resolution rules

* A line in a project's `branches.lines` with no `branch` is on the default branch, which is for the line in development; once the line is past it, that is a warning
* `branch: ""` says the line has no branch yet, and the line is reported as unresolved
* A line may have a branch for each stage (`branch: {dev: dev-v2.16, stable: release-v2.16}`), named in `branches.stages`
* A Holding holds a release of an Artifact, or the commit or the branch of another project; a release holding is written when the producer's project is owned and the consumer is not external
* A repo's `gero.yaml` may only hold `WorkflowsConfig` documents for now; any other kind is an error. The reconciler reads central config, and fetches a repo's `gero.yaml` only when it is about to act on that repo

## Defaults

Only conservative or no-surprise defaults. Anything that changes release behavior is written out.

| Field | Default |
|---|---|
| `repo.owned` | `false`: `gero` does not ship from the repository |
| `repo.external` | `false` |
| `repo.managedBy` | not filled: `orbs` for an owned repo, none for an unowned one, which is shown as unspecified |
| `repo.defaultBranch` | `main` |
| `branches.layout` | `single` |
| `scheme` | `semver` |
| `tagPattern` | `v*` |
| `holds.accept` | `stable` |
| `at.format` | `yaml` |
| `holds.form` | `{tag}` |

The kinds are also published as JSON Schema in [`../generated/jsonschema/`](../generated/jsonschema/), for an editor to validate a file as it is written ([APP_CONFIG](../docs/specs/APP_CONFIG.md#json-schemas)).

`gero config validate` loads this directory, checks every document and the set, and reports each problem naming the file and the document, with a line for each warning.
