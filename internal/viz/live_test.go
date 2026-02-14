package viz

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestLiveReceiver(t *testing.T) {
	port := freePort()
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	receiver := NewLiveReceiver(port, graph, hub)
	go receiver.Run(ctx)

	time.Sleep(200 * time.Millisecond)

	// POST OTLP log event
	otlpBody := `{
		"resourceLogs": [{
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "1700000000000000000",
					"body": {"stringValue": "claude_code.api_request"},
					"attributes": [
						{"key": "model", "value": {"stringValue": "claude-sonnet-4-5-20250929"}},
						{"key": "input_tokens", "value": {"intValue": "1000"}},
						{"key": "output_tokens", "value": {"intValue": "500"}}
					]
				}]
			}]
		}]
	}`

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/v1/logs", port),
		"application/json",
		bytes.NewBufferString(otlpBody),
	)
	if err != nil {
		t.Fatalf("POST /v1/logs failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify graph has nodes
	snap := graph.Snapshot()
	if len(snap.Nodes) < 2 {
		t.Errorf("expected at least 2 nodes (agent + llm), got %d", len(snap.Nodes))
	}
	if len(snap.Edges) < 1 {
		t.Errorf("expected at least 1 edge (model_call), got %d", len(snap.Edges))
	}

	// Verify node types
	foundLLM := false
	foundAgent := false
	for _, n := range snap.Nodes {
		if n.Type == NodeLLM {
			foundLLM = true
		}
		if n.Type == NodeAgent {
			foundAgent = true
		}
	}
	if !foundLLM {
		t.Error("expected LLM endpoint node")
	}
	if !foundAgent {
		t.Error("expected agent node")
	}
}

func TestLiveReceiverMetrics(t *testing.T) {
	port := freePort()
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	receiver := NewLiveReceiver(port, graph, hub)
	go receiver.Run(ctx)

	time.Sleep(200 * time.Millisecond)

	// POST OTLP metrics
	body := `{
		"resourceMetrics": [{
			"scopeMetrics": [{
				"metrics": [{
					"name": "claude_code.token.usage",
					"sum": {
						"dataPoints": [{
							"timeUnixNano": "1700000000000000000",
							"asInt": 1500,
							"attributes": [
								{"key": "model", "value": {"stringValue": "claude-sonnet-4-5-20250929"}}
							]
						}]
					}
				}]
			}]
		}]
	}`

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/v1/metrics", port),
		"application/json",
		bytes.NewBufferString(body),
	)
	if err != nil {
		t.Fatalf("POST /v1/metrics failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	time.Sleep(100 * time.Millisecond)

	snap := graph.Snapshot()
	if len(snap.Nodes) < 1 {
		t.Errorf("expected at least 1 node from metrics, got %d", len(snap.Nodes))
	}
}

func TestLiveReceiverMethodNotAllowed(t *testing.T) {
	port := freePort()
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	receiver := NewLiveReceiver(port, graph, hub)
	go receiver.Run(ctx)

	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/v1/logs", port))
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}
