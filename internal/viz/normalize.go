package viz

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mindspec/mindspec/internal/bench"
)

// resolveAgentID derives an agent identity from OTLP resource attributes and event data.
// Precedence: agent.name > service.name+service.instance.id > service.name+session.id
// > session.id (truncated) > event-specific default.
//
// session.id (a UUID in event data) auto-differentiates multiple Claude Code instances.
func resolveAgentID(eventName string, resource, data map[string]any) (id string, label string) {
	if name, ok := resource["agent.name"].(string); ok && name != "" {
		return "agent:" + name, name
	}
	svcName, _ := resource["service.name"].(string)
	svcInstance, _ := resource["service.instance.id"].(string)
	if svcName != "" && svcInstance != "" {
		return "agent:" + svcName + ":" + svcInstance, svcName
	}
	sessionID, _ := data["session.id"].(string)
	if svcName != "" && sessionID != "" {
		short := truncateSessionID(sessionID)
		return "agent:" + svcName + ":" + short, svcName + " (" + short + ")"
	}
	if svcName != "" {
		return "agent:" + svcName, svcName
	}

	defaultID, defaultLabel := defaultAgentIdentity(eventName)
	if sessionID != "" {
		short := truncateSessionID(sessionID)
		if defaultID == "agent:claude-code" {
			return "agent:session:" + short, defaultLabel + " (" + short + ")"
		}
		return defaultID + ":" + short, defaultLabel + " (" + short + ")"
	}
	return defaultID, defaultLabel
}

func defaultAgentIdentity(eventName string) (id string, label string) {
	if isCodexEvent(eventName) {
		return "agent:codex", "Codex"
	}
	return "agent:claude-code", "Claude Code"
}

// truncateSessionID returns the first 8 hex chars of a session UUID for display.
// 8 hex chars = 4 billion values, effectively collision-free for concurrent agents.
func truncateSessionID(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) > 8 {
		return clean[:8]
	}
	return clean
}

// NormalizeEvent converts a CollectedEvent into graph operations (node upserts and edge events).
func NormalizeEvent(e bench.CollectedEvent) ([]NodeUpsert, []EdgeEvent) {
	var nodes []NodeUpsert
	var edges []EdgeEvent

	ts := parseTimestamp(e.TS)
	agentID, agentLabel := resolveAgentID(e.Event, e.Resource, e.Data)

	// Sub-agent hierarchy: if agent.parent is set, emit parent→child spawn edge
	if parentName, ok := e.Resource["agent.parent"].(string); ok && parentName != "" {
		parentID := "agent:" + parentName
		if parentID != agentID {
			nodes = append(nodes, NodeUpsert{
				ID:    parentID,
				Type:  NodeAgent,
				Label: parentName,
			})
			edges = append(edges, EdgeEvent{
				ID:        fmt.Sprintf("edge:%s->%s:%d", parentID, agentID, ts.UnixNano()),
				Src:       parentID,
				Dst:       agentID,
				Type:      EdgeSpawn,
				Status:    "ok",
				StartTime: ts,
			})
		}
	}

	switch {
	case isAPIRequestEvent(e.Event):
		model := firstString(e.Data, "model", "gen_ai.request.model", "model_name")
		if model == "" {
			model = "unknown-model"
		}
		llmID := "llm:" + model

		nodes = append(nodes, NodeUpsert{
			ID:    llmID,
			Type:  NodeLLM,
			Label: model,
			Attributes: map[string]any{
				"model": model,
			},
		})

		// Create agent node
		nodes = append(nodes, NodeUpsert{
			ID:    agentID,
			Type:  NodeAgent,
			Label: agentLabel,
		})

		// Edge: agent → llm_endpoint
		dur := parseDuration(e.Data)
		edges = append(edges, EdgeEvent{
			ID:         fmt.Sprintf("edge:%s->%s:%d", agentID, llmID, ts.UnixNano()),
			Src:        agentID,
			Dst:        llmID,
			Type:       EdgeModelCall,
			Status:     "ok",
			StartTime:  ts,
			Duration:   dur,
			Attributes: apiCallAttributes(e.Data, model),
		})

	case isToolEvent(e.Event), isCodexSSEToolEvent(e.Event, e.Data):
		toolName := firstString(e.Data, "tool_name", "tool.name", "name")
		if toolName == "" {
			toolName, _ = codexSSEToolName(e.Event, e.Data)
		}
		if toolName == "" {
			return nil, nil
		}

		toolID := "tool:" + toolName

		nodes = append(nodes, NodeUpsert{
			ID:    toolID,
			Type:  NodeTool,
			Label: toolName,
		})
		nodes = append(nodes, NodeUpsert{
			ID:    agentID,
			Type:  NodeAgent,
			Label: agentLabel,
		})

		toolCategory := classifyToolCategory(toolName)
		dur := parseDuration(e.Data)
		status := "ok"
		if errStr, ok := e.Data["error"].(string); ok && errStr != "" {
			status = "error"
		}
		if isToolResultEvent(e.Event) && strings.EqualFold(firstString(e.Data, "status"), "error") {
			status = "error"
		}

		attrs := safeMeta(e.Data, "tool_name", "tool.name", "name", "duration_ms", "status", "event.kind", "event_kind")
		attrs["tool_name"] = toolName
		attrs["tool_category"] = string(toolCategory)
		attrs["event"] = e.Event

		edges = append(edges, EdgeEvent{
			ID:         fmt.Sprintf("edge:%s->%s:%d", agentID, toolID, ts.UnixNano()),
			Src:        agentID,
			Dst:        toolID,
			Type:       EdgeToolCall,
			Status:     status,
			StartTime:  ts,
			Duration:   dur,
			Attributes: attrs,
		})

		// File/data_source node classification
		if toolCategory == EdgeRetrieval || toolCategory == EdgeWrite {
			filePath := extractFilePath(e.Data)
			if filePath != "" {
				fileID := "file:" + filePath
				nodes = append(nodes, NodeUpsert{
					ID:    fileID,
					Type:  NodeDataSource,
					Label: filepath.Base(filePath),
					Attributes: map[string]any{
						"path": filePath,
					},
				})
				edges = append(edges, EdgeEvent{
					ID:        fmt.Sprintf("edge:%s->%s:%d", toolID, fileID, ts.UnixNano()),
					Src:       toolID,
					Dst:       fileID,
					Type:      toolCategory,
					Status:    status,
					StartTime: ts,
				})
			}
		}

	case isMCPCallEvent(e.Event):
		serverName := firstString(e.Data, "server_name", "server", "mcp.server.name")
		if serverName == "" {
			return nil, nil
		}

		mcpID := "mcp:" + serverName
		dur := parseDuration(e.Data)

		nodes = append(nodes, NodeUpsert{
			ID:    mcpID,
			Type:  NodeMCPServer,
			Label: serverName,
		})
		nodes = append(nodes, NodeUpsert{
			ID:    agentID,
			Type:  NodeAgent,
			Label: agentLabel,
		})

		toolName := firstString(e.Data, "tool_name", "tool.name")
		if toolName != "" {
			// Route through tool node: agent→tool + tool→mcp
			toolID := "tool:" + toolName
			nodes = append(nodes, NodeUpsert{
				ID:    toolID,
				Type:  NodeTool,
				Label: toolName,
			})
			edges = append(edges, EdgeEvent{
				ID:        fmt.Sprintf("edge:%s->%s:%d", agentID, toolID, ts.UnixNano()),
				Src:       agentID,
				Dst:       toolID,
				Type:      EdgeToolCall,
				Status:    "ok",
				StartTime: ts,
				Duration:  dur,
				Attributes: safeMeta(e.Data,
					"tool_name", "tool.name", "method", "duration_ms"),
			})
			edges = append(edges, EdgeEvent{
				ID:        fmt.Sprintf("edge:%s->%s:%d", toolID, mcpID, ts.UnixNano()),
				Src:       toolID,
				Dst:       mcpID,
				Type:      EdgeMCPCall,
				Status:    "ok",
				StartTime: ts,
				Attributes: safeMeta(e.Data,
					"server_name", "server", "mcp.server.name", "method"),
			})
		} else {
			// Direct agent→mcp edge (backwards compatible)
			edges = append(edges, EdgeEvent{
				ID:        fmt.Sprintf("edge:%s->%s:%d", agentID, mcpID, ts.UnixNano()),
				Src:       agentID,
				Dst:       mcpID,
				Type:      EdgeMCPCall,
				Status:    "ok",
				StartTime: ts,
				Duration:  dur,
				Attributes: safeMeta(e.Data,
					"server_name", "server", "mcp.server.name", "method", "duration_ms"),
			})
		}

	case isTokenUsageMetricEvent(e.Event), isCostUsageMetricEvent(e.Event):
		model := firstString(e.Data, "model", "gen_ai.request.model", "model_name")
		if model == "" {
			model = "unknown-model"
		}
		llmID := "llm:" + model

		metricAttrs := metricModelCallAttributes(e.Event, e.Data, model)
		llmAttrs := map[string]any{
			"metric": e.Event,
		}
		for k, v := range metricAttrs {
			switch k {
			case "input_tokens", "output_tokens", "cost_usd":
				llmAttrs[k] = v
			}
		}
		nodes = append(nodes, NodeUpsert{
			ID:         llmID,
			Type:       NodeLLM,
			Label:      model,
			Attributes: llmAttrs,
		})

		// Keep existing Claude behavior unchanged: metrics only create/update model nodes.
		if isClaudeEvent(e.Event) {
			break
		}

		nodes = append(nodes, NodeUpsert{
			ID:    agentID,
			Type:  NodeAgent,
			Label: agentLabel,
		})

		edges = append(edges, EdgeEvent{
			ID:         fmt.Sprintf("edge:%s->%s:%d", agentID, llmID, ts.UnixNano()),
			Src:        agentID,
			Dst:        llmID,
			Type:       EdgeModelCall,
			Status:     "ok",
			StartTime:  ts,
			Attributes: metricAttrs,
		})
	}

	return nodes, edges
}

func isEventName(eventName, suffix string) bool {
	return eventName == suffix || strings.HasSuffix(eventName, "."+suffix)
}

func isAPIRequestEvent(eventName string) bool {
	return isEventName(eventName, "api_request")
}

func isToolEvent(eventName string) bool {
	return isEventName(eventName, "tool_use") ||
		isEventName(eventName, "tool_call") ||
		isEventName(eventName, "tool_result")
}

func isToolResultEvent(eventName string) bool {
	return isEventName(eventName, "tool_result")
}

func isMCPCallEvent(eventName string) bool {
	return isEventName(eventName, "mcp_call")
}

func isCodexSSEToolEvent(eventName string, data map[string]any) bool {
	_, ok := codexSSEToolName(eventName, data)
	return ok
}

func codexSSEToolName(eventName string, data map[string]any) (string, bool) {
	if !isEventName(eventName, "sse_event") {
		return "", false
	}
	kind := firstString(data, "event.kind", "event_kind", "kind")
	switch {
	case strings.HasPrefix(kind, "response.web_search_call."):
		// Emit one tool-call edge when the web-search call completes.
		if strings.HasSuffix(kind, ".completed") {
			return "WebSearch", true
		}
		return "", false
	default:
		return "", false
	}
}

func isTokenUsageMetricEvent(eventName string) bool {
	return isEventName(eventName, "token.usage")
}

func isCostUsageMetricEvent(eventName string) bool {
	return isEventName(eventName, "cost.usage")
}

func isClaudeEvent(eventName string) bool {
	return strings.HasPrefix(eventName, "claude_code.") || strings.HasPrefix(eventName, "claude.")
}

func isCodexEvent(eventName string) bool {
	return strings.HasPrefix(eventName, "codex.")
}

func apiCallAttributes(data map[string]any, model string) map[string]any {
	attrs := map[string]any{
		"metric_only": false,
	}
	if model != "" {
		attrs["model"] = model
	}

	if v, ok := firstNumeric(data, "input_tokens", "gen_ai.usage.input_tokens", "token.input", "usage.input_tokens"); ok {
		attrs["input_tokens"] = v
	}
	if v, ok := firstNumeric(data, "output_tokens", "gen_ai.usage.output_tokens", "token.output", "usage.output_tokens"); ok {
		attrs["output_tokens"] = v
	}
	if v, ok := firstNumeric(data, "cache_read_input_tokens", "gen_ai.usage.cache_read_input_tokens", "token.cache_read", "usage.cache_read_input_tokens"); ok {
		attrs["cache_read_input_tokens"] = v
	}
	if v, ok := firstNumeric(data, "cache_creation_input_tokens", "gen_ai.usage.cache_creation_input_tokens", "token.cache_creation", "usage.cache_creation_input_tokens"); ok {
		attrs["cache_creation_input_tokens"] = v
	}
	if v, ok := firstNumeric(data, "cost_usd", "cost", "gen_ai.usage.cost_usd"); ok {
		attrs["cost_usd"] = v
	}

	return attrs
}

func metricModelCallAttributes(eventName string, data map[string]any, model string) map[string]any {
	attrs := map[string]any{
		"metric_only": true,
		"metric":      eventName,
	}
	if model != "" {
		attrs["model"] = model
	}

	value, ok := firstNumeric(data, "value")
	if !ok {
		return attrs
	}

	if isCostUsageMetricEvent(eventName) {
		attrs["cost_usd"] = value
		return attrs
	}

	tokenType := normalizeTokenType(firstString(data, "type", "token_type", "usage_type", "kind"))
	switch tokenType {
	case "output":
		attrs["output_tokens"] = value
	case "cache_read":
		attrs["cache_read_input_tokens"] = value
	case "cache_creation":
		attrs["cache_creation_input_tokens"] = value
	default:
		attrs["input_tokens"] = value
	}

	return attrs
}

func metricStatsDelta(eventName string, data map[string]any) (int64, int64, float64, bool) {
	if !(isTokenUsageMetricEvent(eventName) || isCostUsageMetricEvent(eventName)) {
		return 0, 0, 0, false
	}
	if isClaudeEvent(eventName) {
		// Preserve Claude totals behavior (already sourced from api_request).
		return 0, 0, 0, false
	}

	attrs := metricModelCallAttributes(eventName, data, "")
	inTok := toInt64(attrs["input_tokens"])
	outTok := toInt64(attrs["output_tokens"])
	cost := toFloat64(attrs["cost_usd"])
	if inTok == 0 && outTok == 0 && cost == 0 {
		return 0, 0, 0, false
	}
	return inTok, outTok, cost, true
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

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := data[key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func firstNumeric(data map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		v, ok := data[key]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return n, true
		case int:
			return float64(n), true
		case int64:
			return float64(n), true
		case int32:
			return float64(n), true
		}
	}
	return 0, false
}

func parseTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Now()
	}
	return t
}

func parseDuration(data map[string]any) time.Duration {
	if ms, ok := data["duration_ms"].(float64); ok {
		return time.Duration(ms * float64(time.Millisecond))
	}
	if ms, ok := data["duration_ms"].(int64); ok {
		return time.Duration(ms) * time.Millisecond
	}
	if ms, ok := data["duration_ms"].(string); ok {
		if n, err := strconv.ParseFloat(strings.TrimSpace(ms), 64); err == nil {
			return time.Duration(n * float64(time.Millisecond))
		}
	}
	return 0
}

func classifyToolCategory(toolName string) EdgeType {
	switch toolName {
	case "Read", "Glob", "Grep", "WebFetch", "WebSearch":
		return EdgeRetrieval
	case "Write", "Edit", "NotebookEdit":
		return EdgeWrite
	default:
		return EdgeToolCall
	}
}

func extractFilePath(data map[string]any) string {
	if p, ok := data["file_path"].(string); ok && p != "" {
		return p
	}
	if p, ok := data["path"].(string); ok && p != "" {
		return p
	}
	return ""
}

// sensitiveFields are keys that must never be passed to the frontend.
var sensitiveFields = map[string]bool{
	"prompt":            true,
	"content":           true,
	"message":           true,
	"system_prompt":     true,
	"user_message":      true,
	"assistant_message": true,
	"human_message":     true,
}

func isSensitive(key string) bool {
	return sensitiveFields[strings.ToLower(key)]
}

// safeMeta wraps filterAttrs with a sensitive-field post-check.
func safeMeta(data map[string]any, keys ...string) map[string]any {
	out := filterAttrs(data, keys...)
	for k := range out {
		if isSensitive(k) {
			delete(out, k)
		}
	}
	return out
}

func filterAttrs(data map[string]any, keys ...string) map[string]any {
	out := make(map[string]any)
	for _, k := range keys {
		if v, ok := data[k]; ok {
			out[k] = v
		}
	}
	return out
}
