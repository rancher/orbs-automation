# CLI Output

**Status:** implemented. What is left is under [Not built yet](#not-built-yet).

How `gero` talks to the user: command output, color, logging, and the seam for interactive features. Flags that control these are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md).

## 1. Output

* **Data to stdout, diagnostics to stderr.** A script can always pipe stdout.
* Commands whose results feed shell or GitHub Actions (`gero semver check`, `gero repo build-tags`) also support `--output env`: `KEY=value` lines on stdout, one per line, safe to append to `$GITHUB_OUTPUT` or `$GITHUB_ENV`. Key names are documented next to the command.
* Almost every command that emits data supports `--json` through `encoding/json`, with a stable field naming scheme documented next to the command. JSON output is never interleaved with human text.
* Human output is plain lines and tables laid out by `ui.Align` (the layout `text/tabwriter` gives, measured by what a cell shows, so escapes do not shift a column), and styled by `internal/ui/style` when color is on ([Terminal capabilities](#terminal-capabilities)).
* **Never styled:** `--json` and `--output env`, whatever the setting, and what a shell or a workflow consumes: `gero cd` and `gero shell init`, `gero repo build-tags` and `gero semver check`. `gero version` and cobra's own error line on stderr are plain too.

## 2. Logging

Standard library `log/slog`, written to stderr. Default level `warn`, `-v` for `info`, `-vv` for `debug`, or `GERO_LOG=debug|info|warn|error`. A text handler locally and a JSON handler when running in CI. Business code receives a `*slog.Logger`; there is no package-level logger.

* **Precedence.** `-v` beats `GERO_LOG`, which beats the default, the same order as every other setting ([CLI_CONFIG](CLI_CONFIG.md)). The level is a setting named `log`, so `gero config show` reports it and its source. There is no user-file key for it.
* **stdout is untouched.** Logs go to stderr only, so `-v` never changes `--json` or `--output env` output.
* **Levels.** `warn` is something the user should act on. `info` is what the command decided or did: inferred values, repositories touched, writes. `debug` is how it did it: git invocations, API calls, timings. Which events land at which level is recorded here as commands add them.
* **The root command builds the logger** (`internal/cli/logging`), and `debug` logs the resolved settings once. `gero ship` is handed one; `workspace` logs the resolved config directory and its source at `debug`.
* **Progressive output, for `gero ship`.** The command emits events as it works (`ship.Sink`: resolved, each branch checked, all branches checked, images checked, a branch started, each commit, a branch done, done) and shows them two ways at once. The human sink (`internal/cli/ship`) prints to a `ui.Output` as each event arrives, with the branches held as one aligned table until the last is read, so the slow image check runs with the table already on screen. The log sink records the same events as `slog` records at `info` (a failed branch or image is a `warn`), which a JSON handler on stderr gives as machine-readable progress with `-v`. `--json` stays one document on stdout at the end, with nothing interleaved. A finished plan is shown by replaying its events through the same sink, so the report and the live run cannot differ. `ui.Output` (`internal/ui`) is the seam for other commands and for the REPL.

## 3. Interactive features

* Core commands stay non-interactive and scriptable.
* `gero repl` is the one command that is interactive by nature: a prompt that runs the other commands. It stays outside the core commands' rule above, since they are the same commands and remain scriptable, and it refuses to start without a TTY ([REPL](REPL.md)). Its line editor is `internal/ui/editor`, the one importer of `ergochat/readline`, behind a small `Reader` seam with a plain `bufio` implementation that tests and non-terminal input use.
* Interactive behavior is disabled when stdin or stdout is not a TTY, or when running in CI. The one prompt the core commands have is the confirmation before a write ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)): a plain `[y/N]` that each command asks for itself, in the styled look, skipped with `--non-interactive`.
* No command needs a program that owns the screen, so Bubble Tea and its kin are not used. The web dashboard (see [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md)) is a separate, later surface.

### Terminal capabilities

What the terminal can do is decided once and handed to everything that writes, so no command and no library looks at the environment itself.

* **One value.** `internal/platform/clicfg` works it out next to the context signs and carries it in the settings as `Settings.Terminal` (`clicfg.Terminal`), which `internal/ui` reads: whether stdin and stdout are TTYs, the color profile (none, 16 colors, 256, truecolor), whether the terminal can do cursor control (`TERM` set and not `dumb`), and its width when stdout is a TTY. The loader does no I/O, so what only the process knows (the TTY checks and the width, from `internal/cli/termfacts` on `golang.org/x/term`) is handed to it in `Inputs`, and the rest comes through `Getenv`. Color is on by default: the `color` setting is `auto` unless set, and `auto` means as much color as the terminal supports, not off unless asked.
  * **The setting.** `color` is `auto`, `always` or `never` (`--color` or `--no-color`, then `GERO_COLOR`, then the user file, as every setting). `--no-color` is `--color never`, and loses to an explicit `--color` on the same line.
  * **`never`** is no color. **`always`** is color on, to a pipe too, as `ls --color=always` does, at the depth below and at least 16 colors when the terminal gives none.
  * **`auto`**, in this order: `NO_COLOR` set to anything but empty is no color; else `CLICOLOR_FORCE` set to anything but empty or `0` is color on, as `always`; else stdout not a TTY is no color (so a pipe, a file and a script never get an escape, and CI logs stay plain unless forced); else the depth below. `NO_COLOR` beats `CLICOLOR_FORCE`, and an explicit `--color always` beats both, since it is the one thing the user typed for this run.
  * **Depth**, when color is on: `COLORTERM` of `truecolor` or `24bit` is truecolor; else a `TERM` containing `256color` is 256 colors; else any other `TERM` that is set and not `dumb` is 16 colors; `TERM` unset or `dumb` is none. The order is written down once, here, and tested as a table like any other setting.
* **Commands never ask.** A command names a role (`internal/ui/style`: heading, ok, warn, error, dim, dir, prompt) and a renderer built from that value decides what it looks like: plain when the profile is none, and a nil renderer is plain. A command does not test for color, and no code writes an escape of its own.
* **The palette is the basic 16 colors with bold**, which read on a light or a dark background, since the terminal's background is not known. A role may use a richer color where the terminal has it (`dim` is gray 245 from 256 colors up, and bright black below).
* **The REPL reads the cursor-control and TTY parts** to decide whether it can start at all, and refuses when the terminal cannot support it ([REPL](REPL.md#boundaries)).

### Why these libraries

Styling is lipgloss v2, behind `internal/ui/style`. The line editor is `ergochat/readline`, behind `internal/ui/editor`. The TTY and width checks are `golang.org/x/term`, behind `internal/cli/termfacts`. Each is the one importer of its module, enforced in `.golangci.yml`, and each has its entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md). What the standard library would cost, and what was weighed:

* **Detection stays ours.** TTY, `NO_COLOR`, `--no-color` and the `GERO_*` variables are decided in `internal/platform/clicfg` next to the context signs. A library that reads `CLICOLOR` and friends itself would be a second reader.
* **`x/term`, not `os.File.Stat`.** A character device is not a terminal: `/dev/null` is one, and a REPL started on it ran.
* **A prompt library does not cope without a TTY, so `gero` checks first.** With stdin closed or piped, `go-gh`'s prompter wrote cursor-position escapes and failed, ignoring a piped `y`, and `huh` failed opening `/dev/tty`. The non-TTY rule in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract) (fail unless `--non-interactive`) is a check in `gero`, not something a library gives us.
* **lipgloss v2, not v1, and not through its own detection.** In v1 `Style.Render` consults a `Renderer` and strips color itself; v2 removed it, and `Render` always returns full escapes whatever the environment. In v2 the downgrade happens only when the string is written through `lipgloss.Println` or a `colorprofile.Writer`, and the `Sprint` forms downsample for stdout only, so a string for another writer gets stdout's profile. So `internal/ui/style` decides by our own `Terminal` value, asks only for colors the terminal has, and returns the text unchanged when the profile is none. Advice found online mixes v1 and v2.
* **`reeflective/console` and Bubble Tea are out.** The first runs the cobra commands itself and writes escapes when piped, so it would sit beside the REPL's own commands instead of under them. The second owns the screen, which breaks the one confirmation a write command reads from stdin and the way output and Ctrl-C reach the terminal, and nothing here needs it.

## Not built yet

* A shared yes/no confirmation in `internal/ui`: `ship`, `repo tag`, `shell install` and `doctor self-update` each ask their own.
* The final report of `ship` appended to `$GITHUB_STEP_SUMMARY` in CI, which belongs to `ci:ship`.
* The logger handed to every command: `ship` gets one built in `cmd/`, and the others do not log yet.
* A pty harness that tests the REPL and the prompts in a terminal. They are verified by hand on a real terminal today, since tests run without one. It needs a dependency decision in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md).

## Open questions

None yet.
