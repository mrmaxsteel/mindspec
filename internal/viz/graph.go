package viz

import (
	"encoding/json"
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
	EdgeSpawn     EdgeType = "spawn"
)

// Node represents a node in the visualization graph.
type Node struct {
	ID               string         `json:"id"`
	Type             NodeType       `json:"type"`
	Label            string         `json:"label"`
	Attributes       map[string]any `json:"attributes,omitempty"`
	LastSeen         time.Time      `json:"lastSeen"`
	ActivityCount    int            `json:"activityCount"`
	Stale            bool           `json:"stale"`
	CumulativeTokens int64          `json:"cumulativeTokens,omitempty"`
	CumulativeCost   float64        `json:"cumulativeCost,omitempty"`
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
	Opacity    float64        `json:"opacity"`
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
	Nodes  []Node `json:"nodes"`
	Edges  []Edge `json:"edges"`
	Capped bool   `json:"capped"`
}

// GraphConfig holds configurable thresholds for the graph.
type GraphConfig struct {
	StaleThreshold int           // Events since last seen before marking stale (default 200)
	FadeStart      time.Duration // Duration before edges start fading (default 30s)
	FadeEnd        time.Duration // Duration when edges become fully transparent (default 120s)
	MaxNodes       int           // Hard cap on nodes (default 500)
	MaxEdges       int           // Hard cap on edges (default 2000)
}

// DefaultGraphConfig returns the default configuration.
func DefaultGraphConfig() GraphConfig {
	return GraphConfig{
		StaleThreshold: 200,
		FadeStart:      30 * time.Second,
		FadeEnd:        120 * time.Second,
		MaxNodes:       500,
		MaxEdges:       2000,
	}
}

// GraphStats holds cumulative statistics for the graph.
type GraphStats struct {
	APICalls    int     `json:"apiCalls"`
	TotalTokens int64   `json:"totalTokens"`
	ErrorCount  int     `json:"errorCount"`
	CostUSD     float64 `json:"costUSD"`
}

// Graph is a thread-safe in-memory graph for visualization.
type Graph struct {
	mu     sync.RWMutex
	nodes  map[string]*Node
	edges  map[string]*Edge
	config GraphConfig
	eventN int            // total events processed (for staleness tracking)
	nodeN  map[string]int // node ID → last event number seen
	capped bool

	// Cumulative stats (protected by mu)
	apiCalls    int
	totalTokens int64
	errorCount  int
	costUSD     float64
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
		accumulateNodeStats(existing, u.Attributes)
	} else {
		// Create new node
		attrs := make(map[string]any)
		for k, v := range u.Attributes {
			attrs[k] = v
		}
		node := &Node{
			ID:            u.ID,
			Type:          u.Type,
			Label:         u.Label,
			Attributes:    attrs,
			LastSeen:      time.Now(),
			ActivityCount: 1,
		}
		accumulateNodeStats(node, u.Attributes)
		g.nodes[u.ID] = node
		g.nodeN[u.ID] = g.eventN
	}
}

// accumulateNodeStats adds token and cost values from attributes to a node's cumulative fields.
func accumulateNodeStats(n *Node, attrs map[string]any) {
	n.CumulativeTokens += toInt64(attrs["input_tokens"]) + toInt64(attrs["output_tokens"])
	n.CumulativeCost += toFloat64(attrs["cost_usd"])
}

// toInt64 converts a numeric attribute value to int64.
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

// toFloat64 converts a numeric attribute value to float64.
func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

// AddEdge adds an edge. If an edge with the same src+dst+type exists, increments its call count.
func (g *Graph) AddEdge(e EdgeEvent) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.eventN++
	metricOnly := false
	if v, ok := e.Attributes["metric_only"].(bool); ok && v {
		metricOnly = true
	}

	// Check for existing edge with same combo
	comboKey := e.Src + "|" + e.Dst + "|" + string(e.Type)
	if existing, ok := g.edges[comboKey]; ok {
		if !metricOnly {
			existing.CallCount++
		}
		existing.Status = e.Status
		existing.StartTime = e.StartTime
		existing.EndTime = e.EndTime
		existing.Duration = e.Duration
		existing.Opacity = 1.0
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
	callCount := 1
	if metricOnly {
		callCount = 0
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
		Opacity:    1.0,
		CallCount:  callCount,
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

	// Compute edge opacity via linear interpolation
	for _, e := range g.edges {
		age := now.Sub(e.StartTime)
		switch {
		case age < g.config.FadeStart:
			e.Opacity = 1.0
		case age > g.config.FadeEnd:
			e.Opacity = 0.0
		default:
			e.Opacity = 1.0 - float64(age-g.config.FadeStart)/float64(g.config.FadeEnd-g.config.FadeStart)
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
		if e.Opacity <= 0 {
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

// Reset clears all graph state.
func (g *Graph) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes = make(map[string]*Node)
	g.edges = make(map[string]*Edge)
	g.nodeN = make(map[string]int)
	g.eventN = 0
	g.capped = false
	g.apiCalls = 0
	g.totalTokens = 0
	g.errorCount = 0
	g.costUSD = 0
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

// RecordEdgeStats records edge-level stats (error count tracking).
func (g *Graph) RecordEdgeStats(status string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if status == "error" {
		g.errorCount++
	}
}

// RecordAPIStats records API-level stats (tokens, cost).
func (g *Graph) RecordAPIStats(inputTokens, outputTokens int64, cost float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.apiCalls++
	g.totalTokens += inputTokens + outputTokens
	g.costUSD += cost
}

// RecordTokenStats records token/cost totals without incrementing API request count.
func (g *Graph) RecordTokenStats(inputTokens, outputTokens int64, cost float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.totalTokens += inputTokens + outputTokens
	g.costUSD += cost
}

// Stats returns a snapshot of cumulative graph statistics.
func (g *Graph) Stats() GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return GraphStats{
		APICalls:    g.apiCalls,
		TotalTokens: g.totalTokens,
		ErrorCount:  g.errorCount,
		CostUSD:     g.costUSD,
	}
}
