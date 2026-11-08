# REPL

**Status:** implemented, with the items under [Not built yet](#not-built-yet) waiting for v1 and the team's use. The editor and the prompt are tested without a terminal; what a real terminal does with them is checked by hand.

`gero repl` is an interactive prompt that runs `gero` commands one line at a time, with a few shell-like commands of its own so a user has less reason to leave it. It is not a shell: it is started from one, and exiting returns to it. The goal is fewer trips out, not a replacement for the user's shell.

## Boundaries

* **Started, not logged into.** `gero repl` is a command like any other ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)). It is not meant to be `$SHELL`, and no part of the design depends on that. Anyone who sets it as their login shell gets what it does and nothing more.
* **No shell language.** No pipes, redirects, globbing, quoting beyond splitting a line into words, variables, job control or scripting. A line that needs them goes to a real shell through `shell` below. A clear edge is better than a leaky imitation of a shell.
* **The same commands.** A line is a command from the one command tree ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)), typed without the leading `gero`, which is not accepted: `gero repo status` is an unknown command, and the error says the REPL takes the command alone. There is no second set of commands to keep in step with the CLI, and a command's help, flags and exit behavior are the same in both places.
* **Workstation only.** The REPL needs a person at a terminal. It refuses to start when stdin or stdout is not a TTY or when the context is CI, with a message saying so, using the detection in [CLI_OUTPUT](CLI_OUTPUT.md#terminal-capabilities) and [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md). It also refuses to start when `GERO_REPL` is set, which is how a REPL started from inside its own `shell` is caught. It refuses as well when the terminal is a TTY but cannot do cursor control (`TERM` unset or `dumb`, as in some editor-embedded terminals), saying why. The rule is that the REPL has to be an enhancement on top of the user's shell, with history, editing and completion, and where it cannot be that, the user is better off in their shell and gets a message saying so, not a lesser REPL.

## REPL-only commands

These exist only inside the REPL and are not in the command tree, so `gero pwd` is an unknown command. They are the small set a user otherwise exits for. A name that is neither one of these nor a `gero` command is an error that points to `help` and `shell`, and nothing is passed to the shell by guessing (not built: today it is cobra's unknown-command error).

| Command | Does |
|---|---|
| `pwd` | Prints the working directory |
| `clear` | Clears the screen and the scrollback and returns to the prompt. No argument |
| `ls [target]` | Lists a directory: names only, sorted, a trailing `/` on directories. No flags. The target is described below |
| `cd <project>` | The existing `gero cd` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)), below |
| `!<command line>` | **Deferred until after v1** ([Deferred ideas](#deferred-ideas)). Would run the rest of the line in the user's shell (`$SHELL -c`), in the working directory, with the terminal attached, then returns to the prompt |
| `shell` | Starts the user's `$SHELL` as an interactive child process with `GERO_REPL=1` in its environment. Exiting it returns to the prompt. The bare word only: `shell` with any argument is an error, and the `gero shell` setup commands are not reachable inside the REPL until `!` is built |
| `help` | The command tree's help, plus these commands (not built: today it is cobra's help) |
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

**This is not what `internal/platform/execx`'s `Runner` is for.** `Runner` is the one place `gero` starts an allow-listed program, and it takes a binary from an allow list, an argument vector never a shell string, and a scrubbed environment ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)). A passthrough is the reverse on each count: the user's own `$SHELL`, a line the user typed handed to it as a string (for `!`), the terminal attached, and the environment the user started the REPL with. It is a separate function and type in `execx`, `Passthrough` taking a `PassCmd` (the shell, an argument vector, the directory, the environment, and the input and output streams) and a context, and not a method on `Runner`, so the two policies cannot blur into one. It is used only by the REPL, has its own policy entry, and the REPL core takes it as an injected function so tests need no real shell. It is never reached from a command's `RunE`, and no `gero` command builds a shell string.

## Working directory is the session state

The working directory is what the REPL carries between lines, and commands read it as they do in a shell: the project a command acts on when none is named is the one whose repository the working directory is in ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags), project arguments). So `cd <project>` followed by `repo status` needs no project argument. The default prompt names the project the working directory is in ([The prompt](#the-prompt)).

**Where the directory can be.** The REPL starts in whatever directory `gero repl` was run from, which need not be a project. After that the only thing that moves it is `cd <project>`, and that goes to a project root. So the working directory is always either the starting directory or the root of a project, and it never wanders into a subdirectory or out of the workspace on its own. This is why `cd` has no paths, and it is what lets the prompt's project be worked out when the directory changes and not on every line.

Because the directory changes, `workspace.Open(settings, cwd)` runs for each line with the current directory, not once at startup. `Open` does no I/O ([WORKSPACE](WORKSPACE.md#one-door)), so this costs nothing and cannot go stale. No other state is kept between lines.

**Single-action commands fit without change.** `gero repo branch`, `gero repo tag` and the other commands that run one task ([TASK_COMMANDS](TASK_COMMANDS.md)) ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#2-command-tree-and-global-flags), [PROJECT_LAYOUT](PROJECT_LAYOUT.md#tasks-single-actions-that-compose)) are commands in the same tree, so the REPL runs them as it runs any other, and `cd <project>` supplies their project. There are no pipes, so a line hands nothing to the next: the branch one line made is named on the next line's command. A "last result" the REPL remembers would be new session state beyond the working directory, and is not part of this spec.

## Running a line

* **A fresh command tree for each line.** cobra keeps flag values on the command between `Execute` calls, so reusing the tree would carry one line's `--json` into the next. `cmd/` builds the tree again for each line. Building it is wiring only, so this is cheap.
* **Errors end the line, not the REPL.** A command's error is printed as it is in a shell and the prompt returns. The REPL's own exit status is 0 unless it failed to start. This already holds because commands return errors and never call `os.Exit` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#4-exit-codes)); a command that exits the process would end the session, and that is a bug in the command.
* **The same settings and workspace as every command.** The REPL does not keep a configuration of its own. The `clicfg.Settings` for a line are the ones a command would get: flags, `GERO_*` environment and the user file ([CLI_CONFIG](CLI_CONFIG.md)), with the flags given to `gero repl` as the base and the flags typed on the line on top. Each line's fresh tree loads them through the same `PersistentPreRunE`, so the REPL adds no second loader. The prompt template and the terminal capabilities ([CLI_OUTPUT](CLI_OUTPUT.md#terminal-capabilities)) are read from those settings too. The only state the REPL owns is the working directory and the history file.
* **Global flags given to `gero repl` apply to every line**, `--dry-run` and `--central-config-dir` among them, and a flag typed on a line applies to that line only. A user who starts `gero repl --dry-run` cannot write by mistake.
* **Write commands keep their contract.** Each asks its one confirmation before it writes, as in a shell ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)). The REPL has no way to skip it, and `--yes` on a line works as it does on the command line. The REPL does not add its own prompt on top.
* **Ctrl-C** cancels the command that is running through its context and returns to the prompt. At an empty prompt it clears the line. It never leaves the REPL.
* **Output** goes through `internal/ui` like any command ([CLI_OUTPUT](CLI_OUTPUT.md)). `--json` and the rest work per line.

## The line editor

History, line editing and completion come from `ergochat/readline`, behind `internal/ui/editor`: an editor on a loop, which reads one line and gives it back. The loop owns control flow, so a command writes straight to the terminal, as in a shell, and output, streaming and Ctrl-C behave as they do outside the REPL. A program that owns the screen (Bubble Tea) was weighed and not chosen: it takes the terminal from the commands and from the one confirmation a write command reads on stdin, and nothing here needs it ([CLI_OUTPUT](CLI_OUTPUT.md#why-these-libraries)).

* **The seam.** `editor.Reader` reads a line given a prompt and returns `io.EOF` (Ctrl-D on an empty line) or `editor.ErrInterrupt` (Ctrl-C). The core knows only that: the state (the working directory, the project), the REPL-only commands, line splitting, the prompt template and the running of a line are plain functions in `internal/cli/repl/` with no terminal code. A plain `bufio` reader is the other implementation, which tests drive the loop with. It is not a mode offered to a terminal that cannot do better: such a terminal is refused ([Boundaries](#boundaries)).
* **History** is `$XDG_STATE_HOME/gero/repl_history`, else `~/.local/state/gero/repl_history`: one file of the user's own, in a directory of mode 0700, not shared with the shell's history.
* **Completion** is the same cobra tree `gero shell install` completes from ([SHELL_SETUP](SHELL_SETUP.md)), so a command completes the same in both places, plus the REPL-only commands, project names for `cd` and `ls @`, and the directories under the working directory for `ls`, never outside it.
* **Keys.** Ctrl-D at an empty prompt leaves, and Ctrl-C clears the line or cancels the running command and never leaves. The editor adds the usual ones (Ctrl-A, Ctrl-E, Ctrl-K, Ctrl-U, Ctrl-W, Ctrl-L, Ctrl-R for history search and the arrow keys). There is no Ctrl-X exit: Ctrl-X stays the editor's own prefix key.
* **How a styled prompt meets the editor** (read from the readline source). The editor drops the `ESC [ ... m` sequences, the SGR ones that colors are, when it measures the prompt, so a prompt styled with colors and bold is measured by what shows. It does not know other escapes, so the prompt takes no styling beyond SGR. Its filter reads the rune after an `ESC`, so a prompt that ends in a lone `ESC` would fail, and the prompt never does: it is one styled run and unstyled trailing blanks. The prompt is one line, since the editor takes the width of the whole prompt, a newline counting as zero cells, and a two-line prompt would put the cursor in the wrong column; the template has no way to write a newline. The editor counts wide characters with `x/text/width` and lipgloss counts by grapheme, so emoji sequences can differ, which reaches the prompt only if a project's config name holds one.
* **Where color comes from.** The styled string is made by the renderer, so `NO_COLOR` and a dumb terminal are decided in one place, [CLI_OUTPUT](CLI_OUTPUT.md#terminal-capabilities), and not in the editor.

## The prompt

The prompt is always the expansion of a `PS1`-style template, so the default and a user's own go through one path. Only the template differs.

The default template is plain text, with no color:

* Outside a project: `gero> `
* Inside a project, the project's config name is added: `gero (scc-operator)> `

It is written `gero{project: (%s)}> `, which is the syntax below.

* **Syntax.** `{name}` is replaced by the placeholder's value. `{name:text}` writes `text` only when the value is not empty, with `%s` in it replaced by the value, so a segment can vanish outside a project without leaving stray punctuation. `{{` and `}}` are literal braces. Only `%s` is special in the text of a placeholder: `%%` is not an escape, and stays as typed. Braces and not `PS1`'s backslashes, since a backslash is the first thing a YAML string trips on.
* **Placeholders.** `{project}` (the config name of the project the working directory is in, empty outside one), `{dir}` (the last element of the working directory) and `{path}` (the working directory, with the home directory as `~`). The set grows when a user asks for something it cannot say.
* **Where a user sets it.** A user-file key `repl.prompt`, with its environment variable derived by the rule in [CLI_CONFIG](CLI_CONFIG.md#keys-and-environment), so the usual precedence applies. It is CLI config, how the tool looks, and holds no application fact. No flag: a prompt is not a per-invocation choice. `gero config show` lists it, and an invalid template does not stop any other command: it is checked by `gero repl` alone, when it starts.
* **No ANSI traps.** The template holds plain text and placeholders, and nothing else. A raw escape or any other control character in it is an error, which avoids the classic bug of a shell prompt that counts an invisible escape sequence as visible and then redraws the line in the wrong place. Color arrives later as named styles, applied by the renderer, which knows what is visible and measures the prompt's width itself. A prompt is never built from strings a template author has already styled.
* **No heavy placeholders.** The prompt is drawn on every line, so a placeholder is cheap or it is cached. Every placeholder above comes from the working directory and the already loaded config, with no command run, no network, and no `git` read, and the project is worked out when the directory changes, which is only ever on `cd` and at start. Anything that can change between lines, a branch name or a dirty flag, is not in the first set. If one is added it is read from a cache with an explicit refresh and a stated staleness, and never on the prompt's path. A placeholder that could be slow is a placeholder we do not ship.
* **Errors.** An unknown placeholder, an unbalanced brace or a control character is an error when the REPL starts, naming the key, as an unknown key is in the user file. A prompt must not fail a running session, so the template is checked once, up front, and nothing in it can fail afterward.

## Where it lives

The REPL loop, the REPL-only commands and the shell hand-off are in `internal/cli/repl/`, plain functions without cobra, like any command package ([CLI_FRAMEWORK](CLI_FRAMEWORK.md#1-framework)). The `repl` command in `cmd/` is glue: it checks the TTY, builds the tree for each line and gives the loop a function that runs one line. The loop reads lines, the line editor and the terminal sit behind `internal/ui` ([CLI_OUTPUT](CLI_OUTPUT.md)), and tests drive the loop with a fake reader and a recorded runner, so no test needs a TTY ([TESTING](TESTING.md)).

## Not built yet

The REPL is complete enough for v1. What is left waits for v1 and the team's use, and each item is taken up if the team asks for it, so what gets built is what they run into.

* **`help`.** It is cobra's help, which does not list `pwd`, `ls`, `clear` or `exit`, and describes the tree's `gero shell` (completion setup) where the REPL's `shell` is the child shell. A short block of the REPL's own commands before cobra's help, with `help <command>` left to cobra, is the first thing to try; a REPL-only help is the larger option.
* **An unknown name** gets cobra's unknown-command error. The intended version is one line that points to `help` and `shell`, not a full help screen.
* **The prompt** has no way to show that the last line failed, and only the placeholders `{project}`, `{dir}` and `{path}`; more come when a user asks for something it cannot say.
* **`!<command line>`** and **marking the prompt of the child `shell`** ([Deferred ideas](#deferred-ideas)).

## Deferred ideas

**Marking the prompt of the child `shell`.** A user in a `shell` started from the REPL may want its prompt to say so. Deferred until after v1, and revisited only if the team asks for it once they are using the REPL. Nothing forces it out of the box without cost, so until then it stays an idea, and users have the hook below. What is known:

* **`GERO_REPL=1` is the hook.** It is in the child's environment for this. A user can put `${GERO_REPL:+(gero) }` in their own `PS1` today.
* **Setting `PS1` on the child does not work.** `PS1` is normally a shell variable and not exported, and `~/.bashrc` or `~/.zshrc` (and oh-my-zsh, starship, powerlevel10k) set it after the shell starts, so a value from the environment is overwritten for most users. Fish and PowerShell have no `PS1`.
* **Option, preferred if it is ever done: an opt-in snippet in the block `gero shell install` manages** ([SHELL_SETUP](SHELL_SETUP.md)), prefixing the prompt for bash and zsh, with a fish equivalent, only when `GERO_REPL` is set. A prompt framework that redraws the prompt later can still overwrite it, and for those the documented route is the framework's own environment-variable module keyed on `GERO_REPL`.
* **Option, advised against: start the child with an rc file of ours** (`bash --rcfile`, a temporary `ZDOTDIR` for zsh). It would force the marker, but it is shell-specific, changes how the user's shell starts, rewrites a prompt without being asked, and breaks the rule that `shell` runs the user's `$SHELL` as is.

**`!<command line>`.** Deferred until after v1, and built only if someone asks for a one-line way to run a shell command without opening `shell`. Nothing is decided beyond what `shell` already settled, and it would build on that: the `execx.Passthrough` entry point, the whole environment passed through, and the `$SHELL` rules. What it would add is the `$SHELL -c` line, a rule for its exit status (a line that fails is closer to a command than an interactive shell is, so showing the status is likely), and its tests. Until then nothing in the REPL passes a typed line to a shell as a string, which keeps the REPL further from being a shell.

## Open questions

* Which phase the REPL belongs to, if any ([PHASES](../PHASES.md)). It is not scheduled.
