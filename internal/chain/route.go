package chain

import "slices"

// Route is one way out of a producer: the release holdings one consumer has of one
// of its streams. A consumer that holds artifacts of two streams of a producer has
// two routes from it.
type Route struct {
	Producer *Project
	Consumer *Project
	// Stream is the stream whose releases the route ships: the stream of the artifact
	// of each of its holdings.
	Stream   *Stream
	Holdings []*Holding
}

// AppliesTo reports whether the route has a holding for the consumer's line: one
// whose lines hold it, that the consumer lists, and whose stream serves it.
func (r Route) AppliesTo(line string) bool {
	return slices.ContainsFunc(r.Holdings, func(h *Holding) bool { return h.AppliesTo(line) })
}

// Edits returns the edits that follow the route's holdings, each once, in the order
// they are read.
func (r Route) Edits() []*Edit {
	var out []*Edit
	for _, h := range r.Holdings {
		for _, ed := range h.Edits() {
			if !slices.Contains(out, ed) {
				out = append(out, ed)
			}
		}
	}
	return out
}

// RoutesFrom returns the routes out of the project: a route for each consumer and
// stream of the release holdings made of it, in the order the holdings are read. A
// commit or a branch holding is no route, since no release goes down it.
func (p *Project) RoutesFrom() []Route {
	return routesOf(p.sourced, func(h *Holding) *Project { return h.holder })
}

// RoutesInto returns the routes into the project: a route for each producer and
// stream of the release holdings it has, in the order the holdings are read.
func (p *Project) RoutesInto() []Route {
	return routesOf(p.held, func(h *Holding) *Project { return h.producer })
}

// routeKey says which route a holding is in: the project at the other end, and the
// stream of its artifact.
type routeKey struct {
	peer   *Project
	stream *Stream
}

// routesOf groups the release holdings by the project the peer function names and the
// stream of their artifact, in the order they come.
func routesOf(holdings []*Holding, peer func(*Holding) *Project) []Route {
	var out []Route
	index := map[routeKey]int{}
	for _, h := range holdings {
		if h.Kind() != HoldsRelease || h.artifact == nil {
			continue
		}
		k := routeKey{peer(h), h.artifact.stream}
		i, ok := index[k]
		if !ok {
			i = len(out)
			index[k] = i
			out = append(out, Route{Producer: h.producer, Consumer: h.holder, Stream: k.stream})
		}
		out[i].Holdings = append(out[i].Holdings, h)
	}
	return out
}
