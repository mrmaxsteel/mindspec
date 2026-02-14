package viz

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/mindspec/mindspec/internal/bench"
)

// LiveReceiver is an OTLP/HTTP receiver that normalizes events into the graph.
type LiveReceiver struct {
	otlpPort   int
	graph      *Graph
	hub        *Hub
	server     *http.Server
	eventCount atomic.Int64
	sampling   atomic.Bool
	startTime  time.Time
}

// NewLiveReceiver creates a new OTLP receiver for live mode.
func NewLiveReceiver(otlpPort int, graph *Graph, hub *Hub) *LiveReceiver {
	return &LiveReceiver{
		otlpPort:  otlpPort,
		graph:     graph,
		hub:       hub,
		startTime: time.Now(),
	}
}

// Run starts the OTLP HTTP server and blocks until ctx is cancelled.
func (l *LiveReceiver) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/logs", l.handleLogs)
	mux.HandleFunc("/v1/metrics", l.handleMetrics)

	l.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", l.otlpPort),
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := l.server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	fmt.Fprintf(os.Stderr, "OTLP receiver listening on :%d\n", l.otlpPort)

	// Stats ticker
	go l.statsLoop(ctx)

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return l.server.Shutdown(shutCtx)
}

func (l *LiveReceiver) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	events := bench.ExtractLogEvents(body)
	l.processEvents(events)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}")) //nolint:errcheck
}

func (l *LiveReceiver) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	events := bench.ExtractMetricEvents(body)
	l.processEvents(events)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}")) //nolint:errcheck
}

func (l *LiveReceiver) processEvents(events []bench.CollectedEvent) {
	count := l.eventCount.Load()
	elapsed := time.Since(l.startTime)
	sampleN := int64(1)
	if elapsed.Seconds() > 0 {
		rate := float64(count) / elapsed.Seconds()
		if rate > 100 {
			sampleN = int64(rate/100) + 1
			l.sampling.Store(true)
		} else {
			l.sampling.Store(false)
		}
	}

	for _, e := range events {
		n := l.eventCount.Add(1)
		if sampleN > 1 && n%sampleN != 0 {
			continue
		}

		nodes, edges := NormalizeEvent(e)
		for _, node := range nodes {
			l.graph.UpsertNode(node)
		}
		for _, edge := range edges {
			l.graph.AddEdge(edge)
			l.graph.RecordEdgeStats(edge.Status)
		}

		// Record API-level stats from raw event data
		if e.Event == "claude_code.api_request" {
			inTok, _ := e.Data["input_tokens"].(float64)
			outTok, _ := e.Data["output_tokens"].(float64)
			cost, _ := e.Data["cost_usd"].(float64)
			l.graph.RecordAPIStats(int64(inTok), int64(outTok), cost)
		}

		// Broadcast update
		update := struct {
			Nodes []NodeUpsert `json:"nodes,omitempty"`
			Edges []EdgeEvent  `json:"edges,omitempty"`
		}{Nodes: nodes, Edges: edges}

		l.hub.Broadcast(WSMessage{Type: MsgUpdate, Data: update})
	}
}

func (l *LiveReceiver) statsLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			capped := l.graph.Tick()
			gstats := l.graph.Stats()
			count := l.eventCount.Load()
			elapsed := time.Since(l.startTime)
			eps := 0.0
			if elapsed.Seconds() > 0 {
				eps = float64(count) / elapsed.Seconds()
			}

			l.hub.Broadcast(WSMessage{
				Type: MsgStats,
				Data: StatsData{
					EventsPerSec: eps,
					ErrorCount:   gstats.ErrorCount,
					Connected:    true,
					Capped:       capped,
					Dropped:      l.hub.Dropped(),
					Sampling:     l.sampling.Load(),
					Mode:         "live",
				},
			})
		}
	}
}
