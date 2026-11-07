# rancher-assets

Both: consumes `rancher/charts` (with two other chart repos we do not care about) and feeds `rancher/rancher`. Owned by the release team (`CODEOWNERS` lists `@rancher/release-team` and `@rancher/orbs`). Gathered on 2026-11-07 from a read-only review of a local clone (checked out on a feature branch; `origin/main` is the default). Details were not independently re-verified, and the repo's own docs disagree with its workflows in places, so the workflows are the source here.

## Layout and versioning
* It builds one image, `rancher/rancher-assets`, which bundles the Helm chart repos that Rancher's `ClusterRepos` use for air-gapped installs. One Dockerfile per Rancher minor and stage: `dockerfiles/Dockerfile.2.16` (prod) and `Dockerfile.2.16-dev`.
* **One branch.** `main` is the only release branch, and tags point at commits on it. The Rancher line is a key in `config.yaml`, and the line shows in the tag prefix. Only 2.16 is active now; 2.14 and 2.15 tags remain, and 2.15 was disabled in a commit.
* **Tags** are CalVer: `v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z` for prod and the same with a `-dev` suffix for dev. Examples: `v2.15-20260915T1243Z`, `v2.16-20261030T1111Z-dev`. The timestamp is UTC, to the minute, and it is the commit time of the `main` commit that triggered the release, not the build time. A commit yields at most one tag per minor and stage. Tags are annotated, made by a GitHub App, with the message `Auto-release for dockerfiles/Dockerfile.2.16 from commit <sha>`.
* Prod and dev are not always paired: a prod tag can exist without a dev twin at that commit, and the reverse.
* **Newest tag of a line and stage:** no code selects it. The docs give shell recipes. Filter by the suffix first (`-dev` or none), then sort on the 13-character timestamp. Git's combined `-version:refname` sort mixes the two stages.

## Workflows and release process
* **Bump:** `pr-builder-cron.yml` runs on weekdays at 11:00 UTC and on dispatch. It runs `make update`, which reads upstream branch heads with `git ls-remote` and rewrites `lock.yaml` and the Dockerfiles, and opens a PR (`bot/bump-chart-refs-<timestamp>`). `auto-merge-bump-pr.yml` squash-merges it once Lint, "Test & Verify" and the summary check pass.
* **Tag:** `auto-release.yml` runs on a push to `main` that touches `dockerfiles/**` or `lock.yaml`. `detect-release-targets.sh` compares `lock.yaml`'s `.chart-versions."<minor>".upstream-refs.prod.charts.commit` before and after: if the `rancher/charts` prod commit moved, the target is prod, otherwise dev. A prod tag for any other reason is pushed by hand. It tags with a GitHub App token.
* **Build:** `release.yml` runs on any `v*` tag. It parses the tag with two regexes (dev first), creates a draft GitHub release, builds and pushes through `rancher/ecm-distro-tools/actions/publish-image`, uploads image lists as release assets, and publishes the release. Every tag goes to Docker Hub (`docker.io/rancher/rancher-assets`) and the Prime staging registry, and prod tags also go to the Prime prod registry. The hostnames come from Vault, not files.
* **Inputs:** `config.yaml` maps each minor to its sources per stage and is edited by hand:
  * prod uses `rancher/charts` branch `release-v2.16`, dev uses `dev-v2.16`;
  * `partner-charts` and `rke2-charts` use `main` for both stages.
  * `lock.yaml` holds the resolved commits (`branch`, `commit`, `fetched-at`) for each stage and source, and they are baked into the Dockerfiles as shallow single-commit fetches.
* **Promotion** from charts `dev-vX` to `release-vX` is not here. This repo only reads both branch heads. A promotion shows up as a changed `upstream-refs.prod.charts.commit` after the next cron bump, which causes a prod tag.

## Config and topology facts (and where each lives)
* Charts branch per line and stage: `config.yaml`. The commit actually used: `lock.yaml`, `Dockerfile.2.16*`, and image labels.
* **Tracing a tag to charts:** `git show <tag>:lock.yaml` gives the `rancher/charts`, partner and rke2 commits for both stages. The image also carries labels (`io.rancher.charts.branch`, `io.rancher.charts.commit`, partner and rke2 equivalents, `io.rancher.rancher-minor`, `io.rancher.build-type`, and the OCI `version`, `revision`, `created`, `source` labels). The tag itself does not encode the charts commit, and prod and dev images from one commit can hold different charts commits.
* Chart versions inside the image are not in a manifest here. Only commits are recorded, so versions come from the charts repo at that commit or from the image-list release assets.

## Auth and cross-repo interaction
* GitHub App credentials, read from Vault.
* **Into Rancher:** `.github/workflows/push-to-rancher.yml` runs when a release is published (and on dispatch). `push-to-rancher/update-build-yaml.sh` sets one key in rancher's `build.yaml`: `defaultAssetsImage: rancher/rancher-assets:<tag>` (no registry host). `create-prs.sh` opens a PR titled `[<branch>] Update rancher-assets to <tag>` from `bot/rancher-assets-<tag>-<suffix>-<epoch>`, with the label `status/auto-created`, after running `go generate ./pkg/...`.
* The Rancher branch map in `common.sh` is hardcoded (`"2.16" -> main`) and a version missing from it fails; `RANCHER_BRANCHES_OVERRIDE` overrides it. `config.yaml` also has a `rancher-branch` field, which the scripts do not read.
* **Dev and prod are not told apart on the Rancher side:** one key and one branch per minor. Every published release triggers a PR, dev tags included, so Rancher can end up holding a `-dev` tag. The docs say the PRs are prod only, and the workflows do not enforce it.
* It does not consume backup-restore-operator, and has no BRO content.

## Pain points
* **The docs are stale in places.** The "Manual Release" workflow they describe does not exist. They say `ghcr.io`, and the workflows push to docker.io. The "prod only" claim is not enforced. One doc says there is no automated upstream polling, and the cron does exactly that. The `RANCHER_REPO_TOKEN` and `WORKFLOW_PAT` secrets they mention are no longer used.
* The Rancher branch map is hardcoded, and the branch field in `config.yaml` is unused.
* The stage of a tag is only in its name, and no tool here picks the newest tag.

## Implications for the tool
* The hop **charts to Assets to Rancher is readable from outside** with what already exists: tags from `git ls-remote --tags`, `lock.yaml` at a tag for the charts commit, and the `defaultAssetsImage` key of Rancher's `build.yaml`.
* The Assets to Rancher hop is **already automated by the release team's own scripts**, the same family `gero ship` replaces for BRO and shell. For `gero` it starts as an observed hop: `gero ship` must not open a competing PR for it.
* A tag scheme has to give the line, the stage and the timestamp order, and the stage names differ (`release` for the charts branch, no suffix for the prod tag).
* A line is a tag prefix here and not a branch, so a project can have lines without per-line branches.
* The image tag Rancher holds can be a dev tag, which is normal until the release team cuts a stable one, so a chain view shows its stage and does not warn.
* **One tag is three things.** Each image tag is a git tag on the repository, a published GitHub release of the same name, and the release's assets: the image lists and metadata. Chain tracing reads only the tag today. A later check could confirm that the release exists and is published, or read its metadata. Which chart versions an image holds is traced through `lock.yaml` to the charts repo at the recorded commit, not from the release ([CHAIN_TRACING](../specs/CHAIN_TRACING.md)).
