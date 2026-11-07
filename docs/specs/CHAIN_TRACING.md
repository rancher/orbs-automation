# Chain tracing

**Status:** draft. Nothing here is built. Items marked **(leaning)** are proposals, and **(unverified)** needs a small spike.

`gero lines resolve` and `gero repo status` answer "what is the state of this whole release chain?" from the central config and from what GitHub already holds, with no tracker. Following one hop is built. This spec is how they follow a chain through every hop, including the hop through the Rancher Assets image that connects `rancher/charts` to `rancher/rancher`, which we do not own. What the chain looks like today is in [rancher-assets](../projects/rancher-assets.md) and [rancher-charts](../projects/rancher-charts.md).

## The chain of one line

```
BRO tag ──bump PR──▶ charts dev-v2.15 ──promotion──▶ charts release-v2.15     (the release team promotes)
                          │                                │
                          └── lock.yaml pins the commit ───┘   (rancher-assets builds from both)
                                          ▼
                 rancher-assets tag v2.15-20260915T1243Z[-dev]                 (the release team's CI)
                                          ▼
              rancher/rancher release/v2.15  build.yaml: defaultAssetsImage    (the release team's CI opens the PR)
```

Our own hop is the first one. The others are the release team's, and `gero` reads them.

## Scope and order

The chain is traced from its far end back, one stretch at a time, and each stretch is finished before the next starts:

1. **Assets and Rancher.** What Rancher holds in `defaultAssetsImage`, which Assets tag that is, and the reverse: which Rancher branches hold a given Assets tag. This is the simplest hop to read, and it establishes the tag scheme, `yamlRead`, observed edges and the walk.
2. **Charts to Assets.** Which charts commit an Assets tag was built from, and the dev and release tiers of `rancher/charts`.
3. **Our projects to charts.** BRO's tag into a charts bump, and `chartsPackageBump`.

Only stretch 1 is specified in detail below. Stages (the tiers of `rancher/charts`) and the reading of `lock.yaml` belong to stretch 2, are kept here as the direction, and are marked as such.

## What the config adds

All of it is additive: a config without these keys behaves as it does today.

* **Stages (stretch 2).** A project whose branches come in tiers lists them once, as patterns, with `{line}` standing for the Rancher line:

  ```yaml
  release:
    branching: perLine
    stages:
      - {name: dev, branch: "dev-{line}"}       # the first stage is the default
      - {name: release, branch: "release-{line}"}
    lines:
      - {line: v2.15, version: "110.x"}          # version is the chart series, as for any project
      - {line: v2.14, version: "109.x", branches: {release: release-v2.14-hotfix}}   # an exception
  ```

  A line's `branch` is the first stage's branch, so everything that asks for "the branch of a line" keeps its answer. A producer's edge names the stage it lands on (`stage:`, default the first), so a producer of ours lands on `dev` and the release team promotes to `release`.
* **Lines in tags.** `branching: perLineTag` says the lines of a project are found in its tags and not in per-line branches, and `tagScheme` names how those tags spell a line and a stage. `rancher-assets` has one branch and tags like `v2.16-20261030T1111Z-dev`. `tagScheme` is the one extra field it needs: it defaults to semver and `rancher-assets` sets `calverLineStage`. A `perLineTag` project has no `lines` list (the loader refuses one, so tags are the only source): its lines are the lines its tags carry, and a tag that does not parse is skipped. A view that starts from the hub shows only the lines the hub lists, so Assets tags of a retired line do not appear. All of its tags are on its one branch, the repository's default, so it needs no branch per line. Finding the lines reads the tags, so a view that reads no remote shows such a project without lines. The stage names are the same as in the charts repo, `dev` and `release`, so an edge between same-named stages needs no mapping.
* **Stewardship: `repo.owned` and `repo.managedBy`, two fields that work together.** `owned` says whether `gero` ships from the repository: it is true when the repository's workflows run `gero` ([APP_CONFIG](APP_CONFIG.md)), and false, the default, when shipping to its downstream is not ours. `managedBy` says which team is responsible for the project. It is `orbs` when `owned` is true and `managedBy` is unset; for an unowned project there is no default, and an unset `managedBy` is reported as unspecified and never assumed to be ORBS. The four combinations:

  | `owned` | `managedBy` | What it means |
  |---|---|---|
  | true | `orbs` (the default when owned) | Ours in full: `gero` ships its downstream, and ORBS is responsible for the project. |
  | true | another team | A project we help ship with `gero` tooling: its workflows run `gero`, and the other team stays responsible for it. |
  | false | another team | Not ours: its downstream is shipped by that team's own tooling, and `gero` observes. |
  | false | `orbs`, written out | ORBS's own, with no `gero` workflows in it yet: observed until there are. |

  **Every edge from an unowned project is observed:** `gero` reads where it stands, and `gero ship` never opens a PR for it. An edge into an unowned project is written as usual, which is how `rancher/rancher` receives its bumps. `managedBy` does not change what is written: it is who is responsible, shown in reports and in the message when `gero ship` declines an observed edge. **To adopt `gero` for a repository the release team manages,** set `owned: true` and change its workflows to call `gero` commands; `managedBy` keeps naming the team that is responsible, and its edges become ours to ship.
* **A stage on an edge** means two things. On a managed edge it is the stage the edge writes (the consumer's first by default; stretch 2, since no project of ours has stages yet). On an observed edge it is a stage of the producer: the stream the consumer is expected to hold. On `assets → rancher`, `stage: release` says Rancher should hold a release tag, and Rancher itself has no stages. Without it, an observed edge accepts any stage and does not warn.

## Tag schemes

A scheme is code, named in config and claimed from one registry, as update strategies are, and it is table-tested on real tags. It answers four questions: which line a tag is for, which stage, in what order tags of one line and stage come, and which tag is the newest.

**`calverLineStage`** (the Assets tags):
* The format is `v{minor}-{YYYYMMDD}T{HHMM}Z`, with `-dev` after it for dev. The date is UTC and to the minute, and it is the commit time of the commit that was released, not the build time. Prod and dev tags of one commit share it.
* The stage is the suffix: `-dev` is `dev` and none is `release`. The line is the `v{minor}` prefix.
* **Newest** is the greatest timestamp among the tags of the line and stage. A tag is filtered by line and stage first and sorted second, since sorting every tag together mixes the two stages.
* A tag that does not fit the format is not a release of that line and is skipped.

## Reading a hop

* **`yamlRead`** is a strategy that reads one key of one file at a ref, a branch head, a tag or a commit, and writes nothing. It shares the key-path code of `yamlBump`, and its key may use `{line}`, `{minor}` and `{stage}`. A strategy offers writing through a separate small interface, so a read-only one cannot be asked to write.
* **Assets from charts (stretch 2).** The charts commit an Assets tag was built from is `chart-versions."{minor}".upstream-refs.{stage}.charts.commit` in `lock.yaml` at that tag. Comparing it with the head of the stage's branch in `rancher/charts` says whether the newest tag was built from today's branch: the cheap check. Reading `release.yaml` at that commit says whether a chart version is in it: the precise check.
* **Rancher from Assets.** What Rancher holds is the `defaultAssetsImage` key of `build.yaml` on a branch, read as for any `yamlBump` edge, and its tag is parsed by the scheme. A tag of the `dev` stage held where `release` is expected is a warning.

## What the reports show

* `gero lines resolve` shows a line's chain as a tree from the hub back through every hop, with a hop that does not fit the line reported as a gap at the hop where it breaks. A project reached only through another is no longer under `also`.
* `gero repo status` shows each hop's holdings through the chain, to a depth that `--depth` caps, with a hop that cannot be read reported on its own and the rest still shown.
* An observed hop is marked as observed.
* **Both directions.** From Rancher: the Assets tag each branch holds, with its line, stage and age against the newest tag of that line and stage. From Assets: the Rancher branches that hold a given tag, found by reading `build.yaml` on each Rancher branch of the line.

## Where it lives

* `internal/config` holds the stages, `managedBy`, `perLineTag`, `tagScheme` and the edge's stage, and validates them: `perLineTag` has a known scheme, an observed edge's stage is one the producer's scheme can give, and `managedBy`, when set, is a lowercase slug (`orbs`). For stretch 2: stage names are unique, each branch pattern has `{line}`, and a managed edge's stage exists on its consumer.
* `internal/calver` is pure: the schemes and nothing else.
* `internal/chain` is a pure, cycle-safe walk over the project graph that returns a tree. It asks the graph for edges and a small reader interface for what a hop holds, and has fakes in tests, as the packages that read GitHub do.
* `internal/updatedispatcher` registers `yamlRead`, and `internal/ship` skips edges from an unowned project.
* `internal/cli/linesresolve` and `internal/cli/repostatus` use the walk, and the per-run list of a repository's refs is read once and kept for the run.

## Not built yet

Everything. Stretch 1 is built in this order: the config additions it needs (`owned`, `managedBy`, `perLineTag`, `tagScheme`, an observed edge's stage) and validation, with the skip in `ship`; then `internal/calver`; then `yamlRead` and reading at a ref; then `internal/chain` with `lines resolve`; then `repo status`. Stretches 2 and 3, including stages and `chartsPackageBump`, are designed when stretch 1 is done.

## Decided

* An unowned project need not state `managedBy`: it is reported as unspecified.
* A team name is a lowercase slug: `orbs`, `release-team`.
* **No calendar state.** The release cycle (code freeze, RCs, sign-off, release day) is date-based and partly internal, so `gero` does not model it. A chain view reports alignment from tags and files: which tag a branch holds, its line and stage, and whether it is the newest of that line and stage. A held tag of a stage other than the edge's expected `stage` is shown as differing from what is expected, styled as a warning, and never fails a run, since it is the normal state during an RC phase. A strict mode for CI is possible later without changing the model.
* **Branch of an Assets line is Rancher's.** The walk goes from a line to the hub's branch for it (from the hub's own `lines`), reads `build.yaml` there, and parses the Assets tag, whose line must match. Nothing maps lines between the two projects.

## Open questions

Stretch 1:

* Whether the key path for `yamlRead` needs a quoting rule for keys that contain a dot, such as the line `2.16`.

Later stretches:

* How BRO's tag maps to the chart version in `release.yaml` (`110.0.3+up11.0.4`), and whether that rule is written down by the release team or only encoded in the scripts that make it.
* Whether an Assets tag's charts commit is enough to answer "is this chart version in this tag", or the image's contents have to be read.
* How a charts line differs in how it is bumped beyond the steps that `since` and `until` already vary.
* Whether a held `dev` tag is a warning or an error, and which Rancher branch each Assets line maps to when the release team's own map is hardcoded.
* Whether the key path for `yamlRead` needs a quoting rule for keys that contain a dot, such as the line `2.16`.
