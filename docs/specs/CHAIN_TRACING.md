# Chain tracing

**Status:** draft. Stretch 1 (Assets and Rancher) is implemented; stretches 2 and 3 are not designed in detail, as [Not built yet](#not-built-yet) says.

`gero lines resolve` and `gero repo status` answer "what is the state of this whole release chain?" from the central config and from what GitHub already holds, with no tracker. One hop is followed from any project. This spec is how they follow a chain through every hop, including the hop through the Rancher Assets image that connects `rancher/charts` to `rancher/rancher`, which we do not own. What the chain looks like today is in [rancher-assets](../projects/rancher-assets.md) and [rancher-charts](../projects/rancher-charts.md).

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

1. **Assets and Rancher (implemented).** What Rancher holds in `defaultAssetsImage`, which Assets tag that is, and the reverse: which Rancher branches hold a given Assets tag. This is the simplest hop to read, and it establishes the shared release scheme, observed edges and the walk.
2. **Charts to Assets.** Which charts commit an Assets tag was built from, and the dev and release tiers of `rancher/charts`.
3. **Our projects to charts.** BRO's tag into a charts bump, and `chartsPackageBump`.

Stretch 1 is what the rest of this spec describes as built. Stages (the tiers of `rancher/charts`), the reading of `lock.yaml`, the walk through several hops (`internal/chain`) and `--depth` belong to stretch 2, and are kept here as the direction, marked as such.

## What the config adds

All of it is additive: a config without these keys behaves as it does today.

* **Stages (stretch 2).** A line of a project whose branches come in tiers has a branch for each stage, written where the branch already is. `branch` is a string for a project with one branch per line, as it is today, or a map of stage to branch:

  ```yaml
  release:
    branching: perLine
    lines:
      - line: v2.16
        branch: {dev: dev-v2.16, release: release-v2.16}
      - line: v2.15
        branch: {dev: dev-v2.15, release: release-v2.15}
  ```

  The stages are the two the tag schemes already have, `dev` and `release`, in that order, and the first of a line's stages is its default. Everything that asks for the branch of a line keeps its answer, which is the default stage's branch (`dev-v2.16`). A branch name found in another project's file, such as the charts branch Rancher's `build.yaml` names, maps back to its line and stage through this table, with no naming pattern to get wrong. Only `dev` and `release` are stages, so the order needs no ordering of a YAML map. A producer's edge names the stage it lands on (`stage:`, default the first), so a producer of ours lands on `dev` and the release team promotes to `release`.
* **Lines in tags.** `branching: perLineTag` says the lines of a project are in its tags and not in per-line branches, and `tagScheme` names how those tags spell a line and a stage. `rancher-assets` has one branch and tags like `v2.16-20261030T1111Z-dev`. `tagScheme` is the one extra field it needs: it defaults to semver and `rancher-assets` sets `calverLineStage`. A `perLineTag` project lists its lines, each with just its `line` (no `branch` and no `version`, which the loader refuses): `rancher-assets` lists `v2.16`, the line it is used for, and a line is added when it is used. A tag that does not parse, or that is of a line not listed, is skipped. All of its tags are on its one branch, the repository's default, so it needs no branch per line. Explicit lines keep the reports plain: the edge to Rancher reaches only the lines both projects list, so Rancher's older branches, which hold no Assets image, are not in the report as errors, and no code has to discover lines from tags. The stage names are the same as in the charts repo, `dev` and `release`, so an edge between same-named stages needs no mapping.
* **Stewardship: `repo.owned` and `repo.managedBy`, two fields that work together.** `owned` says whether `gero` ships from the repository: it is true when the repository's workflows run `gero` ([APP_CONFIG](APP_CONFIG.md)), and false, the default, when shipping to its downstream is not ours. `managedBy` says which team is responsible for the project. It is `orbs` when `owned` is true and `managedBy` is unset; for an unowned project there is no default, and an unset `managedBy` is reported as unspecified and never assumed to be ORBS. The four combinations:

  | `owned` | `managedBy` | What it means |
  |---|---|---|
  | true | `orbs` (the default when owned) | Ours in full: `gero` ships its downstream, and ORBS is responsible for the project. |
  | true | another team | A project we help ship with `gero` tooling: its workflows run `gero`, and the other team stays responsible for it. |
  | false | another team | Not ours: its downstream is shipped by that team's own tooling, and `gero` observes. |
  | false | `orbs`, written out | ORBS's own, with no `gero` workflows in it yet: observed until there are. |

  **Every edge from an unowned project is observed:** `gero` reads where it stands, and `gero ship` never opens a PR for it. An edge into an unowned project is written as usual, which is how `rancher/rancher` receives its bumps. `managedBy` does not change what is written: it is who is responsible, shown in reports and in the message when `gero ship` declines an observed edge. **To adopt `gero` for a repository the release team manages,** set `owned: true` and change its workflows to call `gero` commands; `managedBy` keeps naming the team that is responsible, and its edges become ours to ship.
* **A stage on an edge** means two things. On a managed edge it is the stage the edge writes (the consumer's first by default; stretch 2, since no project of ours has stages yet). On an observed edge it is optional and names a stage of the producer to follow, and without it the edge follows whatever stage the consumer holds. `assets → rancher` sets none: Rancher holds a `dev` Assets tag until the release team makes a stable release and opens the PR for the stable tag, so either stage is normal there, and the report shows which one is held.

## Tag schemes

A scheme is code, named in config and claimed from one registry, as update strategies are, and it is table-tested on real tags. It answers four questions: which line a tag is for, which stage, in what order tags of one line and stage come, and which tag is the newest.

In Go there are two layers.

* **`internal/calver`** is the tag format of `calverLineStage` and nothing else: `Parse` of a tag name into a `Tag` with its line, stage and UTC time, `Compare` of two tags of one line and stage, and `Select` (the tags of a line and stage, newest first). It works on names and takes no `git` type.
* **`internal/tagscheme`** is the one thing every reader of a project's releases asks, and is chosen from the project by `release.branching` and `release.tagScheme`. A `Scheme` answers: the lines of the project (`Lines`, the ones its config lists), the line a tag is in (`LineOf`), the newest stable and the newest prerelease of a line (`Latest`: for semver a stable and a prerelease tag, for `calverLineStage` the `release` stage and the `dev` stage, since a dev tag is a prerelease), whether a tag is stable (`IsStable`), and the order of two tags of a line (`Compare`: semver order, or for CalVer the date, since a newer date with the same line prefix is a newer image). The semver implementation is what `git.InSeries`, `git.LatestReleases` and `ship`'s `lineSeries` do today, moved behind it unchanged, and the CalVer one is `internal/calver`. `ship` (the choice of a release, and what a consumer holds against it) and `repo status` (a project's own latest tags) call it and carry no scheme code of their own, so `ship` has no CalVer code in it. `lines resolve` reads the config alone, and `repo tag` and what tag could be made next (`internal/git/releasetag`) are for tags a person makes: they stay semver and do not apply to a `perLineTag` project, whose tags its own CI makes. The release-line policy that `ship` had of its own is this scheme now.

**`calverLineStage`** (the Assets tags):
* The format is `v{minor}-{YYYYMMDD}T{HHMM}Z`, with `-dev` after it for dev. The date is UTC and to the minute, and it is the commit time of the commit that was released, not the build time. Prod and dev tags of one commit share it.
* The stage is the suffix: `-dev` is `dev` and none is `release`. The line is the `v{minor}` prefix.
* **Newest** is the greatest timestamp among the tags of the line and stage. A tag is filtered by line and stage first and sorted second, since sorting every tag together mixes the two stages.
* A tag that does not fit the format is not a release of that line and is skipped.

## Charts as a project (stretch 2, draft)

`rancher/charts` is unowned and managed by the release team. It has no tagged releases: from v2.16 on the real tagged releases of its content are the Assets tags, a project of their own, and before v2.16 Rancher names the charts branch directly in its `build.yaml`. So charts is traced in two ways, by line:

* **From v2.16 (through Assets).** The charts commit an Assets tag was built from is pinned in Assets' `lock.yaml`, and the charts branch for each stage is in its `config.yaml`. The edge from charts to Assets is observed. What Assets holds of charts is that pinned commit, and what it should hold is the head of the stage's branch, so the answer is whether the newest Assets tag was built from today's head, and which tag first contains a given charts commit.
* **Before v2.16 (directly).** The edge from charts to Rancher is observed, and what Rancher holds is a charts branch name in `build.yaml`, which the line's `branch` map turns into the stage held (dev or release). Rancher does not say which commit, so the commit is an **estimate**: the newest commit of that charts branch at or before the time of the Rancher commit, a lower bound ("at least this much") since an image built later holds the head at that later time. It is shown labelled as an estimate and never as a pinned fact.
* **Which edge applies to which line.** An edge may say `since` and `until`, a range of the consumer's lines, as steps and images do, validated against `RancherLines`. The edge into Rancher says `until: v2.15` and the edge into Assets reaches only the lines Assets lists.

## Reading a hop

* **No new strategy.** What a hop holds is read by the edge that already names it: `build-yaml-bump` on Rancher's `build.yaml` has a read half (`yamlBump`'s `Check`, which reads a file at a commit and writes nothing), and `rancher-assets.project.yaml` already has that edge. Read-only comes from the project being unowned, not from a second strategy: `gero ship` refuses an unowned producer, and `Holdings` and `Sources` read it. Reading `lock.yaml` at a ref in stretch 2 is a small helper over the file reader and the key-path code of `yamlBump`, not a strategy, and the quoting rule for a key that contains a dot (the line `2.16`) is settled there.
* **Assets from charts (stretch 2).** See [Charts as a project](#charts-as-a-project-stretch-2-draft). The charts commit an Assets tag was built from is `chart-versions."{minor}".upstream-refs.{stage}.charts.commit` in `lock.yaml` at that tag. Comparing it with the head of the stage's branch in `rancher/charts` says whether the newest tag was built from today's branch: the cheap check. Reading `release.yaml` at that commit says whether a chart version is in it: the precise check.
* **Rancher from Assets.** What Rancher holds is the `defaultAssetsImage` key of `build.yaml` on a branch, read as for any `yamlBump` edge, and its tag is parsed by the scheme. The stage of the tag held is shown, and neither stage is a warning.

## What the reports show

* **`gero repo status`** of a project shows what each consumer branch holds of its releases, and of a project shows what it takes from each source: for Assets and Rancher the Assets tag held on each listed line, whether it is behind the newest release or dev tag (by date), and the stage held in `--json`. The same read gives both directions, from Rancher and from Assets. A project whose lines are in its tags has a row for each line it lists, with its newest release tag as latest stable and a newer dev tag as latest pre-release, and no state or next tag ([REPO_STATUS](REPO_STATUS.md)). A hop that cannot be read is reported on its own and the rest is still shown.
* **`gero lines resolve`** shows Assets as a source of Rancher on the lines Assets lists and says nothing of it on the others ([LINES_RESOLVE](LINES_RESOLVE.md)). It reads the config and never the network.
* **Stretch 2:** `lines resolve` shows a line's chain as a tree from the hub back through every hop, with a hop that does not fit the line reported as a gap where it breaks, a project reached only through another is no longer under `also`, and `repo status` follows the chain to a depth that `--depth` caps. An observed hop is marked as observed.

## Where it lives

* `internal/config` holds `managedBy`, `perLineTag`, `tagScheme` and the edge's stage (and stages, in stretch 2), and validates them: `perLineTag` lists lines with no branch or version and has a scheme of its own, an observed edge's stage, when set, is one the producer's scheme can give, and `managedBy`, when set, is a lowercase slug (`orbs`). For stretch 2: stage names are unique, each branch pattern has `{line}`, and a managed edge's stage exists on its consumer.
* `internal/calver` is pure: the tag format and nothing else. `internal/tagscheme` is the shared release scheme, with the semver implementation and a CalVer one on `internal/calver`.
* `internal/ship` refuses an unowned producer, and asks the scheme for releases, lines and order; `internal/cli/repostatus` asks it for a project's latest tags.
* **Stretch 2:** `internal/chain` is a pure, cycle-safe walk over the project graph that returns a tree. It asks the graph for edges and a small reader interface for what a hop holds, and has fakes in tests, as the packages that read GitHub do. `internal/cli/linesresolve` and `internal/cli/repostatus` use it, and the per-run list of a repository's refs is read once and kept for the run. It waits for stretch 2 because the only second hop, charts to Assets, has no project config yet, so the walk would have one real hop to follow.

## Not built yet

Stretch 1 is implemented: the config fields (`repo.managedBy`, `release.branching: perLineTag` with its listed lines, `release.tagScheme` and an observed edge's `stage`) and their checks, `Project.Steward`, `gero ship` refusing an unowned producer, `gero repo info` showing who manages a project, `internal/calver`, `internal/tagscheme`, `repo status` in both directions and `lines resolve` for Assets.

Not built:

* Stretch 2: `internal/chain` and the tree in `lines resolve`, `--depth`, the charts project config with stages, reading `lock.yaml` at a ref, and checking a held tag's stage against the release team's rule.
* Stretch 3: `chartsPackageBump` and BRO's tag into a charts bump.
* `repo status`'s `UNRELEASED` count and `LOCAL` comparison for a CalVer project are the generic ones (commits since the newest tag), and have not been checked against how Assets tags move.
* `repo tag` and `releasetag` do not use the scheme: they are for tags a person makes.

## Decided

* An unowned project need not state `managedBy`: it is reported as unspecified.
* A team name is a lowercase slug: `orbs`, `release-team`.
* **No calendar state.** The release cycle (code freeze, RCs, sign-off, release day) is date-based and partly internal, so `gero` does not model it. A chain view reports alignment from tags and files: which tag a branch holds, its line and stage, and whether it is the newest of that line and stage. The stage of a held tag is shown as a fact, and never as a warning or a failure: Rancher holds a `dev` Assets tag (a prerelease) until the release team cuts a stable one, so no stage is wrong there.
* **The release team's stage rule is readable, and could be ours.** Their tooling makes a `dev` tag when only the charts dev commit in `lock.yaml` moved, and a prod tag whenever the charts prod commit moved. That is a comparison of `lock.yaml` at two commits, which a read of `lock.yaml` at a ref can do. It belongs to stretch 2: first to check a held tag's stage against the rule, and later, if the release team adopts `gero` for Assets, to make the rule part of a `ship` decision.
* **Lines are listed.** A `perLineTag` project lists its lines, as any project does, without branch or version. Assets is used from v2.16 on and may stop being used, so the list says which lines it is for, and nothing discovers lines from tags.
* **No `yamlRead`.** The read half of `yamlBump` reads what a hop holds, and ownership keeps it read-only.
* **A line judged by what a project declares.** A `perLineTag` project is judged on the lines it lists: `lines resolve` shows no gap for one it does not. A per-line project's missing line is still a gap.
* **The walk waits.** `internal/chain` waits for stretch 2, when there is a second hop to follow.
* **Branch of an Assets line is Rancher's.** The walk goes from a line to the hub's branch for it (from the hub's own `lines`), reads `build.yaml` there, and parses the Assets tag, whose line must match. Nothing maps lines between the two projects.

## Open questions

None for stretch 1. For the later stretches:

* The key in Rancher's `build.yaml` that names the charts branch before v2.16, and whether it holds the branch name alone.
* The read of a pinned commit in Assets' `lock.yaml`: a new small read-only strategy or something simpler, and what the "release" is that a pin is compared with (the head of the stage's branch).
* Whether `since` and `until` on an edge is all the line-range an edge needs.
* Whether the key path for the `lock.yaml` read needs a quoting rule for keys that contain a dot, such as the line `2.16`.
* How BRO's tag maps to the chart version in `release.yaml` (`110.0.3+up11.0.4`), and whether that rule is written down by the release team or only encoded in the scripts that make it.
* Whether an Assets tag's charts commit is enough to answer "is this chart version in this tag". The answer is traced through the `lock.yaml` at that tag, which records the commit the generated Dockerfile used, to the contents of `rancher/charts` at that commit. It belongs to stretch 2, with the charts-to-Assets connection.
* How a charts line differs in how it is bumped beyond the steps that `since` and `until` already vary.
