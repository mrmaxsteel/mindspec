package viz

import (
	"testing"
	"time"
)

func TestGraphUpsertNode(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())

	// First upsert creates node
	g.UpsertNode(NodeUpsert{
		ID:    "tool:Read",
		Type:  NodeTool,
		Label: "Read",
		Attributes: map[string]any{
			"category": "file",
		},
	})

	if g.NodeCount() != 1 {
		t.Fatalf("expected 1 node, got %d", g.NodeCount())
	}

	snap := g.Snapshot()
	if snap.Nodes[0].ActivityCount != 1 {
		t.Fatalf("expected activity 1, got %d", snap.Nodes[0].ActivityCount)
	}

	// Second upsert updates existing
	g.UpsertNode(NodeUpsert{
		ID:    "tool:Read",
		Type:  NodeTool,
		Label: "Read (updated)",
		Attributes: map[string]any{
			"extra": "value",
		},
	})

	if g.NodeCount() != 1 {
		t.Fatalf("expected 1 node after dedup, got %d", g.NodeCount())
	}

	snap = g.Snapshot()
	n := snap.Nodes[0]
	if n.Label != "Read (updated)" {
		t.Fatalf("expected updated label, got %q", n.Label)
	}
	if n.ActivityCount != 2 {
		t.Fatalf("expected activity 2, got %d", n.ActivityCount)
	}
	if n.Attributes["category"] != "file" {
		t.Error("original attribute should be preserved")
	}
	if n.Attributes["extra"] != "value" {
		t.Error("new attribute should be merged")
	}
}

func TestGraphAddEdge(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())

	now := time.Now()
	g.AddEdge(EdgeEvent{
		ID:        "e1",
		Src:       "agent:cc",
		Dst:       "tool:Read",
		Type:      EdgeToolCall,
		Status:    "ok",
		StartTime: now,
		Duration:  100 * time.Millisecond,
	})

	if g.EdgeCount() != 1 {
		t.Fatalf("expected 1 edge, got %d", g.EdgeCount())
	}

	// Same src+dst+type should increment call count
	g.AddEdge(EdgeEvent{
		ID:        "e2",
		Src:       "agent:cc",
		Dst:       "tool:Read",
		Type:      EdgeToolCall,
		Status:    "ok",
		StartTime: now.Add(time.Second),
		Duration:  200 * time.Millisecond,
	})

	if g.EdgeCount() != 1 {
		t.Fatalf("expected 1 edge after dedup, got %d", g.EdgeCount())
	}

	snap := g.Snapshot()
	if snap.Edges[0].CallCount != 2 {
		t.Fatalf("expected call count 2, got %d", snap.Edges[0].CallCount)
	}

	// Different type creates new edge
	g.AddEdge(EdgeEvent{
		ID:        "e3",
		Src:       "agent:cc",
		Dst:       "tool:Read",
		Type:      EdgeRetrieval,
		Status:    "ok",
		StartTime: now,
	})

	if g.EdgeCount() != 2 {
		t.Fatalf("expected 2 edges for different types, got %d", g.EdgeCount())
	}
}

func TestGraphTick_Staleness(t *testing.T) {
	cfg := DefaultGraphConfig()
	cfg.StaleThreshold = 3
	g := NewGraph(cfg)

	g.UpsertNode(NodeUpsert{ID: "n1", Type: NodeTool, Label: "old"})

	// Process more events to make n1 stale
	for i := 0; i < 4; i++ {
		g.UpsertNode(NodeUpsert{ID: "n2", Type: NodeTool, Label: "new"})
	}

	g.Tick()

	snap := g.Snapshot()
	for _, n := range snap.Nodes {
		if n.ID == "n1" && !n.Stale {
			t.Error("n1 should be stale after threshold exceeded")
		}
		if n.ID == "n2" && n.Stale {
			t.Error("n2 should not be stale")
		}
	}
}

func TestGraphTick_GradientOpacity(t *testing.T) {
	cfg := DefaultGraphConfig()
	cfg.FadeStart = 10 * time.Millisecond
	cfg.FadeEnd = 20 * time.Millisecond
	g := NewGraph(cfg)

	// Edge well past FadeEnd — should be opacity 0
	g.AddEdge(EdgeEvent{
		ID:        "e1",
		Src:       "a",
		Dst:       "b",
		Type:      EdgeToolCall,
		StartTime: time.Now().Add(-time.Second),
	})

	// Fresh edge — should be opacity 1.0
	g.AddEdge(EdgeEvent{
		ID:        "e2",
		Src:       "a",
		Dst:       "c",
		Type:      EdgeRetrieval,
		StartTime: time.Now(),
	})

	g.Tick()

	snap := g.Snapshot()
	for _, e := range snap.Edges {
		if e.Src == "a" && e.Dst == "b" {
			if e.Opacity != 0.0 {
				t.Errorf("old edge should have opacity 0.0, got %f", e.Opacity)
			}
		}
		if e.Src == "a" && e.Dst == "c" {
			if e.Opacity != 1.0 {
				t.Errorf("fresh edge should have opacity 1.0, got %f", e.Opacity)
			}
		}
	}
}

func TestGraphTick_GradientOpacityMidpoint(t *testing.T) {
	cfg := DefaultGraphConfig()
	cfg.FadeStart = 100 * time.Millisecond
	cfg.FadeEnd = 200 * time.Millisecond
	g := NewGraph(cfg)

	// Edge at ~midpoint of fade window
	g.AddEdge(EdgeEvent{
		ID:        "e1",
		Src:       "a",
		Dst:       "b",
		Type:      EdgeToolCall,
		StartTime: time.Now().Add(-150 * time.Millisecond),
	})

	g.Tick()

	snap := g.Snapshot()
	if len(snap.Edges) == 0 {
		t.Fatal("edge should still exist")
	}
	op := snap.Edges[0].Opacity
	if op <= 0.0 || op >= 1.0 {
		t.Errorf("midpoint edge should have opacity between 0 and 1, got %f", op)
	}
}

func TestGraphHardCaps(t *testing.T) {
	cfg := DefaultGraphConfig()
	cfg.MaxNodes = 5
	cfg.MaxEdges = 3
	cfg.StaleThreshold = 0 // everything becomes stale immediately
	cfg.FadeStart = 0
	cfg.FadeEnd = 0
	g := NewGraph(cfg)

	// Add more nodes than cap
	for i := 0; i < 10; i++ {
		g.UpsertNode(NodeUpsert{
			ID:    nodeID(i),
			Type:  NodeTool,
			Label: nodeID(i),
		})
	}

	capped := g.Tick()
	if !capped {
		t.Error("expected capped=true")
	}
	if g.NodeCount() > cfg.MaxNodes {
		t.Errorf("expected at most %d nodes, got %d", cfg.MaxNodes, g.NodeCount())
	}

	// Add more edges than cap
	for i := 0; i < 10; i++ {
		g.AddEdge(EdgeEvent{
			ID:        edgeID(i),
			Src:       "a",
			Dst:       nodeID(i),
			Type:      EdgeToolCall,
			StartTime: time.Now().Add(-time.Hour),
		})
	}

	g.Tick()
	if g.EdgeCount() > cfg.MaxEdges {
		t.Errorf("expected at most %d edges, got %d", cfg.MaxEdges, g.EdgeCount())
	}

	snap := g.Snapshot()
	if !snap.Capped {
		t.Error("snapshot should report capped=true")
	}
}

func TestGraphStats(t *testing.T) {
	g := NewGraph(DefaultGraphConfig())

	g.RecordEdgeStats("ok")
	g.RecordEdgeStats("error")
	g.RecordEdgeStats("error")
	g.RecordAPIStats(100, 50, 0.01)
	g.RecordAPIStats(200, 100, 0.02)

	stats := g.Stats()
	if stats.APICalls != 2 {
		t.Errorf("expected 2 API calls, got %d", stats.APICalls)
	}
	if stats.TotalTokens != 450 {
		t.Errorf("expected 450 total tokens, got %d", stats.TotalTokens)
	}
	if stats.ErrorCount != 2 {
		t.Errorf("expected 2 errors, got %d", stats.ErrorCount)
	}
	if stats.CostUSD < 0.029 || stats.CostUSD > 0.031 {
		t.Errorf("expected cost ~0.03, got %f", stats.CostUSD)
	}
}

func nodeID(i int) string {
	return "node-" + string(rune('a'+i))
}

func edgeID(i int) string {
	return "edge-" + string(rune('a'+i))
}
