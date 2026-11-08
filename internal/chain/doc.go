// Package chain is the resolved model of the application config: the Projects,
// Streams, Artifacts, Holdings and Edits of the documents, joined by their refs
// and answering the questions a command asks across them, such as which branch a
// line is on, which images a release must have or who holds a release of a
// project. It is built once from a config that already passed every rule and is
// read-only after. See docs/design/CHAIN_MODEL.md.
package chain
