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

* Not adopted: `viper`, `testify`, color or logging libraries, any TUI library ([CLI_FRAMEWORK](CLI_FRAMEWORK.md), [CLI_OUTPUT](CLI_OUTPUT.md), [TESTING](TESTING.md)).

## Open questions

* Which YAML library.
