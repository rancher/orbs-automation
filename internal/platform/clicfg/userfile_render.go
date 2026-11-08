package clicfg

// UserFileSchemaComment is the first line of a user file gero writes: it points
// the YAML language server of an editor at the schema of the user file on the
// main branch (docs/specs/APP_CONFIG.md#json-schemas).
const UserFileSchemaComment = "# yaml-language-server: $schema=https://raw.githubusercontent.com/rancher/orbs-automation/main/generated/jsonschema/user-config.schema.json"

// UserFileValues are the keys gero init writes to a new user file.
type UserFileValues struct {
	GitHubUser   string
	Fork         string // remote holding the developer's fork
	Upstream     string // remote treated as the real source
	BranchPrefix string
}

// RenderUserFile returns the text of a new user file with v: the schema comment,
// then the keys. It is not written yet.
func RenderUserFile(UserFileValues) string {
	return ""
}
