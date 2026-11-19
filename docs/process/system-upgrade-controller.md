# system-upgrade-controller

SUC: the controller that applies system-level upgrades to nodes, and the chart Rancher ships it in. Takes one image from kuberlr-kubectl, is taken by `rancher/charts`. The hostbusters team owns it.

**Status:** draft. **Written:** 2026-11-18, by Claude, from a read of the repo and of `rancher/charts`; the version table, the owner and the chart's place are the ORBS team lead's account the same day.

## What a release is

### Artifact

* **What:** controller image `rancher/system-upgrade-controller`, for amd64, arm64 and arm; binaries and checksums on the GitHub release
* **Also:** the chart in the repo's `charts/` directory, which is staged for Rancher from a stable release
* **Published to:** the image registries and a GitHub release; the release is made as a draft, then published once the images are pushed
* **Provenance attested:** *(unconfirmed)*

### Version

* **Scheme:** semver, `v0.X.Y`
* **Tells you:** the minor goes with a Rancher line, but not by a rule: the team keeps a table (v0.20 is 2.15, v0.19 is 2.14, v0.17 is 2.13, v0.16 is 2.12), which is out of date: v0.21 is 2.16, checked against the releases
* **Patch:** this project's own

### Stable and prerelease

* **Prerelease:** `-rc.N`; published as a prerelease on GitHub
* **Stable:** no suffix; the only kind that can be staged into charts
* **Downstream may take:** stable only

### Stages

None.

## Lines

* **Shape:** one branch for every line; the version table pairs a release series with each line
* **Branch:** the default branch is `master`, and it is the line in development; an older release branch exists for a long-ended series *(unconfirmed)*
* **Newest line:** `master`, tracking v2.16 today; the chart in it is for that Rancher line
* **Active lines:** lines from v2.12 take a series from the table; v2.15 down have their chart in `rancher/charts`, not here
* **New line:** the controller's minor and the chart's Rancher range are moved on `master`; whether a release branch is cut is *(unconfirmed)*
* **Ended line:** no more releases for the series; tags stay

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to `master` |
| 2 | Tag | A release is wanted | The hostbusters team *(unconfirmed which person)* | A commit | `-rc.N` or stable tag |
| 3 | Publish | A tag is pushed, on any branch | Automation | The tag | Build and tests, e2e, images, draft release, then the release published |
| 4 | Stage the chart | A stable release is published | The hostbusters team, by hand, from `master` | The stable release | The chart staged for Rancher, and a PR in `rancher/charts` |
| 5 | Review and merge the chart PR | A chart PR is open | *(unconfirmed)* | The chart PR | The chart on the charts staging branch |
| 6 | Promote in charts | The release team's own process | Release team | Approved charts | The charts on the charts release branch |

* **Chart staging:** only from the default branch and only for a stable release that already exists; the chart used is the one in `charts/` of `master`, and the release supplies only the controller version
* **Gates:** change checks before merge; tests and e2e pass before images are published; the release is stable and published before its chart is staged
* **Choices:** when to release; which release to stage
* **Failure:** *(unconfirmed)*

## What it takes in

### From kuberlr-kubectl

* **What:** image tag, the major of the line `master` tracks
* **How it arrives:** a change merged to `master` *(unconfirmed that a person, not automation, does it)*
* **What changes here:** the image tag in the chart's values; the controller's own image tag is set when the chart is staged
* **Recorded in:** chart values on `master`
* **Lines:** only the line `master` tracks; the team keeps the file at the newest kuberlr release matching that line

## What it hands on

### To rancher/charts

* **What:** the `system-upgrade-controller` chart, from a stable release
* **Starts when:** a person starts the staging of a stable release
* **Done by:** the hostbusters team starts it; automation stages the chart and opens the PR
* **Lines:** the newest line, from `master`; older lines have their chart customised in `rancher/charts` and take no hand-off from here
* **Chart version:** plain, `111.Z.W`, worked out when staged; the major follows the Rancher line

## Differences between lines

* v2.16: chart from `master`, staged by hand
* until v2.15: chart held in `rancher/charts`, customised there

## Who is responsible

* **Owns:** the hostbusters team, as the repo's code owner
* **Acts:** *(unconfirmed)*
* **Release team:** owns promotion of charts to the release branch
* **Can step in:** *(unconfirmed)*

## Rules

* Only a stable release is staged into charts: charts never takes a prerelease
* Staging runs from the default branch: the chart and script used are the branch's
* The kuberlr tag on `master` is the newest release of the major for the line `master` tracks: the team's rule

## Where it stands

* **Latest release of a line:** highest stable tag with the series the table gives the line
* **What charts holds:** the chart on the line's charts branch; its appVersion says the controller release
* **What a PR means:** an open chart PR is a stable release staged and not yet in charts
* **Not visible from outside:** who stages a release; the version table is not in the repo

## Pain and unknowns

* **Hard today:** the line-to-series table is kept by the team, outside the repo
* **Hard today:** the chart of older lines is customised in `rancher/charts`, so this repo says nothing about it
* **Not known:** the series for v2.17
* **Not known:** whether any release branch is still used

## Config notes

* One branch for every line, yet a series for each line: a single-layout project with a stream that has lines
* The kuberlr holding is for one line only: the line `master` tracks. When `master` moves to a new line, the holding's line moves with it
* Chart staging is by hand and from a stable release; not a send
* The older lines' chart is held in `rancher/charts`, not here
* Chart version is plain, with the major following the line; where is that recorded?

## Evidence

* The repo's chart, release and chart-staging workflows, tags and releases
* `rancher/charts`: the package definition for the chart
* The ORBS team lead, 2026-11-18: the version table, hostbusters as code owner, the chart customised in charts for older lines, `master` tracking v2.16
* [`kuberlr-kubectl.md`](kuberlr-kubectl.md)
* [`compliance-operator.md`](compliance-operator.md)
