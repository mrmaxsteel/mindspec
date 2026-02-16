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
	phase    string  // if set, filter to events within this lifecycle phase
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
	startTime := time.Now()
	sampleN := 1

	// Phase filtering state
	inPhase := r.phase == ""                           // if no filter, all events are "in phase"
	phaseStarted := r.phase == "" || r.phase == "spec" // spec phase starts at beginning

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

		// Phase filtering
		if r.phase != "" {
			switch e.Event {
			case "lifecycle.start":
				if r.phase == "spec" {
					inPhase = true
					phaseStarted = true
				}
			case "lifecycle.phase":
				to, _ := e.Data["to"].(string)
				if inPhase && phaseStarted {
					// We were in our phase, now leaving it
					inPhase = false
				}
				if to == r.phase {
					inPhase = true
					phaseStarted = true
				}
			case "lifecycle.end":
				if inPhase {
					inPhase = false
				}
			default:
				if !inPhase || !phaseStarted {
					continue
				}
			}
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
			r.graph.RecordEdgeStats(edge.Status)
		}

		// Record API-level stats from raw event data
		if isAPIRequestEvent(e.Event) {
			inTok := toInt64(e.Data["input_tokens"])
			outTok := toInt64(e.Data["output_tokens"])
			cost := toFloat64(e.Data["cost_usd"])
			r.graph.RecordAPIStats(inTok, outTok, cost)
		}

		// Codex token/cost metrics update totals without incrementing API call count.
		if inTok, outTok, cost, ok := metricStatsDelta(e.Event, e.Data); ok {
			r.graph.RecordTokenStats(inTok, outTok, cost)
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
			gstats := r.graph.Stats()
			elapsed := time.Since(startTime)
			eps := 0.0
			if elapsed.Seconds() > 0 {
				eps = float64(eventCount) / elapsed.Seconds()
			}

			r.hub.Broadcast(WSMessage{
				Type: MsgStats,
				Data: StatsData{
					EventsPerSec: eps,
					ErrorCount:   gstats.ErrorCount,
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
