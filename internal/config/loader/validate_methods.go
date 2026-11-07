package loader

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
)

// methods checks the structure of a project's update methods: each has a name
// and a strategy, offered params have unique keys, and steps name real lines. What
// depends on the strategy itself (that it exists, that its `with` values fit it,
// that the method offers the inputs it needs) is checked by the update
// dispatcher's engine, which has the strategies.
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
		if m.Strategy == "" {
			v.add("%s: strategy is required", at)
		}
		v.oneOf(at, "accept.when", m.Accept.When, config.AcceptStable, config.AcceptAll)
		offered := map[string]bool{}
		for j, ap := range m.AllowedParams {
			pat := fmt.Sprintf("%s: allowedParams[%d]", at, j)
			switch {
			case ap.Key == "":
				v.add("%s: key is required", pat)
				continue
			case offered[ap.Key]:
				v.add("%s: param %q is listed twice", pat, ap.Key)
			}
			offered[ap.Key] = true
			if _, fixed := m.With[ap.Key]; fixed {
				v.add("%s: %q is both fixed under with and offered to producers", pat, ap.Key)
			}
		}
		v.steps(at, "preEdit", m.PreEdit, false, false, phases, haveLines)
		v.steps(at, "postEdit", m.PostEdit, false, false, phases, haveLines)
	}
}

// edges checks the structure of a project's downstream edges: each names a
// consumer that exists and one of its methods, lines that mean something, and
// steps with real lines and a program the producer may use. The params an edge
// passes are checked against the method by the update dispatcher.
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
			if _, ok := consumer.Method(e.Method); !ok {
				names := make([]string, len(consumer.UpdateMethods))
				for k, m := range consumer.UpdateMethods {
					names[k] = m.Name
				}
				v.add("%s: method %q is not a method of %s: it has %s", at, e.Method, consumer.Name, strings.Join(names, ", "))
			}
		}
		if e.Lines != "" {
			v.oneOf(at, "lines", e.Lines, config.AcceptLinesAll, config.AcceptLinesUniversal)
		}
		if e.Stage != "" {
			v.stage(at, p, e.Stage)
		}
		v.steps(at, "preEdit", e.PreEdit, true, p.Repo.Owned, phases, haveLines)
		v.steps(at, "postEdit", e.PostEdit, true, p.Repo.Owned, phases, haveLines)
	}
}

// stage checks the stage of an observed edge: only an edge from an unowned project
// is observed, and the stage is one its producer's tag scheme gives. For an owned
// producer a stage would mean the consumer's stage the edge writes, which is not
// built yet.
func (v *validator) stage(at string, producer config.Project, stage string) {
	stages := config.TagSchemeStages(orDefault(producer.Release.TagScheme, DefaultTagScheme))
	switch {
	case producer.Repo.Owned:
		v.add("%s: stage is only for an edge from a project that is not owned, whose edges are observed", at)
	case len(stages) == 0:
		v.add("%s: stage needs a producer whose release.tagScheme gives stages, and %s's does not", at, producer.Name)
	default:
		v.oneOf(at, "stage", stage, stages...)
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
