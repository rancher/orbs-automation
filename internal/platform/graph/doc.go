// Package graph is a directed graph over any comparable node, with a payload on
// each edge, and the questions asked of one: the cycles, what a node reaches, and
// the shortest path between two. It knows nothing of what the nodes are, so the
// rules across the documents of a config (where a node is a name) and the joined
// config (where it is an id) use the same code.
//
// Nodes and edges are visited in the order they were added, so every answer is the
// same on every run. A graph is built, then asked: no query changes it.
package graph
