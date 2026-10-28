# Testing

**Status:** agreed. Nothing is built yet.

How the CLI layer is tested.

* Build the root command in a test, run it in process with captured stdout and stderr, and assert on output and exit code. Golden files live under `testdata/`.
* Strong test-first: write the failing test before the code. Use `testing` as the runner, with `testify` (`require` for preconditions, `assert` for checks) and `go.uber.org/mock` where they help ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)).
* Mock only at package seams (interfaces the consumer defines, such as the GitHub access layer). Generated mocks are committed and regenerated with `go generate`; do not hand-edit them. Prefer a small hand-written fake when a mock would just restate the implementation.
* `internal/clicfg` is table-tested: each source alone, each pairwise precedence, unknown key in the user file, malformed file, missing file, CI ignoring the user file ([CLI_CONFIG](CLI_CONFIG.md)).
* Gating is tested by running a CI-only command with and without `GITHUB_ACTIONS` and a token ([CLI_FRAMEWORK](CLI_FRAMEWORK.md) section 3).
* **Two tiers.** Unit tests (root module, fast, no network, run on every commit) and integration tests (the `test/integration` sub-module, black box against the built `gero` binary, own CI job). Heavy test dependencies go only in the sub-module ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 5). If a test needs the network, a container or a real process, it belongs in the second tier.
* Follows the commit workflow in [`ai/red-green-commits.md`](../ai/red-green-commits.md).

## Open questions

None yet.
