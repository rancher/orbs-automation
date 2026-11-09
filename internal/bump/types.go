package bump

import "context"

// Release is what a form may be made from: the tag being shipped, and where it
// comes from. Which of them a holding's form uses is the holding's.
type Release struct {
	Tag        string // as published, such as v0.5.3 or helmtools/v0.1.2
	Prefix     string // the stream's tag prefix, such as helmtools/, or none
	BareTag    string // the tag without the prefix
	Version    string // the tag without the prefix and without a leading v
	Image      string // the artifact's repository path, for an image
	File       string // the artifact's file name, for a file
	URL        string // where the file is among the release's files, for a file
	SourceRepo string // the producer's owner/name
	Commit     string // the commit the tag is at
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

// Check is what the read side finds of one holding, without writing anything.
type Check struct {
	Current  string // what the consumer holds now
	Want     string // what it should hold for the release
	UpToDate bool
	Relation Relation
	Change   Change // what a bump would change; meaningful when not up to date
}

// Result is what the write side did.
type Result struct {
	Changed bool // false when the consumer already held every value
	Changes []Change
}

// Tree is where files are read from: the consumer at some commit on the host, or
// a clone.
type Tree interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// WritableTree is a Tree that can be changed: a throwaway clone.
type WritableTree interface {
	Tree
	WriteFile(ctx context.Context, path string, data []byte) error
}
