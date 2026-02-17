package brownfield

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	llmPromptVersion       = "v1"
	llmClassificationLimit = 16000
)

type llmClassification struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Rationale  string  `json:"rationale"`
}

var supportedLLMProviders = map[string]struct{}{
	"claude-cli": {},
	"claude":     {},
	"anthropic":  {},
}

var allowedCategories = map[string]struct{}{
	"adr":         {},
	"spec":        {},
	"domain":      {},
	"core":        {},
	"context-map": {},
	"glossary":    {},
	"user-docs":   {},
	"unknown":     {},
}

func classifyWithLLM(root string, report *Report) ([]ClassificationEntry, error) {
	provider := strings.ToLower(strings.TrimSpace(report.LLM.Provider))
	if provider == "" || provider == "off" || provider == "none" {
		return report.Classification, nil
	}
	if _, ok := supportedLLMProviders[provider]; !ok {
		return nil, fmt.Errorf("provider %q is not supported for migration classification; supported providers: claude-cli", report.LLM.Provider)
	}

	out := make([]ClassificationEntry, len(report.Classification))
	copy(out, report.Classification)

	for i := range out {
		if !out[i].RequiresLLM {
			continue
		}

		result, err := classifyOneWithClaude(root, out[i].Path, report.LLM.Model)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", out[i].Path, err)
		}

		out[i].Category = result.Category
		out[i].Confidence = clampConfidence(result.Confidence)
		out[i].Rule = "llm:" + llmPromptVersion
		out[i].Rationale = strings.TrimSpace(result.Rationale)
		out[i].RequiresLLM = out[i].Confidence < 0.70
	}

	return out, nil
}

func classifyOneWithClaude(root, relPath, model string) (llmClassification, error) {
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	data, err := os.ReadFile(abs)
	if err != nil {
		return llmClassification{}, fmt.Errorf("read source: %w", err)
	}

	prompt := buildLLMClassificationPrompt(relPath, string(data))
	args := []string{
		"-p", prompt,
		"--max-turns", "1",
		"--no-session-persistence",
	}
	if model != "" && model != "default" {
		args = append(args, "--model", model)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return llmClassification{}, fmt.Errorf("claude classification timed out")
		}
		snippet := strings.TrimSpace(string(out))
		if len(snippet) > 240 {
			snippet = snippet[:240]
		}
		return llmClassification{}, fmt.Errorf("claude classification failed: %w (%s)", err, snippet)
	}

	raw, err := extractJSONObject(string(out))
	if err != nil {
		return llmClassification{}, err
	}

	var parsed llmClassification
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return llmClassification{}, fmt.Errorf("parse classification json: %w", err)
	}

	parsed.Category = normalizeCategory(parsed.Category)
	if _, ok := allowedCategories[parsed.Category]; !ok {
		return llmClassification{}, fmt.Errorf("invalid category %q", parsed.Category)
	}
	parsed.Confidence = clampConfidence(parsed.Confidence)
	parsed.Rationale = strings.TrimSpace(parsed.Rationale)
	if parsed.Rationale == "" {
		parsed.Rationale = "LLM classification rationale unavailable."
	}
	return parsed, nil
}

func buildLLMClassificationPrompt(path, content string) string {
	content = truncateString(content, llmClassificationLimit)
	return fmt.Sprintf(
		`Classify this markdown document for MindSpec migration.

Prompt version: %s
Allowed categories: adr, spec, domain, core, context-map, glossary, user-docs, unknown.
Return strict JSON only (no prose, no markdown, no code fences):
{"category":"<allowed-category>","confidence":<0_to_1>,"rationale":"<brief rationale>"}

Path: %s
Content:
%s`,
		llmPromptVersion,
		path,
		content,
	)
}

func extractJSONObject(raw string) (string, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("no JSON object found in LLM output")
	}
	return strings.TrimSpace(raw[start : end+1]), nil
}

func normalizeCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "adr":
		return "adr"
	case "spec", "specs":
		return "spec"
	case "domain", "domains":
		return "domain"
	case "core":
		return "core"
	case "context-map", "context_map", "context map":
		return "context-map"
	case "glossary":
		return "glossary"
	case "user-docs", "user_docs", "user docs", "user-doc", "user":
		return "user-docs"
	default:
		return "unknown"
	}
}

func clampConfidence(confidence float64) float64 {
	if confidence < 0 {
		return 0
	}
	if confidence > 1 {
		return 1
	}
	return confidence
}

func truncateString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	head := (max * 3) / 4
	tail := max - head
	if head < 1 {
		head = max
		tail = 0
	}
	if tail < 0 {
		tail = 0
	}
	if tail == 0 {
		return s[:max]
	}
	return s[:head] + "\n...\n" + s[len(s)-tail:]
}
