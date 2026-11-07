package repl

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/rancher/orbs-automation/internal/clicfg"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func render(t *testing.T, tmpl string, v Values) string {
	t.Helper()
	parsed, err := ParseTemplate(tmpl)
	require.NoError(t, err, tmpl)
	return parsed.Render(v)
}

func TestTheDefaultTemplateIsPlainOutsideAProjectAndNamesTheProjectInside(t *testing.T) {
	assert.Equal(t, "gero> ", render(t, clicfg.DefaultReplPrompt, Values{Dir: "/work"}))
	assert.Equal(t, "gero (scc-operator)> ", render(t, clicfg.DefaultReplPrompt, Values{Project: "scc-operator", Dir: "/src/scc-operator"}))
}

func TestPlaceholders(t *testing.T) {
	v := Values{Project: "scc-operator", Dir: "/home/dan/GitProjects/SUSE/scc-operator", Home: "/home/dan"}
	rows := []struct {
		name, tmpl string
		v          Values
		want       string
	}{
		{"project", "{project}", v, "scc-operator"},
		{"project outside one is empty", "[{project}]", Values{Dir: "/work"}, "[]"},
		{"dir is the last element", "{dir}", v, "scc-operator"},
		{"dir of the root", "{dir}", Values{Dir: "/"}, "/"},
		{"path with the home as ~", "{path}", v, "~/GitProjects/SUSE/scc-operator"},
		{"path of the home itself", "{path}", Values{Dir: "/home/dan", Home: "/home/dan"}, "~"},
		{"path outside the home", "{path}", Values{Dir: "/srv/work", Home: "/home/dan"}, "/srv/work"},
		{"a directory that only starts like the home", "{path}", Values{Dir: "/home/danger/x", Home: "/home/dan"}, "/home/danger/x"},
		{"path with no home known", "{path}", Values{Dir: "/home/dan/x"}, "/home/dan/x"},
		{"text around", "[{dir}] {project}$ ", v, "[scc-operator] scc-operator$ "},
		{"plain text", "> ", v, "> "},
		{"arrows and wide characters", "→ 世界 ", v, "→ 世界 "},
	}
	require.NotEmpty(t, rows)
	for _, r := range rows {
		assert.Equal(t, r.want, render(t, r.tmpl, r.v), r.name)
	}
}

func TestATextSegmentIsWrittenOnlyWhenTheValueIsNotEmpty(t *testing.T) {
	in, out := Values{Project: "shell", Dir: "/s"}, Values{Dir: "/s"}
	rows := []struct {
		name, tmpl string
		v          Values
		want       string
	}{
		{"with a value", "{project: (%s)}", in, " (shell)"},
		{"without one it vanishes, with no stray punctuation", "a{project: (%s)}b", out, "ab"},
		{"%s twice", "{project:%s/%s}", in, "shell/shell"},
		{"no %s", "{project:in a project}", in, "in a project"},
		{"no %s and no value", "{project:in a project}", out, ""},
		{"a text segment of another placeholder", "{dir:<%s>}", in, "<s>"},
		{"a literal percent stays", "{project:100%% %s}", in, "100%% shell"},
	}
	for _, r := range rows {
		assert.Equal(t, r.want, render(t, r.tmpl, r.v), r.name)
	}
}

func TestDoubledBracesAreLiteral(t *testing.T) {
	v := Values{Project: "x", Dir: "/d"}
	rows := []struct{ name, tmpl, want string }{
		{"open", "{{project}}", "{project}"},
		{"close", "a}}b", "a}b"},
		{"both", "{{}}", "{}"},
		{"inside a text segment", "{project:{{%s}}}", "{x}"},
		{"next to a placeholder", "{{{project}}}", "{x}"},
	}
	for _, r := range rows {
		assert.Equal(t, r.want, render(t, r.tmpl, v), r.name)
	}
}

func TestATemplateThatIsWrongIsAnError(t *testing.T) {
	rows := []struct{ name, tmpl, want string }{
		{"an unknown placeholder", "{branch}> ", "unknown placeholder {branch}"},
		{"an unknown one with text", "{branch: (%s)}", "unknown placeholder {branch}"},
		{"an empty name", "{}", "empty placeholder"},
		{"an unterminated brace", "{project", "unterminated"},
		{"an unterminated text segment", "{project: (%s)", "unterminated"},
		{"a lone close brace", "a } b", "unbalanced }"},
		{"a raw escape", "\x1b[31mgero\x1b[0m> ", "control character"},
		{"a newline", "gero\n> ", "control character"},
		{"a tab", "gero\t> ", "control character"},
		{"delete", "gero\x7f> ", "control character"},
		{"a control character in a text segment", "{project:\x1b[1m%s}", "control character"},
	}
	for _, r := range rows {
		_, err := ParseTemplate(r.tmpl)

		require.Error(t, err, r.name)
		assert.Contains(t, err.Error(), r.want, r.name)
	}
}

func TestTheErrorQuotesWhatIsWrongSoAnEscapeCanBeSeen(t *testing.T) {
	_, err := ParseTemplate("gero\x1b> ")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `\x1b`, "a raw escape would not show in the message")
	assert.NotContains(t, err.Error(), "\x1b")
}

func TestAnEmptyTemplateIsAPromptOfNothing(t *testing.T) {
	assert.Equal(t, "", render(t, "", Values{Dir: "/"}))
}

func settingsWith(t *testing.T, env map[string]string, file string) clicfg.Settings {
	t.Helper()
	in := clicfg.Inputs{Getenv: func(k string) string { return env[k] }, UserFileName: "/home/dan/.config/gero/config.yaml"}
	if file != "" {
		in.UserFile = strings.NewReader(file)
	}
	s, err := clicfg.Load(in)
	require.NoError(t, err)
	return s
}

func TestPromptTemplateOfTheSettingsNamesTheKeyAndWhereItCameFrom(t *testing.T) {
	_, err := PromptTemplate(settingsWith(t, map[string]string{"GERO_REPL_PROMPT": "{foo}> "}, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repl.prompt")
	assert.Contains(t, err.Error(), "GERO_REPL_PROMPT")
	assert.Contains(t, err.Error(), "unknown placeholder {foo}")

	_, err = PromptTemplate(settingsWith(t, nil, "repl:\n  prompt: \"{foo}> \"\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repl.prompt")
	assert.Contains(t, err.Error(), "/home/dan/.config/gero/config.yaml")
}

func TestPromptTemplateOfTheDefaultSettingsIsTheDefaultPrompt(t *testing.T) {
	tmpl, err := PromptTemplate(settingsWith(t, nil, ""))

	require.NoError(t, err)
	assert.Equal(t, "gero> ", tmpl.Render(Values{Dir: "/w"}))
	assert.Equal(t, "gero (shell)> ", tmpl.Render(Values{Project: "shell", Dir: "/w"}))
}

// whereAmI is a ProjectOf that knows two directories and counts what it was asked.
type whereAmI struct {
	projects map[string]string
	asked    []string
}

func (w *whereAmI) of(dir string) string {
	w.asked = append(w.asked, dir)
	return w.projects[dir]
}

func promptsOf(t *testing.T, s *Session, steps ...step) []string {
	t.Helper()
	r := &scripted{steps: steps}
	require.NoError(t, Loop(context.Background(), r, io.Discard, io.Discard, s))
	return r.prompts
}

func TestThePromptNamesTheProjectTheSessionStartedIn(t *testing.T) {
	w := &whereAmI{projects: map[string]string{"/src/scc": "scc-operator"}}
	tmpl, err := ParseTemplate(clicfg.DefaultReplPrompt)
	require.NoError(t, err)
	s := &Session{Dir: "/src/scc", Template: tmpl, ProjectOf: w.of}

	prompts := promptsOf(t, s)

	assert.Equal(t, []string{"gero (scc-operator)> "}, prompts)
}

func TestThePromptOutsideAProjectIsThePlainOne(t *testing.T) {
	w := &whereAmI{}
	tmpl, err := ParseTemplate(clicfg.DefaultReplPrompt)
	require.NoError(t, err)

	prompts := promptsOf(t, &Session{Dir: "/work", Template: tmpl, ProjectOf: w.of})

	assert.Equal(t, []string{"gero> "}, prompts)
}

func TestWithoutATemplateOrALookupThePromptIsTheDefaultOne(t *testing.T) {
	assert.Equal(t, []string{"gero> "}, promptsOf(t, &Session{Dir: "/work"}))
}

// The project is worked out when the directory changes and at the start, and
// not on every line: the prompt is drawn each time and must stay cheap.
func TestTheProjectIsLookedUpAtTheStartAndAfterACdAndNotOnEveryLine(t *testing.T) {
	start := t.TempDir()
	clone := t.TempDir()
	t.Chdir(start)
	w := &whereAmI{projects: map[string]string{clone: "rancher"}}
	tmpl, err := ParseTemplate(clicfg.DefaultReplPrompt)
	require.NoError(t, err)
	s := &Session{Dir: start, Template: tmpl, ProjectOf: w.of, ProjectDir: projects(map[string]string{"rancher": clone})}

	prompts := promptsOf(t, s, step{"pwd", nil}, step{"pwd", nil}, step{"cd rancher", nil}, step{"pwd", nil}, step{"pwd", nil})

	assert.Equal(t, []string{start, clone}, w.asked, "once at the start, once for the cd, none for the five lines")
	assert.Equal(t, []string{
		"gero> ", "gero> ", "gero> ", "gero (rancher)> ", "gero (rancher)> ", "gero (rancher)> ",
	}, prompts)
}

func TestAFailedCdDoesNotLookTheProjectUpAgain(t *testing.T) {
	start := t.TempDir()
	t.Chdir(start)
	w := &whereAmI{}
	s := &Session{Dir: start, ProjectOf: w.of, ProjectDir: projects(nil)}

	_ = promptsOf(t, s, step{"cd nowhere", nil})

	assert.Equal(t, []string{start}, w.asked)
}

func TestThePathPlaceholderUsesTheHomeOfTheSession(t *testing.T) {
	tmpl, err := ParseTemplate("{path}$ ")
	require.NoError(t, err)

	prompts := promptsOf(t, &Session{Dir: "/home/dan/src", Home: "/home/dan", Template: tmpl})

	assert.Equal(t, []string{"~/src$ "}, prompts)
}

func TestTheTemplatesTrailingSpaceIsNotStyledAndThePromptStaysOneLine(t *testing.T) {
	v := style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
	tmpl, err := ParseTemplate("[{dir}]> ")
	require.NoError(t, err)

	prompts := promptsOf(t, &Session{Dir: "/src/scc", Template: tmpl, Style: v})

	require.Len(t, prompts, 1)
	assert.Equal(t, v.Style(style.Prompt, "[scc]>")+" ", prompts[0])
	assert.NotContains(t, prompts[0], "\n")
}

func TestATemplateWithNoTrailingSpaceIsStyledWhole(t *testing.T) {
	v := style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
	tmpl, err := ParseTemplate("{dir}$")
	require.NoError(t, err)

	prompts := promptsOf(t, &Session{Dir: "/src/scc", Template: tmpl, Style: v})

	assert.Equal(t, []string{v.Style(style.Prompt, "scc$")}, prompts)
}

func TestAnEmptyPromptIsNothingAndNeverEscapes(t *testing.T) {
	v := style.New(clicfg.Terminal{StdoutTTY: true, Profile: clicfg.Profile256})
	tmpl, err := ParseTemplate("")
	require.NoError(t, err)

	prompts := promptsOf(t, &Session{Dir: "/", Template: tmpl, Style: v})

	assert.Equal(t, []string{""}, prompts, "no empty escape pair around nothing")
}
