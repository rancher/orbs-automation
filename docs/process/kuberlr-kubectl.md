# kuberlr-kubectl

Builds the image charts use to run `kubectl`, with the `kubectl` versions each Rancher line supports. Start of a chain. ORBS team responsible.

**Status:** draft. **Written:** 2026-11-07, by Claude, from a read of the repo. No account from a person yet.

## What a release is

### Artifact

* **What:** container image `rancher/kuberlr-kubectl`, amd64 and arm64
* **Contains:** `kuberlr`, plus `kubectl` for each Kubernetes minor of the line
* **Also:** test Helm chart attached to the GitHub release
* **Published to:** Docker Hub, GitHub registry, Prime staging; stable also Prime production
* **Signed and attested:** yes

### Version

* **Scheme:** semver
* **Tells you:** major is the Rancher line (v9 is 2.16, v8 is 2.15, v7 is 2.14, v6 is 2.13, v5 is 2.12)
* **Minor and patch:** this project's own

### Stable and prerelease

* **Prerelease:** `-rc.N`; Docker Hub, GitHub registry, Prime staging
* **Stable:** no suffix; also Prime production
* **Downstream may take:** Stable; prereleases are for ORBS internal testing
* **Head image:** each merge to a release branch; GitHub registry only; for testing

### Stages

None.

## Lines

* **Shape:** one stream for each line
* **Branch:** `release/vN.x` for each active line; major is N
* **Newest line:** built from the main branch, no release branch yet
* **Active lines:** the lines Rancher supports
* **New line:** normally after Rancher's release team makes the new Rancher branch; the ORBS team may prepare it earlier; main moves to the next major
* **Ended line:** no more releases or changes; tags stay

## How a release happens

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to the line's branch |
| 2 | Head image | A change is merged | Automation | The branch | Head image |
| 3 | Decide to release | Changes worth releasing are on the branch | ORBS team | The branch | The decision |
| 4 | Tag prerelease | The decision | A person | A commit on the branch | `-rc.N` tag |
| 5 | Tag stable | The prerelease passed CI | A person | A commit on the branch | Stable tag |
| 6 | Publish | A tag is pushed | Automation | The tag | Image in each registry, signature, GitHub release, same tag in GitHub registry |

* **Gates:** change checks pass before merge
* **QA:** by exception only; the image is simple and its CI checks that its binaries are present and run
* **Choices:** when to release; which lines (usually every active line in one round)
* **Failure:** a failed publish leaves the release in draft; nothing downstream starts; an engineer fixes it and re-tags

## What it takes in

None.

## What it hands on

### To backup-restore-operator

* **What:** image tag of the line
* **Starts when:** a release of the line
* **Done by:** the BRO team, in the files of its own repo, before its release reaches charts
* **Lines:** each takes the matching major

### To other charts

* **What:** image tag of the line; every chart that runs `kubectl` uses it, none uses shell
* **Starts when:** a release of the line
* **Done by:** each chart's team, in the files of the repo it owns, before its own release reaches charts
* **Lines:** each line takes its matching major

## Differences between lines

* from v2.16: built from main, no release branch yet
* until v2.15: own release branch
* v2.10, v2.11: ended

## Who is responsible

* **Owns:** ORBS team
* **Acts:** the team reviews, merges, decides, tags; automation publishes
* **QA team:** by exception
* **Can step in:** ORBS team

## Rules

* Major version is the Rancher line: the version alone says which `kubectl` versions to expect
* Only stable reaches Prime production: production never holds an `-rc`
* Image never downloads `kubectl`: it runs air-gapped
* Bundled `kubectl` versions follow the line's Kubernetes range: Kubernetes allows version drift
* Stable does not need QA sign-off: the image is simple and CI verifies it; QA is asked by exception

## Where it stands

* **Latest release of a line:** highest stable tag with the line's major; latest prerelease is the highest `-rc`, if newer
* **What a downstream holds:** image tag in its own files; path and file vary by repo
* **Not visible from outside:** publish status in private registries; who decided to release

## Pain and unknowns

* **Hard today:** same change made on every line's branch
* **Hard today:** a release round is one person tagging each branch
* **Hard today:** repo docs disagree with the branches
* **Not known:** how a new line is cut
* **Not known:** what a release decision is based on
* **Not known:** how downstream picks up a new tag
* **Not known:** whether stable is meant to need a prerelease

## Config notes

* Version series is a function of the line, nothing to configure. True for other projects?
* Bundled `kubectl` versions equal the line's Kubernetes range, which the lines data has. Config needed?
* Stable-only registries: project or common process?
* Releases often cover every line at once. Config needed?
* Hand-off to backup-restore-operator is assumed. Learn it first.
* Newest line moves from main to a branch. Config needed?

## Evidence

* The repo's README, version notes and chart migration guide
* [`config/kuberlr-kubectl.yaml`](../../config/kuberlr-kubectl.yaml), for the assumed hand-off
* No project note in [`../projects/`](../projects/README.md)
