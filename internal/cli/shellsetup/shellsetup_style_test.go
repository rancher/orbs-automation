package shellsetup

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/gero/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return sgr.ReplaceAllString(s, "") }

var withColor = clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256}

func renderer() *style.Renderer { return style.New(withColor) }

// session runs install, or uninstall, over a profile with content, and returns
// what was returned and what was shown before asking, with the profile's path
// made the same from one run to the next.
func session(t *testing.T, install bool, content, answer string, term clicfg.Terminal, tweak func(*Options)) (out, asked string) {
	t.Helper()
	o, profile, prompt := opts(t, answer)
	o.Terminal = term
	if content != "" {
		require.NoError(t, os.WriteFile(profile, []byte(content), 0o600))
	}
	if tweak != nil {
		tweak(&o)
	}
	var err error
	if install {
		out, err = Install(o)
	} else {
		out, err = Uninstall(o)
	}
	require.NoError(t, err)
	return strings.ReplaceAll(out, profile, "PROFILE"), strings.ReplaceAll(prompt.String(), profile, "PROFILE")
}

func TestStyledSessionIsThePlainSessionWithEscapesAdded(t *testing.T) {
	for name, install := range map[string]bool{"install": true, "uninstall": false} {
		content := ""
		if !install {
			content = "export A=1\n\n" + blockOf(t)
		}
		plainOut, plainAsked := session(t, install, content, "y\n", clicfg.Terminal{}, nil)

		out, asked := session(t, install, content, "y\n", withColor, nil)

		assert.Equal(t, plainOut, strip(out), name)
		assert.Equal(t, plainAsked, strip(asked), name)
		assert.NotEqual(t, plainOut, out, name)
		assert.NotEqual(t, plainAsked, asked, name)
		assert.NotContains(t, plainOut+plainAsked, "\x1b", name)
	}
}

func blockOf(t *testing.T) string {
	t.Helper()
	o, profile, _ := opts(t, "y\n")
	o.Yes = true
	_, err := Install(o)
	require.NoError(t, err)
	data, err := os.ReadFile(profile)
	require.NoError(t, err)
	return string(data)
}

func TestTheQuestionIsStyledAndTheChoicesAreSecondary(t *testing.T) {
	v := renderer()

	_, asked := session(t, true, "", "y\n", withColor, nil)

	assert.Contains(t, asked, v.Style(style.Heading, "Continue?"))
	assert.Contains(t, asked, v.Style(style.Dim, "[y/N]"))
	assert.Contains(t, asked, v.Style(style.Heading, "Add to PROFILE:"), "what is about to be added is introduced as a heading")
}

func TestWhatWasDoneTakesTheLookOfIt(t *testing.T) {
	v := renderer()
	installed := "export A=1\n\n" + blockOf(t)

	updated, _ := session(t, true, "", "y\n", withColor, nil)
	already, _ := session(t, true, installed, "", withColor, nil)
	nothing, _ := session(t, false, "export A=1\n", "", withColor, nil)
	dry, _ := session(t, true, "", "", withColor, func(o *Options) { o.DryRun = true })

	assert.Contains(t, updated, v.Style(style.OK, "Updated"))
	assert.Contains(t, already, v.Style(style.OK, "Already set up in PROFILE"))
	assert.Contains(t, nothing, v.Style(style.Dim, "Nothing to remove from PROFILE"))
	assert.Contains(t, dry, v.Style(style.Dim, "Dry run: nothing was written."))
}
