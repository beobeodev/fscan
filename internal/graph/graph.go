package graph

import "sync"

// Graph is a thread-safe directed symbol dependency graph.
type Graph struct {
	nodes map[string]*Node
	mu    sync.RWMutex
}

// New creates an empty Graph.
func New() *Graph {
	return &Graph{nodes: make(map[string]*Node)}
}

// Add adds a node. If a node with the same ID already exists, it is replaced.
func (g *Graph) Add(n *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[n.ID] = n
}

// Get returns the node by ID, or nil.
func (g *Graph) Get(id string) *Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

// GetOrCreate returns an existing node or creates a stub with the given ID.
func (g *Graph) GetOrCreate(id string, kind Kind, name, file string) *Node {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n, ok := g.nodes[id]; ok {
		return n
	}
	n := &Node{ID: id, Kind: kind, Name: name, File: file}
	g.nodes[id] = n
	return n
}

// Connect adds a directed edge from src to dst (src references dst).
// Both nodes must already exist.
func (g *Graph) Connect(srcID, dstID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	src, ok := g.nodes[srcID]
	if !ok {
		return
	}
	dst, ok := g.nodes[dstID]
	if !ok {
		return
	}

	// Avoid duplicate edges
	for _, r := range src.Refs {
		if r == dstID {
			return
		}
	}
	src.Refs = append(src.Refs, dstID)
	dst.RefBy = append(dst.RefBy, srcID)
}

// Nodes returns all nodes (read-only snapshot).
func (g *Graph) Nodes() map[string]*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	snap := make(map[string]*Node, len(g.nodes))
	for k, v := range g.nodes {
		snap[k] = v
	}
	return snap
}

// NodesByKind returns all nodes of a given kind.
func (g *Graph) NodesByKind(kind Kind) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []*Node
	for _, n := range g.nodes {
		if n.Kind == kind {
			result = append(result, n)
		}
	}
	return result
}

// Size returns the number of nodes.
func (g *Graph) Size() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// AddSymbol merges semantic symbol data from the Dart worker into the graph.
// Creates or updates the symbol node and links it to its parent file node.
func (g *Graph) AddSymbol(n *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[n.ID] = n

	// Link file → symbol (file references/contains the symbol)
	fileID := FileID(n.File)
	if fileNode, ok := g.nodes[fileID]; ok {
		for _, r := range fileNode.Refs {
			if r == n.ID {
				return
			}
		}
		fileNode.Refs = append(fileNode.Refs, n.ID)
		n.RefBy = append(n.RefBy, fileID)
	}
}
