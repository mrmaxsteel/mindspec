package viz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bench"
)

const maxOTLPBodySize = 4 << 20 // 4 MB

// LiveReceiver is an OTLP/HTTP receiver that normalizes events into the graph.
type LiveReceiver struct {
	otlpPort   int
	bindAddr   string
	graph      *Graph
	hub        *Hub
	server     *http.Server
	eventCount atomic.Int64
	sampling   atomic.Bool
	startTime  time.Time

	// Event buffer for save-recording
	eventBuf   []bench.CollectedEvent
	eventBufMu sync.Mutex

	// NDJSON disk output (optional)
	outputPath string
	outputFile *os.File
	outputMu   sync.Mutex
}

// NewLiveReceiver creates a new OTLP receiver for live mode.
func NewLiveReceiver(otlpPort int, graph *Graph, hub *Hub) *LiveReceiver {
	return &LiveReceiver{
		otlpPort:  otlpPort,
		bindAddr:  "127.0.0.1",
		graph:     graph,
		hub:       hub,
		startTime: time.Now(),
	}
}

// SetBindAddr sets the address to bind to (default "127.0.0.1").
func (l *LiveReceiver) SetBindAddr(addr string) {
	l.bindAddr = addr
}

// SetOutput configures NDJSON disk output. Must be called before Run().
// Events are appended to the file as they arrive.
func (l *LiveReceiver) SetOutput(path string) {
	l.outputPath = path
}

// Run starts the OTLP HTTP server and blocks until ctx is cancelled.
func (l *LiveReceiver) Run(ctx context.Context) error {
	if l.outputPath != "" {
		f, err := os.OpenFile(l.outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("opening output file: %w", err)
		}
		l.outputFile = f
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/logs", l.handleLogs)
	mux.HandleFunc("/v1/metrics", l.handleMetrics)

	l.server = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", l.bindAddr, l.otlpPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
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
	if err := l.server.Shutdown(shutCtx); err != nil {
		return err
	}

	if l.outputFile != nil {
		l.outputMu.Lock()
		defer l.outputMu.Unlock()
		return l.outputFile.Close()
	}
	return nil
}

func (l *LiveReceiver) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxOTLPBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
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

	r.Body = http.MaxBytesReader(w, r.Body, maxOTLPBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	events := bench.ExtractMetricEvents(body)
	l.processEvents(events)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}")) //nolint:errcheck
}

func (l *LiveReceiver) processEvents(events []bench.CollectedEvent) {
	// Buffer all events for save-recording
	l.eventBufMu.Lock()
	l.eventBuf = append(l.eventBuf, events...)
	l.eventBufMu.Unlock()

	// Write to NDJSON disk output if configured
	if l.outputFile != nil {
		l.outputMu.Lock()
		for _, e := range events {
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}
			data = append(data, '\n')
			l.outputFile.Write(data) //nolint:errcheck
		}
		l.outputMu.Unlock()
	}

	// Debug: log event details to stderr for tool_result events
	for _, e := range events {
		if isToolResultEvent(e.Event) {
			data, _ := json.Marshal(e.Data)
			fmt.Fprintf(os.Stderr, "[otlp] event=%q data=%s\n", e.Event, data)
		}
	}

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
		if isAPIRequestEvent(e.Event) {
			inTok := toInt64(e.Data["input_tokens"])
			outTok := toInt64(e.Data["output_tokens"])
			cost := toFloat64(e.Data["cost_usd"])
			l.graph.RecordAPIStats(inTok, outTok, cost)
		}

		// Codex token/cost metrics update totals without incrementing API call count.
		if inTok, outTok, cost, ok := metricStatsDelta(e.Event, e.Data); ok {
			l.graph.RecordTokenStats(inTok, outTok, cost)
		}

		// Broadcast update
		update := struct {
			Nodes []NodeUpsert `json:"nodes,omitempty"`
			Edges []EdgeEvent  `json:"edges,omitempty"`
		}{Nodes: nodes, Edges: edges}

		l.hub.Broadcast(WSMessage{Type: MsgUpdate, Data: update})
	}
}

// EventsNDJSON returns all buffered events as NDJSON bytes.
func (l *LiveReceiver) EventsNDJSON() ([]byte, int) {
	l.eventBufMu.Lock()
	events := make([]bench.CollectedEvent, len(l.eventBuf))
	copy(events, l.eventBuf)
	l.eventBufMu.Unlock()

	var buf []byte
	for _, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			continue
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	return buf, len(events)
}

// ClearEvents resets the event buffer.
func (l *LiveReceiver) ClearEvents() {
	l.eventBufMu.Lock()
	l.eventBuf = nil
	l.eventBufMu.Unlock()
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
