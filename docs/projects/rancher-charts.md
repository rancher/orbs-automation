# rancher/charts

Consumer. Seen only from the producer side (BRO's `push-to-charts`, frameworks-automation's chart bump scripts) plus a few read-only GitHub API calls. We have no local clone, so this is thinner than the producer notes.

## Structure the producers edit
* `packages/<pkg>/package.yaml` (`url`, `version`, `additionalCharts`), `release.yaml` (package to list of `<chartsver>+up<upstream>`), `charts/`, `assets/`, `index.yaml`.
* Charts versions encode the Rancher line in the major (e.g. 106 is v2.11, 110 is v2.15, 111 is v2.16). No written rule was found; each producer computes it independently.
* Branches: `dev-vX` (development) and `release-vX`.

## How producers bump it
* **BRO:** `make remove`, yq edits to `release.yaml` and `package.yaml`, awk edit of the appVersion line in `Chart.yaml.patch`, `make prepare`, `make patch`, `make clean`, `make charts`, then a yq prepend to `release.yaml`. One commit per step (`chore(charts): …`). Branch `bot/bro-${TAG}-$(date +%s)`, title `[dev-v2.15] rancher-backup 110.0.3+up11.0.4 UnRC` (`bump` for prereleases, `UnRC` for stable), one-line body, no labels. The charts version is computed in bash (same base: keep; major changed: +1.0.0; minor: +1; else patch +1).
* **frameworks-automation:** `sed -i` on `package.yaml` (GNU-only), `PACKAGE=… make charts`, yq prepend to `release.yaml`. It does not run `make prepare` or `make patch`. Patch-only version bump, unchanged when the previous version was an RC. It runs `git fetch origin automation-core` first because `make charts` needs that ref. Branch `automation/…`, title `Bump github.com/rancher/webhook to v0.12.1-rc.14`, body links a tracker issue, no labels.

## PR conventions and checks
* The PR template requires the title `[dev-v2.X] <chart> <version> <action>`, one chart per PR, and `release.yaml` checkpoints. frameworks-automation titles do not follow it.
* Checks: a `Validate` check. The example bot PRs seen (#9224, #9308) were approved by a human or app. Branch protection could not be read (404 without admin access).
* Charts has its own `auto-bump` workflows (`auto-bump.yaml`, `auto-bump-manual-trigger.yaml`, `auto-bump-merge.yaml`, with an `auto-merge` label). They do not fit most of our use cases, so we ignore them. Neither BRO nor shell applies `auto-merge`.
* No CONTRIBUTING or README process section was found; the README is an overview only.

## Auth seen from producers
* BRO: GitHub App (Vault OIDC, `repositories: charts`), clones with `oauth2:$GH_TOKEN`. Also reads `rancher/org` for a freeze check.
* frameworks-automation: a fine-grained `pull_requests--write` token per repo from Vault.

## Seen from rancher-assets
* `rancher-assets` reads the heads of `dev-vX` and `release-vX` for each line (its `config.yaml`) and pins the commits in `lock.yaml`. A promotion from `dev` to `release` is not in that repo: it shows up as a changed `release` commit after the next bump. See [rancher-assets](rancher-assets.md).
* So the two branch families are tiers of one line: our packages land on `dev-vX`, and the release team promotes to `release-vX`. Chain tracing models them as stages of the project ([CHAIN_TRACING](../specs/CHAIN_TRACING.md), stretch 2).

## Implications for the tool
* One shared implementation of the chart bump (version arithmetic, patch refresh, `make` steps), replacing the two divergent copies.
* Standardize producers on the charts PR title format and labels.
* Branch family `dev-vX` / `release-vX` must be representable in the line map.
* Freeze checks and "image/release exists" checks belong in preconditions.
