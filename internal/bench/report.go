package bench

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

// Session holds aggregated metrics from one collected trace file.
type Session struct {
	Label        string
	APICallCount int
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheCreate  int64
	CostUSD      float64
	DurationMs   float64 // wall-clock: last event ts - first event ts
	FirstEvent   time.Time
	LastEvent    time.Time
	ModelBreakdown map[string]*ModelStats
}

// ModelStats tracks per-model metrics.
type ModelStats struct {
	Calls        int
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

// TotalTokens returns input + output tokens.
func (s *Session) TotalTokens() int64 {
	return s.InputTokens + s.OutputTokens
}

// CacheHitRate returns the ratio of cache read tokens to total input tokens.
func (s *Session) CacheHitRate() float64 {
	total := s.InputTokens + s.CacheRead + s.CacheCreate
	if total == 0 {
		return 0
	}
	return float64(s.CacheRead) / float64(total)
}

// ParseSession reads a collected NDJSON file and aggregates metrics.
func ParseSession(path, label string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	s := &Session{
		Label:          label,
		ModelBreakdown: make(map[string]*ModelStats),
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e CollectedEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}

		// Parse timestamp for wall-clock duration
		if ts, err := time.Parse(time.RFC3339Nano, e.TS); err == nil {
			if s.FirstEvent.IsZero() || ts.Before(s.FirstEvent) {
				s.FirstEvent = ts
			}
			if ts.After(s.LastEvent) {
				s.LastEvent = ts
			}
		}

		switch e.Event {
		case "claude_code.api_request", "claude.api_request":
			s.APICallCount++

			inputTok := toInt64(e.Data["input_tokens"])
			outputTok := toInt64(e.Data["output_tokens"])
			cacheRead := toInt64(e.Data["cache_read_tokens"])
			cacheCreate := toInt64(e.Data["cache_creation_tokens"])
			cost := toFloat64(e.Data["cost_usd"])
			model, _ := e.Data["model"].(string)

			s.InputTokens += inputTok
			s.OutputTokens += outputTok
			s.CacheRead += cacheRead
			s.CacheCreate += cacheCreate
			s.CostUSD += cost

			if model != "" {
				ms, ok := s.ModelBreakdown[model]
				if !ok {
					ms = &ModelStats{}
					s.ModelBreakdown[model] = ms
				}
				ms.Calls++
				ms.InputTokens += inputTok
				ms.OutputTokens += outputTok
				ms.CostUSD += cost
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	if !s.FirstEvent.IsZero() && !s.LastEvent.IsZero() {
		s.DurationMs = float64(s.LastEvent.Sub(s.FirstEvent).Milliseconds())
	}

	return s, nil
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

// Report holds the comparison between two sessions.
type Report struct {
	A     *Session
	B     *Session
	Delta *SessionDelta
}

// SessionDelta holds the differences between two sessions.
type SessionDelta struct {
	APICallCount  int
	InputTokens   int64
	OutputTokens  int64
	CacheRead     int64
	CacheCreate   int64
	CostUSD       float64
	DurationMs    float64
	TotalTokens   int64
}

// Compare computes the delta between two sessions. Positive delta means A > B.
func Compare(a, b *Session) *Report {
	return &Report{
		A: a,
		B: b,
		Delta: &SessionDelta{
			APICallCount: a.APICallCount - b.APICallCount,
			InputTokens:  a.InputTokens - b.InputTokens,
			OutputTokens: a.OutputTokens - b.OutputTokens,
			CacheRead:    a.CacheRead - b.CacheRead,
			CacheCreate:  a.CacheCreate - b.CacheCreate,
			CostUSD:      a.CostUSD - b.CostUSD,
			DurationMs:   a.DurationMs - b.DurationMs,
			TotalTokens:  a.TotalTokens() - b.TotalTokens(),
		},
	}
}

// FormatTable produces a human-readable comparison table.
func FormatTable(r *Report) string {
	labelA := r.A.Label
	labelB := r.B.Label
	if labelA == "" {
		labelA = "Session A"
	}
	if labelB == "" {
		labelB = "Session B"
	}

	out := fmt.Sprintf("%-22s %15s %15s %15s\n", "Metric", labelA, labelB, "Delta")
	out += fmt.Sprintf("%s\n", "────────────────────────────────────────────────────────────────────")

	out += fmtRow("API Calls", r.A.APICallCount, r.B.APICallCount, r.Delta.APICallCount)
	out += fmtRow64("Input Tokens", r.A.InputTokens, r.B.InputTokens, r.Delta.InputTokens)
	out += fmtRow64("Output Tokens", r.A.OutputTokens, r.B.OutputTokens, r.Delta.OutputTokens)
	out += fmtRow64("Cache Read Tokens", r.A.CacheRead, r.B.CacheRead, r.Delta.CacheRead)
	out += fmtRow64("Cache Create Tokens", r.A.CacheCreate, r.B.CacheCreate, r.Delta.CacheCreate)
	out += fmtRow64("Total Tokens", r.A.TotalTokens(), r.B.TotalTokens(), r.Delta.TotalTokens)

	out += fmt.Sprintf("%-22s %15s %15s %15s\n",
		"Cost (USD)",
		fmt.Sprintf("$%.4f", r.A.CostUSD),
		fmt.Sprintf("$%.4f", r.B.CostUSD),
		fmtDeltaFloat(r.Delta.CostUSD, "$"))

	out += fmt.Sprintf("%-22s %15s %15s %15s\n",
		"Duration",
		fmtDuration(r.A.DurationMs),
		fmtDuration(r.B.DurationMs),
		fmtDeltaDuration(r.Delta.DurationMs))

	// Cache hit rates
	out += fmt.Sprintf("%-22s %14.1f%% %14.1f%%\n",
		"Cache Hit Rate",
		r.A.CacheHitRate()*100,
		r.B.CacheHitRate()*100)

	// Efficiency: output per input
	effA := float64(0)
	effB := float64(0)
	if r.A.InputTokens > 0 {
		effA = float64(r.A.OutputTokens) / float64(r.A.InputTokens)
	}
	if r.B.InputTokens > 0 {
		effB = float64(r.B.OutputTokens) / float64(r.B.InputTokens)
	}
	out += fmt.Sprintf("%-22s %14.2fx %14.2fx\n", "Output/Input Ratio", effA, effB)

	return out
}

// FormatJSON produces a machine-readable JSON report.
func FormatJSON(r *Report) (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func fmtRow(label string, a, b, delta int) string {
	d := fmt.Sprintf("%+d", delta)
	return fmt.Sprintf("%-22s %15d %15d %15s\n", label, a, b, d)
}

func fmtRow64(label string, a, b, delta int64) string {
	d := fmt.Sprintf("%+d", delta)
	return fmt.Sprintf("%-22s %15d %15d %15s\n", label, a, b, d)
}

func fmtDeltaFloat(v float64, prefix string) string {
	sign := "+"
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%s%.4f", sign, prefix, v)
}

func fmtDuration(ms float64) string {
	if ms < 1000 {
		return fmt.Sprintf("%.0f ms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1f s", ms/1000)
	}
	return fmt.Sprintf("%.1f min", ms/60000)
}

func fmtDeltaDuration(ms float64) string {
	sign := "+"
	if ms < 0 {
		sign = "-"
		ms = math.Abs(ms)
	}
	return sign + fmtDuration(ms)
}
