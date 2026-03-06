package viz

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bench"
)

// CodexFallbackStats reports conversion outcomes for a Codex session import.
type CodexFallbackStats struct {
	Lines            int
	Events           int
	ToolCalls        int
	ToolResults      int
	APIRequests      int
	SkippedMalformed int
	SkippedUnknown   int
	SkippedIgnored   int
}

// ConvertCodexSessionFile converts a Codex session JSONL file into AgentMind NDJSON.
func ConvertCodexSessionFile(inputPath, outputPath string) (*CodexFallbackStats, error) {
	in, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("opening codex session file: %w", err)
	}
	defer in.Close()

	events, stats, err := ConvertCodexSession(in)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}
	out, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("creating output ndjson: %w", err)
	}
	defer out.Close()

	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		data = append(data, '\n')
		if _, err := out.Write(data); err != nil {
			return nil, fmt.Errorf("writing output ndjson: %w", err)
		}
	}

	return stats, nil
}

// ConvertCodexSession converts a Codex session JSONL stream into CollectedEvent records.
func ConvertCodexSession(r io.Reader) ([]bench.CollectedEvent, *CodexFallbackStats, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)

	stats := &CodexFallbackStats{}
	converter := codexFallbackConverter{
		callIDToTool: make(map[string]string),
	}
	var out []bench.CollectedEvent

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		stats.Lines++

		events, status := converter.convertLine(line)
		switch status {
		case lineMalformed:
			stats.SkippedMalformed++
			continue
		case lineUnknown:
			stats.SkippedUnknown++
			continue
		case lineIgnored:
			stats.SkippedIgnored++
			continue
		}

		for _, e := range events {
			stats.Events++
			switch e.Event {
			case "claude_code.tool_use":
				stats.ToolCalls++
			case "claude_code.tool_result":
				stats.ToolResults++
			case "claude_code.api_request":
				stats.APIRequests++
			}
		}
		out = append(out, events...)
	}

	if err := scanner.Err(); err != nil {
		return nil, stats, fmt.Errorf("reading codex session stream: %w", err)
	}

	return out, stats, nil
}

type codexFallbackConverter struct {
	sessionID string
	model     string

	callIDToTool map[string]string
	prevTotal    *codexTokenUsage
}

type codexLineStatus int

const (
	lineOK codexLineStatus = iota
	lineIgnored
	lineUnknown
	lineMalformed
)

type codexSessionLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	ID string `json:"id"`
}

type codexTurnContext struct {
	Model string `json:"model"`
}

type codexResponseItem struct {
	Type      string                 `json:"type"`
	Name      string                 `json:"name"`
	CallID    string                 `json:"call_id"`
	Arguments string                 `json:"arguments"`
	Status    string                 `json:"status"`
	Output    json.RawMessage        `json:"output"`
	Action    map[string]any         `json:"action"`
	Input     any                    `json:"input"`
	Extra     map[string]interface{} `json:"-"`
}

type codexEventMsg struct {
	Type string          `json:"type"`
	Info codexTokenInfo  `json:"info"`
	Item json.RawMessage `json:"item"`
}

type codexTokenInfo struct {
	Last  *codexTokenUsage `json:"last_token_usage"`
	Total *codexTokenUsage `json:"total_token_usage"`
}

type codexTokenUsage struct {
	InputTokens       float64 `json:"input_tokens"`
	CachedInputTokens float64 `json:"cached_input_tokens"`
	OutputTokens      float64 `json:"output_tokens"`
}

var nonZeroExitCodePattern = regexp.MustCompile(`(?i)(exit status|exit code:|exited with code)\s+([1-9][0-9]*)`)

func (c *codexFallbackConverter) convertLine(line []byte) ([]bench.CollectedEvent, codexLineStatus) {
	var rec codexSessionLine
	if err := json.Unmarshal(line, &rec); err != nil {
		return nil, lineMalformed
	}

	ts := normalizeCodexTimestamp(rec.Timestamp)

	switch rec.Type {
	case "session_meta":
		var payload codexSessionMeta
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return nil, lineMalformed
		}
		if payload.ID != "" {
			c.sessionID = payload.ID
		}
		return nil, lineOK

	case "turn_context":
		var payload codexTurnContext
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return nil, lineMalformed
		}
		if payload.Model != "" {
			c.model = payload.Model
		}
		return nil, lineOK

	case "response_item":
		var payload codexResponseItem
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return nil, lineMalformed
		}
		switch payload.Type {
		case "function_call", "custom_tool_call":
			toolName := normalizeToolName(payload.Name, payload.Type, payload.Action)
			if toolName == "" {
				return nil, lineUnknown
			}
			if payload.CallID != "" {
				c.callIDToTool[payload.CallID] = toolName
			}

			data := map[string]any{
				"tool_name": toolName,
			}
			if payload.CallID != "" {
				data["call_id"] = payload.CallID
			}
			augmentFromArguments(data, payload.Arguments)

			return []bench.CollectedEvent{{
				TS:       ts,
				Event:    "claude_code.tool_use",
				Data:     c.attachSession(data),
				Resource: c.resource(),
			}}, lineOK

		case "function_call_output", "custom_tool_call_output":
			toolName := c.callIDToTool[payload.CallID]
			if toolName == "" {
				toolName = "unknown_tool"
			}

			data := map[string]any{
				"tool_name": toolName,
			}
			if payload.CallID != "" {
				data["call_id"] = payload.CallID
			}

			if outputString, ok := rawJSONString(payload.Output); ok && outputIndicatesFailure(outputString) {
				data["error"] = "tool output indicates non-zero exit"
			}
			if payload.Status != "" && !strings.EqualFold(payload.Status, "completed") {
				data["error"] = "tool status is not completed"
			}

			return []bench.CollectedEvent{{
				TS:       ts,
				Event:    "claude_code.tool_result",
				Data:     c.attachSession(data),
				Resource: c.resource(),
			}}, lineOK

		case "web_search_call":
			toolName := webSearchToolName(payload.Action)
			data := map[string]any{
				"tool_name": toolName,
			}
			if query, ok := payload.Action["query"].(string); ok && query != "" {
				data["query"] = query
			}
			if pType, ok := payload.Action["type"].(string); ok && pType != "" {
				data["action_type"] = pType
			}
			return []bench.CollectedEvent{{
				TS:       ts,
				Event:    "claude_code.tool_use",
				Data:     c.attachSession(data),
				Resource: c.resource(),
			}}, lineOK

		case "message", "reasoning":
			return nil, lineIgnored

		default:
			return nil, lineUnknown
		}

	case "event_msg":
		var payload codexEventMsg
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return nil, lineMalformed
		}
		switch payload.Type {
		case "token_count":
			ev, ok := c.tokenUsageEvent(ts, payload.Info)
			if !ok {
				return nil, lineIgnored
			}
			return []bench.CollectedEvent{ev}, lineOK

		case "task_started", "task_complete", "user_message", "agent_reasoning", "agent_message", "item_completed", "context_compacted", "turn_aborted":
			return nil, lineIgnored

		default:
			return nil, lineUnknown
		}

	case "compacted":
		return nil, lineIgnored

	default:
		return nil, lineUnknown
	}
}

func (c *codexFallbackConverter) tokenUsageEvent(ts string, info codexTokenInfo) (bench.CollectedEvent, bool) {
	if info.Total == nil {
		return bench.CollectedEvent{}, false
	}

	if c.prevTotal != nil && tokenUsageEqual(*c.prevTotal, *info.Total) {
		return bench.CollectedEvent{}, false
	}

	var inputTokens, outputTokens, cacheReadTokens float64
	if info.Last != nil {
		inputTokens = info.Last.InputTokens
		outputTokens = info.Last.OutputTokens
		cacheReadTokens = info.Last.CachedInputTokens
	} else if c.prevTotal != nil {
		inputTokens = info.Total.InputTokens - c.prevTotal.InputTokens
		outputTokens = info.Total.OutputTokens - c.prevTotal.OutputTokens
		cacheReadTokens = info.Total.CachedInputTokens - c.prevTotal.CachedInputTokens
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	if cacheReadTokens < 0 {
		cacheReadTokens = 0
	}

	c.prevTotal = info.Total
	if inputTokens == 0 && outputTokens == 0 && cacheReadTokens == 0 {
		return bench.CollectedEvent{}, false
	}

	model := c.model
	if model == "" {
		model = "gpt-5-codex"
	}
	data := map[string]any{
		"model":         model,
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
	}
	if cacheReadTokens > 0 {
		data["cache_read_tokens"] = cacheReadTokens
	}

	return bench.CollectedEvent{
		TS:       ts,
		Event:    "claude_code.api_request",
		Data:     c.attachSession(data),
		Resource: c.resource(),
	}, true
}

func (c *codexFallbackConverter) resource() map[string]any {
	out := map[string]any{
		"agent.name":   "Codex",
		"service.name": "codex",
	}
	if c.sessionID != "" {
		out["service.instance.id"] = truncateSessionID(c.sessionID)
	}
	return out
}

func (c *codexFallbackConverter) attachSession(data map[string]any) map[string]any {
	if c.sessionID != "" {
		data["session.id"] = c.sessionID
	}
	return data
}

func normalizeToolName(name, payloadType string, action map[string]any) string {
	if payloadType == "web_search_call" {
		return webSearchToolName(action)
	}
	if name == "" {
		return ""
	}
	return name
}

func webSearchToolName(action map[string]any) string {
	actionType, _ := action["type"].(string)
	switch actionType {
	case "search_query":
		return "WebSearch"
	case "open", "click", "find", "screenshot":
		return "WebFetch"
	default:
		return "WebSearch"
	}
}

func augmentFromArguments(data map[string]any, args string) {
	if strings.TrimSpace(args) == "" {
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return
	}
	if cmd, ok := parsed["cmd"].(string); ok && cmd != "" {
		data["command"] = cmd
	}
	if path, ok := parsed["path"].(string); ok && path != "" {
		data["path"] = path
	}
	if workdir, ok := parsed["workdir"].(string); ok && workdir != "" {
		data["path"] = workdir
	}
}

func rawJSONString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	return string(raw), true
}

func outputIndicatesFailure(output string) bool {
	return nonZeroExitCodePattern.MatchString(output)
}

func normalizeCodexTimestamp(ts string) string {
	if strings.TrimSpace(ts) == "" {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func tokenUsageEqual(a, b codexTokenUsage) bool {
	return a.InputTokens == b.InputTokens &&
		a.CachedInputTokens == b.CachedInputTokens &&
		a.OutputTokens == b.OutputTokens
}
