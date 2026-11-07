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

* **STATE** is where the line stands, from its newest tag of any kind and the head of its branch: `no release`; `released` (the newest tag is stable and the head is on it); `work since release` (stable, head past it, with the count when the clone can say); `rc open, head on the rc` (the newest tag is a prerelease and the head is on it); `rc open, work since the rc`. "On the tag" is the head's commit being the tag's, so the host alone says it. The rules are `releasetag.StateOf` ([`internal/git/releasetag`](../../internal/git/releasetag/releasetag.go)).
* **NEXT** is the tags the line could have next, as `gero repo tag` would make them, from `releasetag.Candidates`, so the status and the command agree. When the newest tag is a prerelease **both** are shown, the next iteration and the promotion, since either may be wanted; one that may not be made now stays, in parentheses with the reason (`no new commits`, `the branch has moved since the rc`). After a stable tag it is the next prerelease, when that may be made now, and `-` when it may not. Minor and major alternatives are `repo tag --minor` and `--major`.

## Local clone: whether to fetch

The clone's facts only, as their own section: the commits on the branch since the newest tag, and how the clone's branch compares with the host's head. A missing clone or one that lacks the commits leaves the section with the reason, and is not a failure.

## Sources and Consumers: who has picked a release up

Shown only for a project that takes from others (**Sources**, the projects that ship to it) or ships to others (**Consumers**, the projects it ships to); a project that does neither has neither. Each is a section per project, with a row per consumer branch that the producer's releases go to. They are the same data from the two ends, built from the pieces `gero ship` reads a consumer with, so they agree with what a run would do ([PUSH_TO_REPO](PUSH_TO_REPO.md)): `ship.Holdings` for Consumers and `ship.Sources` for Sources, over the branches and the release `ship` would pick for each, and `Invocation.Check` over the consumer's file on the host.

* **HOLDS** is the value the update method reads from the consumer's file on that branch.
* **STATUS** compares it with the line's latest stable release and, when a prerelease is newer, with the newest: `current`; `on stable, <tag> is newer`; `on a prerelease, <tag> is newer`; `behind <tag>`; `ahead of <tag>`; `unknown` when a version cannot be compared; `unreadable: <why>` when the branch has no value to read. One consumer that cannot be traced is its own line (`unavailable: <why>`) and the others are shown.
* **Cost.** A ref listing for each project and one small file read for each branch, whatever the number of releases it is compared with. They are on by default for a project that ships or takes, and `--no-consumers` and `--no-sources` leave them out. A public repository needs no token.

## JSON and failures

`--json` carries the same sections as `targets` (with `state` and `next`), `consumers` and `sources`. When the host cannot be read the report is printed with what is known and the command exits 1; an update method that cannot be used is a `traceError` and not a failure of the rest.

## Not built yet

Each has a `TODO(release-state)` or `TODO(releasetag)` at the code where it goes.

* How many releases a branch is behind, which needs the tags between two releases and the version the strategy found (`ship.Holding`, and `git.LatestReleases` for the per-series index).
* The open PR for a branch, so a consumer that is behind can say a bump is already proposed (`ship.Holding`). It would be a tier of its own, with its cost, using the PR lookup and the marker `ship` uses.
* The images a release publishes, as `repo images exist` checks them (`repostatus.Options`).
* `ship`'s own `ResolveRelease`, `latest` and `lineSeries` folded into `internal/git/releasetag`, so a line's "latest" has one definition (`ship/release.go`).

## Open questions

* Whether a project that is both a producer and a consumer, such as one that takes from another and ships on, should show the Sources and Consumers sections together or only the one asked for.
