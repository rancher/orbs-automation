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
		{"escaped braces around a name", "{{source}}", "{source}"},
		{"escaped braces next to a placeholder", "{{ {source} }}", "{ SCC Operator }"},
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

func TestRenderModifiers(t *testing.T) {
	vars := map[string]string{"line": "v2.16", "stage": "stable", "name": "rancher shell", "tag": "v0.9.1-rc.2", "empty": ""}
	tests := []struct {
		name, text, want string
	}{
		{"upper", "{name|upper}", "RANCHER SHELL"},
		{"lower", "{line|upper|lower}", "v2.16"},
		{"title capitalizes the first letter", "{name|title}", "Rancher shell"},
		{"trimprefix", "{line|trimprefix:v}", "2.16"},
		{"trimprefix of a prefix that is not there", "{line|trimprefix:x}", "v2.16"},
		{"trimsuffix", "{tag|trimsuffix:-rc.2}", "v0.9.1"},
		{"replace of a whole value", "{stage|replace:stable=prod}", "prod"},
		{"replace of a value that is another", "{stage|replace:dev=dev}", "stable"},
		{"replace is not a substring replace", "{name|replace:rancher=x}", "rancher shell"},
		{"modifiers apply left to right", "{tag|trimprefix:v|upper}", "0.9.1-RC.2"},
		{"a chain of two replaces", "{stage|replace:dev=d|replace:stable=p}", "p"},
		{"a modifier on an empty value", "a{empty|upper}b", "ab"},
		{"a modifier next to an escaped brace", "{{ {line|trimprefix:v} }}", "{ 2.16 }"},
		{"the key of an assets lock file", `.chart-versions."{line|trimprefix:v}".upstream-refs.{stage|replace:stable=prod}.charts.commit`,
			`.chart-versions."2.16".upstream-refs.prod.charts.commit`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.text, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestModifierErrors(t *testing.T) {
	vars := map[string]string{"a": "1"}
	tests := []struct {
		name, text, want string
	}{
		{"an unknown modifier", "{a|reverse}", `unknown modifier "reverse"`},
		{"an empty modifier", "{a|}", "empty modifier"},
		{"trimprefix without an argument", "{a|trimprefix}", "trimprefix needs an argument"},
		{"trimsuffix without an argument", "{a|trimsuffix:}", "trimsuffix needs an argument"},
		{"replace without FROM=TO", "{a|replace:abc}", "replace needs FROM=TO"},
		{"replace with an empty FROM", "{a|replace:=x}", "replace needs FROM=TO"},
		{"upper with an argument", "{a|upper:x}", "upper takes no argument"},
		{"a modifier on a name that is not a name", "{a b|upper}", "is not a placeholder name"},
		{"a modifier on an unknown placeholder", "{nope|upper}", "unknown placeholder {nope}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(tt.text, vars)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestPlaceholdersLeaveOutTheModifiers(t *testing.T) {
	got, err := Placeholders("{line|trimprefix:v} {stage|replace:stable=prod} {line}")

	require.NoError(t, err)
	assert.Equal(t, []string{"line", "stage"}, got)
}

func TestValidateChecksTheModifiersToo(t *testing.T) {
	assert.NoError(t, Validate("{line|trimprefix:v}", []string{"line"}))
	assert.Error(t, Validate("{line|nope}", []string{"line"}))
	assert.Error(t, Validate("{other|upper}", []string{"line"}))
}

func TestCheckReversible(t *testing.T) {
	tests := []struct {
		name, text string
		reversible bool
	}{
		{"no modifier", "{image}:{tag}", true},
		{"trimprefix", "{tag|trimprefix:v}", true},
		{"trimsuffix", "{tag|trimsuffix:-rc.1}", true},
		{"replace", "{stage|replace:stable=prod}", true},
		{"a chain of reversible modifiers", "{tag|trimprefix:v|trimsuffix:x}", true},
		{"upper loses the case", "{tag|upper}", false},
		{"lower loses the case", "{tag|lower}", false},
		{"title loses the case", "{tag|title}", false},
		{"a lossy one among reversible ones", "{tag|trimprefix:v|upper}", false},
		{"a lossy one in the second placeholder", "{image}:{tag|lower}", false},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckReversible(tt.text)
			if tt.reversible {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "modifier")
		})
	}
}

func TestCheckReversibleNamesTheModifier(t *testing.T) {
	err := CheckReversible("{tag|upper}")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upper")
}

func TestPrefixRepeats(t *testing.T) {
	tests := []struct {
		name, text string
		want       []string
	}{
		{"the default prefix says nothing twice", "bot/", nil},
		{"the producer", "bot/{source}-", []string{"source"}},
		{"in the order of first use, once", "{project}/{tag}/{project}", []string{"project", "tag"}},
		{"every spelling of the tag and the target", "{version}{branch}{targetBranch}", []string{"version", "branch", "targetBranch"}},
		{"what the key does not say is the user's", "{consumer}-{sourceRepo}", nil},
		{"a modifier does not hide it", "{tag|trimprefix:v}", []string{"tag"}},
		{"a malformed text has none", "bot/{source", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PrefixRepeats(tt.text))
		})
	}
}
