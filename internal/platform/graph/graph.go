package graph

import "slices"

// Edge is one edge out of a node.
type Edge[N comparable, E any] struct {
	From, To N
	Payload  E
}

// Graph is a directed graph of nodes N and edge payloads E.
type Graph[N comparable, E any] struct {
	nodes []N
	index map[N]int
	edges []Edge[N, E]
	out   map[N][]Edge[N, E]
}

// New returns an empty graph.
func New[N comparable, E any]() *Graph[N, E] {
	return &Graph[N, E]{index: map[N]int{}, out: map[N][]Edge[N, E]{}}
}

// AddNode adds a node with no edge. A node that is already there keeps its place.
func (g *Graph[N, E]) AddNode(n N) {
	if _, ok := g.index[n]; !ok {
		g.index[n] = len(g.nodes)
		g.nodes = append(g.nodes, n)
	}
}

// AddEdge adds an edge, and the nodes at its ends when they are new. Two edges
// between the same nodes are both kept.
func (g *Graph[N, E]) AddEdge(from, to N, payload E) {
	g.AddNode(from)
	g.AddNode(to)
	e := Edge[N, E]{From: from, To: to, Payload: payload}
	g.edges = append(g.edges, e)
	g.out[from] = append(g.out[from], e)
}

// Nodes returns every node in the order added.
func (g *Graph[N, E]) Nodes() []N { return append([]N(nil), g.nodes...) }

// Edges returns every edge, in the order added.
func (g *Graph[N, E]) Edges() []Edge[N, E] { return append([]Edge[N, E](nil), g.edges...) }

// Out returns the edges out of a node, in the order added.
func (g *Graph[N, E]) Out(n N) []Edge[N, E] { return append([]Edge[N, E](nil), g.out[n]...) }

// Cycles returns one cycle for each set of nodes that can all reach one another,
// and for each node that points at itself. A cycle is the path of the shortest
// way back to the first node of its set, that node first and without repeating
// it at the end. Cycles come in the order of the first node of each.
func (g *Graph[N, E]) Cycles() [][]N {
	var cycles [][]N
	for _, set := range g.components() {
		first := set[0]
		if len(set) == 1 && !g.hasEdge(first, first) {
			continue
		}
		in := make(map[N]bool, len(set))
		for _, n := range set {
			in[n] = true
		}
		cycles = append(cycles, g.shortest(first, first, in))
	}
	return cycles
}

// Reachable returns every node a path from a node reaches, in the order a
// breadth-first walk finds them. The node itself is there only when it is on a
// cycle.
func (g *Graph[N, E]) Reachable(from N) []N {
	var found []N
	seen := map[N]bool{}
	queue := []N{from}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, e := range g.out[n] {
			if !seen[e.To] {
				seen[e.To] = true
				found = append(found, e.To)
				queue = append(queue, e.To)
			}
		}
	}
	return found
}

// Path returns the shortest path from one node to another, both included, and
// false when there is none. A node has the path of itself alone to itself.
func (g *Graph[N, E]) Path(from, to N) ([]N, bool) {
	if _, ok := g.index[from]; !ok {
		return nil, false
	}
	if from == to {
		return []N{from}, true
	}
	p := g.shortest(from, to, nil)
	return p, p != nil
}

func (g *Graph[N, E]) hasEdge(from, to N) bool {
	for _, e := range g.out[from] {
		if e.To == to {
			return true
		}
	}
	return false
}

// shortest is the breadth-first path from one node to another through the nodes
// of within, or any when it is nil. When from is to, it is the way round, listed
// without the node at the end. It is nil when there is none.
func (g *Graph[N, E]) shortest(from, to N, within map[N]bool) []N {
	prev := map[N]N{}
	seen := map[N]bool{}
	queue := []N{from}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, e := range g.out[n] {
			if within != nil && !within[e.To] {
				continue
			}
			if e.To == to {
				path := []N{n}
				for cur := n; cur != from; {
					cur = prev[cur]
					path = append(path, cur)
				}
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				if from == to {
					return path
				}
				return append(path, to)
			}
			if !seen[e.To] {
				seen[e.To] = true
				prev[e.To] = n
				queue = append(queue, e.To)
			}
		}
	}
	return nil
}

// components returns the sets of nodes that can all reach one another (Tarjan),
// each with its nodes in the order added, and the sets in the order of their first.
func (g *Graph[N, E]) components() [][]N {
	var (
		sets    [][]N
		stack   []N
		on      = map[N]bool{}
		low     = map[N]int{}
		num     = map[N]int{}
		counter int
		visit   func(n N)
	)
	visit = func(n N) {
		counter++
		num[n], low[n] = counter, counter
		stack = append(stack, n)
		on[n] = true
		for _, e := range g.out[n] {
			if num[e.To] == 0 {
				visit(e.To)
				low[n] = min(low[n], low[e.To])
			} else if on[e.To] {
				low[n] = min(low[n], num[e.To])
			}
		}
		if low[n] != num[n] {
			return
		}
		var set []N
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			on[top] = false
			set = append(set, top)
			if top == n {
				break
			}
		}
		sets = append(sets, set)
	}
	for _, n := range g.nodes {
		if num[n] == 0 {
			visit(n)
		}
	}
	byIndex := func(a, b N) int { return g.index[a] - g.index[b] }
	for _, set := range sets {
		slices.SortFunc(set, byIndex)
	}
	slices.SortFunc(sets, func(a, b []N) int { return byIndex(a[0], b[0]) })
	return sets
}
