# REPO_STATUS

**Status:** implemented. What `gero repo status` reports and how each part is derived. The command and its flags are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md); the tag rules it shows are in [TASK_COMMANDS](TASK_COMMANDS.md#gero-repo-tag-project-line-tag).

The report is sections, and each answers one question, so a column means one thing and a fact is shown where it is read from. `repo info` is the same project without the network ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)).

```
rancher/shell

Releases
  LINE   BRANCH          HEAD     LATEST STABLE  LATEST PRE-RELEASE  STATE                      NEXT
  v2.16  main (default)  7cc6dab  -              v0.9.1-rc.2         rc open, head on the rc    v0.9.1 (promotes v0.9.1-rc.2) or (v0.9.1-rc.3: no new commits)
  v2.15  release/v2.15   8c61398  v0.8.3         -                   work since release (3)     v0.8.4-rc.1

Local clone
  LINE   BRANCH         UNRELEASED  LOCAL
  v2.15  release/v2.15  3           behind 2

Consumers: rancher (rancher/rancher), build-yaml-bump
  LINE   BRANCH         HEAD     HOLDS                      LATEST STABLE  STATUS
  v2.16  main           e41f0aa  rancher/shell:v0.9.1-rc.2  -              current
  v2.15  release/v2.15  c0ffee1  rancher/shell:v0.8.2       v0.8.3         behind v0.8.3
```

## Releases: what each line has and could have next

The host's facts, from one listing of its tags and branch heads, and the meaning made of them. Nothing here needs a clone.

* **STATE** is where the line stands, from its newest tag of any kind and the head of its branch: `no release`; `released` (the newest tag is stable and the head is on it); `work since release` (stable, head past it, with the count when the clone can say); `rc open, head on the rc` (the newest tag is a prerelease and the head is on it); `rc open, work since the rc`. "On the tag" is the head's commit being the tag's, so the host alone says it. The rules are `releasetag.StateOf` ([`internal/releasetag`](../../internal/releasetag/releasetag.go)).
* **A project whose lines are in its tags** (a `single` layout whose stream lists the lines, such as `rancher-assets`) has a row for each line it lists, all on its one branch, with the newest `stable` tag as the latest stable and a newer `dev` tag as the latest pre-release ([CHAIN_TRACING](CHAIN_TRACING.md)). It has no STATE and no NEXT, since those are for tags a person makes and its tags are made by its own CI.
* **NEXT** is the tags the line could have next, as `gero repo tag` would make them, from `releasetag.Candidates`, so the status and the command agree. When the newest tag is a prerelease **both** are shown, the next iteration and the promotion, since either may be wanted; one that may not be made now stays, in parentheses with the reason (`no new commits`, `the branch has moved since the rc`). After a stable tag it is the next prerelease, when that may be made now, and `-` when it may not. Minor and major alternatives are `repo tag --minor` and `--major`.

## Local clone: whether to fetch

The clone's facts only, as their own section: the commits on the branch since the newest tag, and how the clone's branch compares with the host's head. A missing clone or one that lacks the commits leaves the section with the reason, and is not a failure.

## Sources and Consumers: who has picked a release up

Shown only for a project that takes from others (**Sources**, the projects that ship to it) or ships to others (**Consumers**, the projects it ships to); a project that does neither has neither. Each is a section per project, with a row per consumer branch that the producer's releases go to. They are the same data from the two ends, built from the pieces `gero ship` reads a consumer with, so they agree with what a run would do ([PUSH_TO_REPO](PUSH_TO_REPO.md)): `ship.Holdings` for Consumers and `ship.Sources` for Sources, over the branches and the release `ship` would pick for each, and `Invocation.Check` over the consumer's file on the host.

* **HOLDING** is the holding of the producer's release that the row is about, and **HOLDS** is the value it reads from the consumer's file on that branch. A consumer branch that holds two files of one release has a row for each. A holding with several locations (a module in a repo's root and nested `go.mod` files) shows each distinct value of them, in the order the holding lists them, when they differ, and the one value when they agree.
* **STATUS** compares it with the line's latest stable release and, when a prerelease is newer, with the newest: `current`; `on stable, <tag> is newer`; `on a prerelease, <tag> is newer`; `behind <tag>`; `ahead of <tag>`; `unknown` when a version cannot be compared; `unreadable: <why>` when the branch has no value to read. When the locations of a holding hold different values, `; locations differ` follows it (the rule `locations-differ`), unless the holding's document acknowledges it for that line, which says nothing here. One consumer that cannot be traced is its own line (`unavailable: <why>`) and the others are shown.
* **Cost.** A ref listing for each project and one small file read for each branch, whatever the number of releases it is compared with. They are on by default for a project that ships or takes, and `--no-consumers` and `--no-sources` leave them out. A public repository needs no token.

## JSON and failures

`--json` carries the same sections as `targets` (with `state` and `next`), `consumers` and `sources`. When the host cannot be read the report is printed with what is known and the command exits 1; an edit or a holding that cannot be used is a `traceError` and not a failure of the rest.

## Not built yet

Each has a `TODO(release-state)` at the code where it goes.

* How many releases a branch is behind: the tags of the line's series newer than the version held. It needs the version the update engine found (`ship.Holding`) and a per-series index of tags, which belongs in `internal/tagscheme`, where each scheme reads its own series.
* The open PR for a branch, so a consumer that is behind can say a bump is already proposed (`ship.Holding`). It would be a tier of its own, with its cost, using the PR lookup and the marker `ship` uses.
* The images a release publishes, as `repo images exist` checks them (`repostatus.Options`).

## Open questions

None yet.
