package v1alpha2

import (
	"strings"
	"testing"
	"time"

	"github.com/rancher/orbs-automation/internal/apis/meta"
	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var rulesCtx = config.Context{Now: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)}

// decodeReport decodes one document of src and returns its object and the
// findings, so a rule is tested through the same path a config takes.
func decodeReport(t *testing.T, src string) ([]Object, config.Report) {
	t.Helper()
	s := meta.NewScheme()
	AddToScheme(s)
	return s.DecodeReport(strings.NewReader(src), "t.yaml", rulesCtx)
}

func projectDoc(spec string) string {
	return "apiVersion: gero/v1alpha2\nkind: Project\nname: p\nspec:\n" + spec
}

func loadProject(t *testing.T, spec string) (*Project, config.Report) {
	t.Helper()
	objs, rep := decodeReport(t, projectDoc(spec))
	if len(objs) == 0 {
		return nil, rep
	}
	require.Len(t, objs, 1)
	return objs[0].(*Project), rep
}

// hasError reports whether an error finding has the path and a message that
// contains want.
func hasError(rep config.Report, path, want string) bool {
	for _, f := range rep.Findings {
		if f.Severity == config.Error && f.Path == path && strings.Contains(f.Msg, want) {
			return true
		}
	}
	return false
}

func TestProjectDefaults(t *testing.T) {
	p, rep := loadProject(t, "  repo: {name: rancher/shell}\n")

	require.NoError(t, rep.Err())
	require.NotNil(t, p)
	assert.Equal(t, "main", p.Spec.Repo.DefaultBranch)
	assert.Equal(t, "single", p.Spec.Branches.Layout)
}

func TestProjectDefaultsDoNotOverwriteWhatWasWritten(t *testing.T) {
	p, rep := loadProject(t, "  repo: {name: rancher/charts, defaultBranch: dev-v2.16}\n  branches: {layout: perLine}\n")

	require.NoError(t, rep.Err())
	require.NotNil(t, p)
	assert.Equal(t, "dev-v2.16", p.Spec.Repo.DefaultBranch)
	assert.Equal(t, "perLine", p.Spec.Branches.Layout)
}

func TestAValidProjectHasNoFindings(t *testing.T) {
	_, rep := loadProject(t, `  repo: {name: rancher/shell, managedBy: shared}
  finalDestination: true
  pullRequests:
    title: "[{branch}] Update {source} to {version}"
    branchPrefix: "bot/"
    labels: [status/auto-created]
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

	assert.Empty(t, rep.Findings)
}

func TestProjectRules(t *testing.T) {
	tests := []struct {
		name, spec, path, want string
	}{
		{"no repo name", "  repo: {defaultBranch: main}\n", "spec.repo.name", "required"},
		{"a repo name that is not owner and name", "  repo: {name: shell}\n", "spec.repo.name", "owner/name"},
		{"a repo name with three parts", "  repo: {name: a/b/c}\n", "spec.repo.name", "owner/name"},
		{"managedBy that is not a slug", "  repo: {name: a/b, managedBy: Release Team}\n", "spec.repo.managedBy", "lowercase slug"},
		{"external and managed", "  repo: {name: a/b, external: true, managedBy: orbs}\n", "spec.repo.external", "no managing team"},
		{"pushTo that is neither", "  repo: {name: a/b}\n  pullRequests: {pushTo: origin}\n", "spec.pullRequests.pushTo", "fork or upstream"},
		{"a title with an unknown placeholder", "  repo: {name: a/b}\n  pullRequests: {title: \"{nope}\"}\n", "spec.pullRequests.title", "unknown placeholder {nope}"},
		{"a branch prefix that uses the key, which is appended to it", "  repo: {name: a/b}\n  pullRequests: {branchPrefix: \"{key}\"}\n", "spec.pullRequests.branchPrefix", "unknown placeholder {key}"},
		{"a branch prefix with a malformed placeholder", "  repo: {name: a/b}\n  pullRequests: {branchPrefix: \"{a\"}\n", "spec.pullRequests.branchPrefix", "unclosed"},
		{"a layout that is neither", "  repo: {name: a/b}\n  branches: {layout: weekly}\n", "spec.branches.layout", "single or perLine"},
		{"lines with the single layout", "  repo: {name: a/b}\n  branches:\n    layout: single\n    lines: [{line: v2.16}]\n", "spec.branches.lines", "single"},
		{"a stage that is empty", "  repo: {name: a/b}\n  branches: {layout: perLine, stages: [dev, \"\"]}\n", "spec.branches.stages[1]", "empty"},
		{"a stage listed twice", "  repo: {name: a/b}\n  branches: {layout: perLine, stages: [dev, dev]}\n", "spec.branches.stages[1]", "twice"},
		{"a line with no id", "  repo: {name: a/b}\n  branches:\n    layout: perLine\n    lines: [{branch: x}]\n", "spec.branches.lines[0].line", "required"},
		{"a line that is not a line id", "  repo: {name: a/b}\n  branches:\n    layout: perLine\n    lines: [{line: 2.16, branch: x}]\n", "spec.branches.lines[0].line", "v2.16"},
		{"a line listed twice", "  repo: {name: a/b}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16, branch: x}, {line: v2.16, branch: y}]\n", "spec.branches.lines[1].line", "twice"},
		{"a branch map with no stages declared", "  repo: {name: a/b}\n  branches:\n    layout: perLine\n    lines: [{line: v2.16, branch: {dev: d}}]\n", "spec.branches.lines[0].branch", "stages"},
		{"a branch map with a stage that is not declared", "  repo: {name: a/b}\n  branches:\n    layout: perLine\n    stages: [dev, stable]\n    lines: [{line: v2.16, branch: {dev: d, prod: p}}]\n", "spec.branches.lines[0].branch.prod", "stage"},
		{"an alias that is not a slug", "  repo: {name: a/b}\n  aliases: [Rk]\n", "spec.aliases[0]", "lowercase slug"},
		{"an alias with a slash", "  repo: {name: a/b}\n  aliases: [a/b]\n", "spec.aliases[0]", "lowercase slug"},
		{"an alias that is empty", "  repo: {name: a/b}\n  aliases: [\"\"]\n", "spec.aliases[0]", "lowercase slug"},
		{"an alias listed twice", "  repo: {name: a/b}\n  aliases: [rk, rk]\n", "spec.aliases[1]", "twice"},
		{"an alias that is the name of the project itself", "  repo: {name: a/b}\n  aliases: [p]\n", "spec.aliases[0]", "name"},
		{"two lines on the default branch", "  repo: {name: a/b}\n  branches:\n    layout: perLine\n    lines: [{line: v2.17}, {line: v2.16}]\n", "spec.branches.lines", "default branch"},
	}
	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, rep := loadProject(t, tt.spec)

			assert.Nil(t, p, "a project that breaks a rule is dropped")
			assert.True(t, hasError(rep, tt.path, tt.want), "want an error at %s containing %q, got %v", tt.path, tt.want, rep.Findings)
		})
	}
}

func TestProjectAliasesAreRead(t *testing.T) {
	p, rep := loadProject(t, "  repo: {name: a/b}\n  aliases: [rk, kubectl]\n")

	require.Empty(t, rep.Findings)
	require.NotNil(t, p)
	assert.Equal(t, []string{"rk", "kubectl"}, p.Spec.Aliases)
}

func TestProjectsReportEveryProblemTogether(t *testing.T) {
	_, rep := loadProject(t, "  repo: {name: shell, managedBy: Bad Team}\n  branches: {layout: weekly}\n")

	assert.Equal(t, 3, joinedCount(rep.Err()))
}

func joinedCount(err error) int {
	for e := err; e != nil; e = unwrapOnce(e) {
		if joined, ok := e.(interface{ Unwrap() []error }); ok {
			return len(joined.Unwrap())
		}
	}
	return 1
}

func unwrapOnce(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func TestARepoNameHasNoSpaces(t *testing.T) {
	p, rep := loadProject(t, "  repo: {name: \"acme/a b\"}\n")

	assert.Nil(t, p)
	assert.True(t, hasError(rep, "spec.repo.name", "space"), "%v", rep.Findings)
}

func TestAPrefixThatRepeatsTheKeyIsAWarningAndStillValid(t *testing.T) {
	rep := validateSet(t, doc("Project", "p", "", projectSpec+"  pullRequests: {branchPrefix: \"bot/{source}-{tag}-\"}\n"))

	require.NoError(t, rep.Err())
	got := warningsOf(rep, RuleBranchPrefixRedundantVar)
	require.Len(t, got, 2, "one for each variable")
	assert.Equal(t, "spec.pullRequests.branchPrefix", got[0].Path)
	assert.Contains(t, got[0].Msg, "{source}")
	assert.Contains(t, got[1].Msg, "{tag}")
}

func TestAPrefixThatSaysNothingTwiceHasNoWarning(t *testing.T) {
	_, rep := loadProject(t, "  repo: {name: a/b}\n  pullRequests: {branchPrefix: \"auto/\"}\n")

	assert.Empty(t, rep.Findings)
}

func TestAnAcknowledgementSilencesTheRedundantPrefixWarning(t *testing.T) {
	spec := projectSpec + "  pullRequests: {branchPrefix: \"bot/{source}-\"}\n"
	rep := validateSet(t, docAck("Project", "p", "", "  - {rule: branch-prefix-redundant-var, reason: the old names stay until the PRs of them are gone}\n", spec))

	assert.NoError(t, rep.Err())
	assert.Empty(t, rep.Warnings())
	assert.Len(t, rep.Acknowledged(), 1)
}
