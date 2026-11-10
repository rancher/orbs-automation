package ship

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
)

// Destination is what the final destination holds of the producer on a line a
// release ships to: the reference the consumer's own holding is compared with. The
// consumer's holding decides the route, and the destination is read to see whether
// the release goes where the destination is.
type Destination struct {
	Project string `json:"project"` // the repository of the destination, owner/name
	Line    string `json:"line"`    // the Rancher line
	// Stream is the stream the destination's holding for the line is of.
	Stream string `json:"stream"`
	Branch string `json:"branch,omitempty"`
	// Held is what the destination's holdings of the producer hold in the branch
	// now, read from the host.
	Held []HeldValue `json:"held,omitempty"`
	// Error is why it could not be read.
	Error string `json:"error,omitempty"`
}

// destinations reads the final destination for each line the specs ship to, when it
// holds the producer on the line and is not the consumer. It returns what it found,
// in the order of the lines, and the warnings of the targets of each line that the
// destination does not agree with. Reading it never stops a run: what could not be
// read is a warning.
func (r resolution) destinations(ctx context.Context, ws Reader, specs []spec) ([]Destination, map[string][]string) {
	if r.route.Consumer.FinalDestination() {
		return nil, nil
	}
	var out []Destination
	warnings := map[string][]string{}
	seen := map[string]bool{}
	for _, sp := range specs {
		line := sp.target.Line
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		held := r.destinationHoldings(line)
		if len(held) == 0 {
			continue
		}
		d := Destination{Project: held[0].Holder().Slug(), Line: line, Stream: held[0].Artifact().Stream().Name()}
		r.readDestination(ctx, ws, held[0].Holder(), held[0].Artifact().Stream(), &d)
		out = append(out, d)
		if w := r.destinationWarnings(d); len(w) > 0 {
			warnings[line] = w
		}
	}
	return out, warnings
}

// destinationHoldings is the holdings of the producer's releases in the final
// destination that are for the line.
func (r resolution) destinationHoldings(line string) []*chain.Holding {
	var out []*chain.Holding
	for _, h := range r.producer.Sourced() {
		if h.Holder().FinalDestination() && h.Kind() == chain.HoldsRelease && h.AppliesTo(line) {
			out = append(out, h)
		}
	}
	return out
}

// readDestination fills d with what the destination holds of the stream on its
// line, read from the host with the pieces that trace a route in gero repo status:
// the destination's route for the stream, and its branch for the line.
func (r resolution) readDestination(ctx context.Context, ws Reader, dest *chain.Project, stream *chain.Stream, d *Destination) {
	engine, engineErr := ws.Bump()
	if engine == nil {
		d.Error = engineErr.Error()
		return
	}
	i := slices.IndexFunc(engine.RoutesFrom(r.producer), func(route bump.Route) bool { return route.Consumer == dest && route.Stream == stream })
	if i < 0 {
		d.Error = "it has no usable route for " + stream.Name()
		return
	}
	res, err := resolveRoute(ctx, ws, engine.RoutesFrom(r.producer)[i])
	if err != nil {
		d.Error = err.Error()
		return
	}
	// A prerelease may be all the stream has, and the release only serves to read
	// the value, so any will do.
	sel, err := res.specs(Input{Lines: []string{d.Line}, Prerelease: true})
	if err != nil || len(sel.specs) == 0 {
		if err == nil {
			err = errors.New("it has no branch for the line")
		}
		d.Error = err.Error()
		return
	}
	h := res.holding(ctx, ws, sel.specs[0], Release{})
	d.Branch, d.Held, d.Error = h.Branch, h.Held, h.Error
	for _, held := range h.Held {
		if d.Error == "" && held.Error != "" {
			d.Error = held.Error // a holding that could not be read
		}
	}
}

// destinationWarnings is what is wrong with the release going where the destination
// is not: the destination could not be read, it holds another stream for the line, or
// what it really holds is not a release of the stream the consumer holds.
func (r resolution) destinationWarnings(d Destination) []string {
	consumer, stream := r.route.Consumer.Slug(), r.route.Stream
	switch {
	case d.Error != "":
		return []string{fmt.Sprintf("could not read %s on %s to compare it with: %s", d.Project, d.Line, d.Error)}
	case d.Stream != stream.Name():
		return []string{fmt.Sprintf("%s holds %s on %s through the stream %s, and %s would hold the stream %s",
			d.Project, r.producer.Name(), d.Line, d.Stream, consumer, stream.Name())}
	}
	var out []string
	for _, held := range d.Held {
		if tag := heldTag(held.Current); tag != "" && !matchesStream(stream, tag) {
			out = append(out, fmt.Sprintf("%s holds %s on %s, which is not a release of the stream %s that %s holds",
				d.Project, tag, d.Line, stream.Name(), consumer))
		}
	}
	return out
}

// matchesStream reports whether a value a holding read is a release of the stream:
// its tag, or the tag without the prefix a Go module goes without.
func matchesStream(s *chain.Stream, value string) bool {
	if s.TagMatches(value) {
		return true
	}
	pattern := s.TagPattern()
	if i := strings.LastIndex(pattern, "/"); i >= 0 {
		return s.TagMatches(pattern[:i+1] + value)
	}
	return false
}
