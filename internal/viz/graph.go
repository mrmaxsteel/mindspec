package viz

import (
	"sync"
	"time"
)

// NodeType represents the type of a graph node.
type NodeType string

const (
	NodeAgent      NodeType = "agent"
	NodeTool       NodeType = "tool"
	NodeMCPServer  NodeType = "mcp_server"
	NodeDataSource NodeType = "data_source"
	NodeLLM        NodeType = "llm_endpoint"
)

// EdgeType represents the type of a graph edge.
type EdgeType string

const (
	EdgeToolCall  EdgeType = "tool_call"
	EdgeMCPCall   EdgeType = "mcp_call"
	EdgeRetrieval EdgeType = "retrieval"
	EdgeWrite     EdgeType = "write"
	EdgeModelCall EdgeType = "model_call"
)

// Node represents a node in the visualization graph.
type Node struct {
	ID            string            `json:"id"`
	Type          NodeType          `json:"type"`
	Label         string            `json:"label"`
	Attributes    map[string]any    `json:"attributes,omitempty"`
	LastSeen      time.Time         `json:"lastSeen"`
	ActivityCount int               `json:"activityCount"`
	Stale         bool              `json:"stale"`
}

// Edge represents an edge in the visualization graph.
type Edge struct {
	ID         string         `json:"id"`
	Src        string         `json:"src"`
	Dst        string         `json:"dst"`
	Type       EdgeType       `json:"type"`
	Status     string         `json:"status"`
	StartTime  time.Time      `json:"startTime"`
	EndTime    time.Time      `json:"endTime,omitempty"`
	Duration   time.Duration  `json:"duration,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Faded      bool           `json:"faded"`
	CallCount  int            `json:"callCount"`
}

// NodeUpsert is a request to create or update a node.
type NodeUpsert struct {
	ID         string         `json:"id"`
	Type       NodeType       `json:"type"`
	Label      string         `json:"label"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// EdgeEvent is a request to create an edge.
type EdgeEvent struct {
	ID         string         `json:"id"`
	Src        string         `json:"src"`
	Dst        string         `json:"dst"`
	Type       EdgeType       `json:"type"`
	Status     string         `json:"status"`
	StartTime  time.Time      `json:"startTime"`
	EndTime    time.Time      `json:"endTime,omitempty"`
	Duration   time.Duration  `json:"duration,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// GraphSnapshot is a serializable copy of the graph state.
type GraphSnapshot struct {
	Nodes   []Node `json:"nodes"`
	Edges   []Edge `json:"edges"`
	Capped  bool   `json:"capped"`
}

// GraphConfig holds configurable thresholds for the graph.
type GraphConfig struct {
	StaleThreshold int           // Events since last seen before marking stale (default 200)
	FadeTimeout    time.Duration // Duration before edges are marked faded (default 120s)
	MaxNodes       int           // Hard cap on nodes (default 500)
	MaxEdges       int           // Hard cap on edges (default 2000)
}

// DefaultGraphConfig returns the default configuration.
func DefaultGraphConfig() GraphConfig {
	return GraphConfig{
		StaleThreshold: 200,
		FadeTimeout:    120 * time.Second,
		MaxNodes:       500,
		MaxEdges:       2000,
	}
}

// Graph is a thread-safe in-memory graph for visualization.
type Graph struct {
	mu     sync.RWMutex
	nodes  map[string]*Node
	edges  map[string]*Edge
	config GraphConfig
	eventN int // total events processed (for staleness tracking)
	nodeN  map[string]int // node ID → last event number seen
	capped bool
}

// NewGraph creates a new graph with the given configuration.
func NewGraph(cfg GraphConfig) *Graph {
	return &Graph{
		nodes:  make(map[string]*Node),
		edges:  make(map[string]*Edge),
		config: cfg,
		nodeN:  make(map[string]int),
	}
}

// UpsertNode creates or updates a node. Deduplicates by ID, merges attributes (latest wins).
func (g *Graph) UpsertNode(u NodeUpsert) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.eventN++

	if existing, ok := g.nodes[u.ID]; ok {
		// Update existing node
		if u.Label != "" {
			existing.Label = u.Label
		}
		if u.Type != "" {
			existing.Type = u.Type
		}
		for k, v := range u.Attributes {
			if existing.Attributes == nil {
				existing.Attributes = make(map[string]any)
			}
			existing.Attributes[k] = v
		}
		existing.LastSeen = time.Now()
		existing.ActivityCount++
		existing.Stale = false
		g.nodeN[u.ID] = g.eventN
	} else {
		// Create new node
		attrs := make(map[string]any)
		for k, v := range u.Attributes {
			attrs[k] = v
		}
		g.nodes[u.ID] = &Node{
			ID:            u.ID,
			Type:          u.Type,
			Label:         u.Label,
			Attributes:    attrs,
			LastSeen:      time.Now(),
			ActivityCount: 1,
		}
		g.nodeN[u.ID] = g.eventN
	}
}

// AddEdge adds an edge. If an edge with the same src+dst+type exists, increments its call count.
func (g *Graph) AddEdge(e EdgeEvent) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.eventN++

	// Check for existing edge with same combo
	comboKey := e.Src + "|" + e.Dst + "|" + string(e.Type)
	if existing, ok := g.edges[comboKey]; ok {
		existing.CallCount++
		existing.Status = e.Status
		existing.StartTime = e.StartTime
		existing.EndTime = e.EndTime
		existing.Duration = e.Duration
		existing.Faded = false
		for k, v := range e.Attributes {
			if existing.Attributes == nil {
				existing.Attributes = make(map[string]any)
			}
			existing.Attributes[k] = v
		}
		return
	}

	attrs := make(map[string]any)
	for k, v := range e.Attributes {
		attrs[k] = v
	}
	g.edges[comboKey] = &Edge{
		ID:         e.ID,
		Src:        e.Src,
		Dst:        e.Dst,
		Type:       e.Type,
		Status:     e.Status,
		StartTime:  e.StartTime,
		EndTime:    e.EndTime,
		Duration:   e.Duration,
		Attributes: attrs,
		CallCount:  1,
	}
}

// Tick applies staleness/fade flags and enforces hard caps. Returns true if caps are active.
func (g *Graph) Tick() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	// Mark stale nodes
	for id, n := range g.nodes {
		lastN, ok := g.nodeN[id]
		if !ok || g.eventN-lastN > g.config.StaleThreshold {
			n.Stale = true
		}
	}

	// Mark faded edges
	for _, e := range g.edges {
		if now.Sub(e.StartTime) > g.config.FadeTimeout {
			e.Faded = true
		}
	}

	// Enforce hard caps — evict oldest stale nodes
	g.capped = false
	if len(g.nodes) > g.config.MaxNodes {
		g.capped = true
		g.evictStaleNodes(len(g.nodes) - g.config.MaxNodes)
	}
	if len(g.edges) > g.config.MaxEdges {
		g.capped = true
		g.evictFadedEdges(len(g.edges) - g.config.MaxEdges)
	}

	return g.capped
}

// evictStaleNodes removes the oldest stale nodes. Must be called with lock held.
func (g *Graph) evictStaleNodes(count int) {
	// First try stale nodes
	evicted := 0
	for id, n := range g.nodes {
		if evicted >= count {
			break
		}
		if n.Stale {
			delete(g.nodes, id)
			delete(g.nodeN, id)
			evicted++
		}
	}
	// If not enough stale, evict oldest by event number
	for evicted < count && len(g.nodes) > 0 {
		oldestID := ""
		oldestN := g.eventN + 1
		for id := range g.nodes {
			if n, ok := g.nodeN[id]; ok && n < oldestN {
				oldestN = n
				oldestID = id
			}
		}
		if oldestID != "" {
			delete(g.nodes, oldestID)
			delete(g.nodeN, oldestID)
			evicted++
		} else {
			break
		}
	}
}

// evictFadedEdges removes the oldest faded edges. Must be called with lock held.
func (g *Graph) evictFadedEdges(count int) {
	evicted := 0
	for id, e := range g.edges {
		if evicted >= count {
			break
		}
		if e.Faded {
			delete(g.edges, id)
			evicted++
		}
	}
	// If not enough faded, evict oldest by start time
	for evicted < count && len(g.edges) > 0 {
		oldestID := ""
		var oldestTime time.Time
		for id, e := range g.edges {
			if oldestID == "" || e.StartTime.Before(oldestTime) {
				oldestTime = e.StartTime
				oldestID = id
			}
		}
		if oldestID != "" {
			delete(g.edges, oldestID)
			evicted++
		} else {
			break
		}
	}
}

// Snapshot returns a copy of the current graph state.
func (g *Graph) Snapshot() GraphSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, *n)
	}

	edges := make([]Edge, 0, len(g.edges))
	for _, e := range g.edges {
		edges = append(edges, *e)
	}

	return GraphSnapshot{
		Nodes:  nodes,
		Edges:  edges,
		Capped: g.capped,
	}
}

// NodeCount returns the number of nodes.
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// EdgeCount returns the number of edges.
func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.edges)
}
