package viz

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestIntegrationReplayWithWebSocket(t *testing.T) {
	// Create test fixture
	fixture := `{"ts":"2026-02-14T12:00:00Z","event":"claude_code.api_request","data":{"model":"claude-sonnet-4-5-20250929","input_tokens":1000,"output_tokens":500}}
{"ts":"2026-02-14T12:00:01Z","event":"claude_code.tool_use","data":{"tool_name":"Read","duration_ms":50}}
{"ts":"2026-02-14T12:00:02Z","event":"claude_code.tool_use","data":{"tool_name":"Write","duration_ms":100}}
{"ts":"2026-02-14T12:00:03Z","event":"claude_code.mcp_call","data":{"server_name":"test-mcp","method":"doStuff"}}
`
	tmpFile, err := os.CreateTemp("", "integration-replay-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(fixture)
	tmpFile.Close()

	port := freePort()
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := NewServer(port, hub, graph)
	go server.Run(ctx)

	time.Sleep(200 * time.Millisecond)

	// Connect WebSocket before replay starts
	wsURL := fmt.Sprintf("ws://localhost:%d/ws", port)
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read initial snapshot (empty)
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read snapshot failed: %v", err)
	}
	var snapMsg WSMessage
	json.Unmarshal(data, &snapMsg)
	if snapMsg.Type != MsgSnapshot {
		t.Errorf("expected snapshot, got %s", snapMsg.Type)
	}

	// Run replay in background
	replay := NewReplay(tmpFile.Name(), 0, graph, hub) // max speed
	go replay.Run(ctx)

	// Read updates with timeout
	received := 0
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()

	for received < 4 {
		_, data, err := conn.Read(readCtx)
		if err != nil {
			break
		}
		var msg WSMessage
		json.Unmarshal(data, &msg)
		if msg.Type == MsgUpdate {
			received++
		}
	}

	if received < 2 {
		t.Errorf("expected at least 2 update messages, got %d", received)
	}

	// Give replay time to finish
	time.Sleep(200 * time.Millisecond)

	// Verify final graph state
	snap := graph.Snapshot()
	if len(snap.Nodes) < 4 {
		t.Errorf("expected at least 4 nodes, got %d", len(snap.Nodes))
	}
}

func TestIntegrationHardCaps(t *testing.T) {
	cfg := DefaultGraphConfig()
	cfg.MaxNodes = 10
	cfg.MaxEdges = 5
	cfg.StaleThreshold = 0
	cfg.FadeStart = 0
	cfg.FadeEnd = 0
	graph := NewGraph(cfg)

	// Flood with synthetic nodes
	for i := 0; i < 50; i++ {
		graph.UpsertNode(NodeUpsert{
			ID:    fmt.Sprintf("tool:tool-%d", i),
			Type:  NodeTool,
			Label: fmt.Sprintf("tool-%d", i),
		})
	}

	// Flood with synthetic edges
	for i := 0; i < 50; i++ {
		graph.AddEdge(EdgeEvent{
			ID:        fmt.Sprintf("e%d", i),
			Src:       fmt.Sprintf("tool:tool-%d", i),
			Dst:       fmt.Sprintf("tool:tool-%d", (i+1)%50),
			Type:      EdgeToolCall,
			StartTime: time.Now().Add(-time.Hour),
		})
	}

	capped := graph.Tick()
	if !capped {
		t.Error("expected capped=true after exceeding limits")
	}

	if graph.NodeCount() > cfg.MaxNodes {
		t.Errorf("nodes exceed cap: %d > %d", graph.NodeCount(), cfg.MaxNodes)
	}
	if graph.EdgeCount() > cfg.MaxEdges {
		t.Errorf("edges exceed cap: %d > %d", graph.EdgeCount(), cfg.MaxEdges)
	}

	snap := graph.Snapshot()
	if !snap.Capped {
		t.Error("snapshot should report capped=true")
	}
}
