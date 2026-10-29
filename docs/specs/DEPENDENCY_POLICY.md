# Dependency Policy

**Status:** agreed. The YAML library is `go.yaml.in/yaml/v3`, moving to v4 once it is stable.

* **Standard library first.** Fewer dependencies means a smaller supply-chain surface and less to audit.
* A new dependency needs an entry here (or the IMPLEMENTATION section it belongs to) saying what the standard library would cost us. Enforced by a `depguard` allow-list in `.golangci.yml`, so an unlisted import fails lint. A second, narrower rule restricts `cmd/` to the standard library, cobra, `cmd/...`, `internal/cli/...`, `internal/clicfg` and `internal/errs`, a `nocobra` rule keeps cobra out of everything except `cmd/`, and a `noyaml` rule keeps YAML out of everything except `internal/clicfg` and `internal/config` ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 6).
* Approved now:

| Module | For |
|---|---|
| `github.com/spf13/cobra` | Command tree, flags, help, completions ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)) |
| `github.com/cli/go-gh/v2` | **Only `pkg/auth`**: token and host resolution, bridging off the user's `gh` setup (IMPLEMENTATION §3). Doing it by hand would mean re-implementing the env var order, the `hosts.yml` reading and the keyring fallback through `gh auth token`. `pkg/api` is not approved: it brings a GraphQL library, `httpretty` and a timezone library, and is added only if GraphQL beats REST for a real need. |
| `github.com/google/go-github` | Typed REST, authenticated with its own `WithAuthToken` (IMPLEMENTATION §3) |
| `github.com/go-git/go-git` | Local git operations: refs, tags, clone, commit (IMPLEMENTATION §3). Preferred over shelling out to `git`: no binary needed on the machine, no subprocess inheriting environment and config, same behavior locally and in CI. Costs a larger transitive tree than `os/exec`, accepted for portability and security. |
| `go.yaml.in/yaml/v3` | **In use.** The one YAML library, for the user file (`internal/clicfg`) and, later, application config. Chosen because it supports strict decoding (`KnownFields(true)`, so an unknown key is a loud error) and multi-document streams (`yaml.NewDecoder`), and it was already in the tree through testify. Stay on v3 until `go.yaml.in/yaml/v4` has a stable release (it is at release candidates), then migrate. Until then it is importable only from `internal/clicfg` and `internal/config` (lint-enforced), so the migration touches known files, and the loader tests pin the behavior that must not change: unknown keys, malformed input, empty files, errors naming the file. |
| `github.com/stretchr/testify` | **Tests only, in use.** `require` and `assert` for the strong test-first workflow ([TESTING](TESTING.md)); the standard library makes table tests and diffs of structs noisy. |
| `go.uber.org/mock` | **Tests only, in use** (`mockgen` is pinned as a `tool` in `go.mod`). `mockgen`-generated mocks for interfaces at package seams (the GitHub access layer, `internal/ui`), so tests drive behavior without network access. |

* **Developer tools that are not imported stay out of `go.mod`.** `gremlins` (mutation testing, [TESTING](TESTING.md)) is optional and installed globally by whoever wants it, because its own dependencies do not fit this policy. Tools pinned with a `tool` directive, like `mockgen`, are fine when their dependencies are acceptable.
* Test-only modules are imported from `_test.go` files and from generated mock packages, never from production code. The `depguard` `main` rule excludes `$test`, so they stay out of the production allow-list. Use them where they help, not by default: plain `testing` is fine for a simple case.
* **Integration-test dependencies live in the `test/integration` sub-module**, never in the root `go.mod` ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 5). The same rule applies: each one gets an entry here saying what the standard library would cost us. None approved yet.
* `os/exec` calling `git` is an escape hatch, allowed only for an operation go-git cannot do (IMPLEMENTATION §3). It is not a runtime fallback and has no obligation to mirror go-git. It is lint-enforced: `os/exec` is importable only from `internal/git/gitcli` and `internal/ghcli`, and go-git only from `internal/git/repo`. In use: `gitcli.ShortCommit` runs `git rev-parse --short HEAD`, because git's abbreviation length depends on the repository's object count and go-git has no equivalent ([BRANCH_TAGS](BRANCH_TAGS.md) section 3). `gitcli.Version` and `gitcli.Config` run `git --version` and `git config --get` for `gero doctor` ([DOCTOR](DOCTOR.md)): the question there is what the git binary would do, with every include and system rule it has, which go-git does not model.
* **`internal/ghcli` runs the `gh` binary for `gero doctor` only**: it finds `gh` the way go-gh does (`GH_PATH`, then `PATH`) and runs `gh --version`, so the report says which `gh` token resolution would use. It is not how tokens are read: go-gh's `pkg/auth` runs `gh auth token` itself, so there is no `gh` call in the API path ([IMPLEMENTATION §3](../IMPLEMENTATION.md)).
* Not adopted: `viper`, color or logging libraries, any TUI library ([CLI_FRAMEWORK](CLI_FRAMEWORK.md), [CLI_OUTPUT](CLI_OUTPUT.md)).

## Open questions

* Whether the integration sub-module needs a fake GitHub server library or hand-rolled `httptest` handlers, decided with the first integration test.