package ship

import "strings"

const markerOpen = "<!-- gero:ship v1"

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
	return markerOpen + "\nsource: " + m.Source + "\nconsumer: " + m.Consumer +
		"\ntarget: " + m.Target + "\nversion: " + m.Version + "\n-->"
}

// ParseMarker finds a marker in a PR body. ok is false when there is none, when
// it is of an unknown format version, or when a field is missing.
func ParseMarker(body string) (m Marker, ok bool) {
	start := strings.Index(body, markerOpen)
	if start < 0 {
		return Marker{}, false
	}
	block := body[start+len(markerOpen):]
	end := strings.Index(block, "-->")
	if end < 0 {
		return Marker{}, false
	}
	for _, line := range strings.Split(block[:end], "\n") {
		key, value, found := strings.Cut(strings.TrimRight(line, "\r"), ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "source":
			m.Source = value
		case "consumer":
			m.Consumer = value
		case "target":
			m.Target = value
		case "version":
			m.Version = value
		}
	}
	if m.Source == "" || m.Consumer == "" || m.Target == "" || m.Version == "" {
		return Marker{}, false
	}
	return m, true
}

// SameBump reports whether two markers are the same bump at possibly different
// versions: the same source, consumer and target. That is the key two PRs
// overlap on, and a newer version supersedes an older one.
func (m Marker) SameBump(o Marker) bool {
	return m.Source == o.Source && m.Consumer == o.Consumer && m.Target == o.Target
}
