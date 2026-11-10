# Shell setup

**Status:** implemented for bash, zsh, fish and PowerShell. Only bash and zsh have been run in a real shell; the fish and PowerShell scripts are untested there.

`gero shell install` adds completion and a `gero` shell function to the user's profile, after asking. The function exists because `gero cd <project>` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)) can only print a path: a program cannot change its parent shell's directory, so the shell has to do the `cd`. Inside `gero repl` there is no parent shell to ask: the REPL owns its working directory and changes it itself, from the same resolver ([REPL](REPL.md)).

## Commands

| Command | Does |
|---|---|
| `gero shell init <bash\|zsh\|fish\|powershell>` | Prints the script: the completion script cobra generates, then the wrapper. Writes nothing |
| `gero shell install` | Adds the marked block to the profile, or updates it in place |
| `gero shell uninstall` | Removes the block |

`install` and `uninstall` take `--shell` and `--profile` to override what is detected, `--yes` to approve without asking, and honor `--dry-run`, which shows the change and writes nothing.

## The block

The profile gets one marked block and nothing else:

```sh
# >>> gero shell >>>
# Added by `gero shell install`: completion, and `gero cd`. Remove with `gero shell uninstall`.
command -v gero >/dev/null 2>&1 && eval "$(command gero shell init zsh)"
# <<< gero shell <<<
```

* **The markers are the safe edit zone.** Install replaces what is between them, uninstall removes it, and nothing outside is read as ours or changed. A profile with one marker and not the other, a second block, or a start marker in the middle of a line is refused with the file named, since there is no safe region to edit.
* **The block asks the installed `gero` for its script**, so a newer `gero` changes the completion and the wrapper without the profile being edited again. The cost is one process start per new shell. With `gero` off the PATH the line does nothing.
* Install is idempotent: a profile that already holds exactly this block is reported as already set up, and nothing is asked.

## The wrapper

A function named `gero`, written for each shell (one script serves bash and zsh; fish and PowerShell have their own, and PowerShell calls the binary by its application path so it never calls itself). It handles one case itself, `gero cd <project>` with exactly one argument that is not a flag: it runs `command gero cd <project>` and `builtin cd`s to the output, returning the binary's exit status when it fails. Everything else, including `gero cd --help` and a cd with the wrong number of arguments, goes to the binary unchanged, so the shell never swallows its errors or help.

Completion registers against the name `gero`, so the function does not affect it. For zsh the script first loads the completion system if the profile has not (`compinit`), since the completion script needs it.

## Detecting the shell and profile

The shell is the one in `$SHELL`. An unset or unsupported value is an error that names `--shell`. PowerShell is never found this way, since it is usually started from another login shell, so it is named with `--shell powershell` (`pwsh` is accepted too). The profile is the file the shell reads for interactive use:

* zsh: `.zshrc` in `$ZDOTDIR`, else the home directory.
* bash: `.bashrc`. On macOS a terminal starts a login shell, which reads `.bash_profile` first, so there `.bash_profile` is used when it exists, `.bashrc` when only that does, and a new `.bash_profile` otherwise.
* fish: `config/fish/config.fish` under `$XDG_CONFIG_HOME`, else `~/.config`.
* PowerShell 7: `Microsoft.PowerShell_profile.ps1` in `powershell` under the same config directory, and on Windows in `Documents\PowerShell` of the user profile directory. Windows PowerShell 5 is not covered.

A profile whose directory does not exist is created along with it.

## Approval and writing

The block is shown and `Continue? [y/N]` is asked before anything is written; only `y` or `yes` counts, and anything else leaves the profile untouched. Without a terminal on stdin the command fails and says to pass `--yes`, rather than guessing. The write goes through a temporary file beside the profile and a rename, so a failure leaves the old content. A symlinked profile, such as one kept in a dotfiles repository, is followed so the link stays a link, and the file's mode is kept.

## Not built yet

* Windows PowerShell 5, and other shells (nushell, elvish).
* Running the fish and PowerShell scripts in a real shell in tests.
* Completing the project argument of `gero cd` from the config.
* A `gero doctor` check that the block is installed and current.

## Open questions

None yet.
