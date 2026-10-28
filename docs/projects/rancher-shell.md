# rancher-shell

Producer. Uses an `automation-core` orphan branch with pure reusable workflows plus thin per-branch stubs.

## Layout and versioning
* Branches: `main`, `release/v2.7` through `release/v2.15`, and a legacy `master` (the old default).
* Tags are `v0.1.x` style, with `-rcN` prereleases (e.g. `v0.1.18-rc1`). Shell minor `0.8` corresponds to Rancher `2.15`.
* `automation-core` is an orphan branch (18 commits, 2026-07-29 to 2026-10-15), all by one author through PRs. Nothing automated writes to it.
* It contains:
  * `.github/workflows/{tests,head-build,fossa}.yml`, each `on: workflow_call` only
  * `actions/check-semver/*` and `actions/push-to-rancher/{action.yml,run.sh,common.sh,create-prs.sh,update-build-yaml.sh}`
  * a README, CODEOWNERS and `renovate.json`
* There is no config or manifest file, no templates and no rendering step.

## Workflows and release process
* **Consumption:** branches hold thin stubs that own triggers and call `rancher/shell/.github/workflows/*@automation-core`. Nothing is synced or copied for tests, head builds and FOSSA.
* **Exception:** `release.yml` is a full workflow in every branch (removed from the orphan branch in PR #490), calling `actions/check-semver` and `actions/push-to-rancher` at `@automation-core`. Copies differ in pins (`publish-image` at v0.77.1 on `main`, v0.66.2 on `release/v2.15`).
* **Adoption:** `main` and `release/v2.12`–`v2.15` use stubs. `release/v2.11` and older still carry full inline workflows, and nothing flags them. `release/v2.14` has no `head-build.yml`.
* **Release:** a `v*` tag runs `release.yml`. `check-semver` strips the `v` and outputs `IS_SEMVER`, `HAS_PRERELEASE`, `HAS_BUILD_META`. Images are pushed to docker.io and Prime staging; a stable tag (semver, no prerelease) also goes to Prime prod. Signing is keyless cosign via OIDC, done inside `ecm-distro-tools/actions/publish-image`. No manual override beyond `workflow_dispatch` on some workflows.
* **Downstream:** `push-to-rancher` verifies the image exists (`docker manifest inspect`), maps shell minor to Rancher branches, clones `rancher/rancher`, sets `defaultShellVersion` in `build.yaml` with `yq`, runs the first `//go:generate` line from `generate.go` (via `eval`), commits, pushes `bot/shell-<tag>-<ver>-<epoch>` and opens a PR (label `status/auto-created`).
* Head builds: `branch-tags.sh` produces `head` plus `main-<sha7>` on `main` and `vX.Y-head` variants on release branches; `oras copy` re-tags.

## Config and topology facts
* No per-repo manifest. Policy is spread across:
  * `SHELL_MINOR_TO_RANCHER_BRANCH` in `push-to-rancher/common.sh` (already lists `0.9` → `release/v2.16`)
  * a branch/tag/Rancher table in the README on `main`, plus k8s min/max per Rancher version (prose only)
  * `.github/renovate.json` (`baseBranchPatterns`, per-branch `packageRules`)
* Repo knobs: `vars.IMAGE_NAME`, `vars.REPO`, `vars.DOCKER_USERNAME`; Vault paths derive from `github.repository`. Image names and the `ci-image/charts` container are baked into shared workflows.

## Auth and cross-repo
* `GITHUB_TOKEN` for port-pr and updatecli workflows.
* Vault via OIDC (`rancher-eio/read-vault-secrets@v3`) for Docker Hub, Prime and FOSSA credentials.
* A GitHub App token scoped to `rancher/rancher` for the cross-repo PR, from the Vault path `…/github/pr-actions-write-app/credentials`.
* No `repository_dispatch`. Cross-repo interaction is a direct clone, push and PR.

## Pain points
Probable bugs found by the review (not re-verified):
* `tests.yml` triggers on `master`, not `main`, so tests do not run on push to `main`.
* `push-to-rancher/action.yml` has a missing slash in the Vault path, and the secret env names mismatch (`APP_ID`/`PRIVATE_KEY` read as `APPID`/`PRIVATEKEY`).
* `head-build` uses `release/v[0-9]+.0`, a glob rather than a regex, so it likely misses `release/v2.15`.
* `branch-tags.sh` emits `prev_static_tag` but the workflow reads `prev_tag`.
* The orphan README describes a release stub that differs from the branches' real `release.yml`.

Other: hand-maintained map and Renovate lists, heavy bash in `create-prs.sh` (repeated skip/cleanup blocks, `eval`, dead `$?` check), `updatecli` only on `main`, about 80 stale branches.

## Implications for the tool
* Thin stubs plus reusable workflows work well for tests, head builds and FOSSA, but not for release, which needs rendering.
* Version shared workflows by tag or SHA, not a floating branch.
* Tag classification as one library function; gate prod promotion on it.
* The image-exists check before opening downstream PRs is worth keeping.
* The line map (shell minor to Rancher branch) belongs in central config as data.
* Replace bash orchestration with Go.
