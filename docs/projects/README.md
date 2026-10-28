# Project Notes

One doc per project repo, recording what we learned about that project to inform the tool. These are observations about how things work **today**. They are not design decisions, and they are not assumed to be good practice. Decisions live in `../IMPLEMENTATION.md` and `../ARCHITECTURE_DESIGN.md`.

Each note was gathered from a read-only review of a local clone (and a few read-only GitHub API calls for downstream consumers) on 2026-10-28. Details were not independently re-verified, so treat specifics (paths, versions, branch names) as a starting point and re-check them before relying on them.

## Index

The groupings below describe how each project behaves in the relationships we reviewed. They are not a fixed role for the project: producer and consumer are properties of an edge, and one project can be both (e.g. `rancher-charts` consumes BRO and feeds Rancher).

**Producers (release something that flows downstream)**
* [backup-restore-operator](backup-restore-operator.md): `automation-core` branch, hybrid reusable + rendered workflows
* [rancher-shell](rancher-shell.md): `automation-core` branch, reusable workflows with thin stubs
* [scc-operator](scc-operator.md): no `automation-core`, single branch for all Rancher versions

**Consumers (receive bumps)**
* [rancher-charts](rancher-charts.md)
* [rancher-rancher](rancher-rancher.md)

**QA**
* [observability-e2e](observability-e2e.md)

**Reference (inspiration only, not a project we automate)**
* [../references/frameworks-automation.md](../references/frameworks-automation.md)

## Cross-Project Observations

Where the same fact is encoded in several places, per repo. This is the strongest evidence for central config holding one shared line map.

| Fact | Where it is encoded today |
|---|---|
| backup-restore-operator major to charts branch | `CHARTS_BRANCH_MAP` in its `push-to-charts/common.sh`; freeze check derives `release/vX` with `sed`; `.github/renovate.json` |
| rancher-shell minor to Rancher branch | `SHELL_MINOR_TO_RANCHER_BRANCH` in its `push-to-rancher/common.sh`; a README table; `.github/renovate.json` |
| scc-operator to Rancher branches | hardcoded branch array in its `push-to-rancher/common.sh` and its README |
| frameworks-automation Rancher line to branches | a `version-md` table copied into each `dependencies/*.yaml`, plus `branch-template` |
| rancher/rancher current line | `chartDefaultBranch` in `build.yaml` |
| Chart version major to Rancher line | an unwritten convention (e.g. 110 is v2.15, 111 is v2.16), computed independently by each producer |

Common patterns seen across projects:
* Release workflows are copied or rendered per branch because signing and attestation need the workflow to live on the tagged branch.
* Shared workflow refs float (`@automation-core`), so a bad push affects every branch.
* None of the reviewed repos has drift detection beyond what reviewers notice.
* Prerelease detection (a `-` after the version) is reimplemented in shell in several repos.
* Branch families differ per repo: `release/vX.Y`, `dev-vX`, `release-vX`, and plain `main`.

## Note Template

```
# <repo>
Role (producer/consumer/QA), owner, one-line summary.
## Layout and versioning
## Workflows and release process
## Config and topology facts (and where each lives)
## Auth and cross-repo interaction
## Pain points
## Implications for the tool
```
