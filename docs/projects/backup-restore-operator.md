# backup-restore-operator (BRO)

Producer. Uses an `automation-core` orphan branch with a hybrid model. The closest existing model to what the central repo should offer.

## Layout and versioning
* Release lines are `release/v7.x` through `v11.x` plus `main`. Each BRO major maps to a Rancher line.
* Tags are `v*`, with `-rcN` prereleases (e.g. `v9.0.2-rc.5`, `v10.0.0`).
* `automation-core` is an orphan branch (46 commits, 2026-07-23 to 2026-10-22), edited by people through PRs plus a nightly bot commit that refreshes action pins. Staged rollout branches (`automation-core-p1-fixes`, `-p2-actions`, `-p3`) are leftovers.
* `automation-core` contains:
  * `actions/`: composite actions (`check-semver`, `compute-branch-tags`, `run-goreleaser`, `undraft-release`, `build-deps`, `test-deps`)
  * reusable workflows `ci.yaml` and `head-builds.yaml`, plus deploy/validate/index-update workflows
  * `templates/*.tmpl` (`ci`, `release`, `head-builds`, `fossa`), rendered by Go with `[[ ]]` delimiters
  * `configs/<branch>.yaml`, one per branch
  * `action-versions.yaml` (central action pins with SHAs) and `.bin-deps.yaml` (tool checksums)

## Workflows and release process
* **Three consumption mechanisms:**
  1. Composite actions called live: `uses: …/actions/<name>@automation-core`.
  2. Reusable workflows (`ci`, `head-builds`) called with `@automation-core` from a stub that holds triggers plus `with:` inputs and `secrets: inherit`.
  3. Rendered templates (`release.yaml`, `ci.yaml`, `fossa.yml`) copied onto each branch by a deployer that opens one PR per branch, with a dry-run diff mode.
* **Why `release.yaml` is rendered:** attestation and keyless signing depend on the OIDC token's workflow source, so the workflow must live on the tagged branch. (Team-confirmed.)
* `ci.yaml` and `head-builds.yaml` also run `git restore --source origin/automation-core .bin-deps.yaml` at runtime and delete it afterwards, duplicated across three workflows.
* Several workflows are still copied by hand per branch: `port-pr`, `port-issue`, `push-to-charts`, `stale`, `unrc`.
* **Release:** any tag push triggers it. `check-semver` sets `HAS_PRERELEASE` from `^[0-9]+\.[0-9]+\.[0-9]+-`. goreleaser creates a draft (`prerelease: auto`), then an `undraft-release` job polls (12 tries, 10 seconds). Prime production images are pushed only when there is no prerelease suffix. Provenance uses `actions/attest`. No cosign step was found in the files read.
* After undraft, a `release: published` event triggers `push-to-charts`, which opens a PR in `rancher/charts`. `unrc.yaml` is dispatched from the charts side and creates the final `vX.Y.Z` tag from the latest RC.
* No override beyond `workflow_dispatch` with an explicit tag, `dryRun` on the deployer, and `automation_core_ref` pinning (described as an escape hatch, unused).

## Config and topology facts
* Per-branch `configs/<branch>.yaml`: `branch`, `go` (version, or `{version, ci_image}`), `k3s_versions[]`, `automation_core_ref`, `workflows: {ci, release, head-builds, fossa: bool}`, `description`. Validated by a bash/yq script. Whether the `workflows:` flags are honored was not confirmed.
* Topology facts live outside those files: `.github/renovate.json` (BRO line to Rancher line), `CHARTS_BRANCH_MAP` in `push-to-charts/common.sh`, `compute-branch-tags` (derives image tag from the branch name) and the CI trigger regex `release/v[0-9]+.(0|x)`. There is no single place saying which BRO line maps to which Rancher line.
* `push-to-charts` lives on `upstream/main` (`.github/workflows/push-to-charts/*`), not on `automation-core`; an earlier commit removed it from there.

## Auth and cross-repo
* `GITHUB_TOKEN` for goreleaser and undraft.
* GitHub Apps for cross-repo work, credentials from Vault via OIDC (`rancher-eio/read-vault-secrets`), minted with `actions/create-github-app-token` (`repositories: charts` for the charts PR). The Vault path for the deployer app was changed several times.
* The deployer pushes `deploy-workflows-<timestamp>` branches and opens one PR per target branch.
* No PATs and no `repository_dispatch` between repos. Vault paths bake in `github.repository`; a `repository_owner == 'rancher'` guard works around forks.

## Pain points
* No drift detection and no "generated, do not edit" header on rendered files. The only check is the on-demand dry-run diff.
* Observed drift: on `main`, `ci.yaml` and `release.yaml` carry `go1.27` and newer k3s versions while `configs/main.yaml` still says `go: 1.25`. A hand-made PR changed the rendered files.
* The rolling `@automation-core` ref means breaking changes hit every branch immediately. Pinning exists in theory only.
* `ci.yaml` is both a rendered template and a reusable workflow, which is confusing. The deployer calls itself via `uses: ./.github/workflows/deploy-workflows.yml`.
* `templates/README.md` documents `{{K3S_VERSIONS}}` placeholders, but the code uses `[[.K3SVersions]]`.
* Over-planned: multi-layer validation, a "legacy mode" discussion and the `automation/vX` tag plan, mostly unused.

## Implications for the tool
* Rendering with a dry-run diff and one PR per branch is a good habit to keep.
* Add a generated header and a content hash, plus a `verify` command.
* Pin shared workflows by version, not a floating branch.
* Release and attestation stay rendered in the target branch, with logic moved into composite actions or scripts.
* Central action-pin index with SHAs, bulk-deployed when a pin changes, is worth keeping.
* Topology facts (line to charts branch) should be central, not in shell and Renovate configs.
