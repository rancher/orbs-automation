// Package bump ships a release of a producer into the holdings of a consumer. It
// reads what a consumer branch holds and says how it compares with a release,
// writes the release into the consumer's files, and runs the edits that follow the
// holdings around the write. It is the domain behind gero ship and any other
// command that reads or changes what a consumer holds. See
// docs/design/UPDATE_ENGINE.md.
package bump
