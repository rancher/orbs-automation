package v1alpha2

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func artifactDoc(spec string) string {
	return "apiVersion: gero/v1alpha2\nkind: Artifact\nname: a\nrefs: {stream: s}\nspec:\n" + spec
}

func loadArtifact(t *testing.T, spec string) (*Artifact, config.Report) {
	t.Helper()
	objs, rep := decodeReport(t, artifactDoc(spec))
	if len(objs) == 0 {
		return nil, rep
	}
	require.Len(t, objs, 1)
	return objs[0].(*Artifact), rep
}

func TestAValidArtifactHasNoFindings(t *testing.T) {
	for name, spec := range map[string]string{
		"an image":                       "  kind: image\n  name: rancher/shell\n",
		"an image with registries":       "  kind: image\n  name: rancher/rancher-server\n  registries: [dockerhub, prime-staging, prime-production]\n  lines: {since: v2.16}\n",
		"a file named by the version":    "  kind: file\n  name: \"rancher-backup-{version}.tgz\"\n",
		"a file named by the tag":        "  kind: file\n  name: \"libfoo-{tag}.zip\"\n",
		"a file with a modifier":         "  kind: file\n  name: \"x-{tag|trimprefix:v}.tgz\"\n",
		"a range of lines":               "  kind: image\n  name: a/b\n  lines: {since: v2.14, until: v2.16}\n",
		"a range with the same two ends": "  kind: image\n  name: a/b\n  lines: {since: v2.15, until: v2.15}\n",
	} {
		t.Run(name, func(t *testing.T) {
			a, rep := loadArtifact(t, spec)

			assert.Empty(t, rep.Findings)
			assert.NotNil(t, a)
		})
	}
}

func TestArtifactRules(t *testing.T) {
	tests := []struct {
		name, spec, path, want string
	}{
		{"no kind", "  name: a/b\n", "spec.kind", "image or file"},
		{"a kind that is neither", "  kind: chart\n  name: a/b\n", "spec.kind", "image or file"},
		{"no name", "  kind: image\n", "spec.name", "required"},
		{"an image name with a variable", "  kind: image\n  name: \"rancher/{tag}\"\n", "spec.name", "variables"},
		{"a file name with an unknown variable", "  kind: file\n  name: \"x-{nope}.tgz\"\n", "spec.name", "unknown placeholder {nope}"},
		{"a file name that is malformed", "  kind: file\n  name: \"x-{tag.tgz\"\n", "spec.name", "unclosed"},
		{"registries on a file", "  kind: file\n  name: x\n  registries: [dockerhub]\n", "spec.registries", "image"},
		{"a registry that does not exist", "  kind: image\n  name: a/b\n  registries: [quay]\n", "spec.registries[0]", "dockerhub, prime-staging or prime-production"},
		{"a registry listed twice", "  kind: image\n  name: a/b\n  registries: [dockerhub, dockerhub]\n", "spec.registries[1]", "twice"},
		{"a since that is not a line", "  kind: image\n  name: a/b\n  lines: {since: 2.16}\n", "spec.lines.since", "v2.16"},
		{"an until that is not a line", "  kind: image\n  name: a/b\n  lines: {until: latest}\n", "spec.lines.until", "v2.16"},
		{"a since after the until", "  kind: image\n  name: a/b\n  lines: {since: v2.16, until: v2.14}\n", "spec.lines", "after"},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, rep := loadArtifact(t, tt.spec)

			assert.Nil(t, a, "an artifact that breaks a rule is dropped")
			assert.True(t, hasError(rep, tt.path, tt.want), "want an error at %s containing %q, got %v", tt.path, tt.want, rep.Findings)
		})
	}
}

func TestAnImageNameHasNoSpaces(t *testing.T) {
	a, rep := loadArtifact(t, "  kind: image\n  name: \"acme/a b\"\n")

	assert.Nil(t, a)
	assert.True(t, hasError(rep, "spec.name", "space"), "%v", rep.Findings)
}
