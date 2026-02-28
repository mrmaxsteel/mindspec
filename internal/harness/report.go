package harness

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Report summarizes the analysis of an agent session.
type Report struct {
	SessionName       string              `json:"session_name"`
	AgentName         string              `json:"agent_name"`
	TotalTurns        int                 `json:"total_turns"`
	TotalEvents       int                 `json:"total_events"`
	TotalDuration     time.Duration       `json:"-"`
	TotalDurationMS   int64               `json:"total_duration_ms"`
	TurnsByClass      map[TurnClass]int   `json:"turns_by_class"`
	TurnSummaries     []TurnSummary       `json:"-"` // omit from JSON (too verbose)
	WrongActions      []WrongActionResult `json:"wrong_actions"`
	PlanFidelityScore float64             `json:"plan_fidelity_score"`
	ForwardTurnRatio  float64             `json:"forward_turn_ratio"`
}

// NewReport creates a report from analysis results.
func NewReport(name, agentName string, summaries []TurnSummary, wrongActions []WrongActionResult, fidelity float64) *Report {
	byClass := make(map[TurnClass]int)
	var totalDuration time.Duration
	totalEvents := 0
	for _, s := range summaries {
		byClass[s.Class]++
		totalEvents += len(s.Events)
		for _, e := range s.Events {
			totalDuration += e.Duration()
		}
	}

	total := len(summaries)
	forwardRatio := 0.0
	if total > 0 {
		forwardRatio = float64(byClass[ClassForward]) / float64(total)
	}

	return &Report{
		SessionName:       name,
		AgentName:         agentName,
		TotalTurns:        total,
		TotalEvents:       totalEvents,
		TotalDuration:     totalDuration,
		TotalDurationMS:   totalDuration.Milliseconds(),
		TurnsByClass:      byClass,
		TurnSummaries:     summaries,
		WrongActions:      wrongActions,
		PlanFidelityScore: fidelity,
		ForwardTurnRatio:  forwardRatio,
	}
}

// FormatText returns a human-readable report.
func (r *Report) FormatText() string {
	var b strings.Builder

	fmt.Fprintf(&b, "=== Session Report: %s ===\n", r.SessionName)
	fmt.Fprintf(&b, "Agent: %s\n", r.AgentName)
	fmt.Fprintf(&b, "Turns: %d (estimated)  Events: %d\n", r.TotalTurns, r.TotalEvents)
	fmt.Fprintf(&b, "Duration: %s\n", r.TotalDuration.Round(time.Millisecond))
	fmt.Fprintf(&b, "Forward ratio: %.1f%%\n", r.ForwardTurnRatio*100)
	fmt.Fprintf(&b, "Plan fidelity: %.1f%%\n", r.PlanFidelityScore*100)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "Turn classification:")
	for _, class := range []TurnClass{ClassForward, ClassRetry, ClassCorrection, ClassRecovery, ClassWrongAction, ClassOverhead} {
		count := r.TurnsByClass[class]
		if count > 0 {
			fmt.Fprintf(&b, "  %-14s %d\n", class, count)
		}
	}

	if len(r.WrongActions) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "Wrong actions (%d):\n", len(r.WrongActions))
		for i, wa := range r.WrongActions {
			fmt.Fprintf(&b, "  %d. [%s] %s\n", i+1, wa.Rule, wa.Reason)
		}
	}

	return b.String()
}

// FormatJSON returns the report as a JSON string.
func (r *Report) FormatJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
