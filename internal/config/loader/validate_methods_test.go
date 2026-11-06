package loader

import (
	"testing"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withMethods is valid() with project a (the consumer, which has release lines)
// offering a method and project b (the producer, which is owned) shipping to it.
func withMethods() config.Config {
	c := valid()
	c.Projects[0].UpdateMethods = []config.UpdateMethod{{
		Name:          "build-yaml",
		Strategy:      "yamlBump",
		With:          map[string]string{"file": "build.yaml"},
		AllowedParams: []config.AllowedParam{{Key: "updateKey", Description: "the key"}},
		PostEdit: []config.Step{
			{Run: config.Command{"scripts/generate"}, Since: "v2.16"},
			{Run: config.Command{"go", "generate"}, Until: "v2.15"},
		},
	}}
	c.Projects[1].Repo.Owned = true
	c.Projects[1].Downstream = []config.Edge{{To: "a", Method: "build-yaml", Params: map[string]string{"updateKey": "k"}}}
	return c
}

func TestValidateAcceptsMethodsAndEdges(t *testing.T) {
	assert.NoError(t, Validate(WithDefaults(withMethods())))
}

func TestValidateMethodAndEdgeProblems(t *testing.T) {
	method := func(c *config.Config) *config.UpdateMethod { return &c.Projects[0].UpdateMethods[0] }
	edge := func(c *config.Config) *config.Edge { return &c.Projects[1].Downstream[0] }
	tests := []struct {
		name   string
		mutate func(*config.Config)
		want   []string
	}{
		{"no strategy", func(c *config.Config) { method(c).Strategy = "" }, []string{"updateMethods[0]", "strategy is required"}},
		{"no name", func(c *config.Config) { method(c).Name = "" }, []string{"updateMethods[0]", "name is required"}},
		{"duplicate name", func(c *config.Config) {
			c.Projects[0].UpdateMethods = append(c.Projects[0].UpdateMethods, *method(c))
		}, []string{"updateMethods[1]", "listed twice"}},
		{"an allowed param with no key", func(c *config.Config) { method(c).AllowedParams[0].Key = "" }, []string{"allowedParams[0]", "key is required"}},
		{"an allowed param listed twice", func(c *config.Config) {
			method(c).AllowedParams = append(method(c).AllowedParams, method(c).AllowedParams[0])
		}, []string{"allowedParams[1]", "listed twice"}},
		{"a param both fixed and allowed", func(c *config.Config) {
			method(c).With["updateKey"] = "k"
		}, []string{"updateKey", "both"}},
		{"an unknown accept.when", func(c *config.Config) { method(c).Accept.When = "rc" }, []string{"accept.when", `"rc"`}},
		{"a step with no command", func(c *config.Config) { method(c).PostEdit[0].Run = nil }, []string{"postEdit[0]", "run"}},
		{"a step since an unknown line", func(c *config.Config) { method(c).PostEdit[0].Since = "v9.9" }, []string{"postEdit[0]", "since", "v9.9"}},
		{"a step until an unknown line", func(c *config.Config) { method(c).PostEdit[1].Until = "v9.9" }, []string{"postEdit[1]", "until", "v9.9"}},
		{"a step since after until", func(c *config.Config) {
			method(c).PostEdit[0].Since, method(c).PostEdit[0].Until = "v2.16", "v2.15"
		}, []string{"postEdit[0]", "since", "after until"}},
		{"from on a method's step", func(c *config.Config) { method(c).PostEdit[0].From = "central" }, []string{"postEdit[0]", "from"}},

		{"an edge to no project", func(c *config.Config) { edge(c).To = "nobody" }, []string{"downstream[0]", "to", `"nobody"`}},
		{"an edge to itself", func(c *config.Config) { edge(c).To = "b" }, []string{"downstream[0]", "itself"}},
		{"an edge to a method the consumer lacks", func(c *config.Config) { edge(c).Method = "missing" }, []string{"downstream[0]", "method", "missing", "build-yaml"}},
		{"unknown lines", func(c *config.Config) { edge(c).Lines = "some" }, []string{"downstream[0]", "lines", `"some"`}},
		{"an edge step from nowhere", func(c *config.Config) {
			edge(c).PostEdit = []config.Step{{Run: config.Command{"x"}, From: "nowhere"}}
		}, []string{"downstream[0]", "postEdit[0]", "from", `"nowhere"`}},
		{"an edge step from the producer when it is not owned", func(c *config.Config) {
			c.Projects[1].Repo.Owned = false
			edge(c).PostEdit = []config.Step{{Run: config.Command{"x"}, From: "producer"}}
		}, []string{"postEdit[0]", "producer", "owned"}},
		{"an edge step since an unknown line", func(c *config.Config) {
			edge(c).PostEdit = []config.Step{{Run: config.Command{"x"}, Since: "v9.9"}}
		}, []string{"downstream[0]", "postEdit[0]", "since", "v9.9"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := withMethods()
			tt.mutate(&c)

			err := Validate(WithDefaults(c))

			require.Error(t, err)
			for _, w := range tt.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

func TestValidateEdgeStepsFromTheProducerOrTheCentralRepo(t *testing.T) {
	c := withMethods()
	c.Projects[1].Downstream[0].PostEdit = []config.Step{
		{Run: config.Command{"hack/bump.sh"}, From: "producer"},
		{Run: config.Command{"sync.sh"}, From: "central"},
		{Run: config.Command{"scripts/fix"}, From: "consumer", Since: "v2.16"},
	}
	assert.NoError(t, Validate(WithDefaults(c)), "the producer is owned")
}

func TestDefaultsGiveAMethodStableAccept(t *testing.T) {
	got := WithDefaults(withMethods()).Projects[0].UpdateMethods[0]
	assert.Equal(t, config.AcceptStable, got.Accept.When)
}
