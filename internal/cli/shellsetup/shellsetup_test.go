package shellsetup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/shellenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opts is a terminal run for zsh with a profile in a temp dir, answering answer.
func opts(t *testing.T, answer string) (Options, string, *strings.Builder) {
	t.Helper()
	profile := filepath.Join(t.TempDir(), ".zshrc")
	prompt := &strings.Builder{}
	return Options{
		Shell: "zsh", Profile: profile, Interactive: true,
		In: strings.NewReader(answer), Out: prompt,
		Getenv: func(string) string { return "" }, GOOS: "linux",
	}, profile, prompt
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestInstallAsksThenWritesTheBlockKeepingTheRest(t *testing.T) {
	o, profile, prompt := opts(t, "y\n")
	require.NoError(t, os.WriteFile(profile, []byte("export A=1\n"), 0o600))

	out, err := Install(o)

	require.NoError(t, err)
	assert.Equal(t, "export A=1\n\n"+shellenv.Block(shellenv.Zsh), read(t, profile))
	assert.Contains(t, prompt.String(), shellenv.BlockStart, "the block is shown before asking")
	assert.Contains(t, prompt.String(), "[y/N]")
	assert.Contains(t, out, "source "+profile)
	info, _ := os.Stat(profile)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the mode is kept")
}

func TestInstallCreatesAMissingProfile(t *testing.T) {
	o, profile, _ := opts(t, "yes\n")

	_, err := Install(o)

	require.NoError(t, err)
	assert.Equal(t, shellenv.Block(shellenv.Zsh), read(t, profile))
}

func TestInstallDeclinedWritesNothing(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", "", "maybe\n"} {
		o, profile, _ := opts(t, answer)

		_, err := Install(o)

		require.ErrorIs(t, err, ErrDeclined, "%q", answer)
		_, statErr := os.Stat(profile)
		assert.ErrorIs(t, statErr, os.ErrNotExist, "%q", answer)
	}
}

func TestInstallWithoutATerminalNeedsYes(t *testing.T) {
	o, profile, _ := opts(t, "y\n")
	o.Interactive = false

	_, err := Install(o)
	require.ErrorIs(t, err, ErrNeedsApproval)
	_, statErr := os.Stat(profile)
	assert.ErrorIs(t, statErr, os.ErrNotExist)

	o.Yes = true
	_, err = Install(o)
	require.NoError(t, err)
	assert.Contains(t, read(t, profile), shellenv.BlockStart)
}

func TestInstallDryRunShowsTheBlockAndWritesNothing(t *testing.T) {
	o, profile, prompt := opts(t, "")
	o.DryRun = true

	out, err := Install(o)

	require.NoError(t, err)
	assert.Contains(t, out, shellenv.BlockStart)
	assert.Contains(t, out, "nothing was written")
	assert.Empty(t, prompt.String(), "a dry run does not ask")
	_, statErr := os.Stat(profile)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestInstallTwiceIsAlreadySetUpAndDoesNotAsk(t *testing.T) {
	o, profile, _ := opts(t, "y\n")
	_, err := Install(o)
	require.NoError(t, err)
	before := read(t, profile)
	o2, _, prompt := opts(t, "")
	o2.Profile = profile

	out, err := Install(o2)

	require.NoError(t, err)
	assert.Contains(t, out, "Already set up")
	assert.Empty(t, prompt.String())
	assert.Equal(t, before, read(t, profile))
}

func TestInstallBringsAnOldBlockUpToDate(t *testing.T) {
	o, profile, _ := opts(t, "y\n")
	old := "keep\n\n" + shellenv.BlockStart + "\nold\n" + shellenv.BlockEnd + "\nalso keep\n"
	require.NoError(t, os.WriteFile(profile, []byte(old), 0o644))

	_, err := Install(o)

	require.NoError(t, err)
	assert.Equal(t, "keep\n\n"+shellenv.Block(shellenv.Zsh)+"also keep\n", read(t, profile))
}

func TestInstallFollowsASymlinkedProfile(t *testing.T) {
	o, profile, _ := opts(t, "y\n")
	dotfile := filepath.Join(t.TempDir(), "dotfiles-zshrc")
	require.NoError(t, os.WriteFile(dotfile, []byte("export A=1\n"), 0o644))
	require.NoError(t, os.Symlink(dotfile, profile))

	_, err := Install(o)

	require.NoError(t, err)
	info, err := os.Lstat(profile)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "still a link")
	assert.Contains(t, read(t, dotfile), shellenv.BlockStart)
}

func TestInstallRefusesADamagedBlock(t *testing.T) {
	o, profile, _ := opts(t, "y\n")
	require.NoError(t, os.WriteFile(profile, []byte(shellenv.BlockStart+"\n"), 0o644))

	_, err := Install(o)

	require.ErrorIs(t, err, shellenv.ErrBrokenBlock)
	assert.Equal(t, shellenv.BlockStart+"\n", read(t, profile))
}

func TestUninstallRemovesOnlyTheBlock(t *testing.T) {
	o, profile, _ := opts(t, "y\n")
	require.NoError(t, os.WriteFile(profile, []byte("export A=1\n\n"+shellenv.Block(shellenv.Zsh)), 0o644))

	_, err := Uninstall(o)

	require.NoError(t, err)
	assert.Equal(t, "export A=1\n", read(t, profile))
}

func TestUninstallWithNoBlockChangesNothing(t *testing.T) {
	o, profile, _ := opts(t, "")
	require.NoError(t, os.WriteFile(profile, []byte("export A=1\n"), 0o644))

	out, err := Uninstall(o)

	require.NoError(t, err)
	assert.Contains(t, out, "Nothing to remove")
	assert.Equal(t, "export A=1\n", read(t, profile))
}

func TestTheShellAndProfileAreDetectedWhenNotGiven(t *testing.T) {
	home := t.TempDir()
	o, _, _ := opts(t, "y\n")
	o.Shell, o.Profile = "", ""
	o.Getenv = func(k string) string { return map[string]string{"SHELL": "/bin/zsh", "HOME": home}[k] }

	_, err := Install(o)

	require.NoError(t, err)
	assert.Contains(t, read(t, filepath.Join(home, ".zshrc")), "gero shell init zsh")
}

func TestAnUnsupportedShellIsAnError(t *testing.T) {
	o, _, _ := opts(t, "y\n")
	o.Shell = "fish"

	_, err := Install(o)

	require.ErrorIs(t, err, shellenv.ErrUnsupportedShell)
}

func TestInitAsksForTheCompletionOfThatShell(t *testing.T) {
	var asked string
	out, err := Init("/bin/bash", func(shell string) (string, error) {
		asked = shell
		return "# completion for " + shell + "\n", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "bash", asked)
	assert.Contains(t, out, "# completion for bash")
	assert.Contains(t, out, "gero() {")
}

func TestInitRefusesAnUnsupportedShellWithoutGeneratingAnything(t *testing.T) {
	_, err := Init("fish", func(string) (string, error) {
		t.Fatal("no completion should be generated")
		return "", nil
	})

	require.ErrorIs(t, err, shellenv.ErrUnsupportedShell)
}
