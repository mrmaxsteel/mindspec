package viz

import (
	"fmt"
	"time"

	"github.com/mindspec/mindspec/internal/bench"
)

// NormalizeEvent converts a CollectedEvent into graph operations (node upserts and edge events).
func NormalizeEvent(e bench.CollectedEvent) ([]NodeUpsert, []EdgeEvent) {
	var nodes []NodeUpsert
	var edges []EdgeEvent

	ts := parseTimestamp(e.TS)

	switch e.Event {
	case "claude_code.api_request":
		model, _ := e.Data["model"].(string)
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
		agentID := "agent:claude-code"
		nodes = append(nodes, NodeUpsert{
			ID:    agentID,
			Type:  NodeAgent,
			Label: "Claude Code",
		})

		// Edge: agent → llm_endpoint
		dur := parseDuration(e.Data)
		edges = append(edges, EdgeEvent{
			ID:        fmt.Sprintf("edge:%s->%s:%d", agentID, llmID, ts.UnixNano()),
			Src:       agentID,
			Dst:       llmID,
			Type:      EdgeModelCall,
			Status:    "ok",
			StartTime: ts,
			Duration:  dur,
			Attributes: filterAttrs(e.Data,
				"input_tokens", "output_tokens", "cache_read_input_tokens",
				"cache_creation_input_tokens", "model", "cost_usd"),
		})

	case "claude_code.tool_use", "tool_use":
		toolName, _ := e.Data["tool_name"].(string)
		if toolName == "" {
			toolName, _ = e.Data["name"].(string)
		}
		if toolName == "" {
			return nil, nil
		}

		toolID := "tool:" + toolName
		agentID := "agent:claude-code"

		nodes = append(nodes, NodeUpsert{
			ID:    toolID,
			Type:  NodeTool,
			Label: toolName,
		})
		nodes = append(nodes, NodeUpsert{
			ID:    agentID,
			Type:  NodeAgent,
			Label: "Claude Code",
		})

		edgeType := classifyToolEdge(toolName)
		dur := parseDuration(e.Data)
		status := "ok"
		if errStr, ok := e.Data["error"].(string); ok && errStr != "" {
			status = "error"
		}

		edges = append(edges, EdgeEvent{
			ID:        fmt.Sprintf("edge:%s->%s:%d", agentID, toolID, ts.UnixNano()),
			Src:       agentID,
			Dst:       toolID,
			Type:      edgeType,
			Status:    status,
			StartTime: ts,
			Duration:  dur,
			Attributes: filterAttrs(e.Data,
				"tool_name", "name", "duration_ms"),
		})

	case "claude_code.mcp_call", "mcp_call":
		serverName, _ := e.Data["server_name"].(string)
		if serverName == "" {
			serverName, _ = e.Data["server"].(string)
		}
		if serverName == "" {
			return nil, nil
		}

		mcpID := "mcp:" + serverName
		agentID := "agent:claude-code"

		nodes = append(nodes, NodeUpsert{
			ID:    mcpID,
			Type:  NodeMCPServer,
			Label: serverName,
		})
		nodes = append(nodes, NodeUpsert{
			ID:    agentID,
			Type:  NodeAgent,
			Label: "Claude Code",
		})

		dur := parseDuration(e.Data)
		edges = append(edges, EdgeEvent{
			ID:        fmt.Sprintf("edge:%s->%s:%d", agentID, mcpID, ts.UnixNano()),
			Src:       agentID,
			Dst:       mcpID,
			Type:      EdgeMCPCall,
			Status:    "ok",
			StartTime: ts,
			Duration:  dur,
			Attributes: filterAttrs(e.Data,
				"server_name", "server", "method", "duration_ms"),
		})

	default:
		// Token/cost metrics create LLM endpoint nodes
		if e.Event == "claude_code.token.usage" || e.Event == "claude_code.cost.usage" {
			model, _ := e.Data["model"].(string)
			if model != "" {
				nodes = append(nodes, NodeUpsert{
					ID:    "llm:" + model,
					Type:  NodeLLM,
					Label: model,
					Attributes: map[string]any{
						"metric": e.Event,
					},
				})
			}
		}
	}

	return nodes, edges
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
	return 0
}

func classifyToolEdge(toolName string) EdgeType {
	switch toolName {
	case "Read", "Glob", "Grep", "WebFetch", "WebSearch":
		return EdgeRetrieval
	case "Write", "Edit", "NotebookEdit":
		return EdgeWrite
	default:
		return EdgeToolCall
	}
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
