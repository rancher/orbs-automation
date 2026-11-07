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

## What the config adds

All of it is additive: a config without these keys behaves as it does today.

* **Stages.** A project whose branches come in tiers lists them once, as patterns, with `{line}` standing for the Rancher line:

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
* **Lines in tags.** `branching: perLineTag` says the lines of a project are found in its tags and not in per-line branches, and `tagScheme` names how those tags spell a line and a stage. `rancher-assets` has one branch and tags like `v2.16-20261030T1111Z-dev`. `tagScheme` is required with `perLineTag`, and a project with per-line branches keeps the default, semver. The stage names are the same as in the charts repo, `dev` and `release`, so an edge between same-named stages needs no mapping.
* **Stewardship: `repo.owned` and `repo.managedBy`, two fields that work together.** `owned` says whether `gero` ships from the repository: it is true when the repository's workflows run `gero` ([APP_CONFIG](APP_CONFIG.md)), and false, the default, when shipping to its downstream is not ours. `managedBy` says which team is responsible for the project. It is `orbs` when `owned` is true and `managedBy` is unset; for an unowned project there is no default, and an unset `managedBy` is reported as unspecified and never assumed to be ORBS. The four combinations:

  | `owned` | `managedBy` | What it means |
  |---|---|---|
  | true | `orbs` (the default when owned) | Ours in full: `gero` ships its downstream, and ORBS is responsible for the project. |
  | true | another team | A project we help ship with `gero` tooling: its workflows run `gero`, and the other team stays responsible for it. |
  | false | another team | Not ours: its downstream is shipped by that team's own tooling, and `gero` observes. |
  | false | `orbs`, written out | ORBS's own, with no `gero` workflows in it yet: observed until there are. |

  **Every edge from an unowned project is observed:** `gero` reads where it stands, and `gero ship` never opens a PR for it. An edge into an unowned project is written as usual, which is how `rancher/rancher` receives its bumps. `managedBy` does not change what is written: it is who is responsible, shown in reports and in the message when `gero ship` declines an observed edge. **To adopt `gero` for a repository the release team manages,** set `owned: true` and change its workflows to call `gero` commands; `managedBy` keeps naming the team that is responsible, and its edges become ours to ship.
* **A stage on an edge** means two things. On a managed edge it is the stage the edge writes (the consumer's first by default). On an observed edge it is the stage the edge follows, and by default it is every stage, paired by name. On `assets → rancher`, `stage: release` is the stream Rancher is expected to hold.

## Tag schemes

A scheme is code, named in config and claimed from one registry, as update strategies are, and it is table-tested on real tags. It answers four questions: which line a tag is for, which stage, in what order tags of one line and stage come, and which tag is the newest.

**`calverLineStage`** (the Assets tags):
* The format is `v{minor}-{YYYYMMDD}T{HHMM}Z`, with `-dev` after it for dev. The date is UTC and to the minute, and it is the commit time of the commit that was released, not the build time. Prod and dev tags of one commit share it.
* The stage is the suffix: `-dev` is `dev` and none is `release`. The line is the `v{minor}` prefix.
* **Newest** is the greatest timestamp among the tags of the line and stage. A tag is filtered by line and stage first and sorted second, since sorting every tag together mixes the two stages.
* A tag that does not fit the format is not a release of that line and is skipped.

## Reading a hop

* **`yamlRead`** is a strategy that reads one key of one file at a ref, a branch head, a tag or a commit, and writes nothing. It shares the key-path code of `yamlBump`, and its key may use `{line}`, `{minor}` and `{stage}`. A strategy offers writing through a separate small interface, so a read-only one cannot be asked to write.
* **Assets from charts.** The charts commit an Assets tag was built from is `chart-versions."{minor}".upstream-refs.{stage}.charts.commit` in `lock.yaml` at that tag. Comparing it with the head of the stage's branch in `rancher/charts` says whether the newest tag was built from today's branch: the cheap check. Reading `release.yaml` at that commit says whether a chart version is in it: the precise check.
* **Rancher from Assets.** What Rancher holds is the `defaultAssetsImage` key of `build.yaml` on a branch, read as for any `yamlBump` edge, and its tag is parsed by the scheme. A tag of the `dev` stage held where `release` is expected is a warning.

## What the reports show

* `gero lines resolve` shows a line's chain as a tree from the hub back through every hop, with a hop that does not fit the line reported as a gap at the hop where it breaks. A project reached only through another is no longer under `also`.
* `gero repo status` shows each hop's holdings through the chain, to a depth that `--depth` caps, with a hop that cannot be read reported on its own and the rest still shown.
* An observed hop is marked as observed.

## Where it lives

* `internal/config` holds the stages, `managedBy`, `perLineTag`, `tagScheme` and the edge's stage, and validates them: stage names are unique, each branch pattern has `{line}`, `perLineTag` has a known scheme, an edge's stage exists on its consumer, and `managedBy`, when set, is a team name.
* `internal/calver` is pure: the schemes and nothing else.
* `internal/chain` is a pure, cycle-safe walk over the project graph that returns a tree. It asks the graph for edges and a small reader interface for what a hop holds, and has fakes in tests, as the packages that read GitHub do.
* `internal/updatedispatcher` registers `yamlRead`, and `internal/ship` skips edges from an unowned project.
* `internal/cli/linesresolve` and `internal/cli/repostatus` use the walk, and the per-run list of a repository's refs is read once and kept for the run.

## Not built yet

Everything. The order it is built in: the config additions and validation with the skip in `ship`, then `internal/calver`, then `yamlRead` and reading at a ref, then `internal/chain` with `lines resolve`, then `repo status`, and `chartsPackageBump`, the write side of the charts hop, separately.

## Open questions

* Whether an unowned project must say `managedBy` outright, and not only be reported as unspecified when it does not, and how the team name is spelled.
* How BRO's tag maps to the chart version in `release.yaml` (`110.0.3+up11.0.4`), and whether that rule is written down by the release team or only encoded in the scripts that make it.
* Whether an Assets tag's charts commit is enough to answer "is this chart version in this tag", or the image's contents have to be read.
* How a charts line differs in how it is bumped beyond the steps that `since` and `until` already vary.
* Whether a held `dev` tag is a warning or an error, and which Rancher branch each Assets line maps to when the release team's own map is hardcoded.
* Whether the key path for `yamlRead` needs a quoting rule for keys that contain a dot, such as the line `2.16`.
