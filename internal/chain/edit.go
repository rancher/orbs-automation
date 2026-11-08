package chain

import "github.com/rancher/orbs-automation/internal/apis/v1alpha2"

// Edit is the extra steps of shipping into the holdings it follows.
type Edit struct {
	Doc *v1alpha2.Edit
}

// Name is the name of the document.
func (e *Edit) Name() string { return "" }

// Holdings returns the holdings the edit follows.
func (e *Edit) Holdings() []*Holding { return nil }

// Strategy is the named rule that needs code, or empty.
func (e *Edit) Strategy() string { return "" }

// With is the strategy's own values.
func (e *Edit) With() map[string]string { return nil }

// Steps returns the steps to run before and after writing, for a line: those
// whose since and until hold it.
func (e *Edit) Steps(_ string) (pre, post []v1alpha2.Step) { return nil, nil }
