# Chain tracing

**Status:** the hop between Assets and Rancher is implemented, and the config for the hops through charts is written. Reading a commit or branch holding, the walk through several hops and BRO into charts are not built ([Not built yet](#not-built-yet)).

`gero lines resolve` and `gero repo status` answer "what is the state of this whole release chain?" from the central config and from what GitHub already holds, with no tracker. This spec is how they follow a chain through its hops, including the hop through the Rancher Assets image that connects `rancher/charts` to `rancher/rancher`, which we do not own. What the chain looks like is in [rancher-assets](../projects/rancher-assets.md) and [rancher-charts](../projects/rancher-charts.md); the graph that answers it is [CHAIN_MODEL](../design/CHAIN_MODEL.md).

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

## Stretches

The chain is traced from its far end back, one stretch at a time:

1. **Assets and Rancher.** What Rancher holds in `defaultAssetsImage`, which Assets tag that is, and the reverse: which Rancher branches hold a given Assets tag. It establishes the shared release scheme, observed edges and stewardship.
2. **Charts to Assets.** Which charts commit an Assets tag was built from, the dev and stable tiers of `rancher/charts`, and the walk through several hops.
3. **Our projects to charts.** BRO's tag into a charts bump, and `chartsPackageBump` ([UPDATE_ENGINE](../design/UPDATE_ENGINE.md)).

## What the config says

The config for all three stretches is in `config/`. Tracing reads these parts of it ([CONFIG_SCHEMA](../design/CONFIG_SCHEMA.md)).

* **Stages.** A project whose branches come in tiers has a branch for each stage per line, `branch: {dev: dev-v2.16, stable: release-v2.16}`, with the stages named in `branches.stages`. The stages are `dev` and `stable`, the two the tag schemes have. A holding names the stage it is found on (`holds.stage`), and the chain answers the branch of a line for a stage (`BranchOfLine(line, stage)`). A branch name found in another project's file maps back to its line and stage through this table, with no naming pattern to get wrong. A producer of ours lands on `dev` and the release team promotes to `stable`.
* **Lines in tags.** A project whose layout is `single` lists its lines in its stream, and the `calverLineStage` scheme says how its tags spell a line and a stage. `rancher-assets` has one branch and tags like `v2.16-20261030T1111Z-dev`. A line is added when the project is used for it. A tag that does not parse, or is of a line not listed, is skipped, so no code discovers lines from tags, and the edge to Rancher reaches only the lines both projects list.
* **Stewardship: `repo.owned` and `repo.managedBy`.** `owned` says whether `gero` ships from the repository, true when its workflows run `gero`. `managedBy` says which team is responsible. It is `orbs` when `owned` is true and it is unset; for an unowned project there is no default, and an unset one is reported as unspecified.

  | `owned` | `managedBy` | What it means |
  |---|---|---|
  | true | `orbs` (the default when owned) | Ours in full: `gero` ships its downstream, and ORBS is responsible |
  | true | another team | We help ship it with `gero` tooling, and the other team stays responsible |
  | false | another team | Not ours: that team's own tooling ships its downstream, and `gero` observes |
  | false | `orbs`, written out | ORBS's own, with no `gero` workflows in it yet: observed until there are |

  A release holding **of an unowned project** is observed: `gero` reads where it stands, and `gero ship` never opens a PR for it. A holding **in** an unowned project is written as usual, which is how `rancher/rancher` receives its bumps. `managedBy` is shown in reports and in the message when `gero ship` declines an observed project, and changes nothing that is written. To adopt `gero` for a repository the release team manages, set `owned: true` and have its workflows call `gero`.
* **A stage on a holding.** On a release holding it is the stage of the holder's branch written to. On a commit or branch holding, `stages` names the stages of the producer to read. A release holding that sets none follows whatever stage the consumer holds: `rancher-holds-assets` sets none, since Rancher holds a `dev` Assets tag until the release team makes a stable release.

## Tag schemes

A scheme is code, named in config and claimed from one registry, as update strategies are, and table-tested on real tags. It answers which line a tag is for, which stage, in what order tags of one line and stage come, and which tag is the newest.

* **`internal/calver`** is the tag format of `calverLineStage` and nothing else: `Parse` a name into a line, stage and UTC time, `Compare` two tags of one line and stage, and `Select` the tags of a line and stage, newest first. It works on names.
* **`internal/tagscheme`** is what every reader of a project's releases asks, chosen from the stream's `scheme`: its lines, the line a tag is in, the newest stable and prerelease of a line, and the newest of either kind (`Newest`), whether a tag is stable, and the order of two tags. `ship` and `repo status` call it and carry no scheme code of their own. `lines resolve` reads the config alone. `repo tag` and `internal/releasetag` are for tags a person makes, so they stay semver and `repo tag` refuses a project whose tags its own CI makes. They are told the newest tag by `tagscheme.Newest` and do not find it themselves.

**`calverLineStage`** (the Assets tags): the format is `v{minor}-{YYYYMMDD}T{HHMM}Z`, with `-dev` after it for dev. The date is UTC and to the minute, and it is the commit time of the commit that was released, not the build time; prod and dev tags of one commit share it. The stage is the suffix (`-dev` is `dev`, none is `stable`) and the line is the `v{minor}` prefix. The newest is the greatest timestamp among the tags of the line and stage, filtered first and sorted second, since sorting every tag together mixes the stages. A tag that does not fit is not a release of that line.

## Charts as a project

`rancher/charts` is unowned and managed by the release team. It has no tagged releases: from v2.16 on the tagged releases of its content are the Assets tags, a project of their own, and before v2.16 Rancher names a charts branch directly. So charts is traced two ways, by line:

* **From v2.16, through Assets.** The charts commit an Assets tag was built from is pinned in Assets' `lock.yaml`, which the `assets-pins-charts` holding names. What Assets holds is that pinned commit, and what it should hold is the head of the stage's branch, so the answer is whether the newest Assets tag was built from today's head, and which tag first contains a given charts commit.
* **Before v2.16, directly.** What Rancher holds is a charts branch name in `build.yaml` (the `rancher-follows-charts-branch` holding, up to v2.15), which the line's `branch` map turns into the stage held. Rancher does not say which commit, so the commit is an **estimate**: the newest commit of that branch at or before the time of the Rancher commit. It is a lower bound, shown labelled as an estimate and never as a pinned fact.

A holding says `holds.lines` to reach only the lines it applies to, so each is read only where it exists.

## Reading a hop

* **No new strategy.** What a hop holds is read by the holding that already names it. `rancher-holds-assets` on Rancher's `build.yaml` is read by the engine's check (`bump.CheckHolding`, which reads a file at a commit and writes nothing). Read-only comes from the project being unowned, not from a second strategy: `gero ship` refuses an unowned producer, and `Holdings` and `Sources` read it.
* **Rancher from Assets.** What Rancher holds is the `defaultAssetsImage` key of `build.yaml` on a branch, and the scheme parses its tag. The stage of the tag held is shown, and neither stage is a warning.
* **Assets from charts.** The commit an Assets tag was built from is `chart-versions."{minor}".upstream-refs.{stage}.charts.commit` in `lock.yaml` at that tag. Comparing it with the head of the stage's branch in `rancher/charts` says whether the newest tag was built from today's branch (the cheap check). Reading `release.yaml` at that commit says whether a chart version is in it (the precise check).

## What the reports show

* **`gero repo status`** of a project shows what each consumer branch holds of its releases, and what it takes from each source. For Assets and Rancher that is the Assets tag held on each listed line, whether it is behind the newest release or dev tag (by date), and the stage held in `--json`. The same read gives both directions. A project whose lines are in its tags has a row for each line it lists, with its newest release tag as latest stable and a newer dev tag as latest pre-release, and no state or next tag ([REPO_STATUS](REPO_STATUS.md)). A hop that cannot be read is reported on its own and the rest is still shown.
* **`gero lines resolve`** shows Assets as a source of Rancher on the lines Assets lists and says nothing of it on the others ([LINES_RESOLVE](LINES_RESOLVE.md)). It reads the config and never the network.
* **Planned:** `lines resolve` shows a line's chain as a tree from the hub back through every hop, with a hop that does not fit the line reported as a gap where it breaks, and `repo status` follows the chain to a depth that `--depth` caps. An observed hop is marked as observed.

## Where it lives

* `internal/apis/v1alpha2` holds `managedBy`, the scheme of a stream, the stages and a holding's stage, and validates them. `internal/chain` answers the branch of a line, the stream and line of a tag, and who holds what ([CHAIN_MODEL](../design/CHAIN_MODEL.md)).
* `internal/calver` and `internal/tagscheme` are above. `internal/ship` refuses an unowned producer and asks the scheme for releases, lines and order; `internal/cli/repostatus` asks it for a project's latest tags.
* **The walk (planned):** a pure, cycle-safe walk over the chain's graph that returns a tree. It asks the graph for holdings and a small reader interface for what a hop holds, with fakes in tests, and `internal/cli/linesresolve` and `internal/cli/repostatus` use it.

## Not built yet

* Reading a commit or a branch holding at a ref, which `assets-pins-charts` and `rancher-follows-charts-branch` need, and the `lock.yaml` read built on it ([UPDATE_ENGINE](../design/UPDATE_ENGINE.md)).
* The walk through several hops, the tree in `lines resolve` and `--depth`.
* Checking a held tag's stage against the release team's rule.
* `chartsPackageBump` and BRO's tag into a charts bump.

## Decided

* An unowned project need not state `managedBy`: it is reported as unspecified. A team name is a lowercase slug.
* **No calendar state.** The release cycle (code freeze, RCs, sign-off, release day) is date-based and partly internal, so `gero` does not model it. A chain view reports alignment from tags and files: which tag a branch holds, its line and stage, and whether it is the newest of that line and stage. The stage of a held tag is a fact, never a warning or a failure.
* **The release team's stage rule is readable, and could be ours.** Their tooling makes a `dev` tag when only the charts dev commit in `lock.yaml` moved, and a prod tag whenever the charts prod commit moved. That is a comparison of `lock.yaml` at two commits, which a read at a ref can do: first to check a held tag's stage against it, later, if the release team adopts `gero` for Assets, to make it part of a `ship` decision.
* **Lines are listed.** A project that finds its lines in its tags lists them in its stream, without a series, and is judged on those lines: `lines resolve` shows no gap for one it does not list. A per-line project's missing line is still a gap.
* **No `yamlRead`.** The read half of a holding reads what a hop holds, and ownership keeps it read-only.
* **The branch of an Assets line is Rancher's.** The walk goes from a line to the hub's branch for it, reads `build.yaml` there, and parses the Assets tag, whose line must match. Nothing maps lines between the two projects.

## Open questions

* The key in Rancher's `build.yaml` that names the charts branch before v2.16, and whether it holds the branch name alone.
* How a pinned commit in `lock.yaml` is read: a small read-only helper over the key-path code, or something else, and what "release" a pin is compared with (the head of the stage's branch). The key path needs a quoting rule for a key with a dot, such as the line `2.16`.
* Whether `since` and `until` on a holding is all the line-range a holding needs.
* How BRO's tag maps to the chart version in `release.yaml` (`110.0.3+up11.0.4`), and whether that rule is written down by the release team or only encoded in the scripts that make it.
* Whether an Assets tag's charts commit is enough to answer "is this chart version in this tag", which is traced through `lock.yaml` at that tag to the contents of `rancher/charts` at the commit it records.
* How a charts line differs in how it is bumped beyond the steps that `since` and `until` already vary.
