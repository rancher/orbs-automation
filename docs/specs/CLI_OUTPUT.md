# CLI Output

**Status:** implemented for output and logging. Color control and `internal/ui` are agreed, not built.

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
* **Today** the root command builds the logger (`internal/cli/logging`) and `debug` logs the resolved settings once. The logger is handed to the first command that logs, and `workspace` then logs the resolved config directory and its source at `debug`.

## 3. TUI and interactive features

Deferred. Nothing in v0.x or v1.0.0 needs prompts beyond handing off to `gh auth login`.

* Core commands stay non-interactive and scriptable.
* Any prompt or rich rendering goes through one internal package, `internal/ui`, behind an interface with a plain-text implementation first. This is the seam that lets a TUI arrive later without touching commands.
* Interactive behavior is automatically disabled when stdin or stdout is not a TTY, or when running in CI. The one prompt the core commands have is the confirmation before a write ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)), a plain yes/no through `internal/ui`, skipped with `--non-interactive`.
* The likely future stack is the Charm libraries (bubbletea, huh, lipgloss). No dependency is added until a feature needs it, and the addition then gets an entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md). The web dashboard (ARCHITECTURE_DESIGN §6) is a separate, later surface.

### Library research for the confirmation, color and tables

Read from module sources and `go.mod` files, **(unverified)** by a build: the sandbox's module proxy was unreachable and the offline cache lacks the dependencies of the extra `go-gh` packages and of `huh`, so only the stdlib variant was built.

| Option | Gives | Cost |
|---|---|---|
| Our own yes/no on `golang.org/x/term` | TTY check, plain `[y/N]` line read from stdin | About 20 lines. `x/term` is already in `go.sum` through go-git, so it is a free promotion to a direct dependency |
| `go-gh/pkg/term` | TTY, size, color-depth detection | Pulls only `termenv`. Reads `GH_FORCE_TTY`, `CLICOLOR*` and `NO_COLOR`, so it follows `gh`'s variables, not `GERO_*` |
| `go-gh/pkg/prompter` | `Confirm`, `Select`, `Input`, `Password` | Built on `AlecAivazis/survey`, which is archived upstream. Also imports `pkg/text`, which pulls lipgloss v2 |
| `go-gh/pkg/tableprinter` | TTY table, TSV when piped | Imports `pkg/text`, so lipgloss v2 and reflow too |
| `huh` (Charm) | Confirm, select, input, forms, an accessible plain mode | About 25 modules: bubbletea, bubbles, lipgloss, catppuccin, xpty and more. Raw mode and redraws, so it needs a real TTY and is harder to test |
| `lipgloss` | Styles, layout, color profile | Worth it for styled tables and panels, not for a few escape codes |

* **The confirmation does not justify a library.** One yes/no is a few lines behind `internal/ui`, which is testable with a fake reader and has no non-TTY path to get wrong. Everything else (`Select`, `Input`, forms) is where `huh` earns its weight, and `internal/ui` is the seam that makes adopting it a change in one file **(leaning)**.
* **Detection stays ours.** TTY, `NO_COLOR`, `--no-color` and the `GERO_*` variables are decided in `internal/clicfg` next to the context signs. Importing `go-gh/pkg/term` would add a second reader of `CLICOLOR` and friends.
* **Avoid `survey`.** It is archived, and `huh` is the maintained route if prompts grow.
* **Tables and color later.** Plain `text/tabwriter` and a handful of escape codes cover v0.x. If output wants real styling, lipgloss is the pick, and `go-gh/pkg/tableprinter` is the cheaper option if only tables are wanted. Either gets an entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) first.
* **Still to check** with network access: the real module count and binary-size delta of `huh` and `go-gh/pkg/tableprinter` (`go mod why`, `go build` of each), and how `huh` behaves with stdin closed, which is the CI case.

## Not built yet

* `--no-color`, and `NO_COLOR` and TTY detection beyond what commands do by default.
* `internal/ui`.
* Handing the logger to commands.

## Open questions

None yet.
