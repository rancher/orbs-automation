package loader

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/strategyspec"
)

// methods checks the update methods of a project: each has a name, a known
// strategy, fixed values and offered params that the strategy knows, and steps
// with real lines.
func (v *validator) methods(where string, p config.Project, phases map[string]string, haveLines bool) {
	names := map[string]bool{}
	for i, m := range p.UpdateMethods {
		at := fmt.Sprintf("%s: updateMethods[%d]", where, i)
		switch {
		case m.Name == "":
			v.add("%s: name is required", at)
		case names[m.Name]:
			v.add("%s: method %q is listed twice", at, m.Name)
		}
		names[m.Name] = true
		v.oneOf(at, "accept.when", m.Accept.When, config.AcceptStable, config.AcceptAll)

		spec, ok := strategyspec.Lookup(m.Strategy)
		if !ok {
			v.add("%s: strategy %q is not known: use one of %s", at, m.Strategy, strings.Join(strategyspec.Names(), ", "))
		} else {
			v.methodParams(at, m, spec)
		}
		v.steps(at, "preEdit", m.PreEdit, false, false, phases, haveLines)
		v.steps(at, "postEdit", m.PostEdit, false, false, phases, haveLines)
	}
}

// methodParams checks the fixed values and the offered params of a method
// against what its strategy takes: only known names are fixed, offered params
// are named once and not fixed too, a consumer-only param is never offered, and
// every param the strategy needs is either fixed or offered.
func (v *validator) methodParams(at string, m config.UpdateMethod, spec strategyspec.Spec) {
	for key := range m.With {
		if _, ok := spec.Param(key); !ok {
			v.add("%s: with %q is not a param of %s", at, key, spec.Name)
		}
	}
	offered := map[string]bool{}
	for i, ap := range m.AllowedParams {
		pat := fmt.Sprintf("%s: allowedParams[%d]", at, i)
		switch {
		case ap.Key == "":
			v.add("%s: key is required", pat)
			continue
		case offered[ap.Key]:
			v.add("%s: param %q is listed twice", pat, ap.Key)
		}
		offered[ap.Key] = true
		if sp, ok := spec.Param(ap.Key); ok && sp.ConsumerOnly {
			v.add("%s: %q is consumer-only for %s, so a producer cannot pass it: fix it under with", pat, ap.Key, spec.Name)
		}
		if _, fixed := m.With[ap.Key]; fixed {
			v.add("%s: %q is both fixed under with and offered to producers", pat, ap.Key)
		}
	}
	for _, sp := range spec.Params {
		_, fixed := m.With[sp.Key]
		if !fixed && !offered[sp.Key] && sp.Required && sp.Default == "" {
			v.add("%s: %s needs the param %q, which is required: fix it under with or offer it under allowedParams", at, spec.Name, sp.Key)
		}
	}
}

// edges checks the downstream edges of a project: each names a consumer that
// exists, one of its methods, params the method allows with every required one
// present, lines that mean something, and steps with real lines and a program
// the producer may use.
func (v *validator) edges(where string, p config.Project, phases map[string]string, haveLines bool) {
	for i, e := range p.Downstream {
		at := fmt.Sprintf("%s: downstream[%d]", where, i)
		var consumer *config.Project
		for j := range v.projects {
			if c := &v.projects[j]; c.Name == e.To || strings.EqualFold(c.Repo.Name, e.To) {
				consumer = c
				break
			}
		}
		switch {
		case consumer == nil:
			v.add("%s: to %q is not a Project", at, e.To)
		case consumer.Name == p.Name:
			v.add("%s: to %q is the project itself", at, e.To)
		default:
			v.edgeMethod(at, e, *consumer)
		}
		if e.Lines != "" {
			v.oneOf(at, "lines", e.Lines, config.AcceptLinesAll, config.AcceptLinesUniversal)
		}
		v.steps(at, "preEdit", e.PreEdit, true, p.Repo.Owned, phases, haveLines)
		v.steps(at, "postEdit", e.PostEdit, true, p.Repo.Owned, phases, haveLines)
	}
}

// edgeMethod checks an edge against the method of the consumer it names.
func (v *validator) edgeMethod(at string, e config.Edge, consumer config.Project) {
	method, ok := consumer.Method(e.Method)
	if !ok {
		names := make([]string, len(consumer.UpdateMethods))
		for i, m := range consumer.UpdateMethods {
			names[i] = m.Name
		}
		v.add("%s: method %q is not a method of %s: it has %s", at, e.Method, consumer.Name, strings.Join(names, ", "))
		return
	}
	allowed := map[string]config.AllowedParam{}
	for _, ap := range method.AllowedParams {
		allowed[ap.Key] = ap
	}
	for key := range e.Params {
		if _, ok := allowed[key]; !ok {
			v.add("%s: params %q is not allowed by method %s of %s", at, key, method.Name, consumer.Name)
		}
	}
	for _, ap := range method.AllowedParams {
		if _, given := e.Params[ap.Key]; !given && ap.IsRequired() {
			v.add("%s: param %q is required by method %s of %s", at, ap.Key, method.Name, consumer.Name)
		}
	}
}

// steps checks a list of steps: each runs something, its range names real lines
// in order, and from is only on an edge's step and names a place the producer
// may use. A producer's own files are for an owned project only.
func (v *validator) steps(at, field string, steps []config.Step, edge, owned bool, phases map[string]string, haveLines bool) {
	froms := []string{"consumer", "producer", "central"}
	for i, s := range steps {
		sat := fmt.Sprintf("%s: %s[%d]", at, field, i)
		if len(s.Run) == 0 || s.Run[0] == "" {
			v.add("%s: run is required", sat)
		}
		for name, line := range map[string]string{"since": s.Since, "until": s.Until} {
			if line == "" {
				continue
			}
			if _, known := phases[line]; !haveLines || !known {
				v.add("%s: %s %q is not in RancherLines", sat, name, line)
			}
		}
		if order, ok := config.CompareLines(s.Since, s.Until); ok && order > 0 {
			v.add("%s: since %s is after until %s", sat, s.Since, s.Until)
		}
		switch {
		case s.From != "" && !edge:
			v.add("%s: from is only for a producer's step: a method's steps run what the consumer has", sat)
		case s.From != "" && !slices.Contains(froms, s.From):
			v.add("%s: from %q is not one of %s", sat, s.From, strings.Join(froms, ", "))
		case s.From == "producer" && !owned:
			v.add("%s: from producer needs the producer to be owned: true", sat)
		}
	}
}
