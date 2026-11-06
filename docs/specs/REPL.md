# REPL

**Status:** draft. Nothing is built.

`gero repl` is an interactive prompt that runs `gero` commands one line at a time, with a few shell-like commands of its own so a user has less reason to leave it. It is not a shell: it is started from one, and exiting returns to it. The goal is fewer trips out, not a replacement for the user's shell.

## Boundaries

* **Started, not logged into.** `gero repl` is a command like any other ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)). It is not meant to be `$SHELL`, and no part of the design depends on that. Anyone who sets it as their login shell gets what it does and nothing more.
* **No shell language.** No pipes, redirects, globbing, quoting beyond splitting a line into words, variables, job control or scripting. A line that needs them goes to a real shell through `!` or `shell` below. A clear edge is better than a leaky imitation of a shell.
* **The same commands.** A line is a command from the one command tree ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)), typed without the leading `gero`, which is not accepted: `gero repo status` is an unknown command, and the error says the REPL takes the command alone. There is no second set of commands to keep in step with the CLI, and a command's help, flags and exit behavior are the same in both places.
* **Workstation only.** The REPL needs a person at a terminal. It refuses to start when stdin or stdout is not a TTY or when the context is CI, with a message saying so, using the detection in [CLI_OUTPUT](CLI_OUTPUT.md#3-tui-and-interactive-features) and [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md). It also refuses to start when `GERO_REPL` is set, which is how a REPL started from inside its own `shell` is caught.

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

A path below a project, such as `ls <project>/docs`, is not accepted: the user changes into the project and lists the path from there. Listing a project is the one place `ls` leaves the working directory.

**`!` and `shell`.** The child gets the REPL's working directory and environment, so a `cd` done in the REPL is seen by `!` and `shell`. The reverse does not hold: a `cd` inside `shell` or `!` ends with that child and does not move the REPL, as with any subprocess. The `gero` function from [SHELL_SETUP](SHELL_SETUP.md) works in the child as it does in any shell. `GERO_REPL=1` is there so a user can mark their prompt and so a nested `gero repl` is refused.

**This is not what `internal/execx` is for.** `execx` is the one place `gero` starts another program, and it takes a binary from an allow list, an argument vector never a shell string, and a scrubbed environment ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)). A passthrough is the reverse on each count: the user's own `$SHELL`, a line the user typed handed to it as a string, the terminal attached, and the environment the user started the REPL with. It is a separate, narrow entry point in `execx`, used only by the REPL, with its own policy entry. It is never reached from a command's `RunE`, and no `gero` command builds a shell string.

## Working directory is the session state

The working directory is what the REPL carries between lines, and commands read it as they do in a shell: the project a command acts on when none is named is the one whose repository the working directory is in ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags), project arguments). So `cd <project>` followed by `repo status` needs no project argument, and the prompt shows the current project when there is one.

Because the directory changes, `workspace.Open(settings, cwd)` runs for each line with the current directory, not once at startup. `Open` does no I/O ([WORKSPACE](WORKSPACE.md#one-door)), so this costs nothing and cannot go stale. No other state is kept between lines.

## Running a line

* **A fresh command tree for each line.** cobra keeps flag values on the command between `Execute` calls, so reusing the tree would carry one line's `--json` into the next. `cmd/` builds the tree again for each line. Building it is wiring only, so this is cheap.
* **Errors end the line, not the REPL.** A command's error is printed as it is in a shell and the prompt returns. The REPL's own exit status is 0 unless it failed to start. This already holds because commands return errors and never call `os.Exit` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#4-exit-codes)); a command that exits the process would end the session, and that is a bug in the command.
* **Global flags given to `gero repl` apply to every line**, `--dry-run` and `--config-dir` among them, and a flag typed on a line applies to that line only. A user who starts `gero repl --dry-run` cannot write by mistake.
* **Write commands keep their contract.** Each asks its one confirmation before it writes, as in a shell ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)). The REPL has no way to skip it, and `--yes` on a line works as it does on the command line. The REPL does not add its own prompt on top.
* **Ctrl-C** cancels the command that is running through its context and returns to the prompt. At an empty prompt it clears the line. It never leaves the REPL.
* **Output** goes through `internal/ui` like any command ([CLI_OUTPUT](CLI_OUTPUT.md)). `--json` and the rest work per line.

## The line editor

History, line editing and completion come from a library, since doing raw-mode editing by hand is not worth it. Completion is generated from the same cobra tree that `gero shell install` completes in a shell ([SHELL_SETUP](SHELL_SETUP.md)), so a command completes the same in both places, and the REPL-only commands are added to it. History is kept in the user's state directory, not shared with the shell's history. The library is not chosen (see Open questions), and it needs an entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) before it is imported.

## Where it lives

The REPL loop, the REPL-only commands and the shell hand-off are in `internal/cli/repl/`, plain functions without cobra, like any command package ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#1-framework)). The `repl` command in `cmd/` is glue: it checks the TTY, builds the tree for each line and gives the loop a function that runs one line. The loop reads lines, the line editor and the terminal sit behind `internal/ui` ([CLI_OUTPUT](CLI_OUTPUT.md)), and tests drive the loop with a fake reader and a recorded runner, so no test needs a TTY ([TESTING](TESTING.md)).

## Not built yet

Everything. In the order it is meant to land, stopping when users stop leaving:

1. The loop, `pwd`, `ls` and `exit`, with the working directory as state.
2. `cd <project>`.
3. `shell` and `GERO_REPL`.
4. History, line editing and completion.
5. The project in the prompt.

`!<command line>` is deferred, and so is everything about how it is built, the `execx` entry point and the environment it passes included. They are decided when it is built. `shell` starts a program the same way, so whichever of the two is built first settles that entry point for both.

## Open questions

* How `ls` tells a bare word that is both a directory under the working directory and a project name or slug, such as `ls docs` when a project is named `docs`. Leaning to the directory, since it is what a shell user means and the project is still reachable by its `owner/name` slug.
* Which line-editing library, decided when the line editor is built, from a spike behind `internal/ui` **(unverified)**: the candidates are `chzyer/readline`, `reeflective/console` and a small raw-mode editor on `golang.org/x/term`, which is already in `go.sum` ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)). The measure is the same as the one used for prompts in [CLI_OUTPUT](CLI_OUTPUT.md): modules added, binary size and behavior without a TTY.
* Whether the passthrough passes the whole environment through, as the user's own shell would have it, or scrubs `GH_TOKEN` and `GITHUB_TOKEN` as `execx` does for every other child. Decided with the first of `!` and `shell` to be built. Passing it through matches what a user expects. Scrubbing keeps a token out of a program the user did not mean to give it to, though the user's shell already holds it.
* Which phase the REPL belongs to, if any ([PHASES](../PHASES.md)). It is not scheduled.
