package config

// AcceptLines says how a release of the producer maps to this project's
// branches: the accept.lines written, else universal when the producer is a
// universal project (it lists no release lines, so its releases belong to no
// line and go to every branch of this one), and all otherwise (a release goes
// to this project's branch for the producer's line of that release). It is
// what to act on; the field stays as written, since an omitted value and an
// explicit "all" are different once the producer is universal.
func (s Source) AcceptLines(producer Project) string {
	return ""
}
