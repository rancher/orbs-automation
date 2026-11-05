package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	vars := map[string]string{"source": "SCC Operator", "version": "v0.5.3", "empty": "", "branch": "v2.15"}
	tests := []struct {
		name, text, want string
	}{
		{"no placeholder", "plain text", "plain text"},
		{"one placeholder", "Update {source}", "Update SCC Operator"},
		{"the title of the real config", "[{branch}] Update {source} to {version}", "[v2.15] Update SCC Operator to v0.5.3"},
		{"a repeated placeholder", "{version} {version}", "v0.5.3 v0.5.3"},
		{"an empty value", "a{empty}b", "ab"},
		{"adjacent placeholders", "{source}{version}", "SCC Operatorv0.5.3"},
		{"an escaped open brace", "{{source}", "{source}"},
		{"escaped braces around a name", "{{ {source} }}", "{ SCC Operator }"},
		{"multi-line text", "## Summary\n{source} {version}\n", "## Summary\nSCC Operator v0.5.3\n"},
		{"markdown and backticks are not special", "`{version}` **{source}**", "`v0.5.3` **SCC Operator**"},
		{"an empty text", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.text, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderErrors(t *testing.T) {
	vars := map[string]string{"a": "1"}
	tests := []struct {
		name, text, want string
	}{
		{"an unknown placeholder", "{nope}", "nope"},
		{"an unclosed brace", "{a", "unclosed"},
		{"a lone close brace", "a}", "}"},
		{"an empty placeholder", "{}", "empty"},
		{"a placeholder that is not a name", "{a b}", "a b"},
		{"a placeholder with a dot", "{a.b}", "a.b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(tt.text, vars)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestPlaceholders(t *testing.T) {
	got, err := Placeholders("[{branch}] {source} to {version} for {source}, {{literal}}")
	require.NoError(t, err)
	assert.Equal(t, []string{"branch", "source", "version"}, got, "in order of first use, without repeats")

	none, err := Placeholders("nothing {{here}}")
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestPlaceholdersMalformed(t *testing.T) {
	_, err := Placeholders("{oops")
	assert.Error(t, err)
}

func TestValidate(t *testing.T) {
	allowed := []string{"source", "version", "branch"}
	assert.NoError(t, Validate("[{branch}] Update {source} to {version}", allowed))
	assert.NoError(t, Validate("no placeholders", allowed))

	err := Validate("Update {sourc} to {version}", allowed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sourc")

	assert.Error(t, Validate("{unclosed", allowed))
}
