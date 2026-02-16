package viz

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestReplayReadsNDJSON(t *testing.T) {
	// Create test fixture
	fixture := `{"ts":"2026-02-14T12:00:00Z","event":"claude_code.api_request","data":{"model":"claude-sonnet-4-5-20250929","input_tokens":1000,"output_tokens":500}}
{"ts":"2026-02-14T12:00:01Z","event":"claude_code.tool_use","data":{"tool_name":"Read","duration_ms":50}}
{"ts":"2026-02-14T12:00:02Z","event":"claude_code.tool_use","data":{"tool_name":"Write","duration_ms":100}}
{"ts":"2026-02-14T12:00:03Z","event":"claude_code.mcp_call","data":{"server_name":"my-server","method":"doStuff"}}
`
	tmpFile, err := os.CreateTemp("", "replay-test-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(fixture)
	tmpFile.Close()

	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	replay := NewReplay(tmpFile.Name(), 0, graph, hub) // max speed
	if err := replay.Run(ctx); err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	// Verify graph state
	snap := graph.Snapshot()

	// Should have: agent:claude-code, llm:claude-sonnet-4-5-20250929, tool:Read, tool:Write, mcp:my-server
	if len(snap.Nodes) < 4 {
		t.Errorf("expected at least 4 nodes, got %d", len(snap.Nodes))
	}

	// Should have edges: model_call, retrieval (Read), write (Write), mcp_call
	if len(snap.Edges) < 4 {
		t.Errorf("expected at least 4 edges, got %d", len(snap.Edges))
	}

	// Verify node types
	types := make(map[NodeType]int)
	for _, n := range snap.Nodes {
		types[n.Type]++
	}

	if types[NodeAgent] != 1 {
		t.Errorf("expected 1 agent node, got %d", types[NodeAgent])
	}
	if types[NodeLLM] != 1 {
		t.Errorf("expected 1 LLM node, got %d", types[NodeLLM])
	}
	if types[NodeTool] != 2 {
		t.Errorf("expected 2 tool nodes, got %d", types[NodeTool])
	}
	if types[NodeMCPServer] != 1 {
		t.Errorf("expected 1 MCP node, got %d", types[NodeMCPServer])
	}
}

func TestReplaySpeedControl(t *testing.T) {
	fixture := `{"ts":"2026-02-14T12:00:00Z","event":"claude_code.api_request","data":{"model":"m1"}}
{"ts":"2026-02-14T12:00:01Z","event":"claude_code.api_request","data":{"model":"m1"}}
`
	tmpFile, err := os.CreateTemp("", "replay-speed-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(fixture)
	tmpFile.Close()

	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// At 10x speed, 1 second delay should take ~100ms
	start := time.Now()
	replay := NewReplay(tmpFile.Name(), 10, graph, hub)
	replay.Run(ctx)
	elapsed := time.Since(start)

	// Should be fast at 10x (< 500ms for 1s of events)
	if elapsed > 500*time.Millisecond {
		t.Errorf("10x replay took too long: %v", elapsed)
	}
}

func TestReplayMaxSpeed(t *testing.T) {
	fixture := `{"ts":"2026-02-14T12:00:00Z","event":"claude_code.api_request","data":{"model":"m1"}}
{"ts":"2026-02-14T12:01:00Z","event":"claude_code.api_request","data":{"model":"m1"}}
`
	tmpFile, err := os.CreateTemp("", "replay-max-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(fixture)
	tmpFile.Close()

	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Max speed (0) should not sleep
	start := time.Now()
	replay := NewReplay(tmpFile.Name(), 0, graph, hub)
	replay.Run(ctx)
	elapsed := time.Since(start)

	// Should be nearly instant
	if elapsed > 100*time.Millisecond {
		t.Errorf("max speed replay too slow: %v", elapsed)
	}
}

func TestReplayCodexMetricTotals(t *testing.T) {
	fixture := `{"ts":"2026-02-14T12:00:00Z","event":"codex.token.usage","data":{"model":"gpt-5-codex","type":"input","value":1200}}
{"ts":"2026-02-14T12:00:01Z","event":"codex.token.usage","data":{"model":"gpt-5-codex","type":"output","value":300}}
{"ts":"2026-02-14T12:00:02Z","event":"codex.cost.usage","data":{"model":"gpt-5-codex","value":0.42}}
`
	tmpFile, err := os.CreateTemp("", "replay-codex-metric-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(fixture)
	tmpFile.Close()

	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	replay := NewReplay(tmpFile.Name(), 0, graph, hub)
	if err := replay.Run(ctx); err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	stats := graph.Stats()
	if stats.TotalTokens != 1500 {
		t.Fatalf("total tokens = %d, want 1500", stats.TotalTokens)
	}
	if stats.CostUSD < 0.419 || stats.CostUSD > 0.421 {
		t.Fatalf("cost = %f, want ~0.42", stats.CostUSD)
	}

	snap := graph.Snapshot()
	foundEdge := false
	for _, edge := range snap.Edges {
		if edge.Type == EdgeModelCall && edge.Src == "agent:codex" {
			foundEdge = true
			break
		}
	}
	if !foundEdge {
		t.Fatal("expected codex metric replay to emit model_call edge")
	}
}
