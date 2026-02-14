package viz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func freePort() int {
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestServerServesHTML(t *testing.T) {
	port := freePort()
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := NewServer(port, hub, graph)
	go server.Run(ctx)

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if !strings.Contains(html, "graph-container") {
		t.Error("expected graph-container element in HTML")
	}

	if !strings.Contains(html, "AgentMind Viz") {
		t.Error("expected title in HTML")
	}
}

func TestServerWebSocket(t *testing.T) {
	port := freePort()
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Pre-populate graph
	graph.UpsertNode(NodeUpsert{
		ID:    "tool:Read",
		Type:  NodeTool,
		Label: "Read",
	})

	server := NewServer(port, hub, graph)
	go server.Run(ctx)

	time.Sleep(200 * time.Millisecond)

	// Connect WebSocket
	wsURL := fmt.Sprintf("ws://localhost:%d/ws", port)
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Should receive snapshot
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}

	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if msg.Type != MsgSnapshot {
		t.Errorf("expected snapshot message, got %q", msg.Type)
	}

	// Parse snapshot data
	snapData, _ := json.Marshal(msg.Data)
	var snap GraphSnapshot
	json.Unmarshal(snapData, &snap)

	if len(snap.Nodes) != 1 {
		t.Errorf("expected 1 node in snapshot, got %d", len(snap.Nodes))
	}
}

func TestServerBroadcastUpdate(t *testing.T) {
	port := freePort()
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := NewServer(port, hub, graph)
	go server.Run(ctx)

	time.Sleep(200 * time.Millisecond)

	// Connect WebSocket
	wsURL := fmt.Sprintf("ws://localhost:%d/ws", port)
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read initial snapshot
	conn.Read(ctx)

	// Broadcast an update
	hub.Broadcast(WSMessage{
		Type: MsgUpdate,
		Data: map[string]any{"test": true},
	})

	// Read the update
	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()

	_, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}

	var msg WSMessage
	json.Unmarshal(data, &msg)
	if msg.Type != MsgUpdate {
		t.Errorf("expected update message, got %q", msg.Type)
	}
}
