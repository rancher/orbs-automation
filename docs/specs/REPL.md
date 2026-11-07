# REPL

**Status:** draft. Step 1 of the build order is built as a core in `internal/cli/repl/` (the loop, `pwd`, `ls`, `exit` and the working directory); `gero repl` runs it, with the plain reader, behind the start checks.

`gero repl` is an interactive prompt that runs `gero` commands one line at a time, with a few shell-like commands of its own so a user has less reason to leave it. It is not a shell: it is started from one, and exiting returns to it. The goal is fewer trips out, not a replacement for the user's shell.

## Boundaries

* **Started, not logged into.** `gero repl` is a command like any other ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)). It is not meant to be `$SHELL`, and no part of the design depends on that. Anyone who sets it as their login shell gets what it does and nothing more.
* **No shell language.** No pipes, redirects, globbing, quoting beyond splitting a line into words, variables, job control or scripting. A line that needs them goes to a real shell through `!` or `shell` below. A clear edge is better than a leaky imitation of a shell.
* **The same commands.** A line is a command from the one command tree ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)), typed without the leading `gero`, which is not accepted: `gero repo status` is an unknown command, and the error says the REPL takes the command alone. There is no second set of commands to keep in step with the CLI, and a command's help, flags and exit behavior are the same in both places.
* **Workstation only.** The REPL needs a person at a terminal. It refuses to start when stdin or stdout is not a TTY or when the context is CI, with a message saying so, using the detection in [CLI_OUTPUT](CLI_OUTPUT.md#3-tui-and-interactive-features) and [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md). It also refuses to start when `GERO_REPL` is set, which is how a REPL started from inside its own `shell` is caught. It refuses as well when the terminal is a TTY but cannot do cursor control (`TERM` unset or `dumb`, as in some editor-embedded terminals), saying why. The rule is that the REPL has to be an enhancement on top of the user's shell, with history, editing and completion, and where it cannot be that, the user is better off in their shell and gets a message saying so, not a lesser REPL.

## REPL-only commands

These exist only inside the REPL and are not in the command tree, so `gero pwd` is an unknown command. They are the small set a user otherwise exits for. A name that is neither one of these nor a `gero` command is an error that suggests `!`, and nothing is passed to the shell by guessing.

| Command | Does |
|---|---|
| `pwd` | Prints the working directory |
| `ls [target]` | Lists a directory: names only, sorted, a trailing `/` on directories. No flags. The target is described below |
| `cd <project>` | The existing `gero cd` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)), below |
| `!<command line>` | Runs the rest of the line in the user's shell (`$SHELL -c`), in the working directory, with the terminal attached, then returns to the prompt |
| `shell` | Starts the user's `$SHELL` as an interactive child process with `GERO_REPL=1` in its environment. Exiting it returns to the prompt |
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

**This is not what `internal/execx` is for.** `execx` is the one place `gero` starts another program, and it takes a binary from an allow list, an argument vector never a shell string, and a scrubbed environment ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)). A passthrough is the reverse on each count: the user's own `$SHELL`, a line the user typed handed to it as a string, the terminal attached, and the environment the user started the REPL with. It is a separate, narrow entry point in `execx`, used only by the REPL, with its own policy entry. It is never reached from a command's `RunE`, and no `gero` command builds a shell string.

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

Step 1 is built: the core (`Session`, `Split`, `Exec`, `Loop`, with the runner and the project lookup of `ls` injected) and the `gero repl` command in `cmd/`, which makes the start checks, builds the tree for each line (`newRootCmd`), carries the global flags given to `gero repl` into every line, and turns Ctrl-C into a cancel of the running line. The REPL sets `GERO_REPL` in its own process, so a `repl` line typed at the prompt is refused. The terminal path has been run only under tests: the unit tests drive the line runner, and a real prompt has not been tried by hand. `help` is not yet its own command, `!` is deferred, and an unknown name gets cobra's error and not a suggestion of `!`. In the order it is meant to land, stopping when users stop leaving:

1. The loop, `pwd`, `ls` and `exit`, with the working directory as state.
2. `cd <project>`.
3. `shell` and `GERO_REPL`.
4. History, line editing and completion.
5. The prompt template, `repl.prompt` and its placeholders, which also puts the project in the prompt for a user who wants it. Until then the prompt is the plain default.

`!<command line>` is deferred, and so is everything about how it is built, the `execx` entry point and the environment it passes included. They are decided when it is built. `shell` starts a program the same way, so whichever of the two is built first settles that entry point for both.

## Open questions

* The prompt's syntax, placeholder set and key name, `repl.prompt`, are **(leaning)** and are fixed when the template is built, along with whether it needs a way to show that the last line failed.
* Which front end, an editor on a loop or a Bubble Tea program, and so which library, decided when the line editor is built, from a spike behind `internal/ui` **(unverified)**. The candidates are `chzyer/readline`, `reeflective/console`, a small raw-mode editor on `golang.org/x/term` (already in `go.sum`), and Bubble Tea v2 with `bubbles` and `lipgloss` v2 ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)). The measure is the one used for prompts in [CLI_OUTPUT](CLI_OUTPUT.md): modules added, binary size and behavior without a TTY, plus two more for the second shape: whether a write command's stdin confirmation and Ctrl-C work unchanged inside it, and which package measures width.
* Whether the passthrough passes the whole environment through, as the user's own shell would have it, or scrubs `GH_TOKEN` and `GITHUB_TOKEN` as `execx` does for every other child. Decided with the first of `!` and `shell` to be built. Passing it through matches what a user expects. Scrubbing keeps a token out of a program the user did not mean to give it to, though the user's shell already holds it.
* Which phase the REPL belongs to, if any ([PHASES](../PHASES.md)). It is not scheduled.
