package ship

import (
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/platform/github/pulls"
)

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
		"\ntarget: " + m.Target + "\nversion: " + m.Version + "\nkey: " + m.Key() + "\n-->"
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

// Key is the bump's overlap key, source, consumer and target, in one exact string:
// the same for every version of the bump, and what the marker records as key so a
// reader need not parse the fields to match it.
func (m Marker) Key() string {
	return m.Source + "|" + m.Consumer + "|" + m.Target
}

// RecordedKey is the key a marker in a PR body says it has, as it was written.
// ok is false when the body has no marker or the marker has no key, as the
// markers written before the key was added.
func RecordedKey(body string) (key string, ok bool) {
	start := strings.Index(body, markerOpen)
	if start < 0 {
		return "", false
	}
	block := body[start+len(markerOpen):]
	end := strings.Index(block, "-->")
	if end < 0 {
		return "", false
	}
	for _, line := range strings.Split(block[:end], "\n") {
		name, value, found := strings.Cut(strings.TrimRight(line, "\r"), ":")
		if found && strings.TrimSpace(name) == "key" && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

// markerWarnings says where a PR with this marker departs from what gero would
// have written: the key its marker records is not the one its fields give, or its
// branch does not end in the key of the name. Neither stops the run; the marker
// is the identity, and these are for the report.
func markerWarnings(p pulls.PR, m Marker) []string {
	var out []string
	if key, ok := RecordedKey(p.Body); ok && key != m.Key() {
		out = append(out, fmt.Sprintf("#%d: the key its marker records, %s, is not the one its fields give, %s", p.Number, key, m.Key()))
	}
	if p.HeadRef != "" && !strings.HasSuffix(p.HeadRef, BranchTail(m.Version, m.Target)) {
		out = append(out, fmt.Sprintf("#%d: its branch %s does not end in %s, as a branch of this bump does", p.Number, p.HeadRef, BranchTail(m.Version, m.Target)))
	}
	return out
}
