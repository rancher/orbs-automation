package config

// IsRequired reports whether a producer must pass the param: when it says so,
// else unless the param has a default.
func (p AllowedParam) IsRequired() bool {
	return false
}

// AppliesTo reports whether the step runs for a target branch in the consumer
// line: when the line is at or after Since and at or before Until, either of
// which may be omitted. A line that cannot be read as a number applies, so a
// step is never skipped on a guess.
func (s Step) AppliesTo(line string) bool {
	return false
}

// AcceptLines says how the edge's releases map to the consumer's branches: the
// lines written, else universal when the producer lists no release lines of its
// own, and all otherwise. It is what to act on; the field stays as written.
func (e Edge) AcceptLines(producer Project) string {
	return ""
}

// LineOfBranch returns the line a branch of the project is the branch of, the
// inverse of Branch, or "" when no line has it. The development line is on the
// default branch.
func (p Project) LineOfBranch(branch string) string {
	return ""
}

// Method returns the project's update method of a name.
func (p Project) Method(name string) (UpdateMethod, bool) {
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
	return nil
}
