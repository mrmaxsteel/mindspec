package viz

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

const maxReplayBodySize = 64 << 20 // 64 MB

//go:embed web/*
var webFS embed.FS

// Server serves the web UI and WebSocket endpoint.
type Server struct {
	port     int
	bindAddr string
	hub      *Hub
	graph    *Graph
	live     *LiveReceiver // optional, set for live mode
	server   *http.Server

	// Active replay cancellation
	replayMu     sync.Mutex
	replayCancel context.CancelFunc
}

// NewServer creates a new visualization server.
func NewServer(port int, hub *Hub, graph *Graph) *Server {
	return &Server{
		port:     port,
		bindAddr: "127.0.0.1",
		hub:      hub,
		graph:    graph,
	}
}

// SetBindAddr sets the address to bind to (default "127.0.0.1").
func (s *Server) SetBindAddr(addr string) {
	s.bindAddr = addr
}

// SetLiveReceiver attaches a live receiver for save-recording support.
func (s *Server) SetLiveReceiver(lr *LiveReceiver) {
	s.live = lr
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("extracting web assets: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(webContent)))
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/api/replay", s.handleReplayUpload)
	mux.HandleFunc("/api/reset", s.handleReset)
	mux.HandleFunc("/api/save-recording", s.handleSaveRecording)
	mux.HandleFunc("/api/debug-events", s.handleDebugEvents)

	s.server = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", s.bindAddr, s.port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	fmt.Printf("AgentMind Viz: http://localhost:%d\n", s.port)

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(shutCtx)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost", "127.0.0.1", "[::1]"},
	})
	if err != nil {
		http.Error(w, "websocket upgrade failed", http.StatusInternalServerError)
		return
	}

	client := NewClient(s.hub, conn)

	// Send initial snapshot
	snap := s.graph.Snapshot()
	data, err := json.Marshal(WSMessage{
		Type: MsgSnapshot,
		Data: snap,
	})
	if err == nil {
		select {
		case client.send <- data:
		default:
		}
	}

	ctx := r.Context()
	go client.WritePump(ctx)
	client.ReadPump(ctx)
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.graph.Reset()
	s.hub.Broadcast(WSMessage{Type: MsgSnapshot, Data: s.graph.Snapshot()})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
}

func (s *Server) handleReplayUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse speed from query param (default 1.0)
	speed := 1.0
	if sp := r.URL.Query().Get("speed"); sp != "" {
		if v, err := strconv.ParseFloat(sp, 64); err == nil && v >= 0 {
			speed = v
		}
	}

	// Read uploaded file body with size limit
	r.Body = http.MaxBytesReader(w, r.Body, maxReplayBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// Write to temp file for the Replay struct to read
	tmpFile, err := os.CreateTemp("", "viz-replay-*.jsonl")
	if err != nil {
		http.Error(w, "failed to create temp file", http.StatusInternalServerError)
		return
	}
	if _, err := tmpFile.Write(body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		http.Error(w, "failed to write temp file", http.StatusInternalServerError)
		return
	}
	tmpFile.Close()
	tmpPath := tmpFile.Name()

	// Cancel any in-progress replay
	s.replayMu.Lock()
	if s.replayCancel != nil {
		s.replayCancel()
	}

	// Reset graph and notify clients
	s.graph.Reset()
	s.hub.Broadcast(WSMessage{Type: MsgSnapshot, Data: s.graph.Snapshot()})

	// Start new replay in background
	ctx, cancel := context.WithCancel(context.Background())
	s.replayCancel = cancel
	s.replayMu.Unlock()

	go func() {
		defer os.Remove(tmpPath)
		defer cancel()

		replay := NewReplay(tmpPath, speed, s.graph, s.hub)
		if err := replay.Run(ctx); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "UI replay error: %v\n", err)
		}

		// Broadcast final "done" stats
		gstats := s.graph.Stats()
		s.hub.Broadcast(WSMessage{
			Type: MsgStats,
			Data: StatsData{
				EventsPerSec: 0,
				ErrorCount:   gstats.ErrorCount,
				Connected:    true,
				Mode:         "replay-done",
			},
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"}) //nolint:errcheck
}

func (s *Server) handleDebugEvents(w http.ResponseWriter, r *http.Request) {
	if s.live == nil {
		http.Error(w, "only available in live mode", http.StatusBadRequest)
		return
	}

	s.live.eventBufMu.Lock()
	counts := make(map[string]int)
	for _, e := range s.live.eventBuf {
		counts[e.Event]++
	}
	total := len(s.live.eventBuf)
	s.live.eventBufMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total":  total,
		"counts": counts,
	}) //nolint:errcheck
}

func (s *Server) handleSaveRecording(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.live == nil {
		http.Error(w, "save-recording only available in live mode", http.StatusBadRequest)
		return
	}

	data, count := s.live.EventsNDJSON()
	if count == 0 {
		http.Error(w, "no events captured", http.StatusNotFound)
		return
	}

	filename := fmt.Sprintf("agentmind-%s.ndjson", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data) //nolint:errcheck
}
