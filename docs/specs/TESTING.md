# Testing

**Status:** implemented for the unit tier. The integration tier is agreed, not built.

How the CLI layer is tested.

* Build the root command in a test, run it in process with captured stdout and stderr, and assert on output and exit code. Golden files live under `testdata/`.
* Strong test-first: write the failing test before the code. Use `testing` as the runner, with `testify` (`require` for preconditions, `assert` for checks) and `go.uber.org/mock` where they help ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)).
* Mock only at package seams (interfaces the consumer defines, such as the GitHub access layer). Generated mocks are committed and regenerated with `go generate`; do not hand-edit them. The convention: a `//go:generate go tool mockgen -source=<file>.go -destination=mock_<name>_test.go -package=<pkg>` directive beside the interface, with the mock written to a `_test.go` file in the same package, so depguard treats it as test code and the mock never reaches the binary. `mockgen` is pinned as a `tool` in `go.mod`, so no global install is needed. A mock earns its place when the call itself is the assertion (for example, CI mode must never call `Head`); otherwise a hand-written fake is simpler (`git.Tags` in `internal/platform/git`). Prefer a small hand-written fake when a mock would just restate the implementation.
* `internal/platform/clicfg` is table-tested: each source alone, each pairwise precedence, unknown key in the user file, malformed file, missing file, CI ignoring the user file ([CLI_CONFIG](CLI_CONFIG.md)).
* Styling is tested without a terminal: each command has a test that holds the styled output, with the escapes taken out, equal to the plain output, and that `--json` and no color carry none. `ui.Align` is checked against `text/tabwriter`, and `internal/cli/termfacts` against a pipe, a file and `/dev/null`. The REPL and the editor are driven with a fake reader or a pipe, and never need a pty. What a real terminal does (the color depth, the prompt, the editing keys) is checked by hand.
* Gating is tested by running a CI-only command with and without `GITHUB_ACTIONS` and a token ([CLI_FRAMEWORK](CLI_FRAMEWORK.md) section 3).
* **Two tiers.** Unit tests (root module, fast, no network, run on every commit) and integration tests (the `test/integration` sub-module, black box against the built `gero` binary, own CI job). Heavy test dependencies go only in the sub-module ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 5). If a test needs the network, a container or a real process, it belongs in the second tier.
* Follows the commit workflow in [`ai/red-green-commits.md`](../ai/red-green-commits.md).
* **Mutation testing is optional, with [gremlins](https://github.com/go-gremlins/gremlins).** To check how strong the tests are, run `gremlins unleash` from the repository root. It mutates a temporary copy, so the working tree is untouched, and it takes seconds at the current size. It is a developer tool each person installs globally (package manager, or `go install github.com/go-gremlins/gremlins/cmd/gremlins@latest`). It is **not** in `go.mod`, since it brings dependencies (viper, among others) that the [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) keeps out of the root module, and it is not a CI gate. Do not mutate code by hand to check a test.
  * Fix **lived** mutants with a test, or with simpler code when the mutant is equivalent (for example `len(s) > 7` to `>=` before `s[:7]`: `min(len(s), 7)` has nothing to mutate).
  * Known noise: gremlins reports a `case` condition in a `switch` as not covered, because Go's coverage profile only attributes the case body. A `main`-style `os.Exit` wrapper such as `cmd.Execute` is also not covered, by design.

## Not built yet

* The `test/integration` sub-module and its CI job.
* Golden files under `testdata/`.

## Open questions

None yet.
