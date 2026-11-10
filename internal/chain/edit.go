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

// Env is the environment the edit's steps and strategy run in: names to templates
// that are rendered for each target.
func (e *Edit) Env() map[string]string { return e.Doc.Spec.Env }

// Strategy is the named rule that needs code, or empty.
func (e *Edit) Strategy() string { return e.Doc.Spec.Strategy }

// With is the strategy's own values.
func (e *Edit) With() map[string]string { return e.Doc.Spec.With }

// Step is a program run around the write of an edit, optionally only for a range
// of lines.
type Step struct {
	Run   []string // the program or script path, then its arguments
	From  string   // where the program comes from: consumer, producer or central; "" is consumer
	Since string
	Until string
}

// FromProducer reports whether the step's program comes from the producer's repo.
func (s Step) FromProducer() bool { return s.From == v1alpha2.FromProducer }

// AppliesTo reports whether the step runs for a line.
func (s Step) AppliesTo(line string) bool {
	return (lines.Range{Since: s.Since, Until: s.Until}).Contains(line)
}

// PreEdit returns every step to run before writing, in order, whatever its lines.
func (e *Edit) PreEdit() []Step { return stepsOf(e.Doc.Spec.PreEdit) }

// PostEdit returns every step to run after writing, in order, whatever its lines.
func (e *Edit) PostEdit() []Step { return stepsOf(e.Doc.Spec.PostEdit) }

// Steps returns the steps to run before and after writing, for a line: those
// whose since and until hold it.
func (e *Edit) Steps(line string) (pre, post []Step) {
	return stepsFor(e.PreEdit(), line), stepsFor(e.PostEdit(), line)
}

func stepsOf(steps []v1alpha2.Step) []Step {
	out := make([]Step, len(steps))
	for i, st := range steps {
		out[i] = Step{Run: st.Run, From: st.From, Since: st.Since, Until: st.Until}
	}
	return out
}

func stepsFor(steps []Step, line string) []Step {
	var out []Step
	for _, st := range steps {
		if st.AppliesTo(line) {
			out = append(out, st)
		}
	}
	return out
}
