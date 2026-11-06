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
* `gero repl` is the one command that is interactive by nature: a prompt that runs the other commands. It stays outside the core commands' rule above, they are the same commands and remain scriptable, and it refuses to start without a TTY ([REPL](REPL.md)). Its line editor sits behind `internal/ui` like any prompt.
* Interactive behavior is automatically disabled when stdin or stdout is not a TTY, or when running in CI. The one prompt the core commands have is the confirmation before a write ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)), a plain yes/no through `internal/ui`, skipped with `--non-interactive`.
* The likely future stack is the Charm libraries (bubbletea, huh, lipgloss). No dependency is added until a feature needs it, and the addition then gets an entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md). The web dashboard (ARCHITECTURE_DESIGN §6) is a separate, later surface.

### Library research for the confirmation, color and tables

A spike built one confirm-and-table program per option in a scratch module (`huh` v2 as `charm.land/huh/v2`, `go-gh` v2.17.0, `golang.org/x/term`), then ran each with stdin closed and piped. It did not run on a real TTY, and the checksum database was off, so the downloads were not verified. Items marked **(unverified)** below come from reading source.

| Option | Gives | Cost |
|---|---|---|
| Our own yes/no on `golang.org/x/term` | TTY check, plain `[y/N]` line read from stdin | About 20 lines, 2 modules and a 2.5 MB binary. `x/term` is already in `go.sum` through go-git, so it is a free promotion to a direct dependency |
| `go-gh/pkg/term` | TTY, size, color-depth detection | Pulls only `termenv`. Reads `GH_FORCE_TTY`, `CLICOLOR*` and `NO_COLOR`, so it follows `gh`'s variables, not `GERO_*` |
| `go-gh/pkg/prompter` | `Confirm`, `Select`, `Input`, `Password` | Built on `AlecAivazis/survey`, which is archived upstream. With the table printer, 29 modules and a 10 MB binary, more than `huh` |
| `go-gh/pkg/tableprinter` | TTY table, TSV when piped | Imports `pkg/text`, so lipgloss v2 and reflow too |
| `huh` (Charm) | Confirm, select, input, forms, an accessible plain mode | 25 modules and a 6.1 MB binary for one confirm. Raw mode and redraws, and it opens `/dev/tty` itself, so it is harder to test |
| `lipgloss` v2 with `colorprofile` | Styles, borders, layout, color downgrade | Measured on top of our own prompt: 17 modules (2 without it) and a 4.9 MB binary (2.5 MB without), so about 15 modules and 2.4 MB for styling alone. Degradation works, but only when output goes through a `colorprofile` writer (see below) |
| `text/tabwriter` | Aligned columns | Stdlib: no modules, 2.4 MB binary |

* **The confirmation does not justify a library.** One yes/no is a few lines behind `internal/ui`, which is testable with a fake reader and has no non-TTY path to get wrong. Everything else (`Select`, `Input`, forms) is where `huh` earns its weight, and `internal/ui` is the seam that makes adopting it a change in one file **(leaning)**.
* **Detection stays ours.** TTY, `NO_COLOR`, `--no-color` and the `GERO_*` variables are decided in `internal/clicfg` next to the context signs. Importing `go-gh/pkg/term` would add a second reader of `CLICOLOR` and friends.
* **Avoid `survey`.** It is archived, and `huh` is the maintained route if prompts grow.
* **Tables and color later.** Plain `text/tabwriter` and a handful of escape codes cover v0.x. If output wants real styling, lipgloss is the pick (about 15 modules and 2.4 MB, measured), and `go-gh/pkg/tableprinter` is the cheaper option if only tables are wanted. Either gets an entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) first.
* **Neither prompt library copes without a TTY, so we check first anyway.** With stdin closed or piped, `go-gh`'s prompter still wrote cursor-position escape codes to stdout and then failed with `could not prompt: EOF`, ignoring a piped `y`. `huh` failed with `could not open TTY: open /dev/tty`. Our own line reader returned `EOF` and `y` correctly. The non-TTY rule in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract) (fail unless `--non-interactive`) is a check in `gero`, not something a library gives us.
* **Lip Gloss v1 and v2 differ here, and advice found online mixes them.** In v1 (`github.com/charmbracelet/lipgloss`) `Style.Render` consults a `Renderer` built on `termenv` and strips color itself: a piped `Render` of a bold truecolor string returned plain `ok`. v2 (`charm.land/lipgloss/v2`) removed the `Renderer`. There `Render` always returns full escapes, and `lipgloss.Println` is only `fmt.Fprintln` on a package-level `colorprofile.Writer`, so `fmt.Println(style.Render(...))` leaks them. We would use v2, as `go-gh` and `huh` v2 do. The v2 helpers, read from source: `Println`, `Printf` and `Print` write to stdout using a profile detected once for stdout. `Fprint`, `Fprintln` and `Fprintf` take any writer and detect its profile on each call, so they suit stderr and a test buffer, which is not a TTY and so is stripped. `Sprint`, `Sprintln` and `Sprintf` return a string downsampled for stdout only, so a string built for stderr with them gets stdout's profile. `internal/ui` should use the `F` forms with the command's own `OutOrStdout()` and `ErrOrStderr()`. In v2, `Style.Render` always returns full truecolor escapes, whatever the environment (checked with `NO_COLOR=1`, `CLICOLOR_FORCE=1` and a piped stdout). The downgrade happens when the string is written through `lipgloss.Println` or a `colorprofile.Writer`, which then maps it by profile: truecolor, 4-bit (`\x1b[1;91m`), ASCII (bold kept, color dropped) and no-TTY (everything stripped, text and layout intact). Borders are Unicode box-drawing characters in every profile, and padding, width and alignment are unaffected by color. So output must never bypass the writer, for example by writing a rendered string to `cmd.OutOrStdout()` directly, and `--no-color` has to select the profile in our own code.
* **Not verified:** `NO_COLOR` against a real TTY (a pty was not available), and whether `CLICOLOR_FORCE` beats `NO_COLOR` as `Detect` reported with both set and a piped stdout. Piped detection returned no-TTY, and `CLICOLOR_FORCE` returned truecolor.
* **Still to check:** both libraries on a real TTY, and `huh`'s accessible mode.

## Not built yet

* `--no-color`, and `NO_COLOR` and TTY detection beyond what commands do by default.
* `internal/ui`.
* Handing the logger to commands.
* **Progressive output.** A command that does several things in order (`gero ship` first) builds its whole report and prints it at the end, so a long run is silent until it is done. The script it replaces printed as it went: a heading per branch, a line per step, a mark for what happened, and in CI the same lines appended to the job summary (`summary()` in `common.sh` writes to stdout and to `$GITHUB_STEP_SUMMARY`). The command should do the same, in the order the work happens: each branch's state as it is read, then the images, then each commit as it is made, then each target's result. The design is one stream of events (a stage started or finished, a target started or finished, with its attributes), and two ways to show it:
  * **Human, the default.** The renderer prints to stdout as each event arrives, so the report is built the way it is read, and the end is the summary. Streaming comes first, and a table is the exception: a table cannot align until it has all its rows, so the renderer can pause streaming while a table is open. `BeginTable(header)` holds output, `Row` adds to it, and `EndTable` prints it aligned, then prints whatever was held while it was open, and streaming resumes. A line printed while no table is open goes out at once. So the checks of every branch print as one aligned table when they are all in, and the commits of each branch stream as they are made, and the report reads the same as it does now.
  * **The seam.** `internal/ui` holds it: `ui.Output` is the interface (a line, and a table held until it is whole), and `ui.Stream` is the plain implementation that prints to a writer. Commands take a `ui.Output` and not an `io.Writer` for what they report as they go, so a REPL or a TUI can supply its own and a command behaves the same ad hoc and inside one. It is deliberately small: it is the calls a progressive report needs, and grows only when a second front end needs more.
  * **Structured.** The same events are `slog` records at `info`, with the target, the status and the commit as attributes, so a JSON handler in CI gives machine-readable progress on stderr and a text handler gives the same locally with `-v`. This is the logging this spec already has ([Logging](#2-logging)), not a second mechanism. `--json` stays one document on stdout at the end, and is not interleaved with progress.
  * In CI the final report is also appended to `$GITHUB_STEP_SUMMARY`, as the script did.
  Not designed beyond this: the event type, whether the renderer is the command's own or shared in `internal/ui`, and what `--json` does for a run too long to wait for.

## Open questions

None yet.
