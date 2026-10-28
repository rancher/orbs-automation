# rancher/rancher

Consumer. Seen only from the producer side (rancher-shell's `push-to-rancher`, scc-operator's `push-to-rancher`, frameworks-automation's bumps) plus a few read-only GitHub API calls. No local clone.

## Structure the producers edit
* `build.yaml`: `defaultShellVersion`, `defaultSccOperatorImage`, `webhookVersion` (e.g. `111.0.0+up0.12.1-rc.12`), `remoteDialerProxyVersion`, `chartDefaultBranch` (currently `dev-v2.16`).
* Go modules: frameworks-automation runs `go get` on every non-vendor `go.mod` that references the module (`GOFLAGS=-mod=mod`), then tidy, vendor and a post-bundle `sync-deps` hook that aligns the `pkg/client` and `pkg/apis` modules to the root.
* Generated files: producers run `go generate` (shell and scc-operator regenerate `pkg/buildconfig/constants.go` and `chart/values.yaml`). `GOTOOLCHAIN` is forced to the downstream's pinned toolchain so generated files do not drift.
* The full webhook version (`111.0.0+up…`) is looked up at runtime from `rancher/charts` `index.yaml` by `appVersion`, which is why the charts PR must merge first.

## Branches
* `main` is the current development line (v2.16). **No `release/v2.16` exists yet**; it is created at branch cut, at which point `main` moves to the next line.
* Release branches `release/v2.X` exist for earlier lines.

## PR conventions and checks
* Example bot PRs: `[main] Update rancher-assets to …` (branch `bot/rancher-assets-…-main-<epoch>`, label `status/auto-created`), and `[v2.15] Update SCC Operator to v0.5.3` on `release/v2.15` (same label). Bot account: `app/rancher-pr-actions-write`.
* Producer conventions differ: shell and scc-operator use `bot/<name>-<tag>-<ver>-<epoch>` and `status/auto-created`. frameworks-automation uses `automation/…` branches, applies no labels, and reuses an existing open PR for its head branch.
* Checks: build-server and build-agent, integration tests, `check-changes`. The example bot PR (#58419) was approved. Branch protection could not be read.

## Auth seen from producers
* A GitHub App token scoped to `repositories: rancher` (shell, scc-operator), or a per-repo fine-grained token (frameworks-automation).

## Implications for the tool
* `main`-as-current-line must be a first-class concept in the line map, with branch-cut as an event.
* Producer bumps differ (`yq` on `build.yaml`, `go get`, index lookups), so the edit step is a named strategy, not config.
* Consumer conventions (labels, title prefix like `[v2.15]`) belong in a per-consumer PR template.
* Fan-out to several Rancher branches needs per-target state and partial-failure handling.
