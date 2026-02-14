package viz

import (
	"testing"

	"github.com/mindspec/mindspec/internal/bench"
)

func TestNormalizeAPIRequest(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model":        "claude-sonnet-4-5-20250929",
			"input_tokens":  int64(1000),
			"output_tokens": int64(500),
		},
	}

	nodes, edges := NormalizeEvent(e)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (llm + agent), got %d", len(nodes))
	}

	var llmNode, agentNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeLLM {
			llmNode = &nodes[i]
		}
		if nodes[i].Type == NodeAgent {
			agentNode = &nodes[i]
		}
	}

	if llmNode == nil {
		t.Fatal("expected LLM endpoint node")
	}
	if llmNode.ID != "llm:claude-sonnet-4-5-20250929" {
		t.Errorf("unexpected LLM node ID: %s", llmNode.ID)
	}
	if agentNode == nil {
		t.Fatal("expected agent node")
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeModelCall {
		t.Errorf("expected model_call edge, got %s", edges[0].Type)
	}
	if edges[0].Src != "agent:claude-code" {
		t.Errorf("expected agent src, got %s", edges[0].Src)
	}
}

func TestNormalizeToolUse(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.tool_use",
		Data: map[string]any{
			"tool_name":   "Read",
			"duration_ms": float64(50),
		},
	}

	nodes, edges := NormalizeEvent(e)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	if edges[0].Type != EdgeRetrieval {
		t.Errorf("Read tool should be classified as retrieval, got %s", edges[0].Type)
	}
}

func TestNormalizeToolUseWrite(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.tool_use",
		Data: map[string]any{
			"tool_name": "Write",
		},
	}

	_, edges := NormalizeEvent(e)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeWrite {
		t.Errorf("Write tool should be classified as write, got %s", edges[0].Type)
	}
}

func TestNormalizeMCPCall(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.mcp_call",
		Data: map[string]any{
			"server_name": "my-server",
			"method":      "doStuff",
		},
	}

	nodes, edges := NormalizeEvent(e)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	var mcpNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeMCPServer {
			mcpNode = &nodes[i]
		}
	}

	if mcpNode == nil {
		t.Fatal("expected MCP server node")
	}
	if mcpNode.ID != "mcp:my-server" {
		t.Errorf("unexpected MCP node ID: %s", mcpNode.ID)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeMCPCall {
		t.Errorf("expected mcp_call edge, got %s", edges[0].Type)
	}
}

func TestNormalizeUnknownEvent(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "unknown.event",
		Data:  map[string]any{},
	}

	nodes, edges := NormalizeEvent(e)
	if len(nodes) != 0 {
		t.Errorf("expected no nodes for unknown event, got %d", len(nodes))
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges for unknown event, got %d", len(edges))
	}
}

func TestNormalizeSensitiveFieldsStripped(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model":         "claude-sonnet-4-5-20250929",
			"input_tokens":  int64(1000),
			"output_tokens": int64(500),
			"prompt":        "secret system prompt text",
			"content":       "secret response content",
		},
	}

	_, edges := NormalizeEvent(e)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	for k := range edges[0].Attributes {
		if isSensitive(k) {
			t.Errorf("sensitive field %q should not appear in attributes", k)
		}
	}
}

func TestNormalizeToolUseCreatesFileNode(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.tool_use",
		Data: map[string]any{
			"tool_name": "Read",
			"file_path": "/src/main.go",
		},
	}

	nodes, edges := NormalizeEvent(e)

	// Should have 3 nodes: agent, tool, file
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes (agent + tool + file), got %d", len(nodes))
	}

	var fileNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeDataSource {
			fileNode = &nodes[i]
		}
	}
	if fileNode == nil {
		t.Fatal("expected data_source node for file")
	}
	if fileNode.ID != "file:/src/main.go" {
		t.Errorf("unexpected file node ID: %s", fileNode.ID)
	}
	if fileNode.Label != "main.go" {
		t.Errorf("file node label should be basename, got %q", fileNode.Label)
	}

	// Should have 2 edges: agent→tool + tool→file
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges (agent→tool + tool→file), got %d", len(edges))
	}
}

func TestNormalizeMCPCallViaToolNode(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.mcp_call",
		Data: map[string]any{
			"server_name": "my-server",
			"tool_name":   "my-tool",
			"method":      "doStuff",
		},
	}

	nodes, edges := NormalizeEvent(e)

	// Should have 3 nodes: agent, tool, mcp_server
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes (agent + tool + mcp), got %d", len(nodes))
	}

	var toolNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeTool {
			toolNode = &nodes[i]
		}
	}
	if toolNode == nil {
		t.Fatal("expected tool node for MCP call with tool_name")
	}
	if toolNode.ID != "tool:my-tool" {
		t.Errorf("unexpected tool node ID: %s", toolNode.ID)
	}

	// Should have 2 edges: agent→tool + tool→mcp
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges (agent→tool + tool→mcp), got %d", len(edges))
	}

	if edges[0].Src != "agent:claude-code" || edges[0].Dst != "tool:my-tool" {
		t.Errorf("first edge should be agent→tool, got %s→%s", edges[0].Src, edges[0].Dst)
	}
	if edges[1].Src != "tool:my-tool" || edges[1].Dst != "mcp:my-server" {
		t.Errorf("second edge should be tool→mcp, got %s→%s", edges[1].Src, edges[1].Dst)
	}
}

func TestNormalizeMCPCallDirectFallback(t *testing.T) {
	// MCP call without tool_name should use direct agent→mcp edge
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.mcp_call",
		Data: map[string]any{
			"server_name": "my-server",
			"method":      "doStuff",
		},
	}

	nodes, edges := NormalizeEvent(e)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (agent + mcp), got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge (agent→mcp), got %d", len(edges))
	}
	if edges[0].Src != "agent:claude-code" || edges[0].Dst != "mcp:my-server" {
		t.Errorf("expected agent→mcp direct edge, got %s→%s", edges[0].Src, edges[0].Dst)
	}
}

func TestNormalizeTokenMetric(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.token.usage",
		Data: map[string]any{
			"model": "claude-sonnet-4-5-20250929",
			"value": float64(1500),
		},
	}

	nodes, edges := NormalizeEvent(e)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node for token metric, got %d", len(nodes))
	}
	if nodes[0].Type != NodeLLM {
		t.Errorf("expected LLM node, got %s", nodes[0].Type)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges for metric, got %d", len(edges))
	}
}
