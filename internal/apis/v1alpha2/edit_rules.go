package v1alpha2

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/platform/config"
	"github.com/rancher/orbs-automation/internal/platform/templates"
)

// Where the program of a step comes from.
const (
	FromConsumer = "consumer"
	FromProducer = "producer"
	FromCentral  = "central"
)

// StrategyGoBump is the strategy that tidies the go.mod files its holdings write.
const StrategyGoBump = "goBump"

// strategyName is what the name of a strategy looks like: a camelCase word.
var strategyName = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

// SetDefaults does nothing: an Edit has no default.
func (e *Edit) SetDefaults() {}

// Validate checks what an Edit can alone: that it does something, that values
// go with a strategy, and each step's program and range of lines. That the
// strategy is known, and that its values are the ones it declares, is for the
// code that runs it.
func (e *Edit) Validate(_ config.Context, r *config.Report) {
	s := e.Spec
	if len(s.PreEdit) == 0 && len(s.PostEdit) == 0 && s.Strategy == "" {
		r.Errorf("spec", "does nothing: give preEdit, postEdit or strategy")
	}
	if s.Strategy != "" && !strategyName.MatchString(s.Strategy) {
		r.Errorf("spec.strategy", "%q is not a name: write it like chartsPackageBump", s.Strategy)
	}
	if len(s.With) > 0 && s.Strategy == "" {
		r.Errorf("spec.with", "values need a strategy to declare them")
	}
	checkSteps(r, "spec.preEdit", s.PreEdit)
	checkSteps(r, "spec.postEdit", s.PostEdit)
	checkEnv(r, s.Env)
}

// envName is what the name of an environment variable looks like.
var envName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// checkEnv checks the env of an Edit: each name is an environment variable of the
// shape programs read and not one of the engine's own, and each value is a template
// over the variables a target gives.
func checkEnv(r *config.Report, env map[string]string) {
	names := make([]string, 0, len(env))
	for name := range env {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		at := "spec.env." + name
		switch {
		case !envName.MatchString(name):
			r.Errorf(at, "%q is not the name of an environment variable: write it like GOTOOLCHAIN", name)
		case strings.HasPrefix(name, "GERO_"):
			r.Errorf(at, "%q is for the engine: names that start with GERO_ are its own", name)
		}
		if err := templates.Validate(env[name], templates.EnvVars); err != nil {
			r.Errorf(at, "%v", err)
		}
	}
}

// checkSteps checks each step's program and range of lines.
func checkSteps(r *config.Report, path string, steps []Step) {
	for i, st := range steps {
		at := fmt.Sprintf("%s[%d]", path, i)
		switch {
		case len(st.Run) == 0:
			r.Errorf(at+".run", "required: the program to run")
		case st.Run[0] == "":
			r.Errorf(at+".run", "the program is empty")
		}
		if st.From != "" {
			oneOf(r, at+".from", st.From, FromConsumer, FromProducer, FromCentral)
		}
		checkRange(r, at, lines.Range{Since: st.Since, Until: st.Until})
	}
}
