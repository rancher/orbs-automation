# shell

The image Rancher uses to install charts and to run the `kubectl` shell in the UI. Takes nothing in from another project, hands its image to `rancher/rancher`. ORBS team responsible.

**Status:** draft. **Written:** 2026-11-07, by Claude, from a read of the repo; corrected by the ORBS team lead the same day.

## What a release is

### Artifact

* **What:** container image `rancher/shell`
* **Contains:** `kuberlr`, `kubectl` for each Kubernetes minor of the line, Helm, `k9s`, and shell tools
* **Published to:** Docker Hub, Prime staging; stable also Prime production
* **Signed:** yes

### Version

* **Scheme:** semver, `v0.MINOR.PATCH`
* **Tells you:** minor is the Rancher line (v0.9 is 2.16, v0.8 is 2.15, v0.7 is 2.14, v0.6 is 2.13, v0.5 is 2.12)
* **Patch:** this project's own

### Stable and prerelease

* **Prerelease:** `-rc.N`
* **Stable:** no suffix; also Prime production
* **Downstream may take:** both; every release is sent to Rancher
* **Head image:** each merge to main or a release branch

### Stages

None.

## Lines

* **Shape:** one stream for each line
* **Branch:** `release/v2.X` for each released line, named for the Rancher line
* **Newest line:** built from main, no release branch yet (v2.16)
* **Active lines:** the lines Rancher supports
* **New line:** normally after Rancher's release team makes the new Rancher branch; the ORBS team may prepare it earlier; main moves to the next minor
* **Ended line:** no more releases; tags stay; its last release lands at its end of life (v0.4, 2.11)

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to the line's branch |
| 2 | Decide to release | Changes worth releasing are on the branch, often a round across lines | ORBS team | The branch | The decision |
| 3 | Tag a prerelease | The decision | A person | A commit on the branch | `-rc.N` tag |
| 4 | Publish | A tag is pushed | Automation | The tag | Image in the registries of its kind, signature |
| 5 | Send to Rancher | The image is published | Automation | The release | PR in `rancher/rancher` on the line's branch |
| 6 | Test the RC | The RC is in Rancher | ORBS QA team | The RC in Rancher | Sign-off to un-RC |
| 7 | Un-RC and tag the stable | Sign-off | An engineer on the ORBS team | The RC's commit | Stable tag |
| 8 | Publish the stable | A stable tag is pushed | Automation | The tag | As step 4, plus Prime production; PR in Rancher for the stable |

* **Another RC:** a change needed after step 3 repeats steps 3 to 6
* **Patch or CVE stable:** a new stable after a stable, with no RC; QA smoke test optional, never a blocker
* **Gates:** change checks before merge; QA sign-off before a stable that has an RC
* **Choices:** when to release; which lines (often all in one round)
* **Failure:** a failed publish leaves the release in draft; nothing downstream starts; an engineer fixes it and re-tags

## What it takes in

None.

## What it hands on

### To rancher/rancher

* **What:** the image tag
* **Starts when:** any release is published, RC or stable
* **Done by:** automation opens the PR; ORBS engineers review and merge
* **Lines:** each shell minor goes to the matching Rancher line's branch, and no other
* **Holds:** Rancher keeps an RC until the stable replaces it

### To charts

None. Charts use kuberlr-kubectl; only Rancher uses shell.

## Differences between lines

* from v2.16: built from main, no release branch yet
* until v2.15: own release branch
* v2.11: ended

## Who is responsible

* **Owns:** ORBS team
* **Acts:** the team reviews, merges, decides and tags; automation publishes and opens the PRs
* **QA team:** the ORBS QA team tests an RC in Rancher and signs off; engineers then un-RC
* **Can step in:** ORBS team

## Rules

* Minor version is the Rancher line: the version alone says which line a release is for
* Each line is sent only to its own Rancher branch: a line's image is not for another line
* Only stable reaches Prime production: production never holds an `-rc`
* Bundled `kubectl` versions follow the line's Kubernetes range: Kubernetes allows version drift
* A stable follows an RC of its version, QA signed off: stable is what was tested
* Exception: a new stable after a stable (patch, CVE) needs no RC: speed, and the change is small
* Even if automation restricts stables to follow an RC, a new stable after a stable must still be possible

## Where it stands

* **Latest release of a line:** highest stable tag with the line's minor; latest prerelease is the highest `-rc`, if newer
* **What Rancher holds:** the tag in Rancher's `build.yaml`, on each line's branch
* **Not visible from outside:** QA sign-off

## Pain and unknowns

* **Hard today:** same change made on every line's branch
* **Hard today:** the line mapping lives in several places

## Config notes

* Series per line is one number: the minor. A function of the line, like kuberlr's major.
* Every release is sent to Rancher, RC included. SCC sends an RC only on request. Per-edge setting?
* Each shell line goes to one Rancher branch. SCC goes to all. Edge reach differs by project.
* The `kubectl` list per branch equals the line's Kubernetes range, again
* Only Rancher consumes shell, so one edge
* A stable may be restricted to follow an RC, with a stable-after-stable path for patches and CVE fixes. Per project?

## Evidence

* The repo's README
* [`../projects/rancher-shell.md`](../projects/rancher-shell.md)
* [`config/shell.project.yaml`](../../config/shell.project.yaml)
* [`rancher.md`](rancher.md), [`scc-operator.md`](scc-operator.md)
