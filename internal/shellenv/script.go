package shellenv

import (
	"fmt"
	"strings"
)

// The lines that mark the block gero owns in a profile. Everything between them,
// and the lines themselves, is gero's to replace or remove; nothing outside is.
// They are comments in every supported shell.
const (
	BlockStart = "# >>> gero shell >>>"
	BlockEnd   = "# <<< gero shell <<<"
)

// Block returns the marked block for a profile. It holds one line that asks the
// installed gero for its script, so a newer gero changes the script without the
// profile being edited again, and does nothing when gero is not on the PATH.
func Block(shell Shell) string {
	load := fmt.Sprintf(`command -v gero >/dev/null 2>&1 && eval "$(command gero shell init %s)"`, shell)
	switch shell {
	case Fish:
		load = "command -q gero; and command gero shell init fish | source"
	case PowerShell:
		load = "if (Get-Command gero -CommandType Application -ErrorAction SilentlyContinue) { gero shell init powershell | Out-String | Invoke-Expression }"
	}
	return strings.Join([]string{
		BlockStart,
		"# Added by `gero shell install`: completion, and `gero cd`. Remove with `gero shell uninstall`.",
		load,
		BlockEnd,
	}, "\n") + "\n"
}

// The wrappers are a shell function in front of the gero binary. Only `gero cd
// <project>` is handled by the shell, since a program cannot change its
// parent's directory: it asks the binary for the path and goes there. Anything
// else, including `gero cd --help` and a cd with the wrong number of arguments,
// goes to the binary unchanged.

// posixWrapper is valid in bash and zsh.
const posixWrapper = `gero() {
  if [ "$1" = cd ] && [ $# -eq 2 ] && [ "${2#-}" = "$2" ]; then
    local dir
    dir=$(command gero cd "$2") || return
    builtin cd -- "$dir"
  else
    command gero "$@"
  fi
}
`

const fishWrapper = `function gero
    if test (count $argv) -eq 2; and test "$argv[1]" = cd; and not string match -q -- '-*' $argv[2]
        set -l dir (command gero cd $argv[2]); or return
        builtin cd -- $dir
    else
        command gero $argv
    end
end
`

// powershellWrapper calls the binary by its application path, so it never
// calls itself.
const powershellWrapper = `function gero {
    $exe = Get-Command gero -CommandType Application | Select-Object -First 1
    if ($args.Count -eq 2 -and $args[0] -eq 'cd' -and -not ([string]$args[1]).StartsWith('-')) {
        $dir = & $exe cd $args[1]
        if ($LASTEXITCODE -eq 0 -and $dir) { Set-Location -LiteralPath $dir }
    } else {
        & $exe @args
    }
}
`

// zshCompinit loads zsh's completion system when the profile has not, since
// the completion script registers itself with it.
const zshCompinit = `if ! whence compdef >/dev/null 2>&1; then
  autoload -Uz compinit && compinit
fi
`

// Init returns the script `gero shell init` prints for the shell, given the
// completion script gero generates for it: completion first, then the
// wrapper.
func Init(shell Shell, completion string) (string, error) {
	switch shell {
	case Bash:
		return completion + "\n" + posixWrapper, nil
	case Zsh:
		return zshCompinit + completion + "\n" + posixWrapper, nil
	case Fish:
		return completion + "\n" + fishWrapper, nil
	case PowerShell:
		return completion + "\n" + powershellWrapper, nil
	}
	return "", fmt.Errorf("%w %q", ErrUnsupportedShell, shell)
}
