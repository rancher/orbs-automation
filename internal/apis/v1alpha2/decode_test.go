package v1alpha2

import (
	"github.com/rancher/orbs-automation/internal/apis/lines"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureDir is the design fixture: the seven projects and two other examples,
// written in this shape (docs/design/fixture).
const fixtureDir = "../../../docs/design/fixture"

func decodeAll(t *testing.T, src string) []Object {
	t.Helper()
	objs, err := Decode(strings.NewReader(src), "test.yaml")
	require.NoError(t, err)
	return objs
}

func decodeOne[T Object](t *testing.T, src string) T {
	t.Helper()
	objs := decodeAll(t, src)
	require.Len(t, objs, 1)
	require.IsType(t, *new(T), objs[0])
	return objs[0].(T)
}

func TestDecodeFixture(t *testing.T) {
	var files []string
	err := filepath.WalkDir(fixtureDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) == ".yaml" {
			files = append(files, path)
		}
		return err
	})
	require.NoError(t, err)
	require.NotEmpty(t, files)

	kinds := map[string]int{}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		objs, err := Decode(strings.NewReader(string(raw)), f)
		require.NoError(t, err, f)
		total += len(objs)
		for _, o := range objs {
			kinds[o.GetKind()]++
		}
	}

	assert.Equal(t, 49, total)
	assert.Equal(t, map[string]int{
		KindProject: 13, KindStream: 10, KindArtifact: 11, KindHolding: 12, KindEdit: 2, KindRancherLines: 1,
	}, kinds)
}

func TestDecodeEnvelope(t *testing.T) {
	p := decodeOne[*Project](t, `
apiVersion: gero/v1alpha2
kind: Project
name: shell
spec:
  repo: {name: rancher/shell, owned: true}
`)

	assert.Equal(t, APIVersion, p.GetAPIVersion())
	assert.Equal(t, KindProject, p.GetKind())
	assert.Equal(t, "shell", p.GetName())
	assert.Empty(t, p.GetRefs())
}

func TestDecodeProject(t *testing.T) {
	p := decodeOne[*Project](t, `
apiVersion: gero/v1alpha2
kind: Project
name: rancher-charts
spec:
  repo: {name: rancher/charts, owned: false, managedBy: release-team, defaultBranch: dev-v2.16}
  finalDestination: true
  pullRequests:
    title: "[{branch}] {package} {chartVersion} {action}"
    branchPrefix: "bot/{source}-"
    labels: [status/auto-created]
    botLabels: [origin/bot]
    pushTo: fork
  branches:
    layout: perLine
    stages: [dev, stable]
    lines:
      - {line: v2.17}
      - {line: v2.16, branch: ""}
      - {line: v2.15, branch: release/v2.15}
      - {line: v2.14, branch: {dev: dev-v2.14, stable: release-v2.14}}
`)

	assert.Equal(t, Repo{Name: "rancher/charts", ManagedBy: "release-team", DefaultBranch: "dev-v2.16"}, p.Spec.Repo)
	assert.True(t, p.Spec.FinalDestination)
	assert.Equal(t, PullRequests{
		Title: "[{branch}] {package} {chartVersion} {action}", BranchPrefix: "bot/{source}-",
		Labels: []string{"status/auto-created"}, BotLabels: []string{"origin/bot"}, PushTo: "fork",
	}, p.Spec.PullRequests)
	assert.Equal(t, "perLine", p.Spec.Branches.Layout)
	assert.Equal(t, []string{"dev", "stable"}, p.Spec.Branches.Stages)

	lines := p.Spec.Branches.Lines
	require.Len(t, lines, 4)
	assert.Nil(t, lines[0].Branch, "omitted: the default branch")
	require.NotNil(t, lines[1].Branch, "an empty string is written, so it is not omitted")
	assert.Equal(t, Branch{}, *lines[1].Branch, "no branch yet")
	assert.Equal(t, &Branch{Name: "release/v2.15"}, lines[2].Branch)
	assert.Equal(t, &Branch{Stages: map[string]string{"dev": "dev-v2.14", "stable": "release-v2.14"}}, lines[3].Branch)
}

func TestDecodeExternalRepo(t *testing.T) {
	p := decodeOne[*Project](t, `
apiVersion: gero/v1alpha2
kind: Project
name: example-sdk
spec:
  repo: {name: third-party/example-sdk, external: true}
`)

	assert.Equal(t, Repo{Name: "third-party/example-sdk", External: true, DefaultBranch: "main"}, p.Spec.Repo)
}

func TestDecodeRepoIsNotExternalByDefault(t *testing.T) {
	p := decodeOne[*Project](t, `
apiVersion: gero/v1alpha2
kind: Project
name: shell
spec:
  repo: {name: rancher/shell, owned: true}
`)

	assert.False(t, p.Spec.Repo.External)
}

func TestDecodeStream(t *testing.T) {
	s := decodeOne[*Stream](t, `
apiVersion: gero/v1alpha2
kind: Stream
name: shell
refs: {project: shell}
spec:
  scheme: semver
  tagPattern: "v*"
  tagExclude: ["*-hotfix*"]
  stages:
    - {name: dev, prerelease: true}
    - {name: stable}
  lines:
    - {line: v2.16, series: "0.9.x"}
    - {line: v2.15, series: "0.8.x"}
  universal: false
  requiresPrerelease: true
`)

	assert.Equal(t, Refs{"project": {"shell"}}, s.GetRefs())
	assert.Equal(t, StreamSpec{
		Scheme: "semver", TagPattern: "v*", TagExclude: []string{"*-hotfix*"},
		Stages:             []Stage{{Name: "dev", Prerelease: true}, {Name: "stable"}},
		Lines:              []StreamLine{{Line: "v2.16", Series: "0.9.x"}, {Line: "v2.15", Series: "0.8.x"}},
		RequiresPrerelease: true,
	}, s.Spec)
}

func TestDecodeArtifact(t *testing.T) {
	a := decodeOne[*Artifact](t, `
apiVersion: gero/v1alpha2
kind: Artifact
name: rancher-server-image
refs: {stream: rancher}
spec:
  kind: image
  name: rancher/rancher-server
  lines: {since: v2.16}
  registries: [dockerhub, prime-staging]
`)

	assert.Equal(t, ArtifactSpec{
		Kind: "image", Name: "rancher/rancher-server", Lines: lines.Range{Since: "v2.16"},
		Registries: []string{"dockerhub", "prime-staging"},
	}, a.Spec)
}

func TestDecodeHolding(t *testing.T) {
	h := decodeOne[*Holding](t, `
apiVersion: gero/v1alpha2
kind: Holding
name: assets-pins-charts
refs: {project: rancher-assets, from: rancher-charts}
spec:
  at:
    file: lock.yaml
    format: yaml
    key: '.chart-versions."{line|trimprefix:v}".upstream-refs.{stage|replace:stable=prod}.charts.commit'
  holds: {kind: commit, stages: [dev, stable], lines: {until: v2.15}, accept: all}
`)

	assert.Equal(t, Refs{"project": {"rancher-assets"}, "from": {"rancher-charts"}}, h.GetRefs())
	assert.Equal(t, At{
		File: "lock.yaml", Format: "yaml",
		Key: `.chart-versions."{line|trimprefix:v}".upstream-refs.{stage|replace:stable=prod}.charts.commit`,
	}, h.Spec.At)
	assert.Equal(t, Holds{
		Kind: "commit", Stages: []string{"dev", "stable"}, Lines: lines.Range{Until: "v2.15"}, Accept: "all",
	}, h.Spec.Holds)
}

func TestDecodeEdit(t *testing.T) {
	e := decodeOne[*Edit](t, `
apiVersion: gero/v1alpha2
kind: Edit
name: rancher-build-yaml-steps
refs: {holding: [rancher-holds-shell, rancher-holds-assets]}
spec:
  preEdit:
    - {run: scripts/prepare}
  postEdit:
    - {run: scripts/generate-buildconfig, since: v2.16}
    - {run: [go, generate, ./pkg/...], until: v2.15, from: consumer}
  strategy: chartsPackageBump
  with: {package: rancher-backup}
`)

	assert.Equal(t, Refs{"holding": {"rancher-holds-shell", "rancher-holds-assets"}}, e.GetRefs())
	assert.Equal(t, EditSpec{
		PreEdit: []Step{{Run: Command{"scripts/prepare"}}},
		PostEdit: []Step{
			{Run: Command{"scripts/generate-buildconfig"}, Since: "v2.16"},
			{Run: Command{"go", "generate", "./pkg/..."}, Until: "v2.15", From: "consumer"},
		},
		Strategy: "chartsPackageBump",
		With:     map[string]string{"package": "rancher-backup"},
	}, e.Spec)
}

func TestDecodeRancherLines(t *testing.T) {
	l := decodeOne[*RancherLines](t, `
apiVersion: gero/v1alpha2
kind: RancherLines
name: rancher
spec:
  lines:
    - id: v2.17
      kubernetes: {min: "1.36", max: "1.38"}
    - id: v2.15
      phase: eom
      dates: {ga: 2026-08-27, eom: 2027-02-27, eol: 2028-02-27}
      kubernetes: {min: "1.34", max: "1.36"}
`)

	require.Len(t, l.Spec.Lines, 2)
	assert.Equal(t, lines.Line{ID: "v2.17", Kubernetes: lines.KubernetesRange{Min: "1.36", Max: "1.38"}}, l.Spec.Lines[0])
	got := l.Spec.Lines[1]
	assert.Equal(t, "v2.15", got.ID)
	assert.Equal(t, "eom", got.Phase)
	assert.Equal(t, time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC), got.Dates.GA)
	assert.Equal(t, time.Date(2027, 2, 27, 0, 0, 0, 0, time.UTC), got.Dates.EOM)
	assert.Equal(t, time.Date(2028, 2, 27, 0, 0, 0, 0, time.UTC), got.Dates.EOL)
	assert.Equal(t, lines.KubernetesRange{Min: "1.34", Max: "1.36"}, got.Kubernetes)
}

func TestDecodeSeveralDocuments(t *testing.T) {
	objs := decodeAll(t, `
apiVersion: gero/v1alpha2
kind: Project
name: a
spec: {repo: {name: acme/a}}
---
apiVersion: gero/v1alpha2
kind: Stream
name: a
refs: {project: a}
spec: {universal: true}
---
apiVersion: gero/v1alpha2
kind: Artifact
name: a-image
refs: {stream: a}
spec: {kind: image, name: acme/a}
`)

	require.Len(t, objs, 3)
	assert.Equal(t, []string{KindProject, KindStream, KindArtifact}, []string{objs[0].GetKind(), objs[1].GetKind(), objs[2].GetKind()})
	assert.Equal(t, []string{"a", "a", "a-image"}, []string{objs[0].GetName(), objs[1].GetName(), objs[2].GetName()})
}

func TestDecodeEmptyInput(t *testing.T) {
	objs, err := Decode(strings.NewReader(""), "empty.yaml")

	require.NoError(t, err)
	assert.Empty(t, objs)
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name, src string
		want      []string
	}{
		{"an apiVersion of the old shape", "apiVersion: gero/v1alpha1\nkind: Project\nname: a\n", []string{"apiVersion", "gero/v1alpha1", "gero/v1alpha2"}},
		{"no apiVersion", "kind: Project\nname: a\n", []string{"apiVersion"}},
		{"an unknown kind", "apiVersion: gero/v1alpha2\nkind: Widget\nname: a\n", []string{"kind", "Widget"}},
		{"no kind", "apiVersion: gero/v1alpha2\nname: a\n", []string{"kind"}},
		{"no name", "apiVersion: gero/v1alpha2\nkind: Project\n", []string{"name is required"}},
		{"an unknown top level field", "apiVersion: gero/v1alpha2\nkind: Project\nname: a\nmetadata: {}\n", []string{"metadata"}},
		{"an unknown spec field", "apiVersion: gero/v1alpha2\nkind: Stream\nname: a\nspec:\n  bogus: 1\n", []string{"bogus"}},
		{"a ref that is not a name or a list", "apiVersion: gero/v1alpha2\nkind: Stream\nname: a\nrefs:\n  project: {x: y}\n", []string{"project", "name"}},
		{"a branch that is a list", "apiVersion: gero/v1alpha2\nkind: Project\nname: a\nspec:\n  branches:\n    lines:\n      - {line: v2.16, branch: [a, b]}\n", []string{"branch"}},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(tt.src), "shell.yaml")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "shell.yaml: document 1")
			for _, w := range tt.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

func TestDecodeReportsEveryBadDocument(t *testing.T) {
	objs, err := Decode(strings.NewReader(`
apiVersion: gero/v1alpha2
kind: Widget
name: first
---
apiVersion: gero/v1alpha2
kind: Project
name: good
spec: {repo: {name: acme/good}}
---
apiVersion: gero/v1alpha2
kind: Project
`), "mixed.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mixed.yaml: document 1")
	assert.Contains(t, err.Error(), "mixed.yaml: document 3")
	require.Len(t, objs, 1, "the good document is still returned")
	assert.Equal(t, "good", objs[0].GetName())
}
