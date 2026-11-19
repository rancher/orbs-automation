package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shorthands is the one table of short flags (docs/specs/CLI_FRAMEWORK.md). A
// flag name has the same letter on every command that has it, and a flag that
// is not here has none. Letters go to global flags first, then to flags several
// commands share, then to the rest.
var shorthands = map[string]string{
	// Global flags: the letter is reserved on every command.
	"verbose":  "v",
	"output":   "o",
	"json":     "j",
	"dry-run":  "n",
	"team":     "t",
	"upstream": "u",
	// Shared by several commands.
	"yes":        "y",
	"line":       "l",
	"preview":    "p",
	"push-to":    "T",
	"prerelease": "r",
	// Of one command.
	"project":    "P",
	"managed-by": "m",
	"external":   "e",
	"holding":    "H",
	"all":        "a",
	"into":       "i",
	"switch":     "s",
	"force":      "f",
	"check":      "c",
}

// walk calls fn for cmd and every command under it, hidden ones too.
func walk(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, c := range cmd.Commands() {
		walk(c, fn)
	}
}

// flagsOf visits the flags cmd answers to: its own and the ones it inherits.
func flagsOf(cmd *cobra.Command, fn func(*pflag.Flag)) {
	cmd.LocalFlags().VisitAll(fn)
	cmd.InheritedFlags().VisitAll(fn)
}

func TestShorthandsAreTheSameOnEveryCommand(t *testing.T) {
	walk(newRootCmd(), func(cmd *cobra.Command) {
		flagsOf(cmd, func(f *pflag.Flag) {
			if f.Name == "help" {
				return
			}
			assert.Equal(t, shorthands[f.Name], f.Shorthand,
				"the shorthand of --%s on %q", f.Name, cmd.CommandPath())
		})
	})
}

func TestEveryShorthandInTheTableIsUsed(t *testing.T) {
	seen := map[string]bool{}
	walk(newRootCmd(), func(cmd *cobra.Command) {
		flagsOf(cmd, func(f *pflag.Flag) { seen[f.Name] = true })
	})
	for name := range shorthands {
		assert.True(t, seen[name], "--%s is in the shorthand table but on no command", name)
	}
}

func TestShorthandLettersAreUnique(t *testing.T) {
	byLetter := map[string]string{}
	for name, letter := range shorthands {
		other, taken := byLetter[letter]
		assert.False(t, taken, "-%s is for both --%s and --%s", letter, other, name)
		byLetter[letter] = name
	}
	assert.NotContains(t, byLetter, "h", "-h is for --help")
}

func TestPrereleaseIsSpelledWithoutAHyphen(t *testing.T) {
	walk(newRootCmd(), func(cmd *cobra.Command) {
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			assert.NotContains(t, f.Name, "pre-release", "the flag --%s on %q", f.Name, cmd.CommandPath())
		})
	})
}

func TestSelfUpdateStillTakesPreReleaseAsAnAlias(t *testing.T) {
	cmd, _, err := newRootCmd().Find([]string{"doctor", "self-update"})
	require.NoError(t, err)

	require.NoError(t, cmd.Flags().Parse([]string{"--pre-release=false"}))

	assert.True(t, cmd.Flags().Changed("prerelease"))
	got, err := cmd.Flags().GetBool("prerelease")
	require.NoError(t, err)
	assert.False(t, got)
}

func TestSelfUpdateHelpListsOnlyPrerelease(t *testing.T) {
	cmd, _, err := newRootCmd().Find([]string{"doctor", "self-update"})
	require.NoError(t, err)

	usage := cmd.LocalFlags().FlagUsages()

	assert.Contains(t, usage, "--prerelease")
	assert.False(t, strings.Contains(usage, "--pre-release"), "the usage lists the old spelling:\n%s", usage)
}
