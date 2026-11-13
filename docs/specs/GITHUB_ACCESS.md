# GitHub Access

**Status:** implemented (`internal/platform/github`): token and host resolution and the REST client, except what [Not built yet](#not-built-yet) lists.

How `gero` talks to GitHub and to git. Two Go libraries each do one job, and a subprocess is used only where it adds something. A spike (go-gh v2.17.0, go-github v93.0.0) confirmed the behavior below. Which modules are approved, and why, is in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md).

## Token and host: `cli/go-gh`, `pkg/auth` only

`pkg/auth` resolves the token and the host, so `gero` bridges off the user's `gh` setup on a workstation. It is an ordinary Go module and does not require running as a `gh` extension.

`pkg/api` (HTTP client, GraphQL) is **not** imported. It pulls in a GraphQL library, `httpretty` and a timezone library, and go-github can authenticate by itself. `pkg/auth` imports only `gopkg.in/yaml.v3` and `safeexec`. The YAML import is a second copy of YAML code in the binary that our code cannot import, used only to read `gh`'s config files. A `replace gopkg.in/yaml.v3 => go.yaml.in/yaml/v4` works, but only at a version different from the one we require (Go rejects the same version under two paths). It swaps an unmaintained source for a maintained one without removing the second copy, and it breaks the day Renovate bumps the replacement to our version. We do not add it.

* **Token order** for `github.com`, `ghe.com` and `github.localhost`: `GH_TOKEN`, `GITHUB_TOKEN`, `hosts.<host>.oauth_token` in `gh`'s config file, then `gh auth token --secure-storage --hostname <host>`. `gh` is found through `GH_PATH`, then `PATH`.
* **Enterprise hosts** (any other host) ignore `GH_TOKEN` and `GITHUB_TOKEN`. They use `GH_ENTERPRISE_TOKEN`, `GITHUB_ENTERPRISE_TOKEN`, the config file, then `gh`.
* **Failures are silent in go-gh**: no token, a missing `gh` or a failing `gh` all give `"", "default"`. `gero` wraps that in an error that says which variables to set, or to run `gh auth login`.
* **The `gh` config file is read once per process**, so a test sets `GH_CONFIG_DIR` before the first call.
* **`DefaultHost`** honors `GH_HOST`, then a lone host in `hosts.yml`, then `github.com`.
* **Only github.com is supported.** Another host is tolerated where the code happens to work, such as token resolution and an Enterprise Server API client, and refused where it does not: `NewClient` for `*.ghe.com` and `github.localhost`, `RawFile`, and the SSH URL of `gero repo clone`. Support for other hosts is built only if someone asks.

All go-gh use stays in `internal/platform/github/gogh.go` (lint-enforced).

## REST: `google/go-github`

Typed REST calls: pull requests (find, create, edit, close), labels and comments, whether a fork exists, how far a branch is behind, user ids and the rate limit today; issues, check runs and releases are planned. It is the main API client and its types stay inside `internal/platform/github` (lint-enforced).

* Authenticate with `github.WithAuthToken(token)` using the token from `pkg/auth`. The token goes only to the client's own API origins, sent as `Authorization: Bearer`.
* Tests inject a fake `http.RoundTripper` with `github.WithTransport`, so no network or port is needed.
* go-github bumps major versions often, and v93 changed `NewClient` to take options and return an error. Pin it and keep usage behind our own interfaces.
* Enterprise hosts need the URLs set explicitly: `WithEnterpriseURLs("https://ghe.example.com", "https://ghe.example.com")` gives `/api/v3/` and `/api/uploads/`.

## Local git: the `git` binary, with go-git for reads

Most local git work runs the `git` binary, through `internal/platform/git/gitcli`, which starts it through `internal/platform/execx`: cloning, fetching, reading the state of a working tree, starting a branch, committing, signing and pushing. It was chosen for speed of building and because the engineer's own git is what a push, a signature and a credential helper need: their SSH agent, `~/.ssh/config`, `gpg.*` and hooks. The reasoning for push and signing is in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md).

`go-git/go-git` reads a repository: the current ref, the abbreviated commit, and tags by date. It needs no `git` binary for that, and only `internal/platform/git/repo` imports it.

Both sit behind our own interfaces, so a go-git implementation of the `gitcli` operations could replace the binary later. `gero repo clone` marks the seam with `TODO(go-git)`. A command that needs a working tree needs the `git` binary on `PATH`, which `gero doctor` checks.

## `gh` via subprocess

Only for interactive or human-facing steps (for example `gh auth login` and browser handoffs) and pure passthrough. It is not used for logic that must be tested or that runs in CI. `gero doctor` runs `gh --version` through `internal/gero/ghcli` so its report says which `gh` token resolution would use. Tokens are never read this way: go-gh runs `gh auth token` itself.

All subprocesses go through `internal/platform/execx`, the one place `gero` starts another program. Its contract is in the package doc.

## GraphQL

Only if REST proves insufficient (for example batched state queries). That is the point to add the rest of `go-gh` (`pkg/api`) as a dependency, with its own entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md). Until then REST goes through go-github.

## Interfaces

* Define **narrow interfaces on the consumer side**, not one large `GitHub` interface mirroring the API.
* The reconciler gets one (list tags, get PR state, list check runs, read and update an issue body, post a comment) so it can be tested against a fake.
* Do not wrap `gh` subprocess calls in the same interface.
* Skip interfaces for one-shot commands (for example `workflows sync`) until a second implementation or a test needs one.
* The reconciler needs its own backoff policy for rate limits and retries.

## Not built yet

* The reconciler's interface over the client, and the reads and writes of issues, check runs and releases.
* Enterprise hosts under `*.ghe.com` and `github.localhost`, built only if someone asks: `NewClient` refuses them until their API address is worked out.
* A go-git client for the git work that runs the `git` binary.

## Open questions

None yet.
