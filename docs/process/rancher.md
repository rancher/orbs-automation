# rancher

Rancher itself: the final destination. Every project in the chain ends here. Takes in many components, hands on to nothing in the chain. Every team owns it together. The release team runs its release; QA and the release team decide when an RC becomes stable.

**Status:** draft. **Written:** 2026-11-07, by Claude, from a read of the repo's release automation; corrected by the ORBS team lead the same day. The release team has not reviewed it.

## What a release is

### Artifact

* **What:** the `rancher-server`, `rancher-agent`, `rancher` (all-in-one) and system-agent installer images, and the `rancher` Helm chart
* **Also:** image lists, checksums and image digests
* **Published to:** prereleases to Docker Hub and Prime staging, any line; a stable to Prime for every line, and to community registries (Docker Hub) for the newest minor only; the chart to Rancher's chart site
* **Which minor is community:** the newest; it changes as a new minor arrives; product strategy may change the rule, never retroactively (it applies from that point on)

### Version

* **Scheme:** semver, `vMAJOR.MINOR.PATCH`
* **Tells you:** minor is the line (`v2.15.3` is line v2.15)
* **Patch:** each release of a line

### Stable and prerelease

* **Prerelease:** `-alphaN` and RC spellings: `-rcN` (Rancher's older spelling of semver `-rc.N`) and `-rcs…`; every RC spelling is treated as an RC
* **Stable:** no suffix
* **Hotfix:** `-hotfix-…` tags fix a specific bug or customer case; outside the standard release
* **Downstream may take:** none; Rancher has no configured downstream (projects that use it as Go code are out of scope)
* **Head build:** each merge to main or a release branch

### Stages

None. Promotion to stable is a step of its own, below.

## Lines

* **Shape:** one stream for each line
* **Branch:** `release/v2.X` for each released line
* **Newest line:** built from main; no release branch and no tag yet (v2.16)
* **Active lines:** the lines Rancher supports (v2.12 to v2.15 released, v2.16 in development)
* **New line:** the release team makes the Rancher branch after a minor release; main moves to the next line
* **Ended line:** no more releases; tags stay
* **Release rounds:** several lines are tagged together
* **Last release of a line:** lands at its end of life; dates sometimes move, so releases can overlap the end

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the owning team reviews | The proposed change | The change merged to the line's branch |
| 2 | Components move in | A component project hands on | See "What it takes in" | The component release | The component's version in Rancher |
| 3 | Decide to release | A release is wanted | Release team | The branch | The decision |
| 4 | Tag a prerelease | The decision | Release team | A commit on the branch | `-rcN` tag |
| 5 | Publish | A tag is pushed | Automation | The tag | Tests, images, chart, installer, image lists, digests |
| 6 | Test the RC | The RC is published | QA teams, each its own components | The RC | Sign-off for each component, to its engineering team |
| 7 | Final un-RC check for components | A QA team signs off a component | The component's engineering team | The component's RC version in Rancher | The component's stable version; teams finish in staggered waves |
| 8 | Tag the stable | Every component is un-RC'd and sign-off is done | Release team and QA decide; release team tags | A commit on the branch | Stable tag |
| 9 | Publish the stable | A stable tag is pushed | Automation | The tag | As step 5 |
| 10 | Promote to stable | Every component is un-RC'd; the release team gets the other teams there | Release team | The stable tag | The images marked latest and stable; the chart published as stable |

* **Gates:** change checks before merge; unit and integration tests pass before anything is published
* **Choices:** when to release; which lines (a patch round usually covers every active line)
* **Un-RC:** components un-RC in staggered waves as teams close their work

## What it takes in

### From scc-operator

* **What:** the operator image tag
* **How it arrives:** a PR on each active line's branch for a stable; an RC only when a person sends it
* **What changes here:** the operator image tag
* **Recorded in:** the build file on the line's branch
* **Lines:** every active line, the same tag

### From shell

* **What:** the shell image tag
* **How it arrives:** a PR on the line's branch for every release
* **What changes here:** the shell image tag
* **Recorded in:** the build file on the line's branch
* **Lines:** each line takes its own shell series

### From rancher-assets

* **What:** the Assets image tag, of either stage
* **How it arrives:** a PR on the line's branch, merged by Rancher's own automation
* **What changes here:** the Assets image tag
* **Recorded in:** the build file on the line's branch
* **Lines:** v2.16 and up

### From rancher/charts

* **What:** chart versions (webhook, fleet, turtles and others), including RC versions
* **How it arrives:** the chart's team applies the version charts assigned, in a PR on the line's branch; charts merges first, so the version is in the index
* **What changes here:** the chart version
* **Recorded in:** the build file on the line's branch
* **Lines:** each line from its own charts branch
* **Also:** the charts branch the line follows is recorded in the same file

### From other components

* Not traced: machine, system agent, wins agent, KDM, kube-api-auth, Helm and others

## What it hands on

None. End of the chain. Projects that use Rancher as Go code are out of scope.

## Differences between lines

* from v2.16: an offline cluster has only the chart versions in the Assets image it uses; an install can override the Assets tag; online clusters refresh from the internet
* until v2.15: every chart was cloned into Rancher's own image
* v2.11: ended; v2.11.19, tagged 2026-10-28, was its final release at its end of life

## Who is responsible

* **Owns:** all teams together (managed by: shared)
* **Release automation and image build:** release team
* **Chart, SCC, telemetry, catalog code:** ORBS team
* **Acts:** release team decides, tags RCs and stables, and promotes; automation tests, builds and publishes
* **QA teams:** each tests its own components and tells the engineering team it works with, who un-RC them
* **Decides RC to stable:** release team and QA
* **Can step in:** Rancher project managers, for release issues around the un-RC time before a release

## Rules

* Minor is the line: the version alone says which line a release is for
* A stable's components are stable: promotion waits until every component is un-RC'd
* Only the newest minor goes to community registries: product decision, applied from when it is made, never backwards
* Release team and QA own when an RC becomes stable
* A hotfix follows its own path: it fixes a specific bug or customer case and is not a standard release

## Where it stands

* **Latest release of a line:** highest stable tag of the line; latest prerelease is the highest `-rc`, if newer
* **What Rancher holds of a component:** the build file on the line's branch
* **Which line a branch is:** release branch name; main is the newest line
* **Not visible from outside:** QA sign-off; Prime registry state

## Pain and unknowns

* **Hard today:** the process is the release team's and is described here only in part

## Config notes

* Rancher is the end of every chain; each line has its own branch
* Many components arrive by PR; the build file is the record of what is held
* Components may be held at an RC until un-RC. A view should show which parts of a line's chain are stable and which are not.
* Several lines are released together
* The line map has no tag yet for the newest line
* The release team's steps are outside the repos of the projects feeding it
* Owned by all teams: managed by `shared`?
* Registry rule depends on which minor is newest, and changes going forward only. Config needs a rule with an effective point.
* A line's last release may land after its end date: release dates move

## Evidence

* The repo's release automation and build file
* [`../projects/rancher-rancher.md`](../projects/rancher-rancher.md)
* [`rancher-assets.md`](rancher-assets.md), [`scc-operator.md`](scc-operator.md)
