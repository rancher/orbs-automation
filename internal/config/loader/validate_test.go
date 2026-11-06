package loader

import (
	"os"
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// valid returns a small config that passes Validate: project a takes updates
// from project b, and external x tracks b.
func valid() config.Config {
	return config.Config{
		Projects: []config.Project{
			{
				Name: "a",
				Repo: config.Repo{Name: "o/a", DefaultBranch: "main"},
				Release: config.Release{
					Branches:   "perLine",
					TagPattern: "v*",
					Lines: []config.ProjectLine{
						{Line: "v2.16"},
						{Line: "v2.15", Branch: "release/v2.15"},
					},
				},
			},
			{
				Name:    "b",
				Repo:    config.Repo{Name: "o/b", DefaultBranch: "main"},
				Release: config.Release{Branches: "single", TagPattern: "v*"},
			},
		},
		Externals: []config.External{{
			Name:          "x",
			Repo:          config.Repo{Name: "t/x", DefaultBranch: "main"},
			Relationships: []config.Relationship{{Project: "b", Action: "trackOnly"}},
		}},
		Lines: &config.RancherLines{Name: "rancher", Lines: []config.Line{
			{ID: "v2.16", Phase: "development"},
			{ID: "v2.15", Phase: "ga"},
		}},
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	assert.NoError(t, Validate(valid()))
}

func TestValidateAcceptsEmptyConfig(t *testing.T) {
	assert.NoError(t, Validate(config.Config{}))
}

func TestValidateProblems(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*config.Config)
		want   []string // every one must appear in the error
	}{
		{"repo name without owner", func(c *config.Config) { c.Projects[0].Repo.Name = "a" }, []string{`Project "a"`, "repo.name"}},
		{"repo name with trailing slash", func(c *config.Config) { c.Projects[0].Repo.Name = "o/" }, []string{"repo.name"}},
		{"repo name with extra path", func(c *config.Config) { c.Projects[0].Repo.Name = "a/b/c" }, []string{"repo.name"}},
		{"empty default branch", func(c *config.Config) { c.Projects[0].Repo.DefaultBranch = "" }, []string{"repo.defaultBranch"}},
		{"unknown release.branches", func(c *config.Config) { c.Projects[0].Release.Branches = "weekly" }, []string{"release.branches", `"weekly"`}},
		{"empty tag pattern", func(c *config.Config) { c.Projects[0].Release.TagPattern = "" }, []string{"release.tagPattern"}},
		{"line not in RancherLines", func(c *config.Config) { c.Projects[0].Release.Lines[1].Line = "v9.9" }, []string{"release.lines[1]", "v9.9"}},
		{"empty line", func(c *config.Config) { c.Projects[0].Release.Lines[1].Line = "" }, []string{"release.lines[1]", "line is required"}},
		{
			"line without branch outside development",
			func(c *config.Config) { c.Projects[0].Release.Lines[1].Branch = "" },
			[]string{"release.lines[1]", "v2.15", "has no branch"},
		},
		{
			"line listed twice",
			func(c *config.Config) {
				c.Projects[0].Release.Lines = append(c.Projects[0].Release.Lines, config.ProjectLine{Line: "v2.15", Branch: "x"})
			},
			[]string{"release.lines[2]", "listed twice"},
		},
		{"lines used without RancherLines", func(c *config.Config) { c.Lines = nil }, []string{"RancherLines"}},
		{
			"image without a name",
			func(c *config.Config) { c.Projects[0].Images = []config.Image{{Name: ""}} },
			[]string{"images[0]", "name is required"},
		},
		{
			"image name with a space",
			func(c *config.Config) { c.Projects[0].Images = []config.Image{{Name: "o/my image"}} },
			[]string{"images[0]", "name"},
		},
		{
			"image listed twice",
			func(c *config.Config) { c.Projects[0].Images = []config.Image{{Name: "o/a"}, {Name: "o/a"}} },
			[]string{"images[1]", "listed twice"},
		},
		{
			"unknown registry",
			func(c *config.Config) {
				c.Projects[0].Images = []config.Image{{Name: "o/a", Registries: []string{"dockerhub", "ecr"}}}
			},
			[]string{"images[0]", "registries[1]", `"ecr"`},
		},
		{
			"registry listed twice",
			func(c *config.Config) {
				c.Projects[0].Images = []config.Image{{Name: "o/a", Registries: []string{"dockerhub", "dockerhub"}}}
			},
			[]string{"images[0]", "registries[1]", "listed twice"},
		},
		{
			"since is not a known line",
			func(c *config.Config) { c.Projects[0].Images = []config.Image{{Name: "o/a", Since: "v9.9"}} },
			[]string{"images[0]", "since", `"v9.9"`},
		},
		{
			"since on a project with no lines",
			func(c *config.Config) {
				c.Projects[0].Release.Lines = nil
				c.Projects[0].Images = []config.Image{{Name: "o/a", Since: "v2.15"}}
			},
			[]string{"images[0]", "since", "release.lines"},
		},
		{
			"unknown placeholder in the title",
			func(c *config.Config) { c.Projects[0].PullRequests.Title = "[{branch}] Update {sourc} to {tag}" },
			[]string{"pullRequests.title", "sourc"},
		},
		{
			"malformed title",
			func(c *config.Config) { c.Projects[0].PullRequests.Title = "[{branch] Update" },
			[]string{"pullRequests.title"},
		},
		{
			"unknown placeholder in the branch prefix",
			func(c *config.Config) { c.Projects[0].PullRequests.BranchPrefix = "bot/{nope}-" },
			[]string{"pullRequests.branchPrefix", "nope"},
		},
		{"external repo name", func(c *config.Config) { c.Externals[0].Repo.Name = "x" }, []string{`External "x"`, "repo.name"}},
		{
			"relationship to unknown project",
			func(c *config.Config) { c.Externals[0].Relationships[0].Project = "nope" },
			[]string{"relationships[0]", `"nope"`},
		},
		{
			"unknown relationship action",
			func(c *config.Config) { c.Externals[0].Relationships[0].Action = "watch" },
			[]string{"relationships[0]", "action", `"watch"`},
		},
		{
			"two actions for one pair",
			func(c *config.Config) {
				c.Externals[0].Relationships = append(c.Externals[0].Relationships, config.Relationship{Project: "b", Action: "notify"})
			},
			[]string{"relationships[1]", "more than one"},
		},
		{"unknown phase", func(c *config.Config) { c.Lines.Lines[0].Phase = "beta" }, []string{"lines[0]", "phase", `"beta"`}},
		{"empty line id", func(c *config.Config) { c.Lines.Lines[0].ID = "" }, []string{"lines[0]", "id is required"}},
		{
			"duplicate line id",
			func(c *config.Config) { c.Lines.Lines[1].ID = "v2.16" },
			[]string{"lines[1]", "duplicate line"},
		},
		{"repo shared by two projects", func(c *config.Config) { c.Projects[1].Repo.Name = "o/a" }, []string{`Project "b"`, "also used by", `Project "a"`}},
		{"repo shared by a project and an external", func(c *config.Config) { c.Externals[0].Repo.Name = "O/B" }, []string{`External "x"`, "also used by", `Project "b"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.mutate(&c)

			err := Validate(c)

			require.Error(t, err)
			for _, w := range tt.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

func TestValidateAcceptsPullRequestTemplates(t *testing.T) {
	c := valid()
	c.Projects[0].PullRequests.Title = "[{branch}] Update {source} to {version} ({tag}) {{literal}}"
	c.Projects[0].PullRequests.BranchPrefix = "bot/{source}-"
	assert.NoError(t, Validate(c))
}

func TestValidateStrategyVariablesInThePRTitle(t *testing.T) {
	c := withMethods()
	c.Projects[0].PullRequests.Title = "[{branch}] {package} {chartVersion} {action}"
	m := &c.Projects[0].UpdateMethods[0]
	m.Strategy = "chartsPackageBump"
	m.With = map[string]string{"package": "rancher-backup"}
	m.AllowedParams = nil
	c.Projects[1].Downstream[0].Params = nil
	assert.NoError(t, Validate(WithDefaults(c)), "the strategy of one of its methods supplies those variables")

	m.Strategy = "yamlBump"
	m.With = map[string]string{"file": "build.yaml"}
	m.AllowedParams = []config.AllowedParam{{Key: "updateKey"}}
	c.Projects[1].Downstream[0].Params = map[string]string{"updateKey": "k"}
	err := Validate(WithDefaults(c))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "package", "no method supplies them")
}

func TestValidateAcceptsImages(t *testing.T) {
	c := valid()
	c.Projects[0].Images = []config.Image{
		{Name: "rancher/rancher-server", Since: "v2.15"},
		{Name: "rancher/rancher", Registries: []string{config.RegistryDockerHub, config.RegistryPrimeProduction}},
		{Name: "rancher/rancher-agent"},
	}
	assert.NoError(t, Validate(c))
}

func TestValidateReportsEveryProblem(t *testing.T) {
	c := valid()
	c.Projects[0].Repo.Name = "a"
	c.Projects[0].Release.Branches = "weekly"
	c.Externals[0].Relationships[0].Action = "watch"

	err := Validate(c)

	require.Error(t, err)
	for _, w := range []string{"repo.name", "release.branches", "action"} {
		assert.Contains(t, err.Error(), w)
	}
}

func TestValidateAcceptsLineWithoutBranchWhileInDevelopment(t *testing.T) {
	c := valid()
	require.Empty(t, c.Projects[0].Release.Lines[0].Branch)

	assert.NoError(t, Validate(c))
}

// The checked-in directories must validate once defaults are applied.
func TestValidateRepoConfigDirs(t *testing.T) {
	for _, dir := range []string{"../../../config", "../../../config-mock"} {
		t.Run(dir, func(t *testing.T) {
			c, err := Load(os.DirFS(dir))
			require.NoError(t, err)

			assert.NoError(t, Validate(WithDefaults(c)))
		})
	}
}
