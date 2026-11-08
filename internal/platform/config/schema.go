package config

import "time"

// Schema is a target the engine can load.
type Schema interface {
	// SetDefaults fills the fields that have a default and were left out.
	SetDefaults()
	// Validate checks the object, reporting each problem on r. It sees the
	// defaulted values.
	Validate(ctx Context, r *Report)
}

// Context is what validation may depend on besides the object. Time is injected
// so a rule that reads a lifecycle date can be tested on any date.
type Context struct {
	Now time.Time
}
