package clicfg

import (
	"regexp"
	"strconv"
	"strings"
)

// UserFileSchemaComment is the first line of a user file gero writes: it points
// the YAML language server of an editor at the schema of the user file on the
// main branch (docs/specs/APP_CONFIG.md#json-schemas).
const UserFileSchemaComment = "# yaml-language-server: $schema=https://raw.githubusercontent.com/rancher/orbs-automation/main/generated/jsonschema/user-config.schema.json"

// UserFileValues are the keys gero init writes to a new user file. A key left
// empty is left out of the file.
type UserFileValues struct {
	GitHubUser   string
	Fork         string // remote holding the developer's fork
	Upstream     string // remote treated as the real source
	BranchPrefix string
	ConfigDir    string   // configDir: the central application config
	Search       []string // repos.search: directories scanned for clones that exist
	CloneRoot    string   // repos.cloneRoot: where gero repo clone puts new clones
}

// RenderUserFile returns the text of a new user file with v: the schema comment,
// then the keys that are set, in the order of docs/specs/CLI_CONFIG.md.
func RenderUserFile(v UserFileValues) string {
	var b strings.Builder
	b.WriteString(UserFileSchemaComment + "\n")
	for _, kv := range [][2]string{
		{"githubUser", v.GitHubUser},
		{"fork", v.Fork},
		{"upstream", v.Upstream},
		{"branchPrefix", v.BranchPrefix},
	} {
		if kv[1] != "" {
			b.WriteString(kv[0] + ": " + yamlScalar(kv[1]) + "\n")
		}
	}
	if v.ConfigDir != "" {
		b.WriteString("configDir: " + pathScalar(v.ConfigDir) + "\n")
	}
	if len(v.Search) > 0 || v.CloneRoot != "" {
		b.WriteString("repos:\n")
	}
	if len(v.Search) > 0 {
		b.WriteString("  search:\n")
		for _, dir := range v.Search {
			b.WriteString("    - " + pathScalar(dir) + "\n")
		}
	}
	if v.CloneRoot != "" {
		b.WriteString("  cloneRoot: " + pathScalar(v.CloneRoot) + "\n")
	}
	return b.String()
}

var plainScalar = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_./@-]*$`)

// yamlWords are the plain words YAML 1.1 reads as something other than a string.
var yamlWords = map[string]bool{
	"y": true, "n": true, "yes": true, "no": true, "true": true, "false": true,
	"on": true, "off": true, "null": true,
}

// yamlScalar returns v as a YAML scalar that reads back as the same string: plain
// when that is certain, double-quoted otherwise.
func yamlScalar(v string) string {
	if plainScalar.MatchString(v) && !yamlWords[strings.ToLower(v)] {
		return v
	}
	return strconv.Quote(v)
}

var plainPath = regexp.MustCompile(`^[~/$][A-Za-z0-9_./@${}~-]*$`)

// pathScalar is yamlScalar for a path: "~/src", "/src" and "$HOME/src" stay plain,
// since a path is read for a leading "~" and for $VAR, and anything with a space
// or another character YAML reads is quoted.
func pathScalar(v string) string {
	if plainPath.MatchString(v) && v != "~" {
		return v
	}
	return strconv.Quote(v)
}
