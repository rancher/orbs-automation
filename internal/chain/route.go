package chain

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
func (r Route) AppliesTo(line string) bool { return false }

// Edits returns the edits that follow the route's holdings, each once, in the order
// they are read.
func (r Route) Edits() []*Edit { return nil }

// RoutesFrom returns the routes out of the project: a route for each consumer and
// stream of the release holdings made of it, in the order the holdings are read. A
// commit or a branch holding is no route, since no release goes down it.
func (p *Project) RoutesFrom() []Route { return nil }

// RoutesInto returns the routes into the project: a route for each producer and
// stream of the release holdings it has, in the order the holdings are read.
func (p *Project) RoutesInto() []Route { return nil }
