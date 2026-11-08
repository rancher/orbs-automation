# Release process

**Status:** draft, high level. **Written:** 2026-11-07 from the [project docs](README.md#index) and the ORBS team lead's corrections. Each project's own steps are in its doc; this is what holds across them.

## Terms

* **Line:** a Rancher line, such as `v2.16`
* **Stage:** a tier of one project's releases, such as `dev` and `stable`
* **Hop:** one hand-off between two projects
* **RC:** a prerelease of a version; **un-RC:** replace a held RC with its stable
* **Destination:** the project every chain ends in, `rancher/rancher`

## The chain of a line

* kuberlr-kubectl, picked up by each chart team in its own repo
* backup-restore-operator and other chart projects, into `rancher/charts` (`dev`, then the release branch)
* `rancher/charts`, into rancher-assets from v2.16 (and chart versions into Rancher)
* rancher-assets, scc-operator and shell, into `rancher/rancher`

## Projects at a glance

| Project | Lines | Stages | Reaches | How it reaches |
|---|---|---|---|---|
| kuberlr-kubectl | one stream each, major is the line | none | each chart team's repo | picked up by the chart team |
| backup-restore-operator | one stream each, major is the line | none | charts `dev` of its line | PR for every RC and stable |
| rancher-charts | one stream each, two branches | `dev`, `stable` | Assets from v2.16; chart versions to Rancher | pulled by Assets; chart team applies the version in Rancher |
| rancher-assets | lines in the tags | `dev`, `stable` | Rancher's branch of the line | PR when a build finishes |
| scc-operator | one stream for every line | none | every active line | PR for a stable; an RC only on request |
| shell | one stream each, minor is the line | none | one Rancher branch for each line | PR for every release |
| rancher | one stream each | none | nothing | end of the chain |

## Kinds of hand-off

* **Sent:** the producer's automation opens a PR in the consumer
* **Pulled:** the consumer reads the producer's branches on its own cycle
* **Picked up:** the consuming team updates its own files with the producer's release
* **What is sent varies:** every release, stable only, an RC on request, or either stage
* **Order matters:** a system chart goes to charts first; its version then goes in Rancher's build file, because the version must be in the index first

## The shape

* A project is one stream for every line, one stream for each line, or lines only in its tags
* The newest line is built from the main branch; a release branch is cut when the next line starts
* A line's last release lands at its end of life; release dates move
* Only the release team creates branches in `rancher/charts`
* A new line: the release team makes the Rancher branch after a minor release; ORBS projects cut theirs after that, or earlier if prepared
* Several unreleased lines may exist at once; a line cannot resolve into Rancher until Rancher has its branch, and `main` points at one future line only

## One release

* Changes are proposed, reviewed, then merged to the line's branch
* A person decides to release and tags a prerelease; automation tests and publishes
* The release is handed on, and the next project holds it as an RC
* QA tests the RC; each QA team covers its own components
* After sign-off, engineers un-RC the component and tag the stable
* The stable is handed on in the same way
* The destination's stable needs every component it holds to be stable; the release team gets other teams there, and components un-RC in staggered waves as teams finish

## Publishing (ORBS projects)

In order:

* CI and lint pass first
* The draft release is created (by the release tool, or by hand if the project has none)
* The image is built
* Any other release assets are collected
* Attestation is applied to everything going in the release
* Only after every step succeeds, the draft is published
* Steps that must be part of the release run after that

A failure at any step leaves the release in draft, so nothing downstream starts. An engineer fixes the cause and re-tags.

## Charts

* A chart ships to charts first, which assigns its version; the major is the Rancher line
* Every chart version lands on `dev`, an RC or a stable
* The release team promotes only approved, un-RC'd charts from `dev` to the release branch
* An RC still on `dev` at promotion is removed; its team prepares an out-of-band release with the release team, or resubmits for the next release
* A released chart is never removed
* From v2.16, offline clusters take chart versions from the Assets image they use; an install can override its tag

## Who does what

* **Engineering team of a project:** reviews and merges changes, decides, tags RCs, un-RCs after sign-off, tags stables
* **QA team:** tests an RC for the components it is assigned and signs off
* **Chart owner:** the team that makes a chart; ORBS owns `rancher-backup`
* **Release team:** maintains `rancher/charts`, promotes `dev` to release, runs the destination's release automation, and with QA decides when the destination's RC becomes stable
* **Automation:** tests, publishes, opens the hand-off PRs
* **Rancher repo:** owned by all teams together

## Stable after stable

* A stable normally follows an RC of its version, with QA sign-off
* A new stable may follow a stable directly, for a small patch or a CVE fix
* It needs no RC; a QA smoke test for regressions is optional and never a blocker
* This holds even where automation restricts stables to follow an RC: the path must stay open

## Stable and prerelease

* A prerelease is an `-rc` suffix, or the `dev` stage
* Prereleases and dev builds go to Docker Hub and Prime staging, for every line
* A component's stable goes to Docker Hub and Prime production
* The destination's stable goes to Prime for every line, and to community registries for the newest minor only
* The community rule is product policy, paid versus open-source access; it changes as new minors arrive, and by product decision; never retroactively
* The destination holds a prerelease of a component until its stable replaces it

## Not modelled

* The release calendar: code freeze, RC windows, release day
* QA sign-off: it is outside every repo
* Components not traced through a configured project

## Open

* Who adds a line in rancher-assets
