# Dependency Policy

**Status:** agreed. The YAML library is still to be chosen.

* **Standard library first.** Fewer dependencies means a smaller supply-chain surface and less to audit.
* A new dependency needs an entry here (or the IMPLEMENTATION section it belongs to) saying what the standard library would cost us. Enforced by a `depguard` allow-list in `.golangci.yml`, so an unlisted import fails lint. A second, narrower rule restricts `cmd/` to the standard library, cobra, `cmd/...`, `internal/cli/...` and `internal/errs`, and a `nocobra` rule keeps cobra out of everything except `cmd/` ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 6).
* Approved now:

| Module | For |
|---|---|
| `github.com/spf13/cobra` | Command tree, flags, help, completions ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)) |
| `github.com/cli/go-gh` | GitHub auth, host resolution, HTTP client (IMPLEMENTATION §3) |
| `github.com/google/go-github` | Typed REST (IMPLEMENTATION §3) |
| `github.com/go-git/go-git` | Local git operations: refs, tags, clone, commit (IMPLEMENTATION §3). Preferred over shelling out to `git`: no binary needed on the machine, no subprocess inheriting environment and config, same behavior locally and in CI. Costs a larger transitive tree than `os/exec`, accepted for portability and security. |
| One YAML library | Application config and the user file. **(unverified)** Pick one that supports multi-document decoding and rejecting unknown fields, and record it here. |
| `github.com/stretchr/testify` | **Tests only, in use.** Pulls in `go.yaml.in/yaml/v3` transitively, which is a test-time dependency and does not decide the YAML library question below. `require` and `assert` for the strong test-first workflow ([TESTING](TESTING.md)); the standard library makes table tests and diffs of structs noisy. |
| `go.uber.org/mock` | **Tests only, in use** (`mockgen` is pinned as a `tool` in `go.mod`). `mockgen`-generated mocks for interfaces at package seams (the GitHub access layer, `internal/ui`), so tests drive behavior without network access. |

* **Developer tools that are not imported stay out of `go.mod`.** `gremlins` (mutation testing, [TESTING](TESTING.md)) is optional and installed globally by whoever wants it, because its own dependencies do not fit this policy. Tools pinned with a `tool` directive, like `mockgen`, are fine when their dependencies are acceptable.
* Test-only modules are imported from `_test.go` files and from generated mock packages, never from production code. The `depguard` `main` rule excludes `$test`, so they stay out of the production allow-list. Use them where they help, not by default: plain `testing` is fine for a simple case.
* **Integration-test dependencies live in the `test/integration` sub-module**, never in the root `go.mod` ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 5). The same rule applies: each one gets an entry here saying what the standard library would cost us. None approved yet.
* `os/exec` calling `git` is an escape hatch, allowed only for an operation go-git cannot do (IMPLEMENTATION §3). It is not a runtime fallback and has no obligation to mirror go-git. It is lint-enforced: `os/exec` is importable only from `internal/git/gitcli`, and go-git only from `internal/git/repo`. In use: `gitcli.ShortCommit` runs `git rev-parse --short HEAD`, because git's abbreviation length depends on the repository's object count and go-git has no equivalent ([BRANCH_TAGS](BRANCH_TAGS.md) section 3).
* Not adopted: `viper`, color or logging libraries, any TUI library ([CLI_FRAMEWORK](CLI_FRAMEWORK.md), [CLI_OUTPUT](CLI_OUTPUT.md)).

## Open questions

* Whether the integration sub-module needs a fake GitHub server library or hand-rolled `httptest` handlers, decided with the first integration test.
* Which YAML library.
