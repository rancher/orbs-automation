# rancher-assets

Builds the image that bundles the Helm chart repos for air-gapped Rancher installs. Takes charts in from `rancher/charts`, hands its image to `rancher/rancher`. Release team owns it; ORBS team helps keep its code correct.

**Status:** draft. **Written:** 2026-11-07, by Claude, from a read of the repo; corrected by the ORBS team lead the same day.

## What a release is

### Artifact

* **What:** container image `rancher/rancher-assets`
* **Contains:** a snapshot of `rancher/charts`, `partner-charts` and `rke2-charts` at pinned commits
* **Also:** a GitHub release
* **Used as:** an init container copies the charts to the place Rancher pods expect; the image can be swapped
* **Published to:** Docker Hub, Prime staging; stable stage also Prime production
* **GitHub registry:** CI testing and dev only, never a release

### Version

* **Scheme:** calendar, `v{line}-{YYYYMMDD}T{HHMM}Z`, with `-dev` for the dev stage
* **Tells you:** the Rancher line, the stage, and the commit time (UTC, to the minute)
* **Newest:** greatest timestamp among tags of the same line and stage
* **One tag:** for each commit, line and stage

### Stable and prerelease

* **Prerelease:** the dev stage, tag ends `-dev`; GitHub release marked prerelease
* **Stable:** the stable stage, no suffix
* **Downstream may take:** both; Rancher holds either, and neither is wrong

### Stages

* **dev:** built from the charts `dev` branch of the line
* **stable:** built from the charts release branch of the line

## Lines

* **Shape:** lines only in the tags; one branch (main) for every line
* **Active lines:** v2.16 and up; the project's config lists them
* **Earlier lines:** not used; tags remain
* **New line:** an entry is added for the line, naming its charts branches *(unconfirmed: who)*
* **Ended line:** the entry is disabled; tags stay

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to main |
| 2 | Tag | A merge changed a line's inputs | Automation | The merge | A tag for each changed line: stable stage if the charts release-branch commit moved, else dev |
| 3 | Tag on demand, exceptional | A tag is wanted that step 2 did not make | A named engineer (two today), or an on-demand run that tags only if one is needed | A commit | A tag of the chosen line and stage |
| 4 | Publish | A tag is pushed | Automation | The tag | Image in the registries of its stage, GitHub release |
| 5 | Hand-off PR | The build has finished | Automation | The release | PR in `rancher/rancher` (see hand-off) |

* **Cadence:** automatic end to end, on a regular cycle; no one acts
* **Automation does:** proposes and approves the bump, tags after merge, publishes from the tag, opens the PR to Rancher once the build finishes
* **Gates:** a pull request must build every image it changes before merge
* **Choices:** a stage that step 2 did not choose: stable stage for another reason, a rebuild with no change, an older commit, an emergency
* **Stable and dev together:** step 2 makes only the stable tag; the dev tag is made by hand if needed
* **Failure:** a rebuild with no change needs a new commit, because a tag exists for each commit, line and stage

## What it takes in

### From rancher/charts

* **What:** the head of the line's dev branch and of its release branch, for the dev and stable stages
* **How it arrives:** pulled on a regular cycle, as a change like any other
* **What changes here:** the pinned commits
* **Recorded in:** the lock file, the source of truth for which commits an image holds
* **Lines:** each line names its own charts branches
* **Also:** `partner-charts` and `rke2-charts` come in the same way, from their main branch for both stages; not traced

## What it hands on

### To rancher/rancher

* **What:** the image tag
* **Starts when:** a release's build has finished, of either stage
* **Done by:** automation opens the PR; Rancher's own automation approves and merges
* **Lines:** each line goes to Rancher's branch for that line
* **Holds:** one Assets tag for each line, of either stage; neither is wrong

## Differences between lines

* from v2.16: used
* until v2.15: not used; tags remain

## Who is responsible

* **Owns:** release team
* **Helps:** ORBS team, on the code
* **Acts:** automation does a release from charts to Rancher; a person tags by hand only in exceptions
* **Charts promotion:** the release team promotes charts from `dev` to the release branch; the next cycle then makes a stable-stage tag
* **Can step in:** release team, ORBS team

## Rules

* A stable-stage tag is made when the charts release-branch commit moved: that is what a stage means
* Any other change makes a dev-stage tag: only a charts release commit makes a stable
* Neither stage in Rancher is wrong: judging it needs release calendar state, which is excluded
* A tag's time is its commit's time, not the build time: the same commit gives the same stamp
* A tag is one image of one line and stage: one tag for each commit, line and stage
* Each image is a complete snapshot at pinned commits: nothing changes after the tag

## Where it stands

* **Latest release of a line and stage:** greatest timestamp among tags with that line and stage
* **Charts commits in an image:** the lock file at the tag
* **What Rancher holds:** the Assets tag in Rancher's `build.yaml` on the line's branch
* **Not visible from outside:** chart versions inside an image, only commits; whether the stage Rancher holds is the right one (the release calendar, not modelled)

## Pain and unknowns

* **Hard today:** the repo's docs disagree with the config and each other (earlier lines shown as active)
* **Hard today:** the stage is only in the tag name
* **Hard today:** no tool picks the newest tag of a line and stage
* **Not known:** who adds a line

## Config notes

* Lines are only in the tags: a project of its own shape
* Stage is decided by which input moved. A rule of the project, or of the process?
* Stages are named as in charts, `dev` and `stable`. Edge stages match by name. The code and CHAIN_TRACING call the second stage `release`; it becomes `stable`.
* The edge from charts is a pull of branch heads, not a send. Edge wording?
* The edge to Rancher is observed, with no stage: Rancher holds either
* Each line names its own charts branches, in the project's own config file
* A hand-made tag is a variant of the release path

## Evidence

* The repo's README, versioning notes and release team guide
* The ORBS team lead, 2026-11-07: ownership, active lines, registries, the stage Rancher holds
* [`../projects/rancher-assets.md`](../projects/rancher-assets.md)
* [`../specs/CHAIN_TRACING.md`](../specs/CHAIN_TRACING.md)
