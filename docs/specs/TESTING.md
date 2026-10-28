# Testing

**Status:** agreed. Nothing is built yet.

How the CLI layer is tested.

* Build the root command in a test, run it in process with captured stdout and stderr, and assert on output and exit code. Golden files live under `testdata/`.
* Standard library `testing` only; no `testify` for now ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)).
* `internal/clicfg` is table-tested: each source alone, each pairwise precedence, unknown key in the user file, malformed file, missing file, CI ignoring the user file ([CLI_CONFIG](CLI_CONFIG.md)).
* Gating is tested by running a CI-only command with and without `GITHUB_ACTIONS` and a token ([CLI_FRAMEWORK](CLI_FRAMEWORK.md) section 3).
* Follows the commit workflow in [`ai/red-green-commits.md`](../ai/red-green-commits.md).

## Open questions

None yet.
