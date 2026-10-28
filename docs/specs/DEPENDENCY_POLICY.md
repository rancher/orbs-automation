# Dependency Policy

**Status:** agreed. The YAML library is still to be chosen.

* **Standard library first.** Fewer dependencies means a smaller supply-chain surface and less to audit.
* A new dependency needs an entry here (or the IMPLEMENTATION section it belongs to) saying what the standard library would cost us. Enforced by a `depguard` allow-list in `.golangci.yml`, so an unlisted import fails lint.
* Approved now:

| Module | For |
|---|---|
| `github.com/spf13/cobra` | Command tree, flags, help, completions ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)) |
| `github.com/cli/go-gh` | GitHub auth, host resolution, HTTP client (IMPLEMENTATION §3) |
| `github.com/google/go-github` | Typed REST (IMPLEMENTATION §3) |
| One YAML library | Application config and the user file. **(unverified)** Pick one that supports multi-document decoding and rejecting unknown fields, and record it here. |
| `github.com/stretchr/testify` | **Tests only.** `require` and `assert` for the strong test-first workflow ([TESTING](TESTING.md)); the standard library makes table tests and diffs of structs noisy. |
| `go.uber.org/mock` | **Tests only.** `mockgen`-generated mocks for interfaces at package seams (the GitHub access layer, `internal/ui`), so tests drive behavior without network access. |

* Test-only modules are imported from `_test.go` files and from generated mock packages, never from production code. The `depguard` `main` rule excludes `$test`, so they stay out of the production allow-list. Use them where they help, not by default: plain `testing` is fine for a simple case.
* **Integration-test dependencies live in the `test/integration` sub-module**, never in the root `go.mod` ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 5). The same rule applies: each one gets an entry here saying what the standard library would cost us. None approved yet.
* Not adopted: `viper`, color or logging libraries, any TUI library ([CLI_FRAMEWORK](CLI_FRAMEWORK.md), [CLI_OUTPUT](CLI_OUTPUT.md)).

## Open questions

* Whether the integration sub-module needs a fake GitHub server library or hand-rolled `httptest` handlers, decided with the first integration test.
* Which YAML library.
