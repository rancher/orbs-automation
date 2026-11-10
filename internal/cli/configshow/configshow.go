// Package configshow holds the logic of gero config show: it renders the
// resolved CLI Settings and the layer each value came from. It is
// framework-free: cmd/ owns the cobra wiring.
package configshow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/clicfg"
	"github.com/rancher/orbs-automation/internal/ui"
	"github.com/rancher/orbs-automation/internal/ui/style"
	"github.com/rancher/orbs-automation/internal/workspace"
)

// Entry is one resolved setting.
type Entry struct {
	Key    string // user-file key, dotted for nested keys (repos.paths.<slug>)
	Value  string
	Source string // clicfg layer name (default, file, env, flag) or a resolver's reason
}

// scalarKeys are the keys that have a flag and an environment variable, and so
// the ones the env format can express.
var scalarKeys = []string{"centralConfigDir", "output", "color", "repl.prompt", "log", "configSource", "bundleChannel", "bundleFile", "upstream", "team"}

// hints say what an unset key falls back to. Text output shows one in place of
// "(unset)"; json and env never do, so scripts never read a hint as data. They
// describe the rules in docs/specs/WORKSPACE.md and docs/specs/CLI_CONFIG.md. Nothing
// infers the upstream, the fork or githubUser, so each says none.
var hints = map[string]string{
	"userFile":         "none",
	"centralConfigDir": "none; ./config in a central checkout",
	"bundleFile":       "none",
	"upstream":         "none",
	"fork":             "none",
	"githubUser":       "none",
	"branchPrefix":     "none",
	"pushTo":           "none: the fork when one is set, else the upstream",
}

// Entries flattens s into one Entry per setting, in display order: the user
// file the values came from, the scalar keys, then the repos sections with their
// map keys sorted. dir is the resolved
// central config directory and overrides the centralConfigDir entry; the zero Result
// (nothing resolved) leaves it as loaded.
func Entries(s clicfg.Settings, dir workspace.CentralConfigDir) []Entry {
	centralConfigDir := entry(s, "centralConfigDir", s.CentralConfigDir)
	if dir != (workspace.CentralConfigDir{}) {
		centralConfigDir = Entry{Key: "centralConfigDir", Value: dir.Dir, Source: dir.Source}
	}
	es := []Entry{
		entry(s, "userFile", s.UserFile),
		centralConfigDir,
		entry(s, "output", s.Output),
		entry(s, "color", s.Color),
		entry(s, "repl.prompt", s.ReplPrompt),
		entry(s, "log", s.LogLevel),
		entry(s, "configSource", s.ConfigSource),
		entry(s, "bundleChannel", s.BundleChannel),
		entry(s, "bundleFile", s.BundleFile),
		entry(s, "upstream", s.Upstream),
		entry(s, "team", s.Team),
		entry(s, "fork", s.Fork),
		entry(s, "githubUser", s.GitHubUser),
		entry(s, "branchPrefix", s.BranchPrefix),
		entry(s, "pushTo", s.PushTo),
	}
	if len(s.Repos.Search) > 0 {
		es = append(es, Entry{"repos.search", strings.Join(s.Repos.Search, ","), clicfg.SourceFile.String()})
	}
	if s.Repos.CloneRoot != "" {
		es = append(es, entry(s, "repos.cloneRoot", s.Repos.CloneRoot))
	}
	for _, slug := range sortedKeys(s.Repos.Paths) {
		es = append(es, Entry{"repos.paths." + slug, s.Repos.Paths[slug], clicfg.SourceFile.String()})
	}
	for _, slug := range sortedKeys(s.Repos.Overrides) {
		o := s.Repos.Overrides[slug]
		if o.Upstream != "" {
			es = append(es, Entry{"repos.overrides." + slug + ".upstream", o.Upstream, clicfg.SourceFile.String()})
		}
		if o.PushTo != "" {
			es = append(es, Entry{"repos.overrides." + slug + ".pushTo", o.PushTo, clicfg.SourceFile.String()})
		}
	}
	return es
}

func entry(s clicfg.Settings, key, value string) Entry {
	return Entry{Key: key, Value: value, Source: s.Source(key).String()}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Render formats s as clicfg.OutputText, OutputJSON or OutputEnv. The env
// format lists only the keys that have an environment variable.
func Render(s clicfg.Settings, dir workspace.CentralConfigDir, format string) (string, error) {
	es := Entries(s, dir)
	switch format {
	case clicfg.OutputText:
		return renderText(es, style.New(s.Terminal)), nil
	case clicfg.OutputJSON:
		return renderJSON(es)
	case clicfg.OutputEnv:
		return renderEnv(es), nil
	}
	return "", fmt.Errorf("unknown output format %q", format)
}

func renderText(es []Entry, st *style.Renderer) string {
	rows := [][]string{{st.Style(style.Dim, "KEY"), st.Style(style.Dim, "VALUE"), st.Style(style.Dim, "SOURCE")}}
	search := false
	for _, e := range es {
		search = search || e.Key == "repos.search"
		v := e.Value
		if v == "" {
			v = "(unset)"
			if h, ok := hints[e.Key]; ok && e.Source == clicfg.SourceDefault.String() {
				v = "(" + h + ")"
			}
			v = st.Style(style.Dim, v)
		}
		source := e.Source
		if source == clicfg.SourceDefault.String() {
			source = st.Style(style.Dim, source)
		}
		rows = append(rows, []string{e.Key, v, source})
	}
	out := ui.Align(rows, st.Width)
	if search {
		out += "\n" + st.Style(style.Dim, "run `gero config resolve repos` to see where each repo resolves") + "\n"
	}
	return out
}

func renderJSON(es []Entry) (string, error) {
	type field struct {
		Value  string `json:"value"`
		Source string `json:"source"`
	}
	m := make(map[string]field, len(es))
	for _, e := range es {
		m[e.Key] = field{e.Value, e.Source}
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out) + "\n", nil
}

func renderEnv(es []Entry) string {
	var b strings.Builder
	for _, e := range es {
		for _, k := range scalarKeys {
			if e.Key == k {
				fmt.Fprintf(&b, "%s=%s\n", clicfg.EnvName(k), e.Value)
			}
		}
	}
	return b.String()
}
