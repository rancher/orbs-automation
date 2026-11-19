# compliance-operator

Rancher Compliance: the operator and the `rancher-compliance` chart that runs compliance scans (CIS and similar) on a cluster. Takes one image from kuberlr-kubectl, is taken by `rancher/charts`. The security team and the Infracloud team own it together.

**Status:** draft. **Written:** 2026-11-18, by Claude, from a read of the repo and of `rancher/charts`; the branch-to-line table and the owners are the ORBS team lead's account the same day.

## What a release is

### Artifact

* **What:** operator image `rancher/compliance-operator`, amd64 and arm64, each also published on its own with an architecture suffix
* **Also:** the chart and its CRDs (the `rancher-compliance` and CRD charts) are sources in the repo, not packaged on the release; assets are uploaded to the GitHub release
* **Published to:** Docker Hub and Prime registry, for every tag
* **Provenance attested:** *(unconfirmed)*

### Version

* **Scheme:** semver
* **Tells you:** major and minor are the Rancher line (v1.6 is 2.16, v1.5 is 2.15, v1.4 is 2.14, v1.3 is 2.13, v1.2 is 2.12); the minor is the line's number minus ten
* **Patch:** this project's own

### Stable and prerelease

* **Prerelease:** `-rc.N`; published like any release; marked as a full release on GitHub, not as a prerelease *(unconfirmed)*
* **Stable:** no suffix; published the same way
* **Downstream may take:** both; the chart is built from the line's branch and the charts `dev` branch receives every RC and every stable
* **Head image:** none known

### Stages

None.

## Lines

* **Shape:** one stream for each line
* **Branch:** `release/v1.N` for each released line
* **Newest line:** built from main, no release branch yet (v2.17)
* **Active lines:** the lines Rancher supports (v1.2 to v1.6, and v2.17 on main); a release branch for an ended line is not tracked
* **New line:** a `release/v1.N` branch is cut, and the repo's branch table is updated to add it and drop what is no longer needed
* **Ended line:** no more releases; tags stay; its branch is not tracked
* **Dependencies by line:** the same Kubernetes minor as the Rancher line, and a matching security-scan release

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to the line's branch |
| 2 | Tag | A release is wanted, often a round across lines | An engineer *(unconfirmed which team)* | A commit on the branch | `-rc.N` or stable tag |
| 3 | Publish | A tag is pushed | Automation | The tag | Image for each architecture, the manifest, GitHub release, assets |
| 4 | Take into charts | A release is published | Automation in `rancher/charts` | The release's branch | PR in `rancher/charts` on the line's `dev` branch |
| 5 | Review and merge the chart PR | A chart PR is open | Chart team *(unconfirmed)* | The chart PR | The release on the charts `dev` branch |
| 6 | Promote in charts | The release team's own process | Release team | Approved, un-RC'd charts on `dev` | The charts on the charts release branch |

* **Another RC:** a change needed after the first RC repeats steps 2 to 5
* **Stable tag:** whether it needs QA sign-off, and whether it is tagged on the last RC's commit, is *(unconfirmed)*
* **Gates:** change checks before merge; tests pass before anything is published *(unconfirmed)*
* **Choices:** when to release; which lines
* **Failure:** *(unconfirmed)*

## What it takes in

### From kuberlr-kubectl

* **What:** image tag, the line's major
* **How it arrives:** a change merged to the line's branch *(unconfirmed that a person, not automation, does it)*
* **What changes here:** the image tag in the chart's values, and the same image written again in the chart's image list in its metadata
* **Recorded in:** chart values on the line's branch; the chart metadata on the line's branch
* **Lines:** each takes the matching kuberlr major; branches are not always on the newest release of it

## What it hands on

### To rancher/charts

* **What:** the `rancher-compliance` chart and its CRDs, from each RC and each stable
* **Starts when:** a release is published; charts pulls it from the line's branch
* **Done by:** automation in charts opens the PR; the chart team reviews and merges *(unconfirmed)*
* **Lines:** each branch goes to the `dev` branch of its line (v1.6 to 2.16, v1.5 to 2.15, v1.4 to 2.14, v1.3 to 2.13, v1.2 to 2.12); main has no charts branch yet
* **Chart version:** its own scheme, `{chart}+up{release}`; the chart's major follows the Rancher line

## Differences between lines

* v2.17: built from main, no release branch, no charts `dev` branch yet
* until v2.16: own release branch
* v2.16: release candidates only so far

## Who is responsible

* **Owns:** the security team and the Infracloud team together; the repo's code ownership names the security team for everything, and both for the chart, the CRDs, the image build file, dependencies and tests
* **Acts:** *(unconfirmed)*
* **Release team:** owns promotion of charts from `dev` to the release branch
* **Can step in:** *(unconfirmed)*

## Rules

* Major and minor are the Rancher line: the version alone says which line a release is for
* Release branches take bug fixes and security-related dependency bumps only: active development is on main
* The line's Kubernetes minor is the Rancher line's: compatibility across the ecosystem
* Only approved, un-RC'd charts are promoted from `dev`: the release team's rule

## Where it stands

* **Latest release of a line:** highest tag with the line's minor; latest prerelease is the highest `-rc`, if newer
* **What charts holds:** chart version on the line's charts `dev` branch, `{chart}+up{release}`; on the release branch once promoted
* **What a PR means:** an open chart PR is a release not yet in charts
* **Not visible from outside:** who signs off a release; publish status in private registries

## Pain and unknowns

* **Hard today:** the repo's branch table stops at v1.4; later lines are only known from branch names
* **Hard today:** the kuberlr tag is written twice in the chart, and the two can disagree
* **Not known:** which of the two owning teams tags and publishes a release
* **Not known:** whether QA signs off a release candidate before a stable, and whether a CVE fix takes a shortcut
* **Not known:** the meaning of the security-scan release the table pairs with each line; not part of this config for now

## Config notes

* Takes kuberlr by line; the hand-off is a change here, not a send
* The kuberlr tag is in two places in the chart: a values key and an entry in the metadata's image list, where the tag is part of an image reference in a list. Can a holding address that?
* A line gets its own `release/v1.N` branch; main is the newest line (v2.17) while the charts side has no branch for it
* Charts takes the release by its own pull, not a send; the hand-off direction is the reverse of BRO's
* Ended lines (v1.0 and before v1.2) are left out
* Chart version is its own scheme, derived from this project's; where is that recorded?
* Two teams own it together, and a project has one managing team: a joint slug stands for both for now, and a list of teams is planned

## Evidence

* The repo's README, release workflow, chart and code ownership file; its tags and releases
* `rancher/charts`: the package definition on each line's `dev` branch, and its recent chart PRs
* The ORBS team lead, 2026-11-18: the branch-to-line table, v1.5 and v1.6, main is v2.17, ended lines left out, security-scan out of scope
* [`kuberlr-kubectl.md`](kuberlr-kubectl.md)
* [`backup-restore-operator.md`](backup-restore-operator.md)
