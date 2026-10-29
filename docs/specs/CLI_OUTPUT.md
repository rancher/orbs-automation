# CLI Output

**Status:** agreed. Logging is built (the `-v` flag, `GERO_LOG`, the logger in the root command). Output rules in section 1 are followed by the commands that exist; `--no-color` and `internal/ui` are not built.

How `gero` talks to the user: command output, logging, and the seam for interactive features. Flags that control these are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md).

## 1. Output

* **Data to stdout, diagnostics to stderr.** A script can always pipe stdout.
* Commands whose results feed shell or GitHub Actions (`gero semver check`, `gero repo branch-tags`) also support `--output env`: `KEY=value` lines on stdout, one per line, safe to append to `$GITHUB_OUTPUT` or `$GITHUB_ENV`. Key names are documented next to the command.
* Human output uses `text/tabwriter` and plain lines. No table or styling library.
* Every command that emits data supports `--json` through `encoding/json`, with a stable field naming scheme documented next to the command. JSON output is never interleaved with human text.
* Color is off when `NO_COLOR` is set, `--no-color` is passed, or stdout is not a TTY. No color library until output needs more than a few escape codes.

## 2. Logging

Standard library `log/slog`, written to stderr. Default level `warn`, `-v` for `info`, `-vv` for `debug`, or `GERO_LOG=debug|info|warn|error`. A text handler locally and a JSON handler when running in CI. Business code receives a `*slog.Logger`; there is no package-level logger.

* **Precedence.** `-v` beats `GERO_LOG`, which beats the default, the same order as every other setting ([CLI_CONFIG](CLI_CONFIG.md)). The level is a setting named `log`, so `gero config show` reports it and its source. There is no user-file key for it.
* **stdout is untouched.** Logs go to stderr only, so `-v` never changes `--json` or `--output env` output.
* **Levels.** `warn` is something the user should act on. `info` is what the command decided or did: inferred values, repositories touched, writes. `debug` is how it did it: git invocations, API calls, timings. Which events land at which level is recorded here as commands add them.
* **Built so far.** `debug` logs the resolved settings once, after the flags and the environment are read. No command logs yet.

## 3. TUI and interactive features

Deferred. Nothing in v0.x or v1.0.0 needs prompts beyond handing off to `gh auth login`.

* Core commands stay non-interactive and scriptable.
* Any prompt or rich rendering goes through one internal package, `internal/ui`, behind an interface with a plain-text implementation first. This is the seam that lets a TUI arrive later without touching commands.
* Interactive behavior is opt-in and automatically disabled when stdin or stdout is not a TTY, or when running in CI.
* The likely future stack is the Charm libraries (bubbletea, huh, lipgloss). No dependency is added until a feature needs it, and the addition then gets an entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md). The web dashboard (ARCHITECTURE_DESIGN §6) is a separate, later surface.

## Open questions

None yet.
