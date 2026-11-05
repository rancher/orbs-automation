package config

import "strings"

// IsRequired reports whether a producer must pass the param: when it says so,
// else unless the param has a default.
func (p AllowedParam) IsRequired() bool {
	if p.Required != nil {
		return *p.Required
	}
	return p.Default == ""
}

// AppliesTo reports whether the step runs for a target branch in the consumer
// line: when the line is at or after Since and at or before Until, either of
// which may be omitted. A line that cannot be read as a number applies, so a
// step is never skipped on a guess.
func (s Step) AppliesTo(line string) bool {
	return lineInRange(line, s.Since, s.Until)
}

// AcceptLines says how the edge's releases map to the consumer's branches: the
// lines written, else universal when the producer lists no release lines of its
// own, and all otherwise. It is what to act on; the field stays as written.
func (e Edge) AcceptLines(producer Project) string {
	switch {
	case e.Lines != "":
		return e.Lines
	case producer.IsUniversal():
		return AcceptLinesUniversal
	}
	return AcceptLinesAll
}

// LineOfBranch returns the line a branch of the project is the branch of, the
// inverse of Branch, or "" when no line has it. The development line is on the
// default branch.
func (p Project) LineOfBranch(branch string) string {
	for _, l := range p.Release.Lines {
		if b, _ := p.Branch(l); b == branch {
			return l.Line
		}
	}
	return ""
}

// Method returns the project's update method of a name.
func (p Project) Method(name string) (UpdateMethod, bool) {
	for _, m := range p.UpdateMethods {
		if m.Name == name {
			return m, true
		}
	}
	return UpdateMethod{}, false
}

// Link is one producer-to-consumer route: the edge, the consumer project and the
// method it names.
type Link struct {
	Edge     Edge
	Consumer Project
	Method   UpdateMethod
}

// Links returns the routes out of the producer, one for each downstream edge
// whose consumer and method exist, in config order.
func (c Config) Links(producer Project) []Link {
	var out []Link
	for _, e := range producer.Downstream {
		for _, p := range c.Projects {
			if p.Name != e.To && !strings.EqualFold(p.Repo.Name, e.To) {
				continue
			}
			if method, ok := p.Method(e.Method); ok {
				out = append(out, Link{Edge: e, Consumer: p, Method: method})
			}
			break
		}
	}
	return out
}
