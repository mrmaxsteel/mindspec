package viz

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertCodexSessionMapsKnownRecords(t *testing.T) {
	fixture := strings.Join([]string{
		`{"timestamp":"2026-02-16T10:00:00Z","type":"session_meta","payload":{"id":"019c6694-aa05-76e0-98b1-46390fb71add"}}`,
		`{"timestamp":"2026-02-16T10:00:00Z","type":"turn_context","payload":{"model":"gpt-5.3-codex"}}`,
		`{"timestamp":"2026-02-16T10:00:01Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"cat README.md\",\"workdir\":\"/repo\"}","call_id":"call-1"}}`,
		`{"timestamp":"2026-02-16T10:00:02Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","status":"completed","output":"ok"}}`,
		`{"timestamp":"2026-02-16T10:00:03Z","type":"response_item","payload":{"type":"custom_tool_call","name":"bash","arguments":"{\"cmd\":\"false\"}","call_id":"call-2"}}`,
		`{"timestamp":"2026-02-16T10:00:04Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"call-2","status":"completed","output":"Process exited with code 1"}}`,
		`{"timestamp":"2026-02-16T10:00:05Z","type":"response_item","payload":{"type":"web_search_call","action":{"type":"search_query","query":"mindspec codex"}}}`,
		`{"timestamp":"2026-02-16T10:00:06Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":4},"total_token_usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":4}}}}`,
		`{"timestamp":"2026-02-16T10:00:07Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0},"total_token_usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":4}}}}`,
		`{"timestamp":"2026-02-16T10:00:08Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":15,"cached_input_tokens":2,"output_tokens":8}}}}`,
		`{bad-json`,
		"",
	}, "\n")

	events, stats, err := ConvertCodexSession(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("ConvertCodexSession returned error: %v", err)
	}

	if stats.Lines != 11 {
		t.Fatalf("stats.Lines = %d, want 11", stats.Lines)
	}
	if stats.Events != 7 {
		t.Fatalf("stats.Events = %d, want 7", stats.Events)
	}
	if stats.ToolCalls != 3 {
		t.Fatalf("stats.ToolCalls = %d, want 3", stats.ToolCalls)
	}
	if stats.ToolResults != 2 {
		t.Fatalf("stats.ToolResults = %d, want 2", stats.ToolResults)
	}
	if stats.APIRequests != 2 {
		t.Fatalf("stats.APIRequests = %d, want 2", stats.APIRequests)
	}
	if stats.SkippedMalformed != 1 {
		t.Fatalf("stats.SkippedMalformed = %d, want 1", stats.SkippedMalformed)
	}

	if len(events) != 7 {
		t.Fatalf("len(events) = %d, want 7", len(events))
	}

	first := events[0]
	if first.Event != "claude_code.tool_use" {
		t.Fatalf("events[0].Event = %q, want claude_code.tool_use", first.Event)
	}
	if got, _ := first.Data["tool_name"].(string); got != "exec_command" {
		t.Fatalf("events[0].Data[tool_name] = %q, want exec_command", got)
	}
	if got, _ := first.Data["command"].(string); got != "cat README.md" {
		t.Fatalf("events[0].Data[command] = %q, want cat README.md", got)
	}
	if got, _ := first.Data["path"].(string); got != "/repo" {
		t.Fatalf("events[0].Data[path] = %q, want /repo", got)
	}
	if got, _ := first.Data["session.id"].(string); got != "019c6694-aa05-76e0-98b1-46390fb71add" {
		t.Fatalf("events[0].Data[session.id] = %q, want session id", got)
	}
	if got, _ := first.Resource["agent.name"].(string); got != "Codex" {
		t.Fatalf("events[0].Resource[agent.name] = %q, want Codex", got)
	}
	if got, _ := first.Resource["service.name"].(string); got != "codex" {
		t.Fatalf("events[0].Resource[service.name] = %q, want codex", got)
	}
	if got, _ := first.Resource["service.instance.id"].(string); got != "019c6694" {
		t.Fatalf("events[0].Resource[service.instance.id] = %q, want 019c6694", got)
	}

	failedTool := events[3]
	if failedTool.Event != "claude_code.tool_result" {
		t.Fatalf("events[3].Event = %q, want claude_code.tool_result", failedTool.Event)
	}
	if got, _ := failedTool.Data["tool_name"].(string); got != "bash" {
		t.Fatalf("events[3].Data[tool_name] = %q, want bash", got)
	}
	if got, _ := failedTool.Data["error"].(string); got == "" {
		t.Fatalf("events[3] should include error for non-zero tool output")
	}

	webTool := events[4]
	if got, _ := webTool.Data["tool_name"].(string); got != "WebSearch" {
		t.Fatalf("events[4].Data[tool_name] = %q, want WebSearch", got)
	}
	if got, _ := webTool.Data["query"].(string); got != "mindspec codex" {
		t.Fatalf("events[4].Data[query] = %q, want mindspec codex", got)
	}

	api1 := events[5]
	if api1.Event != "claude_code.api_request" {
		t.Fatalf("events[5].Event = %q, want claude_code.api_request", api1.Event)
	}
	if got, _ := api1.Data["model"].(string); got != "gpt-5.3-codex" {
		t.Fatalf("events[5].Data[model] = %q, want gpt-5.3-codex", got)
	}
	if got, _ := api1.Data["input_tokens"].(float64); got != 10 {
		t.Fatalf("events[5].Data[input_tokens] = %v, want 10", got)
	}
	if got, _ := api1.Data["output_tokens"].(float64); got != 4 {
		t.Fatalf("events[5].Data[output_tokens] = %v, want 4", got)
	}
	if got, _ := api1.Data["cache_read_tokens"].(float64); got != 2 {
		t.Fatalf("events[5].Data[cache_read_tokens] = %v, want 2", got)
	}

	api2 := events[6]
	if got, _ := api2.Data["input_tokens"].(float64); got != 5 {
		t.Fatalf("events[6].Data[input_tokens] = %v, want 5", got)
	}
	if got, _ := api2.Data["output_tokens"].(float64); got != 4 {
		t.Fatalf("events[6].Data[output_tokens] = %v, want 4", got)
	}
	if _, ok := api2.Data["cache_read_tokens"]; ok {
		t.Fatalf("events[6] should not include cache_read_tokens when delta is zero")
	}
}

func TestConvertCodexSessionSkipsMalformedAndUnknown(t *testing.T) {
	fixture := strings.Join([]string{
		`{"timestamp":"2026-02-16T10:00:00Z","type":"unknown_type","payload":{}}`,
		`{"timestamp":"2026-02-16T10:00:01Z","type":"response_item","payload":{"type":"message"}}`,
		`{"timestamp":"2026-02-16T10:00:02Z","type":"response_item","payload":{"type":"mystery_item"}}`,
		`{bad-json`,
		"",
	}, "\n")

	events, stats, err := ConvertCodexSession(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("ConvertCodexSession returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
	if stats.Lines != 4 {
		t.Fatalf("stats.Lines = %d, want 4", stats.Lines)
	}
	if stats.SkippedUnknown != 2 {
		t.Fatalf("stats.SkippedUnknown = %d, want 2", stats.SkippedUnknown)
	}
	if stats.SkippedIgnored != 1 {
		t.Fatalf("stats.SkippedIgnored = %d, want 1", stats.SkippedIgnored)
	}
	if stats.SkippedMalformed != 1 {
		t.Fatalf("stats.SkippedMalformed = %d, want 1", stats.SkippedMalformed)
	}
}

func TestConvertCodexSessionFileProducesReplayableNDJSON(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "codex-session.jsonl")
	fixture := strings.Join([]string{
		`{"timestamp":"2026-02-16T11:00:00Z","type":"session_meta","payload":{"id":"019c6694-aa05-76e0-98b1-46390fb71add"}}`,
		`{"timestamp":"2026-02-16T11:00:00Z","type":"turn_context","payload":{"model":"gpt-5.3-codex"}}`,
		`{"timestamp":"2026-02-16T11:00:01Z","type":"response_item","payload":{"type":"function_call","name":"Read","arguments":"{\"path\":\"/repo/README.md\"}","call_id":"call-read"}}`,
		`{"timestamp":"2026-02-16T11:00:02Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-read","status":"completed","output":"ok"}}`,
		`{"timestamp":"2026-02-16T11:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":12,"cached_input_tokens":3,"output_tokens":6},"total_token_usage":{"input_tokens":12,"cached_input_tokens":3,"output_tokens":6}}}}`,
		"",
	}, "\n")
	if err := os.WriteFile(inputPath, []byte(fixture), 0644); err != nil {
		t.Fatalf("writing input fixture: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "converted.ndjson")
	stats, err := ConvertCodexSessionFile(inputPath, outputPath)
	if err != nil {
		t.Fatalf("ConvertCodexSessionFile returned error: %v", err)
	}
	if stats.Events != 3 {
		t.Fatalf("stats.Events = %d, want 3", stats.Events)
	}

	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	replay := NewReplay(outputPath, 0, graph, hub)
	if err := replay.Run(ctx); err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	snap := graph.Snapshot()
	if !hasNodeID(snap.Nodes, "agent:Codex") {
		t.Fatalf("expected replay to include agent:Codex node")
	}
	if !hasNodeID(snap.Nodes, "tool:Read") {
		t.Fatalf("expected replay to include tool:Read node")
	}
	if !hasNodeID(snap.Nodes, "llm:gpt-5.3-codex") {
		t.Fatalf("expected replay to include llm:gpt-5.3-codex node")
	}
	if !hasEdgeType(snap.Edges, EdgeToolCall) {
		t.Fatalf("expected replay to include tool_call edge")
	}
	if !hasEdgeType(snap.Edges, EdgeModelCall) {
		t.Fatalf("expected replay to include model_call edge")
	}
}

func hasNodeID(nodes []Node, id string) bool {
	for _, n := range nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}

func hasEdgeType(edges []Edge, edgeType EdgeType) bool {
	for _, e := range edges {
		if e.Type == edgeType {
			return true
		}
	}
	return false
}
