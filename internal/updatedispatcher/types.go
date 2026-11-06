package updatedispatcher

import "context"

// Inputs are the validated values of a call: what the producer's edge passed,
// with the defaults of the params the method offers filled in. A strategy reads
// them by name and never sees how they were declared.
type Inputs map[string]string

// Release is what a strategy may use to make a value: the tag being shipped and
// where it comes from.
type Release struct {
	Tag        string // as published, such as v0.5.3
	Version    string // the tag without a leading v
	Image      string // the producer's first image, such as rancher/scc-operator
	SourceRepo string // the producer's owner/name
}

// Relation says how the value a release wants compares with what the consumer
// holds.
type Relation string

// The relations.
const (
	RelationUnknown Relation = "unknown" // no version could be found in one of them
	RelationSame    Relation = "same"
	RelationNewer   Relation = "newer" // the release is newer: an upgrade
	RelationOlder   Relation = "older" // the release is older: a downgrade
)

// Change is one thing a bump changes: the file, in words, and the old and new
// values. The PR body's list of changes is made from these, so it names only what
// was changed.
type Change struct {
	Path        string
	Description string
	From, To    string
}

// Check is what the read side finds, without writing anything.
type Check struct {
	Current  string // what the consumer holds now
	Want     string // what it should hold for the release
	UpToDate bool
	Relation Relation
	Change   Change // what a bump would change; meaningful when not up to date
}

// Result is what the write side did.
type Result struct {
	Changed bool // false when the consumer already held the value
	Changes []Change
}

// Tree is where an updater reads files from: the consumer at some commit on the
// host, or a clone.
type Tree interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// WritableTree is a Tree that can be changed: a throwaway clone.
type WritableTree interface {
	Tree
	WriteFile(ctx context.Context, path string, data []byte) error
}

// Reader is the read side of an Updater. It never writes.
type Reader interface {
	Check(ctx context.Context, src Tree, in Inputs, rel Release) (Check, error)
}

// Writer is the write side of an Updater.
type Writer interface {
	Apply(ctx context.Context, dst WritableTree, in Inputs, rel Release) (Result, error)
}

// Updater is a strategy configured for one method. It is immutable, so one can
// serve every edge that uses the method.
type Updater interface {
	Reader
	Writer
}

// UpdateStrategy is reusable code that claims a name. The name is what a
// method's `strategy` says, and it is defined here and nowhere else.
type UpdateStrategy interface {
	Name() string
	// Requires lists the inputs the strategy cannot work without, by name only.
	// A method that uses the strategy must offer each under allowedParams.
	// Descriptions, defaults and which others are optional are config's.
	Requires() []string
	// Configure checks the method's `with` values and returns the Updater they
	// configure. It is where a typo in `with` is found, when the config loads.
	Configure(with map[string]string) (Updater, error)
}
