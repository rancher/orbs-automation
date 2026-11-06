package ship

import domain "github.com/rancher/orbs-automation/internal/ship"

// replay returns the events a run of plan would have emitted, in the order it
// would have emitted them. A finished plan can then be shown by the same code
// that shows a run as it happens, so the two cannot differ.
//
// STUB: no events until a later commit.
func replay(plan domain.Plan) []domain.Event { return nil }
