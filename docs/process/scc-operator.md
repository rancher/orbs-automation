# scc-operator

The Rancher SCC operator: registers Rancher with SUSE Customer Center. Takes nothing in from another project, hands its image to `rancher/rancher`. ORBS team responsible.

**Status:** draft. **Written:** 2026-11-07, by Claude, from a read of the repo; corrected by the ORBS team lead the same day.

## What a release is

### Artifact

* **What:** operator image `rancher/scc-operator`, amd64 and arm64
* **Also:** operator binaries on the GitHub release
* **Published to:** Docker Hub, Prime staging; stable also Prime production
* **Provenance attested:** binaries
* **Not published:** a chart

### Version

* **Scheme:** semver, `v0.MINOR.PATCH`
* **Tells you:** nothing about a Rancher line
* **Minor and patch:** this project's own

### Stable and prerelease

* **Prerelease:** `-rc.N`; also test builds of a pull request (`v0.0.0-pr.N.bM`)
* **Stable:** no suffix; also Prime production
* **Downstream may take:** stable goes to Rancher by itself; an RC only when a person sends it
* **Head image:** each merge to main

### Stages

None.

## Lines

* **Shape:** one stream for every line (universal); one branch (main)
* **Line used:** the same image on every active Rancher line; Rancher's own file says which tag each branch holds
* **Compatibility:** the operator reads Rancher's version when it runs; no matrix is kept here
* **New line:** its Rancher branch starts receiving stables once Rancher has the branch; the ORBS team adds it
* **Ended line:** its Rancher branch stops receiving stables; the ORBS team removes it

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to main |
| 2 | Decide to release | Changes worth releasing are on main | ORBS team | Main | The decision |
| 3 | Tag a prerelease | The decision | A person | A commit on main | `-rc.N` tag |
| 4 | Publish | A tag is pushed | Automation | The tag | Tests, image, binaries, attestation, GitHub release |
| 5 | Send the RC to Rancher | A person wants it tested in Rancher | A person | The RC tag, chosen Rancher branches | PRs in `rancher/rancher` |
| 6 | Test the RC | The RC is in Rancher | ORBS QA team | The RC in Rancher | Sign-off to un-RC, or a reason not to |
| 7 | Tag the stable | Sign-off | An engineer on the ORBS team | The RC's commit | Stable tag |
| 8 | Publish the stable | A stable tag is pushed | Automation | The tag | As step 4, plus Prime production |
| 9 | Hand-off PRs | The stable release is published | Automation | The release | A PR in `rancher/rancher` for every active line's branch |

* **Another RC:** a change needed after step 3 repeats steps 3 to 6
* **Gates:** change checks before merge; tests pass before anything is published; a stable follows an RC of its version, QA signed off; not enforced today, and a CVE patch may skip it
* **Choices:** when to release; which Rancher branches get an RC
* **Failure:** a failed publish leaves the release in draft; nothing downstream starts; an engineer fixes it and re-tags

## What it takes in

None.

## What it hands on

### To rancher/rancher

* **What:** the image tag, written where Rancher keeps its SCC operator image
* **Starts when:** a stable is published; an RC only when a person sends it
* **Done by:** automation opens the PRs; a person sends an RC
* **Lines:** a PR for every active line's branch, the same tag in each
* **Reviewed and merged by:** ORBS engineering team

## Differences between lines

None known.

## Who is responsible

* **Owns:** ORBS team
* **Acts:** engineers review and merge changes and the PRs in Rancher, and tag; automation publishes and opens the PRs
* **QA team:** tests an RC in Rancher, signs off to un-RC
* **Can step in:** ORBS team

## Rules

* A stable is sent to every active line by itself: the same image works on every line
* An RC is sent only by a person: it is for testing in chosen branches
* Only stable reaches Prime production: production never holds an `-rc`
* A stable follows an RC of its version, QA signed off: stable is what was tested
* Exception: a CVE patch may go to stable with no RC: speed, and the change is small

## Where it stands

* **Latest release:** highest stable tag; latest prerelease is the highest `-rc`, if newer
* **What Rancher holds:** the tag in Rancher's `build.yaml`, on each line's branch
* **What a PR means:** an open PR is a tag not yet in that branch
* **Not visible from outside:** QA sign-off

## Pain and unknowns

* **Hard today:** which branches hold version X is only known by reading Rancher
* **Hard today:** nothing enforces that a stable follows an RC
* **Not known:** whether a line ever skips a stable, and how
* **Not known:** who adds or removes a line

## Config notes

* One stream serving every line: no series, no branches. Nothing per line to write?
* The edge to Rancher sends to every active line; the lines come from Rancher's own list
* An RC is sent by a person to chosen branches; a stable is sent automatically. Two kinds of send on one edge.
* Compatibility is in Rancher's file, not here
* A stable needs an RC first, with an override for a CVE patch. Per-project setting?
* QA sign-off is outside the repo and config. How does a view show it?

## Evidence

* The repo's README and development notes
* [`../projects/scc-operator.md`](../projects/scc-operator.md)
* [`config/scc-operator.project.yaml`](../../config/scc-operator.project.yaml)
