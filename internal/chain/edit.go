package chain

import (
	"github.com/rancher/orbs-automation/internal/apis/lines"
	"github.com/rancher/orbs-automation/internal/apis/v1alpha2"
)

// Edit is the extra steps of shipping into the holdings it follows.
type Edit struct {
	Doc      *v1alpha2.Edit
	holdings []*Holding
}

// Name is the name of the document.
func (e *Edit) Name() string { return e.Doc.Name }

// Holdings returns the holdings the edit follows.
func (e *Edit) Holdings() []*Holding { return e.holdings }

// Strategy is the named rule that needs code, or empty.
func (e *Edit) Strategy() string { return e.Doc.Spec.Strategy }

// With is the strategy's own values.
func (e *Edit) With() map[string]string { return e.Doc.Spec.With }

// Steps returns the steps to run before and after writing, for a line: those
// whose since and until hold it.
func (e *Edit) Steps(line string) (pre, post []v1alpha2.Step) {
	return stepsFor(e.Doc.Spec.PreEdit, line), stepsFor(e.Doc.Spec.PostEdit, line)
}

func stepsFor(steps []v1alpha2.Step, line string) []v1alpha2.Step {
	var out []v1alpha2.Step
	for _, st := range steps {
		if (lines.Range{Since: st.Since, Until: st.Until}).Contains(line) {
			out = append(out, st)
		}
	}
	return out
}
