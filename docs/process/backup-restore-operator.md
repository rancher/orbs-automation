# backup-restore-operator

Rancher Backup: the operator and the chart that backs up and restores Rancher. Takes one image from kuberlr-kubectl, hands its chart to `rancher/charts`. ORBS team responsible.

**Status:** draft. **Written:** 2026-11-07, by Claude, from a read of the repo; corrected by the ORBS team lead the same day.

## What a release is

### Artifact

* **What:** operator image `rancher/backup-restore-operator`, amd64 and arm64
* **Also:** `rancher-backup` and `rancher-backup-crd` chart packages, operator binaries and `bro-tool`, all on the GitHub release
* **Published to:** Docker Hub, Prime staging; stable also Prime production
* **Provenance attested:** yes

### Version

* **Scheme:** semver
* **Tells you:** major is the Rancher line (v12 is 2.16, v11 is 2.15, v10 is 2.14, v9 is 2.13, v8 is 2.12)
* **Minor and patch:** this project's own

### Stable and prerelease

* **Prerelease:** `-rc.N`; published like any release; chart PR opened for it
* **Stable:** no suffix; also Prime production
* **Downstream may take:** both; `rancher/charts` receives every RC and every stable
* **Head image:** each merge to a release branch or main; Docker Hub and GitHub registry (varies across ORBS projects)

### Stages

None.

## Lines

* **Shape:** one stream for each line
* **Branch:** `release/vN.x` for each released line; major is N
* **Newest line:** built from main, no release branch yet (v12)
* **Active lines:** the lines Rancher supports (v8 to v11, and v12 on main)
* **New line:** normally after Rancher's release team makes the new Rancher branch; the ORBS team may prepare it earlier; main moves to the next major
* **Ended line:** no more releases; tags stay; follows Rancher's end of life; its last release lands at the end of life, and dates sometimes move (v7, 2.11, has ended)
* **Patch branch:** `release/vX.Y.Z`, cut from a tag when Rancher is in code freeze; for that one version only; backports in, nothing forward

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to the line's branch |
| 2 | Decide to release | Changes worth releasing are on the branch, often a round across lines | ORBS team | The branch | The decision |
| 3 | Tag a prerelease | The decision | ORBS team | A commit on the branch | `-rc.N` tag |
| 4 | Publish | A tag is pushed | Automation | The tag | Tests, image, chart packages, binaries, attestation, GitHub release |
| 5 | Chart PR | The release is published | Automation | The release | PR in `rancher/charts` |
| 6 | Review and merge the chart PR | A chart PR is open | ORBS team | The chart PR | The RC on the charts `dev` branch |
| 7 | Test the RC | The RC is on the charts `dev` branch | QA team | The RC in charts | Sign-off to un-RC, or a reason not to |
| 8 | Tag the stable, by hand | Sign-off | An engineer on the ORBS team | The latest RC's commit | Stable tag on that commit |
| 9 | Publish the stable | The stable tag is pushed | Automation | The tag | As step 4, plus Prime production; chart PR for the stable |
| 10 | Review and merge the stable chart PR | A chart PR is open | ORBS team | The chart PR | The stable on the charts `dev` branch |
| 11 | Promote in charts | The release team's own process | Release team | Approved, un-RC'd charts on `dev` | The charts on the charts release branch |

* **Another RC:** a change needed after step 3 repeats steps 3 to 7
* **CVE fix:** a fix PR is merged, then a new stable is tagged and published from steps 8 to 11, with no RC; QA may run a final smoke test for regressions, not always, never a blocker
* **Automated stable tag:** an attempt to replace step 8; tried on one line only; not the process yet
* **Gates:** change checks before merge; tests pass before anything is published; QA sign-off before a stable that has an RC
* **Choices:** when to release; which lines; when a patch branch is needed
* **Failure:** a failed publish leaves the release in draft; nothing downstream starts; an engineer fixes it and re-tags

## What it takes in

### From kuberlr-kubectl

* **What:** image tag, the line's major
* **How it arrives:** a change merged to the line's branch
* **What changes here:** the image tag in the chart's values
* **Recorded in:** chart values on the line's branch
* **Lines:** each takes the matching kuberlr major; `main` holds the wrong one today, tracked and to be fixed by hand

## What it hands on

### To rancher/charts

* **What:** `rancher-backup` and `rancher-backup-crd` charts from each RC and each stable
* **Starts when:** a release is published
* **Done by:** automation opens the PR; the ORBS team reviews and merges
* **Lines:** each BRO major goes to the `dev` branch of its line (12 to 2.16, 11 to 2.15, 10 to 2.14, 9 to 2.13, 8 to 2.12)
* **Never blocked:** the charts `dev` branch is never frozen; the release team controls what lands from `dev` in the release branch
* **Promotion to the charts release branch:** the release team, from the approved, un-RC'd charts on `dev`
* **Chart version:** its own scheme; patch, minor or major bump follows BRO's; none from RC to RC or RC to stable
* **Replaces:** a new RC or the stable removes the previous RC of the same version from charts

## Differences between lines

* from v2.16: built from main, no release branch yet
* until v2.15: own release branch
* v2.11: ended

## Who is responsible

* **Owns:** ORBS team
* **Acts:** the team reviews and merges changes and chart PRs, decides, and tags RCs and stables; automation publishes and opens chart PRs
* **QA team:** tests an RC in charts, signs off to un-RC
* **Release team:** owns promotion of charts from `dev` to the release branch
* **Can step in:** ORBS team

## Rules

* Major version is the Rancher line: the version alone says which line a release is for
* A stable is tagged on the latest RC's commit after QA sign-off: stable is exactly what was tested; a CVE fix is the exception
* Exception: a CVE fix may go straight to a new stable with no RC; QA smoke test optional, never a blocker: speed, and the change is small
* Only stable reaches Prime production: production never holds an `-rc`
* Previous RC leaves charts when the next arrives: charts holds one RC of a version
* Only approved, un-RC'd charts are promoted from `dev`: the release team's rule
* Patch branches take backports only: the fix is proven on the main line first

## Where it stands

* **Latest release of a line:** highest stable tag with the line's major; latest prerelease is the highest `-rc`, if newer
* **What charts holds:** chart version on the line's charts `dev` branch, `{chart}+up{BRO version}`; on the release branch once promoted
* **What a PR means:** an open chart PR is an RC or stable not yet in charts
* **Not visible from outside:** QA sign-off; publish status in private registries

## Pain and unknowns

* **Hard today:** same change made on every line's branch
* **Hard today:** the line mapping lives in several places, and repo docs disagree with it
* **Hard today:** main holds kuberlr v8, the line maps to v9 (tracked elsewhere)

## Config notes

* Chart PR targets one branch for each line; the lines data has no branches. Project config?
* Charts' `dev` and `stable` are different stages, owned by different teams. The hand-off stops at `dev`.
* The stable depends on QA sign-off, not on anything in config. How does a view show it?
* A CVE stable has no RC and optional QA. A variant of the release path; known to config?
* A line's last release lands at its end of life, and release dates move. Refusing every tag after the end date would be wrong.
* Chart version is its own scheme, derived from BRO's. Where is that recorded?
* Patch branches are a variant of a line's branch. Config needed?
* Takes kuberlr by line; the hand-off is a change here, not a send

## Evidence

* The repo's README, version notes, release docs and patch branch guide
* The ORBS team lead, 2026-11-07: charts review and promotion, QA sign-off, hand-made stable tags, CVE stables, head image registries, v7 ended
* [`../projects/backup-restore-operator.md`](../projects/backup-restore-operator.md)
* [`kuberlr-kubectl.md`](kuberlr-kubectl.md)
* [`config-mock/backup-restore-operator.project.yaml`](../../config-mock/backup-restore-operator.project.yaml)
