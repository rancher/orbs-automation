# rancher-charts

The Helm chart repository Rancher's Apps & Marketplace serves, one set of charts for each line. Takes charts in from many projects, hands its branches to rancher-assets and chart versions to `rancher/rancher`. Release team maintains it; each chart is owned by the team that makes it.

**Status:** draft. **Written:** 2026-11-07, by Claude, from a read of the repo and the existing project note; corrected by the ORBS team lead the same day.

## What a release is

### Artifact

* **What:** a version of a Helm chart; the repo is the collection of them
* **Published to:** Rancher's chart repository
* **Tags:** none; a release is a new chart version in the list of versions on a branch

### Version

* **Scheme:** `CHART+upUPSTREAM`, such as `111.0.0+up12.0.0-rc.4`
* **Tells you:** the chart's major is the Rancher line (111 is 2.16, 110 is 2.15); the part after `+up` is the project's own version
* **Next version:** a net-new version, exactly one more patch or minor than the last released

### Stable and prerelease

* **Prerelease:** the project's version after `+up` has a suffix (`-rc.N`, `-alpha.N`, `-beta.N`)
* **Stable:** no suffix
* **Released:** a stable; an RC is not released
* **Downstream may take:** both; what is on a branch is taken

### Stages

* **dev:** the line's `dev` branch; every chart version lands here, RC or stable
* **stable:** the line's release branch; approved, un-RC'd charts only

## Lines

* **Shape:** one stream for each line, two branches for each (`dev` and release)
* **Branch:** `dev-v2.X` and `release-v2.X`
* **Newest line:** has both branches (v2.16), the release branch holding only what was promoted
* **Active lines:** the lines Rancher supports
* **New line:** the release team creates both branches; only the release team creates branches here
* **Ended line:** no more changes; branches stay

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the chart's owning team reviews | The proposed change | The change merged to `dev` |
| 2 | Chart version added | A project releases | The project's team | The project's release | A new chart version on `dev`, an RC or a stable |
| 3 | Validate | A PR is opened | Automation | The PR | A build check result |
| 4 | Test the RC | The RC is on `dev` | The QA team of the component | The RC | Sign-off to un-RC |
| 5 | Un-RC | Sign-off | The component's engineering team | The RC | The stable version on `dev` |
| 6 | Promote | The release team's own process | Release team | Approved, un-RC'd charts on `dev` | The charts on the release branch; any RC still on `dev` is removed first |

* **Gates:** a PR must pass its build check; the release team promotes only approved, un-RC'd charts
* **Choices:** which charts to promote, and when
* **Missed promotion:** a chart still an RC is removed before promotion; its team prepares an out-of-band release with the release team, or resubmits for the next release
* **Automatic bump and merge:** exists in the repo; the ORBS team does not use it

## What it takes in

### From backup-restore-operator

* **What:** `rancher-backup` and `rancher-backup-crd`, every RC and every stable
* **How it arrives:** a PR on the line's `dev` branch; ORBS engineers review and merge
* **What changes here:** a new chart version; a new RC replaces the previous RC
* **Recorded in:** the list of versions on the branch
* **Lines:** each BRO major goes to its line's `dev` branch

### From kuberlr-kubectl

No direct hand-off. Each chart's team updates the kuberlr image in the chart files in the repo it owns, so its release already carries the right version when it reaches charts.

### From other projects

* Not traced: webhook, remotedialer-proxy, fleet, turtles, operators, monitoring and the rest
* Other teams may use the repo's automatic bump and merge; ORBS does not

## What it hands on

### To rancher-assets

* **What:** the head of the line's `dev` branch and of its release branch
* **Starts when:** Assets pulls them on its own cycle
* **Done by:** automation, Assets' side
* **Lines:** v2.16 and up
* **Stages:** `dev` feeds Assets' dev stage, the release branch feeds its stable stage

### To rancher/rancher

* **What:** the chart version of each chart that is a system chart for Rancher (webhook, fleet, turtles and others), and the charts branch the line follows
* **Starts when:** the chart's version is in charts; charts merges first, so the version is in the index
* **Done by:** the chart's team, who apply the same version in Rancher's build file
* **Lines:** every line
* **Version:** assigned by charts; the team takes it from there

## Differences between lines

* from v2.16: the charts are packaged as the Assets image, a separate image an init container copies to the place Rancher pods expect, so it can be swapped; an offline cluster has only the chart versions in the Assets image it uses; an online cluster refreshes from the internet; the Assets image tag can be overridden when installing Rancher, so offline charts update apart from the main Rancher image
* until v2.15: every chart was cloned into Rancher's own image

## Who is responsible

* **Maintains:** release team; only it creates branches
* **Owns a chart:** the team that makes it (ORBS owns `rancher-backup`)
* **Acts:** project teams and automation add versions; the owning team un-RCs; the release team promotes
* **QA teams:** each tests its own components
* **Can step in:** release team

## Rules

* Never remove a released chart: users may hold it
* An RC may be removed or replaced: it is not released
* A PR changes one chart and its dependencies: one release decision each
* A chart version is a net-new one patch or minor above the last released: versions never repeat
* A PR title states the line, the chart, the version and the action (bump, remove or un-RC)
* Only approved, un-RC'd charts are promoted: the release team's rule
* The chart's major is the Rancher line, assigned by charts: the version alone says which line
* A system chart ships to charts first, then its version goes in Rancher's build file: the version must be in the index first
* An RC left on `dev` at promotion is removed: only un-RC'd charts ship

## Where it stands

* **Latest version of a chart on a line:** highest version in the list on the branch
* **Promoted:** a version on the release branch; on `dev` only means not yet promoted
* **RC or stable:** the suffix after `+up`
* **What a downstream holds:** see the projects' own docs
* **Not visible from outside:** QA sign-off; the release team's promotion decision

## Pain and unknowns

* **Hard today:** each project works out the next chart version on its own
* **Hard today:** PR title formats differ between projects
* **Hard today:** an RC that misses promotion is removed, and its team must go out of band or wait

## Config notes

* One project with two stages, each its own branch, on every line
* There are no tags: a release is a chart version in a list
* Many projects feed it, each into one chart; the edge carries which chart
* A system chart goes to charts first, then its version goes to Rancher's build file; the second step depends on the first
* From v2.16 offline clusters take chart versions from the Assets image in use; an install can override its tag
* The chart's major is the line
* Charts is unowned and managed by the release team, but each chart has an owning team
* An RC is replaced by the next RC; an RC left at promotion is removed by the release team

## Evidence

* The repo's README, pull request template and code owners
* [`../projects/rancher-charts.md`](../projects/rancher-charts.md)
* [`backup-restore-operator.md`](backup-restore-operator.md), [`rancher-assets.md`](rancher-assets.md), [`kuberlr-kubectl.md`](kuberlr-kubectl.md)
