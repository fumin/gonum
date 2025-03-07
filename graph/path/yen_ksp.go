// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"cmp"
	"math"
	"slices"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
)

// YenKShortestPaths returns the k-shortest loopless paths from s to t in g
// with path costs no greater than cost beyond the shortest path.
// If k is negative, only path cost will be used to limit the set of returned
// paths. YenKShortestPaths will panic if g contains a negative edge weight.
func YenKShortestPaths(g graph.Graph, k int, cost float64, s, t graph.Node) [][]graph.Node {
	// See https://en.wikipedia.org/wiki/Yen's_algorithm and
	// the paper at https://doi.org/10.1090%2Fqam%2F253822.

	_, isDirected := g.(graph.Directed)
	yk := yenKSPAdjuster{
		Graph:      g,
		isDirected: isDirected,
	}

	if wg, ok := g.(Weighted); ok {
		yk.weight = wg.Weight
	} else {
		yk.weight = UniformCost(g)
	}

	shortest, weight := DijkstraFromTo(s, t, yk)
	cost += weight // Set cost to absolute cost limit.
	switch len(shortest) {
	case 0:
		return nil
	case 1:
		return [][]graph.Node{shortest}
	}
	paths := [][]graph.Node{shortest}

	var pot []yenShortest
	var root []graph.Node
	for i := int64(1); k < 0 || i < int64(k); i++ {
		// The spur node ranges from the first node to the next
		// to last node in the previous k-shortest path.
		for n := 0; n < len(paths[i-1])-1; n++ {
			yk.reset()

			spur := paths[i-1][n]
			root := append(root[:0], paths[i-1][:n+1]...)

			for _, path := range paths {
				if len(path) <= n {
					continue
				}
				ok := true
				for x := 0; x < len(root); x++ {
					if path[x].ID() != root[x].ID() {
						ok = false
						break
					}
				}
				if ok {
					yk.removeEdge(path[n].ID(), path[n+1].ID())
				}
			}
			for _, u := range root[:len(root)-1] {
				yk.removeNode(u.ID())
			}

			spath, weight := DijkstraFromTo(spur, t, yk)
			if weight > cost || math.IsInf(weight, 1) {
				continue
			}
			if len(root) > 1 {
				var rootWeight float64
				for x := 1; x < len(root); x++ {
					w, _ := yk.weight(root[x-1].ID(), root[x].ID())
					rootWeight += w
				}
				spath = append(root[:len(root)-1], spath...)
				weight += rootWeight
			}

			// Add the potential k-shortest path if it is new.
			isNewPot := true
			for x := range pot {
				if isSamePath(pot[x].path, spath) {
					isNewPot = false
					break
				}
			}
			if isNewPot {
				pot = append(pot, yenShortest{spath, weight})
			}
		}

		if len(pot) == 0 {
			break
		}

		slices.SortFunc(pot, func(a, b yenShortest) int {
			return cmp.Compare(a.weight, b.weight)
		})
		best := pot[0]
		if len(best.path) <= 1 || best.weight > cost {
			break
		}
		paths = append(paths, best.path)
		pot = pot[1:]
	}

	return paths
}

func isSamePath(a, b []graph.Node) bool {
	if len(a) != len(b) {
		return false
	}

	for i, x := range a {
		if x.ID() != b[i].ID() {
			return false
		}
	}
	return true
}

// yenShortest holds a path and its weight for sorting.
type yenShortest struct {
	path   []graph.Node
	weight float64
}

// yenKSPAdjuster allows walked edges to be omitted from a graph
// without altering the embedded graph.
type yenKSPAdjuster struct {
	graph.Graph
	isDirected bool

	// weight is the edge weight function
	// used for shortest path calculation.
	weight Weighting

	// visitedNodes holds the nodes that have
	// been removed by Yen's algorithm.
	visitedNodes map[int64]struct{}

	// visitedEdges holds the edges that have
	// been removed by Yen's algorithm.
	visitedEdges map[[2]int64]struct{}
}

func (g yenKSPAdjuster) From(id int64) graph.Nodes {
	if _, blocked := g.visitedNodes[id]; blocked {
		return graph.Empty
	}
	nodes := graph.NodesOf(g.Graph.From(id))
	for i := 0; i < len(nodes); {
		if g.canWalk(id, nodes[i].ID()) {
			i++
			continue
		}
		nodes[i] = nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
	}
	if len(nodes) == 0 {
		return graph.Empty
	}
	return iterator.NewOrderedNodes(nodes)
}

func (g yenKSPAdjuster) canWalk(u, v int64) bool {
	if _, blocked := g.visitedNodes[v]; blocked {
		return false
	}
	_, blocked := g.visitedEdges[[2]int64{u, v}]
	return !blocked
}

func (g yenKSPAdjuster) removeNode(u int64) {
	g.visitedNodes[u] = struct{}{}
}

func (g yenKSPAdjuster) removeEdge(u, v int64) {
	g.visitedEdges[[2]int64{u, v}] = struct{}{}
	if !g.isDirected {
		g.visitedEdges[[2]int64{v, u}] = struct{}{}
	}
}

func (g *yenKSPAdjuster) reset() {
	g.visitedNodes = make(map[int64]struct{})
	g.visitedEdges = make(map[[2]int64]struct{})
}

func (g yenKSPAdjuster) Weight(xid, yid int64) (w float64, ok bool) {
	return g.weight(xid, yid)
}
