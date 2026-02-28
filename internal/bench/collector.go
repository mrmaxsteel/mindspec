package bench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

const maxCollectorBodySize = 4 << 20 // 4 MB

// Collector is a lightweight OTLP/HTTP JSON receiver that extracts
// Claude Code telemetry events and writes them as NDJSON.
type Collector struct {
	port       int
	output     string
	appendMode bool
	mu         sync.Mutex
	w          io.WriteCloser
	server     *http.Server
	count      int
}

// NewCollector creates a collector that listens on the given port
// and writes NDJSON to the output path (truncates existing file).
func NewCollector(port int, output string) *Collector {
	return &Collector{port: port, output: output}
}

// NewCollectorAppend creates a collector that appends to an existing output file
// instead of truncating it. Used for recording restarts.
func NewCollectorAppend(port int, output string) *Collector {
	return &Collector{port: port, output: output, appendMode: true}
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) error {
	var f *os.File
	var err error
	if c.appendMode {
		f, err = os.OpenFile(c.output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	} else {
		f, err = os.Create(c.output)
	}
	if err != nil {
		return fmt.Errorf("opening output file: %w", err)
	}
	c.w = f

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/logs", c.handleLogs)
	mux.HandleFunc("/v1/metrics", c.handleMetrics)

	c.server = &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", c.port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := c.server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	fmt.Fprintf(os.Stderr, "Collecting on :%d → %s (Ctrl-C to stop)\n", c.port, c.output)

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c.server.Shutdown(shutCtx) //nolint:errcheck

	c.mu.Lock()
	defer c.mu.Unlock()
	fmt.Fprintf(os.Stderr, "\nCollected %d events → %s\n", c.count, c.output)
	return c.w.Close()
}

// handleLogs processes OTLP/HTTP JSON log export requests.
// Extracts claude_code.api_request events.
func (c *Collector) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCollectorBodySize)
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

	events := extractLogEvents(body)
	c.writeEvents(events)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}")) //nolint:errcheck
}

// handleMetrics processes OTLP/HTTP JSON metric export requests.
// Extracts claude_code.token.usage and claude_code.cost.usage metrics.
func (c *Collector) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCollectorBodySize)
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

	events := extractMetricEvents(body)
	c.writeEvents(events)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}")) //nolint:errcheck
}

func (c *Collector) writeEvents(events []CollectedEvent) {
	if len(events) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		data = append(data, '\n')
		c.w.Write(data) //nolint:errcheck
		c.count++
	}
}

// CollectedEvent is the normalized NDJSON schema for collected telemetry.
type CollectedEvent struct {
	TS       string         `json:"ts"`
	Event    string         `json:"event"`
	Data     map[string]any `json:"data,omitempty"`
	Resource map[string]any `json:"resource,omitempty"`
}

// extractLogEvents parses an OTLP ExportLogsServiceRequest JSON body
// and extracts claude_code.api_request events.
// ExtractLogEvents is the exported wrapper for extractLogEvents.
func ExtractLogEvents(body []byte) []CollectedEvent {
	return extractLogEvents(body)
}

func extractLogEvents(body []byte) []CollectedEvent {
	// OTLP JSON structure (simplified):
	// { "resourceLogs": [ { "scopeLogs": [ { "logRecords": [ ... ] } ] } ] }
	var req struct {
		ResourceLogs []struct {
			Resource struct {
				Attributes []otlpKeyValue `json:"attributes"`
			} `json:"resource"`
			ScopeLogs []struct {
				LogRecords []struct {
					TimeUnixNano string         `json:"timeUnixNano"`
					Body         otlpValue      `json:"body"`
					Attributes   []otlpKeyValue `json:"attributes"`
				} `json:"logRecords"`
			} `json:"scopeLogs"`
		} `json:"resourceLogs"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}

	var events []CollectedEvent
	for _, rl := range req.ResourceLogs {
		var resAttrs map[string]any
		if len(rl.Resource.Attributes) > 0 {
			resAttrs = flattenAttributes(rl.Resource.Attributes)
		}

		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				attrs := flattenAttributes(lr.Attributes)

				// Determine event name: prefer the longest/most-qualified name.
				// Body has full name ("claude_code.api_request") in real Claude Code,
				// event.name attr may have short name ("api_request") or full name.
				bodyName := lr.Body.StringValue
				attrName, _ := attrs["event.name"].(string)
				eventName := bodyName
				if len(attrName) > len(eventName) {
					eventName = attrName
				}
				if eventName == "" {
					eventName = bodyName
				}

				if eventName == "" {
					continue
				}

				ts := parseOTLPTimestamp(lr.TimeUnixNano)
				e := CollectedEvent{
					TS:       ts,
					Event:    eventName,
					Data:     attrs,
					Resource: resAttrs,
				}
				delete(e.Data, "event.name")
				events = append(events, e)
			}
		}
	}
	return events
}

// extractMetricEvents parses OTLP ExportMetricsServiceRequest JSON body
// and extracts claude_code.token.usage and claude_code.cost.usage data points.
// ExtractMetricEvents is the exported wrapper for extractMetricEvents.
func ExtractMetricEvents(body []byte) []CollectedEvent {
	return extractMetricEvents(body)
}

func extractMetricEvents(body []byte) []CollectedEvent {
	var req struct {
		ResourceMetrics []struct {
			Resource struct {
				Attributes []otlpKeyValue `json:"attributes"`
			} `json:"resource"`
			ScopeMetrics []struct {
				Metrics []struct {
					Name string `json:"name"`
					Sum  *struct {
						DataPoints []struct {
							TimeUnixNano string         `json:"timeUnixNano"`
							AsInt        *int64         `json:"asInt"`
							AsDouble     *float64       `json:"asDouble"`
							Attributes   []otlpKeyValue `json:"attributes"`
						} `json:"dataPoints"`
					} `json:"sum"`
				} `json:"metrics"`
			} `json:"scopeMetrics"`
		} `json:"resourceMetrics"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}

	var events []CollectedEvent
	for _, rm := range req.ResourceMetrics {
		var resAttrs map[string]any
		if len(rm.Resource.Attributes) > 0 {
			resAttrs = flattenAttributes(rm.Resource.Attributes)
		}

		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Sum == nil {
					continue
				}
				for _, dp := range m.Sum.DataPoints {
					attrs := flattenAttributes(dp.Attributes)
					var value float64
					if dp.AsInt != nil {
						value = float64(*dp.AsInt)
					} else if dp.AsDouble != nil {
						value = *dp.AsDouble
					}
					attrs["value"] = value
					attrs["metric"] = m.Name

					ts := parseOTLPTimestamp(dp.TimeUnixNano)
					events = append(events, CollectedEvent{
						TS:       ts,
						Event:    m.Name,
						Data:     attrs,
						Resource: resAttrs,
					})
				}
			}
		}
	}
	return events
}

// otlpValue represents an OTLP AnyValue.
// IntValue uses json.RawMessage because OTLP sends it as either a string or number.
type otlpValue struct {
	StringValue string          `json:"stringValue"`
	IntValue    json.RawMessage `json:"intValue"`
	DoubleValue *float64        `json:"doubleValue"`
}

// otlpKeyValue represents an OTLP KeyValue.
type otlpKeyValue struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

// flattenAttributes converts OTLP attributes to a flat map.
func flattenAttributes(attrs []otlpKeyValue) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		if a.Value.StringValue != "" {
			m[a.Key] = a.Value.StringValue
		} else if len(a.Value.IntValue) > 0 {
			// IntValue can be a JSON string ("123") or number (123)
			var v int64
			s := string(a.Value.IntValue)
			// Strip quotes if present
			if len(s) >= 2 && s[0] == '"' {
				s = s[1 : len(s)-1]
			}
			fmt.Sscanf(s, "%d", &v)
			m[a.Key] = v
		} else if a.Value.DoubleValue != nil {
			m[a.Key] = *a.Value.DoubleValue
		}
	}
	return m
}

// parseOTLPTimestamp converts a nanosecond Unix timestamp string to RFC3339Nano.
func parseOTLPTimestamp(nanos string) string {
	var n int64
	fmt.Sscanf(nanos, "%d", &n)
	if n == 0 {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return time.Unix(0, n).UTC().Format(time.RFC3339Nano)
}
