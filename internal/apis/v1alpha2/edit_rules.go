package v1alpha2

import (
	"fmt"
	"regexp"

	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/platform/config"
)

// Where the program of a step comes from.
const (
	FromConsumer = "consumer"
	FromProducer = "producer"
	FromCentral  = "central"
)

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
