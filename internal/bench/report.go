package bench

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// Session holds aggregated metrics from one collected trace file.
type Session struct {
	Label          string
	APICallCount   int
	InputTokens    int64
	OutputTokens   int64
	CacheRead      int64
	CacheCreate    int64
	CostUSD        float64
	DurationMs     float64 // wall-clock: last event ts - first event ts
	FirstEvent     time.Time
	LastEvent      time.Time
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

// ParseSession reads a collected NDJSON file and aggregates all events (no filtering).
// Use this for standalone per-session files or backward compatibility with legacy files.
func ParseSession(path, label string) (*Session, error) {
	return parseSessionFiltered(path, label, "")
}

// ParseSessionByLabel reads a shared NDJSON file and aggregates only events
// where Resource["bench.label"] matches the given label. This is used when
// multiple sessions write to a single collector/file differentiated by
// OTEL_RESOURCE_ATTRIBUTES.
func ParseSessionByLabel(path, label string) (*Session, error) {
	return parseSessionFiltered(path, label, label)
}

// parseSessionFiltered reads events from an NDJSON file. If filterLabel is non-empty,
// only events where Resource["bench.label"] matches filterLabel are aggregated.
func parseSessionFiltered(path, label, filterLabel string) (*Session, error) {
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

		// Filter by bench.label if requested
		if filterLabel != "" {
			bl, _ := e.Resource["bench.label"].(string)
			if bl != filterLabel {
				continue
			}
		}

		aggregateEvent(s, &e)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	if !s.FirstEvent.IsZero() && !s.LastEvent.IsZero() {
		s.DurationMs = float64(s.LastEvent.Sub(s.FirstEvent).Milliseconds())
	}

	return s, nil
}

// ExtractSessionIDs scans an NDJSON file for unique session.id values
// where Resource["bench.label"] matches the given label.
func ExtractSessionIDs(path, label string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e CollectedEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		bl, _ := e.Resource["bench.label"].(string)
		if bl != label {
			continue
		}
		sid, _ := e.Data["session.id"].(string)
		if sid != "" {
			seen[sid] = struct{}{}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// aggregateEvent adds one CollectedEvent's metrics to a Session.
func aggregateEvent(s *Session, e *CollectedEvent) {
	// Parse timestamp for wall-clock duration
	if ts, err := time.Parse(time.RFC3339Nano, e.TS); err == nil {
		if s.FirstEvent.IsZero() || ts.Before(s.FirstEvent) {
			s.FirstEvent = ts
		}
		if ts.After(s.LastEvent) {
			s.LastEvent = ts
		}
	}

	switch {
	case isEventAlias(e.Event, "api_request"):
		// Legacy log-based events with flat fields
		s.APICallCount++

		inputTok := firstInt64Data(e.Data, "input_tokens", "gen_ai.usage.input_tokens")
		outputTok := firstInt64Data(e.Data, "output_tokens", "gen_ai.usage.output_tokens")
		cacheRead := firstInt64Data(e.Data, "cache_read_tokens", "cache_read_input_tokens", "gen_ai.usage.cache_read_input_tokens")
		cacheCreate := firstInt64Data(e.Data, "cache_creation_tokens", "cache_creation_input_tokens", "gen_ai.usage.cache_creation_input_tokens")
		cost := firstFloat64Data(e.Data, "cost_usd", "gen_ai.usage.cost_usd")
		model := firstStringData(e.Data, "model", "gen_ai.request.model", "model_name")

		s.InputTokens += inputTok
		s.OutputTokens += outputTok
		s.CacheRead += cacheRead
		s.CacheCreate += cacheCreate
		s.CostUSD += cost

		if model != "" {
			ms := getOrCreateModel(s, model)
			ms.Calls++
			ms.InputTokens += inputTok
			ms.OutputTokens += outputTok
			ms.CostUSD += cost
		}

	case isEventAlias(e.Event, "token.usage"):
		// OTLP metric events: data.type = input|output|cacheRead|cacheCreation, data.value = delta
		tokType := normalizeTokenType(firstStringData(e.Data, "type", "token_type", "usage_type", "kind"))
		val := toInt64(e.Data["value"])
		switch tokType {
		case "input":
			s.InputTokens += val
		case "output":
			s.OutputTokens += val
		case "cache_read":
			s.CacheRead += val
		case "cache_creation":
			s.CacheCreate += val
		}
		if model := firstStringData(e.Data, "model", "gen_ai.request.model", "model_name"); model != "" {
			ms := getOrCreateModel(s, model)
			switch tokType {
			case "input":
				ms.InputTokens += val
			case "output":
				ms.OutputTokens += val
			}
		}

	case isEventAlias(e.Event, "cost.usage"):
		// OTLP metric events: data.value = cost delta (USD)
		cost := toFloat64(e.Data["value"])
		s.CostUSD += cost
		if model := firstStringData(e.Data, "model", "gen_ai.request.model", "model_name"); model != "" {
			ms := getOrCreateModel(s, model)
			ms.CostUSD += cost
		}
	}
}

func isEventAlias(eventName, suffix string) bool {
	return eventName == suffix || strings.HasSuffix(eventName, "."+suffix)
}

func normalizeTokenType(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	replacer := strings.NewReplacer("-", "_", ".", "_", " ", "_")
	v = replacer.Replace(v)
	switch v {
	case "cache_read", "cacheread", "cache_read_input":
		return "cache_read"
	case "cache_creation", "cachecreate", "cache_creation_input", "cachecreation":
		return "cache_creation"
	case "output", "out":
		return "output"
	case "input", "in":
		return "input"
	default:
		return v
	}
}

func firstInt64Data(data map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if v, ok := data[key]; ok {
			return toInt64(v)
		}
	}
	return 0
}

func firstFloat64Data(data map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := data[key]; ok {
			return toFloat64(v)
		}
	}
	return 0
}

func firstStringData(data map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := data[key]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// mergedModelNames returns the sorted union of model names from both sessions.
func mergedModelNames(a, b *Session) []string {
	seen := make(map[string]struct{})
	for m := range a.ModelBreakdown {
		seen[m] = struct{}{}
	}
	for m := range b.ModelBreakdown {
		seen[m] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for m := range seen {
		names = append(names, m)
	}
	sort.Strings(names)
	return names
}

func getOrCreateModel(s *Session, model string) *ModelStats {
	ms, ok := s.ModelBreakdown[model]
	if !ok {
		ms = &ModelStats{}
		s.ModelBreakdown[model] = ms
	}
	return ms
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
	APICallCount int
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheCreate  int64
	CostUSD      float64
	DurationMs   float64
	TotalTokens  int64
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

	// Per-model breakdown
	models := mergedModelNames(r.A, r.B)
	if len(models) > 0 {
		out += fmt.Sprintf("\n%-22s %15s %15s %15s\n", "Per-Model Breakdown", labelA, labelB, "Delta")
		out += fmt.Sprintf("%s\n", "────────────────────────────────────────────────────────────────────")
		for _, model := range models {
			msA := r.A.ModelBreakdown[model]
			msB := r.B.ModelBreakdown[model]
			if msA == nil {
				msA = &ModelStats{}
			}
			if msB == nil {
				msB = &ModelStats{}
			}
			out += fmt.Sprintf("  %s\n", model)
			out += fmtRow64("    Tokens In", msA.InputTokens, msB.InputTokens, msA.InputTokens-msB.InputTokens)
			out += fmtRow64("    Tokens Out", msA.OutputTokens, msB.OutputTokens, msA.OutputTokens-msB.OutputTokens)
			out += fmt.Sprintf("%-22s %15s %15s %15s\n",
				"    Cost",
				fmt.Sprintf("$%.4f", msA.CostUSD),
				fmt.Sprintf("$%.4f", msB.CostUSD),
				fmtDeltaFloat(msA.CostUSD-msB.CostUSD, "$"))
		}
	}

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

// MultiReport holds an N-way comparison of sessions (no deltas).
type MultiReport struct {
	Sessions []*Session
}

// CompareN creates an N-way comparison from 2 or more sessions.
func CompareN(sessions []*Session) *MultiReport {
	return &MultiReport{Sessions: sessions}
}

// FormatTableN produces a human-readable N-way side-by-side table (no delta column).
func FormatTableN(r *MultiReport) string {
	n := len(r.Sessions)
	if n == 0 {
		return ""
	}

	// Determine column width based on longest label
	colW := 15
	for _, s := range r.Sessions {
		l := s.Label
		if l == "" {
			l = "Session"
		}
		if len(l)+2 > colW {
			colW = len(l) + 2
		}
	}

	labels := make([]string, n)
	for i, s := range r.Sessions {
		labels[i] = s.Label
		if labels[i] == "" {
			labels[i] = fmt.Sprintf("Session %d", i+1)
		}
	}

	// Header
	out := fmt.Sprintf("%-22s", "Metric")
	for _, l := range labels {
		out += fmt.Sprintf(" %*s", colW, l)
	}
	out += "\n"

	// Separator
	sepLen := 22 + n*(colW+1)
	for i := 0; i < sepLen; i++ {
		out += "─"
	}
	out += "\n"

	// API Calls
	out += fmtRowN("API Calls", colW, mapInt(r.Sessions, func(s *Session) int64 { return int64(s.APICallCount) }))
	out += fmtRowN("Input Tokens", colW, mapInt(r.Sessions, func(s *Session) int64 { return s.InputTokens }))
	out += fmtRowN("Output Tokens", colW, mapInt(r.Sessions, func(s *Session) int64 { return s.OutputTokens }))
	out += fmtRowN("Cache Read Tokens", colW, mapInt(r.Sessions, func(s *Session) int64 { return s.CacheRead }))
	out += fmtRowN("Cache Create Tokens", colW, mapInt(r.Sessions, func(s *Session) int64 { return s.CacheCreate }))
	out += fmtRowN("Total Tokens", colW, mapInt(r.Sessions, func(s *Session) int64 { return s.TotalTokens() }))

	// Cost
	out += fmt.Sprintf("%-22s", "Cost (USD)")
	for _, s := range r.Sessions {
		out += fmt.Sprintf(" %*s", colW, fmt.Sprintf("$%.4f", s.CostUSD))
	}
	out += "\n"

	// Duration
	out += fmt.Sprintf("%-22s", "Duration")
	for _, s := range r.Sessions {
		out += fmt.Sprintf(" %*s", colW, fmtDuration(s.DurationMs))
	}
	out += "\n"

	// Cache Hit Rate
	out += fmt.Sprintf("%-22s", "Cache Hit Rate")
	for _, s := range r.Sessions {
		out += fmt.Sprintf(" %*.1f%%", colW-1, s.CacheHitRate()*100)
	}
	out += "\n"

	// Output/Input Ratio
	out += fmt.Sprintf("%-22s", "Output/Input Ratio")
	for _, s := range r.Sessions {
		eff := float64(0)
		if s.InputTokens > 0 {
			eff = float64(s.OutputTokens) / float64(s.InputTokens)
		}
		out += fmt.Sprintf(" %*.2fx", colW-1, eff)
	}
	out += "\n"

	// Per-model breakdown
	models := mergedModelNamesN(r.Sessions)
	if len(models) > 0 {
		out += "\n"
		out += fmt.Sprintf("%-22s", "Per-Model Breakdown")
		for _, l := range labels {
			out += fmt.Sprintf(" %*s", colW, l)
		}
		out += "\n"
		for i := 0; i < sepLen; i++ {
			out += "─"
		}
		out += "\n"

		for _, model := range models {
			out += fmt.Sprintf("  %s\n", model)

			// Tokens In
			out += fmt.Sprintf("%-22s", "    Tokens In")
			for _, s := range r.Sessions {
				ms := s.ModelBreakdown[model]
				if ms == nil {
					out += fmt.Sprintf(" %*d", colW, 0)
				} else {
					out += fmt.Sprintf(" %*d", colW, ms.InputTokens)
				}
			}
			out += "\n"

			// Tokens Out
			out += fmt.Sprintf("%-22s", "    Tokens Out")
			for _, s := range r.Sessions {
				ms := s.ModelBreakdown[model]
				if ms == nil {
					out += fmt.Sprintf(" %*d", colW, 0)
				} else {
					out += fmt.Sprintf(" %*d", colW, ms.OutputTokens)
				}
			}
			out += "\n"

			// Cost
			out += fmt.Sprintf("%-22s", "    Cost")
			for _, s := range r.Sessions {
				ms := s.ModelBreakdown[model]
				if ms == nil {
					out += fmt.Sprintf(" %*s", colW, "$0.0000")
				} else {
					out += fmt.Sprintf(" %*s", colW, fmt.Sprintf("$%.4f", ms.CostUSD))
				}
			}
			out += "\n"
		}
	}

	return out
}

// mergedModelNamesN returns the sorted union of model names from all sessions.
func mergedModelNamesN(sessions []*Session) []string {
	seen := make(map[string]struct{})
	for _, s := range sessions {
		for m := range s.ModelBreakdown {
			seen[m] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for m := range seen {
		names = append(names, m)
	}
	sort.Strings(names)
	return names
}

func mapInt(sessions []*Session, fn func(*Session) int64) []int64 {
	vals := make([]int64, len(sessions))
	for i, s := range sessions {
		vals[i] = fn(s)
	}
	return vals
}

func fmtRowN(label string, colW int, vals []int64) string {
	out := fmt.Sprintf("%-22s", label)
	for _, v := range vals {
		out += fmt.Sprintf(" %*d", colW, v)
	}
	out += "\n"
	return out
}
