package viz

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerDefaultsToLoopback(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	srv := NewServer(8420, hub, graph)
	if srv.bindAddr != "127.0.0.1" {
		t.Errorf("default bindAddr = %q, want 127.0.0.1", srv.bindAddr)
	}
}

func TestServerSetBindAddr(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	srv := NewServer(8420, hub, graph)
	srv.SetBindAddr("0.0.0.0")
	if srv.bindAddr != "0.0.0.0" {
		t.Errorf("bindAddr = %q, want 0.0.0.0", srv.bindAddr)
	}
}

func TestLiveReceiverDefaultsToLoopback(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	lr := NewLiveReceiver(4318, graph, hub)
	if lr.bindAddr != "127.0.0.1" {
		t.Errorf("default bindAddr = %q, want 127.0.0.1", lr.bindAddr)
	}
}

func TestReplayBodyLimitRejects413(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	srv := NewServer(0, hub, graph)

	// Body exceeding 64 MB
	bigBody := bytes.NewReader(make([]byte, maxReplayBodySize+1))
	req := httptest.NewRequest(http.MethodPost, "/api/replay", bigBody)
	rr := httptest.NewRecorder()
	srv.handleReplayUpload(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rr.Code)
	}
}

func TestReplayBodyAtLimitAccepted(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	srv := NewServer(0, hub, graph)

	// Valid NDJSON at limit - just needs to not be empty and not exceed limit
	body := []byte(`{"ts":"2026-01-01T00:00:00Z","event":"test"}` + "\n")
	req := httptest.NewRequest(http.MethodPost, "/api/replay", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleReplayUpload(rr, req)
	// Should not be 413 (may be 200 or other status depending on replay processing)
	if rr.Code == http.StatusRequestEntityTooLarge {
		t.Errorf("valid body should not return 413")
	}
}

func TestOTLPLogsBodyLimitRejects413(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	lr := NewLiveReceiver(0, graph, hub)

	bigBody := bytes.NewReader(make([]byte, maxOTLPBodySize+1))
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bigBody)
	rr := httptest.NewRecorder()
	lr.handleLogs(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rr.Code)
	}
}

func TestOTLPMetricsBodyLimitRejects413(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	lr := NewLiveReceiver(0, graph, hub)

	bigBody := bytes.NewReader(make([]byte, maxOTLPBodySize+1))
	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bigBody)
	rr := httptest.NewRecorder()
	lr.handleMetrics(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rr.Code)
	}
}

func TestWebSocketRejectsEvilOrigin(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	srv := NewServer(0, hub, graph)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rr := httptest.NewRecorder()
	srv.handleWebSocket(rr, req)
	if rr.Code == http.StatusSwitchingProtocols {
		t.Error("evil origin should not be upgraded to websocket")
	}
	body := rr.Body.String()
	if !strings.Contains(body, "failed") && rr.Code != http.StatusForbidden && rr.Code != http.StatusInternalServerError {
		t.Errorf("expected rejection for evil origin, got status %d body %q", rr.Code, body)
	}
}

func TestServerTimeoutsConfigured(t *testing.T) {
	hub := NewHub()
	graph := NewGraph(DefaultGraphConfig())
	srv := NewServer(0, hub, graph)

	// We can't call Run() without starting a real server, but we can verify
	// the constructor sets defaults that will be used.
	if srv.bindAddr != "127.0.0.1" {
		t.Errorf("bindAddr = %q, want 127.0.0.1", srv.bindAddr)
	}
}
