# scc-operator

Producer. Does **not** use `automation-core`. A single branch (`main`) serves every Rancher version that uses it. The key case showing the tool cannot assume release branches.

## Layout and versioning
* `main` is the only live line. A stale `release/v0.1.x` exists upstream (last commit PR #22), and `head-builds.yml` and `fossa.yml` still list `release/**` patterns nothing uses.
* About 58 `v*` tags: alpha (`v0.0.0-alpha.N`), PR builds (`v0.0.0-pr.112.b1`), RC (`v0.5.2-rc.4`, up to `rc.6` per patch since `v0.3.1`), stable (`v0.5.3`).
* Stable versus RC is decided only by the semver prerelease suffix. `.github/scripts/check-semver` implements it (regex `^[0-9]+\.[0-9]+\.[0-9]+-`) and `common.sh` duplicates it as `is_stable_release`.
* Compatibility is not expressed in this repo: no support matrix, no per-Rancher branch. The operator reads Rancher's version at runtime. The only mapping is the downstream branch list; the compatible image per Rancher line lives in rancher/rancher's `build.yaml` (`defaultSccOperatorImage`).

## Workflows and release process
* All workflows are hand-written here, with no shared workflows from other orbs repos. Files: `ci.yml` (reused by release via `workflow_call`), `lint.yml`, `fossa.yml`, `head-builds.yml`, `release.yml`, `push-to-rancher.yml`, `renovate-vault.yml`, `label-all-new-issues.yml`, `stale.yml`.
* External shared pieces: `rancher/ecm-distro-tools/actions/publish-image`, `rancher-eio/read-vault-secrets`, `rancher/renovate-config` (SHA-pinned).
* **Releases are cut by humans** pushing a `v*`-style tag. Nothing in the repo creates tags. `release.yml` triggers on tag pattern `*` (any tag).
* `release.yml` job order: `ci`, then `goreleaser` and `image` in parallel, then `release` (undraft; polls 12 × 10 seconds because goreleaser creates a draft), then `push-to-rancher` only for non-prerelease tags in `rancher/scc-operator`.
* Publishing: images to Docker Hub and Prime staging, plus Prime prod for stable tags only. Binaries (linux amd64/arm64, dev/debug) get a provenance attestation. No chart is published yet. There is no explicit cosign step; signing probably happens inside `publish-image` (not verified).
* **Downstream:** `push-to-rancher` is bash (`run-gha.sh`, `create-prs.sh`, `update-build-yaml.sh`). It validates the image exists in every applicable registry, then per Rancher branch creates `bot/scc-operator-<tag>-<branch>-<epoch>`, sets `defaultSccOperatorImage` with `yq`, runs `go generate`, commits, pushes and opens `[v2.x] Update SCC Operator to <tag>` (label `status/auto-created`). A failing branch is skipped and listed in the summary.
* **Fan-out:** every stable tag goes to all five Rancher branches unconditionally (`main`, `release/v2.12`–`v2.15`). RCs are manual-only via `workflow_dispatch`.

## Config and topology facts
* The only topology is the hardcoded branch array in `.github/workflows/push-to-rancher/common.sh` plus its README ("Add new Rancher versions as they're released. Remove EOL versions."). Overridable with `RANCHER_BRANCHES_OVERRIDE` or `--branches`.
* `.github/renovate.json` sets `baseBranchPatterns: [main]`. Version is injected via goreleaser ldflags and `scripts/version`.

## Auth and cross-repo
* `GITHUB_TOKEN` for release and goreleaser.
* Vault via OIDC for Docker Hub, Prime staging/prod, FOSSA and the GitHub App credentials.
* `actions/create-github-app-token` scoped to `repositories: rancher` to push to rancher/rancher.
* Many steps are guarded by `github.repository == 'rancher/scc-operator'` because forks run the same workflows.

## Pain points
* The downstream branch list is manual and encoded in bash; nobody can ask which Rancher branches carry operator version X without scanning rancher/rancher.
* `push-to-rancher.yml` uses `env.RANCHER_REF`, which is never defined, so the initial checkout uses the default branch.
* Semver check duplicated in two places; `release/v0.1.x` and the `release/**` triggers are dead or speculative; `release.yml` triggers on any tag with no format check.

## Implications for the tool
* Release branches must be optional; a repo declares its release-branch policy (`single` or `perLine`).
* A repo's version line is separate from its consumers' branches. One line, N consumers, where each consumer declares its own `accept` rules (stable-only, RC-only, which lines).
* Fan-out reports per-target state and tolerates partial failure.
* The reconciler observes human-pushed tags.
* Borrowable details: image-exists validation, skip when the value already matches (exit code 2), bot branch naming, `--dry-run` and local-run parity, manual dispatch for RC pushes.
