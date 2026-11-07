# REPL

**Status:** draft. Steps 1 to 4 of the build order are built: the core in `internal/cli/repl/` (the loop, `pwd`, `ls`, `cd`, `shell`, `exit` and the working directory) and `gero repl`, which runs it on a line editor (`internal/ui/editor`, on `ergochat/readline`) with history, completion and a styled prompt, behind the start checks. The editor has been tested without a terminal only: what a real terminal does with it is checked by hand.

`gero repl` is an interactive prompt that runs `gero` commands one line at a time, with a few shell-like commands of its own so a user has less reason to leave it. It is not a shell: it is started from one, and exiting returns to it. The goal is fewer trips out, not a replacement for the user's shell.

## Boundaries

* **Started, not logged into.** `gero repl` is a command like any other ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)). It is not meant to be `$SHELL`, and no part of the design depends on that. Anyone who sets it as their login shell gets what it does and nothing more.
* **No shell language.** No pipes, redirects, globbing, quoting beyond splitting a line into words, variables, job control or scripting. A line that needs them goes to a real shell through `shell` below. A clear edge is better than a leaky imitation of a shell.
* **The same commands.** A line is a command from the one command tree ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)), typed without the leading `gero`, which is not accepted: `gero repo status` is an unknown command, and the error says the REPL takes the command alone. There is no second set of commands to keep in step with the CLI, and a command's help, flags and exit behavior are the same in both places.
* **Workstation only.** The REPL needs a person at a terminal. It refuses to start when stdin or stdout is not a TTY or when the context is CI, with a message saying so, using the detection in [CLI_OUTPUT](CLI_OUTPUT.md#3-tui-and-interactive-features) and [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md). It also refuses to start when `GERO_REPL` is set, which is how a REPL started from inside its own `shell` is caught. It refuses as well when the terminal is a TTY but cannot do cursor control (`TERM` unset or `dumb`, as in some editor-embedded terminals), saying why. The rule is that the REPL has to be an enhancement on top of the user's shell, with history, editing and completion, and where it cannot be that, the user is better off in their shell and gets a message saying so, not a lesser REPL.

## REPL-only commands

These exist only inside the REPL and are not in the command tree, so `gero pwd` is an unknown command. They are the small set a user otherwise exits for. A name that is neither one of these nor a `gero` command is an error that points to `shell`, and nothing is passed to the shell by guessing. (Not built: today it is cobra's unknown-command error.)

| Command | Does |
|---|---|
| `pwd` | Prints the working directory |
| `ls [target]` | Lists a directory: names only, sorted, a trailing `/` on directories. No flags. The target is described below |
| `cd <project>` | The existing `gero cd` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)), below |
| `!<command line>` | **Deferred until after v1** ([Deferred ideas](#deferred-ideas)). Would run the rest of the line in the user's shell (`$SHELL -c`), in the working directory, with the terminal attached, then returns to the prompt |
| `shell` | Starts the user's `$SHELL` as an interactive child process with `GERO_REPL=1` in its environment. Exiting it returns to the prompt. The bare word only: `shell` with any argument is an error, and the `gero shell` setup commands are not reachable inside the REPL until `!` is built |
| `help` | The command tree's help, plus these commands |
| `exit`, Ctrl-D | Leaves the REPL |

**`cd`.** `cd <project>` is the project-based `gero cd` and nothing more: it takes the project argument and rules `gero cd` takes and refuses what it refuses, `.` included. It takes project names only: there is no `cd ..`, no relative or absolute path and no `cd` with no argument, so the REPL never grows a second way to move. To look below a project root, `cd` to the project and `ls` a path under it. It resolves the clone through the same `Workspace.ConfiguredRepoPath` ([WORKSPACE](WORKSPACE.md#resolving-repo-paths)), so the same user file gives the same answer as in a shell. The one difference is who changes the directory. `gero cd` only prints a path, because a program cannot change its parent shell's directory, which is why [SHELL_SETUP](SHELL_SETUP.md) has a wrapper. The REPL is the process that owns the working directory, so it takes the resolved path and changes into it itself, and needs no wrapper. The REPL runs the resolver and `os.Chdir`; it does not run the `gero cd` command and parse its output.

**`ls`.** With no argument it lists the working directory. The target is one of:

* `.`, the working directory, which is also the project's root when that is where the user is.
* A path under the working directory, such as `docs` or `./test/integration`. Its resolved location must be inside the working directory, so `..` and absolute paths that leave it are refused. A path that is not a directory is an error; a file is not listed.
* A project, by the config `name` or the `owner/name` slug, case-insensitive, as in the project arguments of [CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags). It lists that project's clone root, found through the same `ConfiguredRepoPath` as `cd`, and is an error when the project is unknown or has no unique clone. Unlike `cd`, a project is looked at without moving, and `.` is the working directory, not the project it is in.
* A project, written explicitly with a leading `@`: `ls @scc-operator` or `ls @rancher/scc-operator`. It takes both spellings a project argument takes, the config `name` (`name:` in the project file) and the repository slug (`spec.repo.name`), case-insensitive, by the same lookup (`Workspace.Project`). The `@` says the word is a project and never a path, so it is how a user gets the project when a bare word would mean a directory, and it is accepted whenever a project is meant, ambiguous or not.

**A bare word.** `ls docs` is read as a path first: if a directory of that name exists under the working directory it is listed, as a shell user means. Only when there is none is the word tried as a project. An `owner/name` slug that is not a directory under the working directory is a project, and the `@` form is never a directory, so a project is always reachable. A bare word that is both a directory and a project lists the directory, and the output does not say so; a user who meant the project writes `@`. To list a directory whose own name starts with `@`, write `./@name`. `@` is `ls`'s alone: `cd` takes project names only and has no ambiguity to settle, so `cd @docs` is an unknown project, not an alias.

A path below a project, such as `ls <project>/docs`, is not accepted: the user changes into the project and lists the path from there. Listing a project is the one place `ls` leaves the working directory.

**`!` and `shell`.** The child gets the REPL's working directory and environment, so a `cd` done in the REPL is seen by `!` and `shell`. The reverse does not hold: a `cd` inside `shell` or `!` ends with that child and does not move the REPL, as with any subprocess. The `gero` function from [SHELL_SETUP](SHELL_SETUP.md) works in the child as it does in any shell. `GERO_REPL=1` is there so a user can mark their prompt and so a nested `gero repl` is refused.

**`shell` in detail** (decided). The child is `$SHELL` run as is, with no arguments: with the terminal attached it is interactive on its own and reads the user's usual startup files, as one started from a terminal does. No `-i` and no `-l`.

* **Shadows the tree.** The REPL-only `shell` takes the name from the tree's `gero shell` (`init`, `install`, `uninstall`). A line whose first word is `shell` is the child, and with any other word after it is an error saying `shell` takes no arguments, never a pass-through to the tree. The setup commands are for a real shell, where `gero shell install` works as before, and they are reachable from the REPL by starting `shell` and running them there, where the user's own `gero` is on the path.
* **`$SHELL` unset or not an executable file is an error** naming `$SHELL`. The REPL does not guess `/bin/sh`: it is the user's shell or no shell, as the REPL itself is the user's REPL or none.
* **The whole environment is passed through**, plus `GERO_REPL=1`. This is what a user expects of their own shell, which already holds any token in it. It also sets the policy for `!`, to be built, which gets the same environment.
* **The exit status of the child is not shown and not an error.** The status of an interactive shell is that of the last command run in it, not a failure of `shell`, so the REPL returns quietly to the prompt. Only a failure to start the child is an error.
* **Ctrl-C belongs to the child** while it runs. It is in the foreground of the terminal and receives the signal, and the REPL neither cancels anything nor leaves while it waits. The REPL's interrupt handling resumes when the child exits.

**This is not what `internal/execx`'s `Runner` is for.** `Runner` is the one place `gero` starts an allow-listed program, and it takes a binary from an allow list, an argument vector never a shell string, and a scrubbed environment ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)). A passthrough is the reverse on each count: the user's own `$SHELL`, a line the user typed handed to it as a string (for `!`), the terminal attached, and the environment the user started the REPL with. It is a separate function and type in `execx`, `Passthrough` taking a `PassCmd` (the shell, an argument vector, the directory, the environment, and the input and output streams) and a context, and not a method on `Runner`, so the two policies cannot blur into one. It is used only by the REPL, has its own policy entry, and the REPL core takes it as an injected function so tests need no real shell. It is never reached from a command's `RunE`, and no `gero` command builds a shell string.

## Working directory is the session state

The working directory is what the REPL carries between lines, and commands read it as they do in a shell: the project a command acts on when none is named is the one whose repository the working directory is in ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags), project arguments). So `cd <project>` followed by `repo status` needs no project argument. The default prompt does not show the project; a user's own prompt can ([The prompt](#the-prompt)).

**Where the directory can be.** The REPL starts in whatever directory `gero repl` was run from, which need not be a project. After that the only thing that moves it is `cd <project>`, and that goes to a project root. So the working directory is always either the starting directory or the root of a project, and it never wanders into a subdirectory or out of the workspace on its own. This is why `cd` has no paths, and it is what lets the prompt's project be worked out when the directory changes and not on every line.

Because the directory changes, `workspace.Open(settings, cwd)` runs for each line with the current directory, not once at startup. `Open` does no I/O ([WORKSPACE](WORKSPACE.md#one-door)), so this costs nothing and cannot go stale. No other state is kept between lines.

**Single-action commands fit without change.** `gero repo branch`, `gero pr create` and the other commands that run one task ([TASK_COMMANDS](TASK_COMMANDS.md)) ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags), [PROJECT_LAYOUT](PROJECT_LAYOUT.md#tasks-single-actions-that-compose)) are commands in the same tree, so the REPL runs them as it runs any other, and `cd <project>` supplies their project. There are no pipes, so a line hands nothing to the next: the branch one line made is named on the next line's command. A "last result" the REPL remembers would be new session state beyond the working directory, and is not part of this spec.

## Running a line

* **A fresh command tree for each line.** cobra keeps flag values on the command between `Execute` calls, so reusing the tree would carry one line's `--json` into the next. `cmd/` builds the tree again for each line. Building it is wiring only, so this is cheap.
* **Errors end the line, not the REPL.** A command's error is printed as it is in a shell and the prompt returns. The REPL's own exit status is 0 unless it failed to start. This already holds because commands return errors and never call `os.Exit` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#4-exit-codes)); a command that exits the process would end the session, and that is a bug in the command.
* **The same settings and workspace as every command.** The REPL does not keep a configuration of its own. The `clicfg.Settings` for a line are the ones a command would get: flags, `GERO_*` environment and the user file ([CLI_CONFIG](CLI_CONFIG.md)), with the flags given to `gero repl` as the base and the flags typed on the line on top. Each line's fresh tree loads them through the same `PersistentPreRunE`, so the REPL adds no second loader. The prompt template and the terminal capabilities ([CLI_OUTPUT](CLI_OUTPUT.md#terminal-capabilities)) are read from those settings too. The only state the REPL owns is the working directory and the history file.
* **Global flags given to `gero repl` apply to every line**, `--dry-run` and `--config-dir` among them, and a flag typed on a line applies to that line only. A user who starts `gero repl --dry-run` cannot write by mistake.
* **Write commands keep their contract.** Each asks its one confirmation before it writes, as in a shell ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)). The REPL has no way to skip it, and `--yes` on a line works as it does on the command line. The REPL does not add its own prompt on top.
* **Ctrl-C** cancels the command that is running through its context and returns to the prompt. At an empty prompt it clears the line. It never leaves the REPL.
* **Output** goes through `internal/ui` like any command ([CLI_OUTPUT](CLI_OUTPUT.md)). `--json` and the rest work per line.

## The line editor and the front end

History, line editing and completion come from a library, since doing raw-mode editing by hand is not worth it. Completion is generated from the same cobra tree that `gero shell install` completes in a shell ([SHELL_SETUP](SHELL_SETUP.md)), so a command completes the same in both places, and the REPL-only commands are added to it. History is kept in the user's state directory, not shared with the shell's history. Whatever the library, it needs an entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) before it is imported.

There are two shapes the front end can take, and the choice is the one open question that shapes the rest:

* **An editor on a loop.** The loop owns control flow and an editor reads one line at a time (`chzyer/readline`, `reeflective/console`, or a small editor on `golang.org/x/term`). A command writes straight to the terminal, as in a shell, so output, streaming and Ctrl-C behave as they do outside the REPL.
* **A program that owns the screen.** Bubble Tea, with its model, update and view loop, gives complete control over the prompt, completion menu and any status line, and is the natural home for a prompt template and later for styling (`lipgloss` for color, with width measured by the one measuring package the spike settles on). The cost is that the terminal is the program's while it runs. A command's output must go through the model, or the program must give the terminal up around each line (as `tea.ExecProcess` does for a child), and the one confirmation a write command asks reads a line from stdin ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)), so input has to reach it too. Both are the part of "the same commands in both places" that can quietly break.

**Spike results.** One minimal program per candidate in a scratch module, each with a `gero> ` prompt, history, and completion where the library has it, built and run with stdin piped. Checksums were verified. **No pty was available in the sandbox** (`openpty: Operation not permitted`), so editing, redraw, Ctrl-C and Ctrl-D on a real terminal are **(unverified)**, and what follows is what the build and the piped run showed plus what the source says. The baseline is a plain `bufio` reader: no modules, 2.4 MB.

| Candidate | Modules added | Binary | Piped stdin | Fit to the core |
|---|---|---|---|---|
| `golang.org/x/term` `Terminal` | 0 new: `x/term` is already in `go.sum`, `x/sys` is already used | 2.5 MB | Fails at once, `MakeRaw: inappropriate ioctl for device` | Has history in memory, Ctrl-A, Ctrl-E, Ctrl-K, Ctrl-U, Ctrl-W, and a tab callback for one completion. No history file, no Ctrl-R, no menu, and each of those is ours to write |
| `ergochat/readline` v0.1.3 (the maintained fork of `chzyer/readline`; last release 2024-09) | 2 (`x/sys`, `x/text`) | 2.9 MB | Reads plain lines and writes no escape codes, so a test can drive it without a TTY | Loop-shaped: `ReadLine` returns a line, `ErrInterrupt` on Ctrl-C and `io.EOF` on Ctrl-D. History file, Ctrl-R search, Ctrl-A, Ctrl-E, Ctrl-L, and tab completion from a prefix tree or any function of ours (`AutoCompleter`). `FuncFilterInputRune` sees every key before the editor, which is where a Ctrl-X exit would go |
| `chzyer/readline` v1.5.1 | 0 (3.1 MB binary) | 3.1 MB | Same as the fork | Same API the fork grew from, but unmaintained since 2022-07. No reason to pick it over the fork |
| `reeflective/console` v0.5.0 | 10 (`cobra`, `carapace`, `mvdan.cc/sh`, `reeflective/readline`, `yaml.v3` and more) | 11 MB | Writes cursor escape sequences to stdout even when piped, and ran `exit` as an unknown cobra command | It owns the loop and runs the cobra commands itself, so it would sit beside our core's `Exec` and its REPL-only commands and not under them. Its own `exit`, history and completion would have to be switched off or wrapped |
| Bubble Tea v2 with `bubbles` `textinput` and `lipgloss` v2 | 17 | 5.5 MB | Fails, `error opening TTY: could not open TTY: open /dev/tty`, as `huh` did | The program owns the screen. A command's output and the one confirmation that reads stdin must go around the program, as `tea.ExecProcess` does for a child. That is the break the spec warns of |

**On the real binary.** The sizes above are standalone programs, where the standard library is most of what is linked. What matters is the cost on `gero` itself, so each candidate was linked into a scratch copy of the repository (a program using it, run only when an environment variable is set, so it is not discarded) and `gero` built. Binaries are the default `go build`, not stripped, and the modules are all those of the build.

| `gero` with | Modules | Size | Over today's |
|---|---|---|---|
| nothing added (today) | 28 | 16.8 MB | |
| `ergochat/readline` | 30 | 17.1 MB | +0.25 MB (+1.5%) |
| lipgloss v2 | 43 | 17.8 MB | +1.0 MB (+6%) |
| lipgloss v2 and `ergochat/readline` | 45 | 18.1 MB | +1.3 MB, so +0.28 MB for the editor once styling is in |
| Bubble Tea v2 with `bubbles` `textinput` (pulls lipgloss v2) | 46 | 18.6 MB | +1.8 MB (+11%) |
| Bubble Tea v2, `textinput` and lipgloss v2 used | 46 | 18.6 MB | +1.8 MB, so +0.8 MB over lipgloss alone |

* **The editor is the cheap part, whichever way styling goes.** `ergochat/readline` costs a quarter of a megabyte with or without lipgloss, since it shares almost nothing with it.
* **Bubble Tea costs 0.8 MB more than lipgloss alone, and 15 modules more than nothing.** It is the right pick only if the REPL, and later commands, use enough of it to need that, such as a model-driven prompt with a completion menu and a status line, or `huh` forms (`huh` is built on it, so the cost would then be shared). `textinput` alone is not that.
* **`reeflective/console` is out** (10 modules and 11 MB standalone, and it takes over running commands).
* These are from linking one small use of each. A larger use grows the number a little, not the shape of it.

The measures the spec asked for the second shape, whether a write command's stdin confirmation and Ctrl-C work unchanged inside Bubble Tea, and which package measures width, are **(unverified)**: they need a pty and a longer program, and the table already shows its cost in modules and in the way it takes the terminal.

**Direction: `ergochat/readline` for the editor, an editor on a loop, with lipgloss v2 for styling, both behind `internal/ui`.** Size does not separate the options: the base binary is already large from the dependencies it has, and every candidate but `reeflective/console` adds under 2 MB. What separates them is fit. The REPL does not need a program that owns the screen (a model-driven prompt, a completion menu redrawn live, a status line), and the earlier "real time" trouble in `gero` was about output modes and how they were built, not about terminal limits. An editor on a loop keeps the core as it is (a line in, a line given back), keeps commands writing straight to the terminal so output, streaming and Ctrl-C behave as in a shell, and gives history in a file, Ctrl-R and completion that `x/term` would have us build. Bubble Tea stays an option for if a feature ever needs the screen. `x/term` is the fallback if the readline fork's behavior on a real terminal disappoints. It is **(unverified)** on a terminal, and is confirmed there before it is imported. Whichever is picked gets its entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) and a `depguard` rule that keeps it behind `internal/ui`.

**How a styled prompt meets the editor** (read from the readline source, **(unverified)** on a terminal):

* **Colors are handled.** `ergochat/readline` takes ANSI escapes in the prompt and drops `ESC [ ... m` sequences, the SGR ones that colors are, when it measures the prompt for redraws. A prompt that lipgloss styles with colors and bold is measured by what is visible. Its width code does not know other escapes (hyperlinks, cursor moves), so the prompt takes no styling beyond SGR, which is all [The prompt](#the-prompt) allows (plain text, with named styles applied by the renderer).
* **Its filter has one sharp edge.** The color filter reads the rune after an `ESC`, so a prompt that ends in a lone `ESC` would fail. A rendered prompt never does, and the renderer asserts that the prompt it hands over holds only complete SGR sequences.
* **One line only.** The editor takes the width of the whole prompt, newline included (a control character, so zero cells), as the width of the line it redraws from. A prompt of two lines, or a border drawn around it, would put the cursor in the wrong column, and `chzyer/readline` has the same code. The prompt is one line, and the template syntax has no way to write a newline **(leaning)**; a multi-line prompt would be tried on a real terminal first.
* **Widths come from two places.** The editor counts wide East Asian characters and zero-width marks with `x/text/width`, and lipgloss counts by grapheme (with `uniseg`, so emoji sequences are one cell run). They agree for ASCII and the common wide characters, and can differ for emoji sequences. This reaches the prompt only if a project's config name holds one, and the typed line only if a user types one; both are rare. The prompt's width, the one the editor needs, is measured by the editor itself, so the two never have to agree.
* **Where color comes from.** The styled string is made by the renderer, and handed to the editor already downsampled for the terminal, so `NO_COLOR` and a dumb terminal are decided in one place, [CLI_OUTPUT](CLI_OUTPUT.md), and not in the editor.

**Keeping the choice cheap.** The REPL core does not know which shape it is in. The state (the working directory, the project), the REPL-only commands, the line splitting, the prompt template expansion and the running of a line are plain data and functions in `internal/cli/repl/` with no terminal code. A front end reads lines, draws what the core gives it, and gives each line back. Both shapes can drive that core, which is also what lets a test drive it with a fake reader. An Elm-style front end is one such driver, not the design of the core.

**A plain front end comes first, and is not a fallback.** A bare `bufio` reader is the first front end and the driver tests use, so the loop is complete and tested before any library is chosen. It is not offered to a terminal that cannot do better: such a terminal is refused ([Boundaries](#boundaries)). Until the editor lands (step 4 below) the plain reader is what runs on a capable terminal, which is a stage of the build and not a supported mode.

**Measuring.** A prompt or a completion list that is drawn needs the width of text with wide characters and escapes in it. The pick is one package, not two: `lipgloss` v2 measures width itself and is expected to do so through `charmbracelet/x/ansi` **(unverified)**, in which case neither `muesli/reflow/ansi` nor `mattn/go-runewidth` is added as a direct dependency. The spike checks this in `go mod graph`.

## The prompt

The prompt is always the expansion of a `PS1`-style template, so the default and a user's own go through one path. Only the template differs.

The default template is plain text, with no color:

* Outside a project: `gero> `
* Inside a project, the project's config name is added: `gero (scc-operator)> `

It is written `gero{project: (%s)}> `, which is the syntax below.

* **Syntax** **(leaning)**. `{name}` is replaced by the placeholder's value. `{name:text}` writes `text` only when the value is not empty, with `%s` in it replaced by the value, so a segment can vanish outside a project without leaving stray punctuation. `{{` and `}}` are literal braces. Braces and not `PS1`'s backslashes, since a backslash is the first thing a YAML string trips on.
* **Placeholders.** `{project}` (the config name of the project the working directory is in, empty outside one), `{dir}` (the last element of the working directory) and `{path}` (the working directory, with the home directory as `~`). The set grows when a user asks for something it cannot say.
* **Where a user sets it.** A user-file key `repl.prompt`, with its environment variable derived by the rule in [CLI_CONFIG](CLI_CONFIG.md#keys-and-environment), so the usual precedence applies. It is CLI config, how the tool looks, and holds no application fact. No flag: a prompt is not a per-invocation choice. Until it is built the default is the only prompt, and nothing reads the key.
* **No ANSI traps.** The template holds plain text and placeholders, and nothing else. A raw escape or any other control character in it is an error, which avoids the classic bug of a shell prompt that counts an invisible escape sequence as visible and then redraws the line in the wrong place. Color arrives later as named styles, applied by the renderer, which knows what is visible and measures the prompt's width itself. A prompt is never built from strings a template author has already styled.
* **No heavy placeholders.** The prompt is drawn on every line, so a placeholder is cheap or it is cached. Every placeholder above comes from the working directory and the already loaded config, with no command run, no network, and no `git` read, and the project is worked out when the directory changes, which is only ever on `cd` and at start. Anything that can change between lines, a branch name or a dirty flag, is not in the first set. If one is added it is read from a cache with an explicit refresh and a stated staleness, and never on the prompt's path. A placeholder that could be slow is a placeholder we do not ship.
* **Errors.** An unknown placeholder, an unbalanced brace or a control character is an error when the REPL starts, naming the key, as an unknown key is in the user file. A prompt must not fail a running session, so the template is checked once, up front, and nothing in it can fail afterward.

## Where it lives

The REPL loop, the REPL-only commands and the shell hand-off are in `internal/cli/repl/`, plain functions without cobra, like any command package ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#1-framework)). The `repl` command in `cmd/` is glue: it checks the TTY, builds the tree for each line and gives the loop a function that runs one line. The loop reads lines, the line editor and the terminal sit behind `internal/ui` ([CLI_OUTPUT](CLI_OUTPUT.md)), and tests drive the loop with a fake reader and a recorded runner, so no test needs a TTY ([TESTING](TESTING.md)).

## Not built yet

Steps 1 to 4 are built: the core (`Session`, `Split`, `Exec`, `Loop`, with the runner and the project lookup of `cd` and `ls` injected) and the `gero repl` command in `cmd/`, which makes the start checks, builds the tree for each line (`newRootCmd`), carries the global flags given to `gero repl` into every line, and turns Ctrl-C into a cancel of the running line. `cd <project>` asks the same `cdpath` lookup as `gero cd` and then calls `os.Chdir` itself, so the process's working directory is the REPL's and commands see it. `shell` is `repl.Session.shell`: it checks `$SHELL`, marks the environment and calls `execx.Passthrough` through an injected function, tested with a fake and, for the real hand-off, with a non-interactive `sh`. A real interactive child on a terminal has not been tried by hand. The REPL sets `GERO_REPL` in its own process, so a `repl` line typed at the prompt is refused. The terminal path has been run only under tests: the unit tests drive the line runner, and a real prompt has not been tried by hand. `help` is not yet its own command, `!` is deferred, and an unknown name gets cobra's error and not a suggestion of `!`. In the order it is meant to land, stopping when users stop leaving:

1. The loop, `pwd`, `ls` and `exit`, with the working directory as state.
2. `cd <project>` (built).
3. `shell` (built), with `GERO_REPL` in its child's environment and the `execx` `Passthrough` entry point. The REPL already sets `GERO_REPL` in its own process, so a nested `gero repl` is refused today.
4. History, line editing and completion (built). The editor is `ergochat/readline` behind `internal/ui/editor`; history is `$XDG_STATE_HOME/gero/repl_history`, else `~/.local/state/gero/repl_history`, one file of the user's own (directory mode 0700); completion is cobra's own answer for the tree (the one `gero shell install` completes from), the REPL-only commands, project names for `cd` and `ls @`, and directories under the working directory for `ls`; Ctrl-C clears the line and never leaves, and Ctrl-D leaves. The prompt is the plain default in the look of a prompt where the terminal has color (one line, styled by the renderer).
5. The prompt template, `repl.prompt` and its placeholders, which also puts the project in the prompt for a user who wants it. Until then the prompt is the plain default.

`!<command line>` is deferred until after v1 ([Deferred ideas](#deferred-ideas)). `shell` is the one way to the user's shell for now.

## Deferred ideas

**Marking the prompt of the child `shell`.** A user in a `shell` started from the REPL may want its prompt to say so. Deferred until after v1, and revisited only if the team asks for it once they are using the REPL. Nothing forces it out of the box without cost, so until then it stays an idea, and users have the hook below. What is known:

* **`GERO_REPL=1` is the hook.** It is in the child's environment for this. A user can put `${GERO_REPL:+(gero) }` in their own `PS1` today.
* **Setting `PS1` on the child does not work.** `PS1` is normally a shell variable and not exported, and `~/.bashrc` or `~/.zshrc` (and oh-my-zsh, starship, powerlevel10k) set it after the shell starts, so a value from the environment is overwritten for most users. Fish and PowerShell have no `PS1`.
* **Option, preferred if it is ever done: an opt-in snippet in the block `gero shell install` manages** ([SHELL_SETUP](SHELL_SETUP.md)), prefixing the prompt for bash and zsh, with a fish equivalent, only when `GERO_REPL` is set. A prompt framework that redraws the prompt later can still overwrite it, and for those the documented route is the framework's own environment-variable module keyed on `GERO_REPL`.
* **Option, advised against: start the child with an rc file of ours** (`bash --rcfile`, a temporary `ZDOTDIR` for zsh). It would force the marker, but it is shell-specific, changes how the user's shell starts, rewrites a prompt without being asked, and breaks the rule that `shell` runs the user's `$SHELL` as is.

**`!<command line>`.** Deferred until after v1, and built only if someone asks for a one-line way to run a shell command without opening `shell`. Nothing is decided beyond what `shell` already settled, and it would build on that: the `execx.Passthrough` entry point, the whole environment passed through, and the `$SHELL` rules. What it would add is the `$SHELL -c` line, a rule for its exit status (a line that fails is closer to a command than an interactive shell is, so showing the status is likely), and its tests. Until then nothing in the REPL passes a typed line to a shell as a string, which keeps the REPL further from being a shell.

## Open questions

* **Keyboard shortcuts** (decided). Ctrl-D at an empty prompt leaves, and Ctrl-C clears the line or cancels the running command and never leaves. The editor adds the usual ones (Ctrl-A, Ctrl-E, Ctrl-K, Ctrl-U, Ctrl-W, Ctrl-L, Ctrl-R for history search and the arrow keys). There is no Ctrl-X exit: Ctrl-D is the way out, and Ctrl-X stays the editor's own prefix key. What each does on a real terminal is checked by hand.
* The prompt's syntax, placeholder set and key name, `repl.prompt`, are **(leaning)** and are fixed when the template is built, along with whether it needs a way to show that the last line failed.
* Which front end (decided): an editor on a loop, `ergochat/readline` behind `internal/ui/editor`, with lipgloss v2 for the look of the prompt. Still **(unverified)** on a real terminal: prompt redraw with color, wide characters, paste, window resize, and Ctrl-C and Ctrl-D in practice.
* Which phase the REPL belongs to, if any ([PHASES](../PHASES.md)). It is not scheduled.
