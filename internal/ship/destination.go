package ship

import (
	"context"
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
// in the order of the lines, and a warning for the line of each target that does not
// agree with it.
func (r resolution) destinations(ctx context.Context, ws Reader, specs []spec, in Input) ([]Destination, map[string][]string) {
	return nil, nil
}
