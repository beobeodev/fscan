package graph

// Reachability performs a BFS from the given entry node IDs and returns
// the set of all reachable node IDs.
func (g *Graph) Reachability(entryIDs []string) map[string]bool {
	visited := make(map[string]bool, len(g.nodes))
	queue := make([]string, 0, len(entryIDs))

	for _, id := range entryIDs {
		if g.Get(id) != nil {
			queue = append(queue, id)
			visited[id] = true
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		node := g.Get(cur)
		if node == nil {
			continue
		}

		for _, refID := range node.Refs {
			if !visited[refID] {
				visited[refID] = true
				queue = append(queue, refID)
			}
		}
	}

	return visited
}

// UnreachableFiles returns file nodes not reachable from entryIDs.
// Excludes generated files, part-of files, and test files.
func (g *Graph) UnreachableFiles(entryIDs []string) []*Node {
	reachable := g.Reachability(entryIDs)

	var unreachable []*Node
	for _, n := range g.NodesByKind(KindFile) {
		if n.IsGenerated || n.IsPartOf {
			continue
		}
		if !reachable[n.ID] {
			unreachable = append(unreachable, n)
		}
	}
	return unreachable
}
