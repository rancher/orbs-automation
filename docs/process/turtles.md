# turtles

Rancher Turtles: the Cluster API integration in Rancher, and the `rancher-turtles` chart it ships in. Hands its chart to `rancher/charts` and its version to `rancher/rancher`; it takes nothing from kuberlr-kubectl. The Highlander team is responsible.

**Status:** draft. **Written:** 2026-11-18, by Claude, from a read of the repo and its release docs; the owner and the kuberlr finding are the ORBS team lead's account the same day.

## What a release is

### Artifact

* **What:** controller image `rancher/turtles`, amd64 and arm64 (a community image and a Prime image); the `rancher-turtles` chart package and a GitHub release
* **Also:** a testing-framework tag and a ClusterClass examples tag, made with each release tag; a separate `rancher-turtles-providers` chart, released with each Turtles version
* **Published to:** Docker Hub for the community image; Prime staging and Prime production registries for the Prime image
* **Provenance attested:** *(unconfirmed)*

### Version

* **Scheme:** semver, `v0.X.Y`
* **Tells you:** the minor is the Rancher line (v0.28 is 2.16, v0.27 is 2.15, v0.26 is 2.14, v0.25 is 2.13); the line is also written in each branch's chart as the Rancher versions it supports
* **Patch:** this project's own

### Stable and prerelease

* **Prerelease:** `-rc.N`, first `-rc.0` for a new line; published as a prerelease; a chart PR is opened for it
* **Stable:** no suffix; marked as the latest release when it is the newest
* **Downstream may take:** both; charts and Rancher both receive release candidates, and the stable replaces the last one

### Stages

None.

## Lines

* **Shape:** one stream for each line
* **Branch:** `release/v0.N` for each line; the minor is N
* **Newest line:** `main` and `release/v0.28` carry the same Rancher range today (2.16); main is the source for the next minor
* **Active lines:** three release branches, one for each Rancher minor under maintenance (v0.25 to v0.28 now)
* **New line:** a release branch is cut from `main` with the first `-rc.0`; a Rancher release branch may not exist yet, so Rancher is first updated on its `main`
* **Ended line:** no more releases; its branch and tags stay; many older `release-0.N` branches are from before the current scheme

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to the line's branch |
| 2 | Prepare | A release is wanted, a new patch RC | The Highlander team | The line's branch | Dependencies, CAPI providers and the providers chart's version brought to the release |
| 3 | Tag | The branch is ready | An engineer on the team | A commit on the branch | Signed, annotated release, testing-framework and examples tags |
| 4 | Publish | A release tag is pushed | Automation | The tag | Images, chart package, GitHub release |
| 5 | Write release notes | The release is published | The team | The generated notes | Notes rewritten for non-developers, marked latest if stable |
| 6 | Chart PR in charts | The release has its chart package | An engineer, by hand | The release, the previous version | PR in `rancher/charts` on the line's `dev` branch |
| 7 | Review and merge the chart PR | A chart PR is open | The team | The chart PR | The release on the charts `dev` branch |
| 8 | PR in Rancher | The chart PR is merged | An engineer, by hand | The release, the previous version | PR in `rancher/rancher`, once for each branch that takes it |
| 9 | Review and merge the Rancher PR | A PR is open | The team | The PR | The release in Rancher's branch |
| 10 | Release the providers chart | The release | An engineer, by hand | The providers chart, tagged with the same version | PR in Prime charts for the providers chart |
| 11 | Docs | A new minor or major | The team | The release notes | Community docs published; a ticket to sync product docs |

* **Another RC:** a change needed after a candidate repeats steps 3 to 9
* **Order:** the charts PR is merged before the Rancher PR is made: Rancher then holds a version that exists in charts
* **Gates:** change checks before merge; the release's GitHub release exists with the chart package before charts is updated; QA signs a candidate before it is un-RC'd *(unconfirmed)*
* **Choices:** when to release; which Rancher branches take it
* **Failure:** *(unconfirmed)*

## What it takes in

None. It used to hold the kuberlr-kubectl image; the chart no longer does, and the release docs still list the image in a bump checklist.

## What it hands on

### To rancher/charts

* **What:** the `rancher-turtles` chart, from each RC and stable
* **Starts when:** a person starts it, after the release is published
* **Done by:** an engineer starts it; automation opens the PR; the team reviews and merges
* **Lines:** each branch to the `dev` branch of its line (v0.28 to 2.16, v0.27 to 2.15, v0.26 to 2.14, v0.25 to 2.13); the target can be worked out from the release branch
* **Chart version:** its own scheme; the major bumps when the Turtles minor does, chosen when the PR is made

### To rancher/rancher

* **What:** the Turtles version Rancher holds, from each RC and stable
* **Starts when:** the chart PR is merged; a person starts it
* **Done by:** an engineer starts it; automation opens the PR; the team reviews and merges
* **Lines:** while Rancher's branch for the line does not exist, `main` only; once it does, its release branch and `main`; for a patch of an older line, only that line's branch, because `main` is already on a newer one

### To rancher/prime-charts

* **What:** the `rancher-turtles-providers` chart, with each Turtles version
* **Starts when:** a person starts it; for a new minor the chart's source branch is first set to the new release branch
* **Done by:** an engineer, by hand; automation opens the PR; the team reviews and merges
* **Lines:** each Turtles line to the matching Prime charts `dev` branch

## Differences between lines

* v0.28 (2.16): the line in development, on `main` and `release/v0.28`
* until v0.27: own release branch only; Rancher's matching release branch exists

## Who is responsible

* **Owns:** the Highlander team, as the repo's code owner
* **Acts:** the team reviews and merges, tags, and starts the hand-offs by hand
* **QA team:** tests a candidate *(unconfirmed)*
* **Release team:** owns promotion of charts from `dev` to the release branch
* **Can step in:** *(unconfirmed)*

## Rules

* The minor is the Rancher line: the version alone says which line a release is for
* Charts is updated before Rancher: Rancher must hold a version that exists in charts
* A patch of an older line goes only to that line's Rancher branch: `main` already follows a newer line
* The tag is signed and annotated, and comes with a testing-framework tag and an examples tag: every release carries them

## Where it stands

* **Latest release of a line:** highest stable tag of the line's minor; the latest candidate is the highest `-rc`, if newer
* **What charts holds:** the chart version on the line's charts `dev` branch
* **What Rancher holds:** the Turtles version in the line's Rancher branch
* **What a PR means:** an open PR in charts or Rancher is a release not yet taken there
* **Not visible from outside:** QA sign-off

## Pain and unknowns

* **Hard today:** the hand-offs are started by hand, with the previous and the new version typed in, in an order that must hold
* **Hard today:** the line-to-branch mapping is partly derived in the automation, partly typed in
* **Not known:** who tags, who signs off a candidate, and how a CVE fix is released
* **Not known:** whether the release docs' kuberlr bump is out of date; the chart no longer holds the image

## Config notes

* No kuberlr holding: not modelled for the kuberlr chain
* One stream for each line, with branches that map to Rancher lines by the chart's own supported range
* The newest line is on two branches at once (`main` and the newest release branch)
* Three hand-offs from one release (charts, Rancher, Prime charts) in a fixed order, each started by hand
* A patch of an older line skips `main` in Rancher; a new line goes first to Rancher's `main`
* The testing-framework and examples tags are made with each release tag: are they artifacts?
* Chart version is its own scheme with the major tied to the Turtles minor; where is that recorded?

## Evidence

* The repo's release docs, release notes guide, release-automation guide, chart, code ownership and tags
* The ORBS team lead, 2026-11-18: the owner, that the kuberlr image has been removed from the chart
* [`backup-restore-operator.md`](backup-restore-operator.md)
* [`compliance-operator.md`](compliance-operator.md)
