package ship

// Marker is the hidden block in the body of every PR gero ship makes. It is how
// a later run, from this machine or from CI, recognises a PR as the same bump,
// whatever the branch is called. Its fields are additive: a reader ignores the
// ones it does not know, and a block of a format version it does not know is
// not a marker.
type Marker struct {
	Source   string // the producer's owner/name
	Consumer string // the consumer's owner/name
	Target   string // the consumer's branch
	Version  string // the producer's tag
}

// String renders the marker as an HTML comment, which GitHub does not show.
func (m Marker) String() string {
	return ""
}

// ParseMarker finds a marker in a PR body. ok is false when there is none, when
// it is of an unknown format version, or when a field is missing.
func ParseMarker(body string) (m Marker, ok bool) {
	return Marker{}, false
}

// SameBump reports whether two markers are the same bump at possibly different
// versions: the same source, consumer and target. That is the key two PRs
// overlap on, and a newer version supersedes an older one.
func (m Marker) SameBump(o Marker) bool {
	return false
}
