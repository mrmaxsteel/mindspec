package viz

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"nhooyr.io/websocket"
)

//go:embed web/*
var webFS embed.FS

// Server serves the web UI and WebSocket endpoint.
type Server struct {
	port   int
	hub    *Hub
	graph  *Graph
	server *http.Server
}

// NewServer creates a new visualization server.
func NewServer(port int, hub *Hub, graph *Graph) *Server {
	return &Server{
		port:  port,
		hub:   hub,
		graph: graph,
	}
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

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
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
		InsecureSkipVerify: true, // localhost only
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
