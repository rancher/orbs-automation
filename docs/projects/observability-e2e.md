# observability-e2e

QA repo, owned by `@rancher/orbs_qa`. Informs the v3 QA handshake, which is undecided.

## What it is
* End-to-end tests for Rancher observability, backup/restore and related install/upgrade flows.
* Go (Ginkgo suites under `tests/e2e/`, `tests/backuprestore/…`), Playwright/TypeScript UI tests (`tests/ui/`), Terraform for AWS/RKE2 setup, helpers under `tests/helper/`.
* The artifact under test is a **Rancher Manager version**, not an RC tag of one component. The observability charts (monitoring, alerting, logging, promfed, shell, SCC, Syslog) are installed through Rancher's catalog. The only artifact pin found is an optional `shell_image_version`. There is no way to say "test chart X RC Y".

## Workflows and triggers
* Mostly `workflow_dispatch`, many with a `schedule` (docker Mon/Wed/Fri, k3s Sun/Tue/Thu, nightly backup-restore) and some on PRs. No `repository_dispatch`, no `workflow_run`. `master-e2e.yaml` and `backup-restore-e2e.yaml` are internal `workflow_call` workflows.
* Typical inputs: `rancher_version` (e.g. `v2.15-head`), `upstream_cluster_version` (RKE2), `rancher_repo`, `chart`, `shell_image_version`, `tests_to_run`, `release` + `milestone` + `qase_run_id`. Backup/restore adds `rollback_rancher_version`, `backup_restore_chart_version`, `test_suite`, `is_ha`.
* A run identifies its build only by free-text `rancher_version`. Scheduled matrices are computed from a chart index (`charts.optimus.rancher.io/server-charts/latest/index.yaml`).

## Results and sign-off
* Reporting goes to **Qase** (project code `RM`, hardcoded). Pre/post jobs create, report to and complete a Qase run; the GitHub run URL goes in the Qase run description, and the Qase report URL goes to `$GITHUB_STEP_SUMMARY`.
* `release: true` plus `milestone` resolves or creates a Qase runner tagged `orbs-release-testing` for that milestone. That milestone is the only release concept.
* No GitHub issues, PR comments, check runs or statuses on other repos, no Slack, no labels, environments or approvals. Sign-off today is a human reading Qase or the run.

## Auth and cross-repo
* Repo-scoped secrets (AWS, Rancher password, SSH key, Qase tokens, SCC regcode, registry credentials, encryption key). No cross-repo GitHub interaction; no dispatches.
* Reads Rancher chart indexes over HTTP and consumes `rancher/renovate-config` (SHA-pinned).

## Release-to-test mapping
* No mapping file. Rancher tracks are hardcoded in several places (release lists in workflows, nightly `rke2_216_head`–`rke2_213_head` inputs, `ui-tests` choices, defaults), so a new Rancher minor means editing them all. Defaults disagree across files (e.g. different `upstream_cluster_version` defaults and chart repo URLs).

## Pain points
* Heavy duplication of Qase pre/post jobs. Fragile `awk '/available:/'` parsing of the Qase report URL. Long runs (60–120 minute timeouts) needing real AWS resources, so triggering per RC is costly. A `rancher_version` typo fails late.

## Implications for the v3 handshake (nothing decided)
* QA needs, at minimum, `milestone`, `rancher_version` or track, `chart`, `shell_image_version`, plus a new additive RC reference and a correlation ID. Some of that needs QA-side work to be consumed.
* Realistic sign-off signals: a GitHub workflow conclusion (needs correlation), a QA-written label or status on a tracking issue or commit (needs a cross-repo token and QA changes), or Qase API polling (couples us to a third-party SaaS).
* Leaning, to agree with the QA owners: dispatch a thin QA workflow with correlation ID, milestone and Rancher version; observe the run conclusion; record it on the tracking issue as evidence; keep the stable-tag gate a human decision.
* Avoid initially: polling Qase directly, QA writing status into engineering repos, per-RC full-matrix runs.
* Prerequisite: decide what a QA run proves it tested (Rancher version, chart version or image).
