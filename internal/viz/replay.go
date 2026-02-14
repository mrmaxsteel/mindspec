package viz

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mindspec/mindspec/internal/bench"
)

// Replay reads a NDJSON file and streams events to the graph and hub.
type Replay struct {
	path     string
	speed    float64 // 0 = max speed
	graph    *Graph
	hub      *Hub
	sampling bool
}

// NewReplay creates a new replayer.
func NewReplay(path string, speed float64, graph *Graph, hub *Hub) *Replay {
	return &Replay{
		path:  path,
		speed: speed,
		graph: graph,
		hub:   hub,
	}
}

// Run replays the file. Blocks until complete or ctx is cancelled.
func (r *Replay) Run(ctx context.Context) error {
	f, err := os.Open(r.path)
	if err != nil {
		return fmt.Errorf("opening replay file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Increase buffer for large lines
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var prevTS time.Time
	var eventCount int
	var errorCount int
	var totalLatencyMs float64
	startTime := time.Now()
	sampleN := 1

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var e bench.CollectedEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}

		eventCount++

		// Sampling: if event rate exceeds 100 events/sec (only in timed mode)
		if r.speed > 0 {
			elapsed := time.Since(startTime)
			if elapsed.Seconds() > 0.1 {
				rate := float64(eventCount) / elapsed.Seconds()
				if rate > 100 {
					sampleN = int(rate/100) + 1
					r.sampling = true
				} else {
					sampleN = 1
					r.sampling = false
				}
			}
			if sampleN > 1 && eventCount%sampleN != 0 {
				continue
			}
		}

		// Speed control
		if r.speed > 0 {
			ts := parseTimestamp(e.TS)
			if !prevTS.IsZero() && !ts.IsZero() {
				delay := ts.Sub(prevTS)
				if delay > 0 {
					scaledDelay := time.Duration(float64(delay) / r.speed)
					if scaledDelay > 0 {
						timer := time.NewTimer(scaledDelay)
						select {
						case <-ctx.Done():
							timer.Stop()
							return ctx.Err()
						case <-timer.C:
						}
					}
				}
			}
			prevTS = ts
		}

		// Normalize and apply
		nodes, edges := NormalizeEvent(e)
		for _, n := range nodes {
			r.graph.UpsertNode(n)
		}
		for _, edge := range edges {
			r.graph.AddEdge(edge)
			if edge.Status == "error" {
				errorCount++
			}
			if edge.Duration > 0 {
				totalLatencyMs += float64(edge.Duration) / float64(time.Millisecond)
			}
		}

		// Broadcast update
		update := struct {
			Nodes []NodeUpsert `json:"nodes,omitempty"`
			Edges []EdgeEvent  `json:"edges,omitempty"`
		}{Nodes: nodes, Edges: edges}

		r.hub.Broadcast(WSMessage{Type: MsgUpdate, Data: update})

		// Periodic tick + stats
		if eventCount%10 == 0 {
			capped := r.graph.Tick()
			elapsed := time.Since(startTime)
			eps := 0.0
			if elapsed.Seconds() > 0 {
				eps = float64(eventCount) / elapsed.Seconds()
			}
			avgLatency := 0.0
			if eventCount > 0 {
				avgLatency = totalLatencyMs / float64(eventCount)
			}

			r.hub.Broadcast(WSMessage{
				Type: MsgStats,
				Data: StatsData{
					EventsPerSec: eps,
					ErrorCount:   errorCount,
					AvgLatencyMs: avgLatency,
					Connected:    true,
					Capped:       capped,
					Dropped:      r.hub.Dropped(),
					Sampling:     r.sampling,
					Mode:         "replay",
				},
			})
		}
	}

	// Final stats
	r.graph.Tick()
	snap := r.graph.Snapshot()
	fmt.Fprintf(os.Stderr, "Replay complete: %d events, %d nodes, %d edges\n",
		eventCount, len(snap.Nodes), len(snap.Edges))

	return scanner.Err()
}
