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
			"model":         "claude-sonnet-4-5-20250929",
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

	if edges[0].Type != EdgeToolCall {
		t.Errorf("all tool edges should be tool_call, got %s", edges[0].Type)
	}
	if cat, ok := edges[0].Attributes["tool_category"].(string); !ok || cat != "retrieval" {
		t.Errorf("Read tool should have tool_category=retrieval, got %v", edges[0].Attributes["tool_category"])
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
	if edges[0].Type != EdgeToolCall {
		t.Errorf("all tool edges should be tool_call, got %s", edges[0].Type)
	}
	if cat, ok := edges[0].Attributes["tool_category"].(string); !ok || cat != "write" {
		t.Errorf("Write tool should have tool_category=write, got %v", edges[0].Attributes["tool_category"])
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

func TestNormalizeAgentIdentityFromAgentName(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model":        "claude-sonnet-4-5-20250929",
			"input_tokens": int64(100),
		},
		Resource: map[string]any{
			"agent.name": "main",
		},
	}

	nodes, edges := NormalizeEvent(e)

	var agentNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeAgent {
			agentNode = &nodes[i]
		}
	}
	if agentNode == nil {
		t.Fatal("expected agent node")
	}
	if agentNode.ID != "agent:main" {
		t.Errorf("agent ID = %q, want agent:main", agentNode.ID)
	}
	if agentNode.Label != "main" {
		t.Errorf("agent label = %q, want main", agentNode.Label)
	}
	if edges[0].Src != "agent:main" {
		t.Errorf("edge src = %q, want agent:main", edges[0].Src)
	}
}

func TestNormalizeAgentIdentityFromServiceName(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model": "claude-sonnet-4-5-20250929",
		},
		Resource: map[string]any{
			"service.name":        "foo",
			"service.instance.id": "bar",
		},
	}

	nodes, _ := NormalizeEvent(e)

	var agentNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeAgent {
			agentNode = &nodes[i]
		}
	}
	if agentNode == nil {
		t.Fatal("expected agent node")
	}
	if agentNode.ID != "agent:foo:bar" {
		t.Errorf("agent ID = %q, want agent:foo:bar", agentNode.ID)
	}
	if agentNode.Label != "foo" {
		t.Errorf("agent label = %q, want foo", agentNode.Label)
	}
}

func TestNormalizeAgentFallbackNoResource(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model": "claude-sonnet-4-5-20250929",
		},
	}

	nodes, edges := NormalizeEvent(e)

	var agentNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeAgent {
			agentNode = &nodes[i]
		}
	}
	if agentNode == nil {
		t.Fatal("expected agent node")
	}
	if agentNode.ID != "agent:claude-code" {
		t.Errorf("agent ID = %q, want agent:claude-code", agentNode.ID)
	}
	if agentNode.Label != "Claude Code" {
		t.Errorf("agent label = %q, want Claude Code", agentNode.Label)
	}
	if edges[0].Src != "agent:claude-code" {
		t.Errorf("edge src = %q, want agent:claude-code", edges[0].Src)
	}
}

func TestNormalizeSubAgentSpawnEdge(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model": "claude-sonnet-4-5-20250929",
		},
		Resource: map[string]any{
			"agent.name":   "sub-1",
			"agent.parent": "main",
		},
	}

	nodes, edges := NormalizeEvent(e)

	// Should have 3 nodes: parent agent, child agent, LLM
	agentNodes := []NodeUpsert{}
	for _, n := range nodes {
		if n.Type == NodeAgent {
			agentNodes = append(agentNodes, n)
		}
	}
	if len(agentNodes) != 2 {
		t.Fatalf("expected 2 agent nodes (parent + child), got %d", len(agentNodes))
	}

	// Should have spawn edge + model_call edge
	var spawnEdge, modelEdge *EdgeEvent
	for i := range edges {
		switch edges[i].Type {
		case EdgeSpawn:
			spawnEdge = &edges[i]
		case EdgeModelCall:
			modelEdge = &edges[i]
		}
	}
	if spawnEdge == nil {
		t.Fatal("expected spawn edge")
	}
	if spawnEdge.Src != "agent:main" || spawnEdge.Dst != "agent:sub-1" {
		t.Errorf("spawn edge = %s→%s, want agent:main→agent:sub-1", spawnEdge.Src, spawnEdge.Dst)
	}
	if modelEdge == nil {
		t.Fatal("expected model_call edge")
	}
	if modelEdge.Src != "agent:sub-1" {
		t.Errorf("model_call src = %q, want agent:sub-1", modelEdge.Src)
	}
}

func TestNormalizeAgentIdentityFromSessionID(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model":      "claude-sonnet-4-5-20250929",
			"session.id": "a595bc37-ba5a-4345-8fbe-bbdaab766b2d",
		},
	}

	nodes, edges := NormalizeEvent(e)

	var agentNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeAgent {
			agentNode = &nodes[i]
		}
	}
	if agentNode == nil {
		t.Fatal("expected agent node")
	}
	// session.id "a595bc37..." → first 8 hex chars after stripping hyphens = "a595bc37"
	if agentNode.ID != "agent:session:a595bc37" {
		t.Errorf("agent ID = %q, want agent:session:a595bc37", agentNode.ID)
	}
	if agentNode.Label != "Claude Code (a595bc37)" {
		t.Errorf("agent label = %q, want %q", agentNode.Label, "Claude Code (a595bc37)")
	}
	if edges[0].Src != "agent:session:a595bc37" {
		t.Errorf("edge src = %q, want agent:session:a595bc37", edges[0].Src)
	}
}

func TestNormalizeAgentIdentityServiceNamePlusSessionID(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model":      "claude-sonnet-4-5-20250929",
			"session.id": "7f3b1234-0000-0000-0000-000000000000",
		},
		Resource: map[string]any{
			"service.name": "claude-code",
		},
	}

	nodes, _ := NormalizeEvent(e)

	var agentNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeAgent {
			agentNode = &nodes[i]
		}
	}
	if agentNode == nil {
		t.Fatal("expected agent node")
	}
	if agentNode.ID != "agent:claude-code:7f3b1234" {
		t.Errorf("agent ID = %q, want agent:claude-code:7f3b1234", agentNode.ID)
	}
	if agentNode.Label != "claude-code (7f3b1234)" {
		t.Errorf("agent label = %q, want %q", agentNode.Label, "claude-code (7f3b1234)")
	}
}

func TestNormalizeAgentNameTakesPrecedenceOverSessionID(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model":      "claude-sonnet-4-5-20250929",
			"session.id": "a595bc37-ba5a-4345-8fbe-bbdaab766b2d",
		},
		Resource: map[string]any{
			"agent.name": "research",
		},
	}

	nodes, _ := NormalizeEvent(e)

	var agentNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeAgent {
			agentNode = &nodes[i]
		}
	}
	if agentNode == nil {
		t.Fatal("expected agent node")
	}
	// agent.name should win over session.id
	if agentNode.ID != "agent:research" {
		t.Errorf("agent ID = %q, want agent:research", agentNode.ID)
	}
}

func TestNormalizeSubAgentSelfLoopSkipped(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "claude_code.api_request",
		Data: map[string]any{
			"model": "claude-sonnet-4-5-20250929",
		},
		Resource: map[string]any{
			"agent.name":   "main",
			"agent.parent": "main",
		},
	}

	nodes, edges := NormalizeEvent(e)

	// Should NOT create a spawn edge (self-loop)
	for _, edge := range edges {
		if edge.Type == EdgeSpawn {
			t.Error("should not create spawn edge when parent == self")
		}
	}

	// Only one agent node (no duplicate)
	agentCount := 0
	for _, n := range nodes {
		if n.Type == NodeAgent {
			agentCount++
		}
	}
	if agentCount != 1 {
		t.Errorf("expected 1 agent node (no duplicate), got %d", agentCount)
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

func TestNormalizeCodexAPIRequest(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "codex.api_request",
		Data: map[string]any{
			"model":                      "gpt-5-codex",
			"gen_ai.usage.input_tokens":  float64(1200),
			"gen_ai.usage.output_tokens": float64(320),
		},
	}

	nodes, edges := NormalizeEvent(e)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (agent + llm), got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeModelCall {
		t.Fatalf("edge type = %s, want model_call", edges[0].Type)
	}
	if edges[0].Src != "agent:codex" {
		t.Fatalf("edge src = %s, want agent:codex", edges[0].Src)
	}
	if edges[0].Attributes["metric_only"] != false {
		t.Fatalf("metric_only = %v, want false", edges[0].Attributes["metric_only"])
	}
	if edges[0].Attributes["input_tokens"] != float64(1200) {
		t.Fatalf("input_tokens = %v, want 1200", edges[0].Attributes["input_tokens"])
	}
	if edges[0].Attributes["output_tokens"] != float64(320) {
		t.Fatalf("output_tokens = %v, want 320", edges[0].Attributes["output_tokens"])
	}
}

func TestNormalizeCodexToolCallAlias(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "codex.tool_call",
		Data: map[string]any{
			"tool.name": "Read",
		},
	}

	_, edges := NormalizeEvent(e)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeToolCall {
		t.Fatalf("edge type = %s, want tool_call", edges[0].Type)
	}
	if edges[0].Attributes["tool_name"] != "Read" {
		t.Fatalf("tool_name = %v, want Read", edges[0].Attributes["tool_name"])
	}
}

func TestNormalizeCodexSSEWebSearchCompletedAsToolCall(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-16T17:43:27.954Z",
		Event: "codex.sse_event",
		Data: map[string]any{
			"event.kind":  "response.web_search_call.completed",
			"duration_ms": "2165",
		},
	}

	nodes, edges := NormalizeEvent(e)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (agent + tool), got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 tool_call edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeToolCall {
		t.Fatalf("edge type = %s, want tool_call", edges[0].Type)
	}
	if edges[0].Src != "agent:codex" {
		t.Fatalf("edge src = %s, want agent:codex", edges[0].Src)
	}
	if edges[0].Dst != "tool:WebSearch" {
		t.Fatalf("edge dst = %s, want tool:WebSearch", edges[0].Dst)
	}
	if edges[0].Attributes["tool_name"] != "WebSearch" {
		t.Fatalf("tool_name = %v, want WebSearch", edges[0].Attributes["tool_name"])
	}
}

func TestNormalizeCodexSSEWebSearchInProgressIgnored(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-16T17:43:25.752Z",
		Event: "codex.sse_event",
		Data: map[string]any{
			"event.kind": "response.web_search_call.in_progress",
		},
	}

	nodes, edges := NormalizeEvent(e)
	if len(nodes) != 0 || len(edges) != 0 {
		t.Fatalf("expected no graph changes for in_progress, got nodes=%d edges=%d", len(nodes), len(edges))
	}
}

func TestNormalizeCodexTokenMetricCreatesModelEdge(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "codex.token.usage",
		Data: map[string]any{
			"model": "gpt-5-codex",
			"type":  "input",
			"value": float64(700),
		},
	}

	nodes, edges := NormalizeEvent(e)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (agent + llm), got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 model_call edge, got %d", len(edges))
	}
	if edges[0].Type != EdgeModelCall {
		t.Fatalf("edge type = %s, want model_call", edges[0].Type)
	}
	if edges[0].Attributes["metric_only"] != true {
		t.Fatalf("metric_only = %v, want true", edges[0].Attributes["metric_only"])
	}
	if edges[0].Attributes["input_tokens"] != float64(700) {
		t.Fatalf("input_tokens = %v, want 700", edges[0].Attributes["input_tokens"])
	}
}

func TestNormalizeCodexSessionFallbackIdentity(t *testing.T) {
	e := bench.CollectedEvent{
		TS:    "2026-02-14T12:00:00Z",
		Event: "codex.api_request",
		Data: map[string]any{
			"model":      "gpt-5-codex",
			"session.id": "a595bc37-ba5a-4345-8fbe-bbdaab766b2d",
		},
	}

	nodes, _ := NormalizeEvent(e)
	var agentNode *NodeUpsert
	for i := range nodes {
		if nodes[i].Type == NodeAgent {
			agentNode = &nodes[i]
			break
		}
	}
	if agentNode == nil {
		t.Fatal("expected agent node")
	}
	if agentNode.ID != "agent:codex:a595bc37" {
		t.Fatalf("agent ID = %q, want agent:codex:a595bc37", agentNode.ID)
	}
	if agentNode.Label != "Codex (a595bc37)" {
		t.Fatalf("agent label = %q, want Codex (a595bc37)", agentNode.Label)
	}
}
