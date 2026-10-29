// Package configshow holds the logic of gero config show: it renders the
// resolved CLI Settings and the layer each value came from. It is
// framework-free: cmd/ owns the cobra wiring.
package configshow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rancher/orbs-automation/internal/clicfg"
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
var scalarKeys = []string{"configDir", "output", "upstream"}

// hints say what an unset key falls back to. Text output shows one in place of
// "(unset)"; json and env never do, so scripts never read a hint as data. They
// describe the rules in docs/specs/CLI_CONFIG.md sections 3a and 4, and each
// goes away as internal/workspace infers a real value for its key.
var hints = map[string]string{
	"configDir":    "none; ./config in a central checkout",
	"upstream":     "inferred from remotes",
	"fork":         "inferred from remotes",
	"githubUser":   "inferred from the fork remote",
	"branchPrefix": "none",
}

// Entries flattens s into one Entry per setting, in display order: the scalar
// keys, then the repos sections with their map keys sorted. dir is the resolved
// central config directory and overrides the configDir entry; the zero Result
// (nothing resolved) leaves it as loaded.
func Entries(s clicfg.Settings, dir workspace.ConfigDir) []Entry {
	configDir := entry(s, "configDir", s.ConfigDir)
	if dir != (workspace.ConfigDir{}) {
		configDir = Entry{Key: "configDir", Value: dir.Dir, Source: dir.Source}
	}
	es := []Entry{
		configDir,
		entry(s, "output", s.Output),
		entry(s, "upstream", s.Upstream),
		entry(s, "fork", s.Fork),
		entry(s, "githubUser", s.GitHubUser),
		entry(s, "branchPrefix", s.BranchPrefix),
	}
	if len(s.Repos.Search) > 0 {
		es = append(es, Entry{"repos.search", strings.Join(s.Repos.Search, ","), clicfg.SourceFile.String()})
	}
	for _, slug := range sortedKeys(s.Repos.Paths) {
		es = append(es, Entry{"repos.paths." + slug, s.Repos.Paths[slug], clicfg.SourceFile.String()})
	}
	for _, slug := range sortedKeys(s.Repos.Overrides) {
		es = append(es, Entry{"repos.overrides." + slug + ".upstream", s.Repos.Overrides[slug].Upstream, clicfg.SourceFile.String()})
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
func Render(s clicfg.Settings, dir workspace.ConfigDir, format string) (string, error) {
	es := Entries(s, dir)
	switch format {
	case clicfg.OutputText:
		return renderText(es), nil
	case clicfg.OutputJSON:
		return renderJSON(es)
	case clicfg.OutputEnv:
		return renderEnv(es), nil
	}
	return "", fmt.Errorf("unknown output format %q", format)
}

func renderText(es []Entry) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tVALUE\tSOURCE")
	for _, e := range es {
		v := e.Value
		if v == "" {
			v = "(unset)"
			if h, ok := hints[e.Key]; ok && e.Source == clicfg.SourceDefault.String() {
				v = "(" + h + ")"
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", e.Key, v, e.Source)
	}
	_ = w.Flush() // writes to a strings.Builder, which cannot fail
	return b.String()
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
