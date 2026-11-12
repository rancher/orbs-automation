package shellenv

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestParse(t *testing.T) {
	for in, want := range map[string]Shell{
		"zsh": Zsh, "/bin/zsh": Zsh, "/usr/local/bin/bash": Bash, "-zsh": Zsh, " bash\n": Bash, "/usr/bin/fish": Fish, "pwsh": PowerShell,
		"powershell": PowerShell, "pwsh.exe": PowerShell, "PowerShell.EXE": PowerShell,
	} {
		got, err := Parse(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "nu", "tcsh"} {
		_, err := Parse(in)
		require.ErrorIs(t, err, ErrUnsupportedShell, in)
	}
}

func TestDetectUsesLoginShell(t *testing.T) {
	got, err := Detect(env(map[string]string{"SHELL": "/bin/zsh"}))
	require.NoError(t, err)
	assert.Equal(t, Zsh, got)

	_, err = Detect(env(nil))
	require.ErrorIs(t, err, ErrUnsupportedShell)
	assert.Contains(t, err.Error(), "--shell")
}

func TestProfile(t *testing.T) {
	has := func(files ...string) func(string) bool {
		return func(p string) bool {
			for _, f := range files {
				if f == p {
					return true
				}
			}
			return false
		}
	}
	home := map[string]string{"HOME": "/home/me"}
	cases := []struct {
		name   string
		shell  Shell
		goos   string
		env    map[string]string
		exists func(string) bool
		want   string
	}{
		{"zsh", Zsh, "linux", home, has(), "/home/me/.zshrc"},
		{"zsh honors ZDOTDIR", Zsh, "darwin", map[string]string{"HOME": "/home/me", "ZDOTDIR": "/cfg/zsh"}, has(), "/cfg/zsh/.zshrc"},
		{"bash on linux", Bash, "linux", home, has("/home/me/.bash_profile"), "/home/me/.bashrc"},
		{"bash on macOS with a .bash_profile", Bash, "darwin", home, has("/home/me/.bash_profile", "/home/me/.bashrc"), "/home/me/.bash_profile"},
		{"bash on macOS with only a .bashrc", Bash, "darwin", home, has("/home/me/.bashrc"), "/home/me/.bashrc"},
		{"bash on macOS with neither", Bash, "darwin", home, has(), "/home/me/.bash_profile"},
		{"fish", Fish, "linux", home, has(), "/home/me/.config/fish/config.fish"},
		{"fish honors XDG_CONFIG_HOME", Fish, "darwin", map[string]string{"HOME": "/home/me", "XDG_CONFIG_HOME": "/xdg"}, has(), "/xdg/fish/config.fish"},
		{"powershell on macOS and Linux", PowerShell, "darwin", home, has(), "/home/me/.config/powershell/Microsoft.PowerShell_profile.ps1"},
		{"powershell honors XDG_CONFIG_HOME", PowerShell, "linux", map[string]string{"XDG_CONFIG_HOME": "/xdg"}, has(), "/xdg/powershell/Microsoft.PowerShell_profile.ps1"},
		{"powershell on Windows", PowerShell, "windows", map[string]string{"USERPROFILE": "C:\\Users\\me"}, has(), filepath.Join("C:\\Users\\me", "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")},
	}
	for _, c := range cases {
		got, err := Profile(c.shell, c.goos, env(c.env), c.exists)
		require.NoError(t, err, c.name)
		assert.Equal(t, c.want, got, c.name)
	}

	_, err := Profile(Bash, "linux", env(nil), has())
	assert.ErrorContains(t, err, "--profile")
	_, err = Profile(Zsh, "linux", env(nil), has())
	assert.ErrorContains(t, err, "--profile")
	_, err = Profile("tcsh", "linux", env(home), has())
	assert.ErrorIs(t, err, ErrUnsupportedShell)
}

func TestBlockAsksTheInstalledGeroAndIsSafeWithoutIt(t *testing.T) {
	b := Block(Zsh)

	assert.Contains(t, b, `command -v gero >/dev/null 2>&1 && eval "$(command gero shell init zsh)"`)
	assert.Contains(t, Block(Bash), "gero shell init bash")
	lines := strings.Split(strings.TrimSuffix(b, "\n"), "\n")
	assert.Equal(t, BlockStart, lines[0])
	assert.Equal(t, BlockEnd, lines[len(lines)-1])
}

func TestInitHasCompletionThenTheWrapper(t *testing.T) {
	// per shell: how the function starts, how it goes to the directory, and how
	// everything else reaches the binary
	cases := map[Shell][3]string{
		Bash:       {"gero() {", `builtin cd -- "$dir"`, `command gero "$@"`},
		Zsh:        {"gero() {", `builtin cd -- "$dir"`, `command gero "$@"`},
		Fish:       {"function gero", `builtin cd -- $dir`, `command gero $argv`},
		PowerShell: {"function gero {", `Set-Location -LiteralPath $dir`, `& $exe @args`},
	}
	require.Len(t, cases, len(Shells), "every shell is covered")
	for shell, want := range cases {
		out, err := Init(shell, "# completion\n")

		require.NoError(t, err)
		assert.Contains(t, out, "# completion\n", string(shell))
		assert.Less(t, strings.Index(out, "# completion"), strings.Index(out, want[0]), string(shell))
		assert.Contains(t, out, want[1], string(shell))
		assert.Contains(t, out, want[2], string(shell)+": everything else goes to the binary")
	}
}

func TestEveryBlockIsOneCommentedRegionThatLoadsThatShellsScript(t *testing.T) {
	for _, shell := range Shells {
		b := Block(shell)

		assert.True(t, strings.HasPrefix(b, BlockStart+"\n"), string(shell))
		assert.True(t, strings.HasSuffix(b, BlockEnd+"\n"), string(shell))
		assert.Contains(t, b, "gero shell init "+string(shell), string(shell))
	}
}

func TestInitZshLoadsCompletionSystemOnlyWhenMissing(t *testing.T) {
	zsh, _ := Init(Zsh, "")
	bash, _ := Init(Bash, "")

	assert.Contains(t, zsh, "whence compdef")
	assert.Contains(t, zsh, "compinit")
	assert.NotContains(t, bash, "compinit")
}

func TestInitRejectsOtherShells(t *testing.T) {
	_, err := Init("tcsh", "")

	require.ErrorIs(t, err, ErrUnsupportedShell)
}

func TestInsertAppendsAfterABlankLine(t *testing.T) {
	block := Block(Bash)
	for name, c := range map[string]struct{ in, want string }{
		"empty":                {"", block},
		"ends with newline":    {"export A=1\n", "export A=1\n\n" + block},
		"ends with blank line": {"export A=1\n\n", "export A=1\n\n" + block},
		"no trailing newline":  {"export A=1", "export A=1\n\n" + block},
	} {
		got, changed, err := Insert(c.in, block)

		require.NoError(t, err, name)
		assert.True(t, changed, name)
		assert.Equal(t, c.want, got, name)
	}
}

func TestInsertIsIdempotentAndReplacesAnOldBlock(t *testing.T) {
	block := Block(Zsh)
	once, _, _ := Insert("alias a=b\n", block)

	again, changed, err := Insert(once, block)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, once, again)

	old := "alias a=b\n\n" + BlockStart + "\nold line\n" + BlockEnd + "\nalias c=d\n"
	got, changed, err := Insert(old, block)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "alias a=b\n\n"+block+"alias c=d\n", got, "only the block changes")
}

func TestRemoveRestoresWhatWasThere(t *testing.T) {
	for _, in := range []string{"", "export A=1\n", "export A=1", "a\n\nb\n"} {
		with, _, err := Insert(in, Block(Bash))
		require.NoError(t, err)

		got, changed, err := Remove(with)

		require.NoError(t, err)
		assert.True(t, changed, "%q", in)
		want := in
		if in == "export A=1" {
			want = "export A=1\n" // the newline Insert had to add stays
		}
		assert.Equal(t, want, got, "%q", in)
	}
}

func TestRemoveWithNoBlockChangesNothing(t *testing.T) {
	got, changed, err := Remove("export A=1\n")

	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, "export A=1\n", got)
}

func TestDamagedBlocksAreRefused(t *testing.T) {
	for name, in := range map[string]string{
		"start only":       BlockStart + "\nx\n",
		"end only":         "x\n" + BlockEnd + "\n",
		"two blocks":       Block(Bash) + Block(Bash),
		"end before start": BlockEnd + "\n" + BlockStart + "\n",
		"start mid-line":   "echo " + BlockStart + "\n" + BlockEnd + "\n",
	} {
		_, _, err := Insert(in, Block(Bash))
		require.ErrorIs(t, err, ErrBrokenBlock, name)
		_, _, err = Remove(in)
		require.ErrorIs(t, err, ErrBrokenBlock, name)
	}
}
