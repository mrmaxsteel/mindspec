package recording

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CodexOTLPResult describes a Codex OTEL bootstrap/update attempt.
type CodexOTLPResult struct {
	ConfigPath       string
	Changed          bool
	Conflict         bool
	ExistingEndpoint string
	ExpectedEndpoint string
}

// DefaultCodexConfigPath returns the default Codex config location under a home directory.
func DefaultCodexConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".codex", "config.toml")
}

// EnsureCodexOTLP configures Codex OTEL settings for AgentMind OTLP ingestion.
//
// The function is idempotent and preserves unrelated TOML sections/keys.
// If a non-AgentMind endpoint already exists, it does not overwrite unless force is true.
func EnsureCodexOTLP(configPath string, force bool) (CodexOTLPResult, error) {
	result := CodexOTLPResult{
		ConfigPath:       configPath,
		ExpectedEndpoint: fmt.Sprintf("http://localhost:%d", defaultRecordingPort),
	}

	content, err := readIfExists(configPath)
	if err != nil {
		return result, fmt.Errorf("reading codex config: %w", err)
	}

	existingEndpoint, ok := tomlStringValue(content, `otel.exporter."otlp-http"`, "endpoint")
	if !ok {
		// Accept the unquoted section form as well.
		existingEndpoint, ok = tomlStringValue(content, "otel.exporter.otlp-http", "endpoint")
	}
	if ok && normalizeEndpoint(existingEndpoint) != normalizeEndpoint(result.ExpectedEndpoint) && !force {
		result.Conflict = true
		result.ExistingEndpoint = existingEndpoint
		return result, nil
	}

	updated := content
	changed := false

	updated, c := upsertTomlValue(updated, "otel", "exporter", `"otlp-http"`)
	changed = changed || c
	updated, c = upsertTomlValue(updated, "otel", "trace_exporter", `"none"`)
	changed = changed || c
	updated, c = upsertTomlValue(updated, "otel", "log_user_prompt", "false")
	changed = changed || c
	updated, c = upsertTomlValue(updated, `otel.exporter."otlp-http"`, "endpoint", fmt.Sprintf("%q", result.ExpectedEndpoint))
	changed = changed || c

	if !changed {
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return result, fmt.Errorf("creating codex config dir: %w", err)
	}

	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	if err := os.WriteFile(configPath, []byte(updated), 0644); err != nil {
		return result, fmt.Errorf("writing codex config: %w", err)
	}

	result.Changed = true
	return result, nil
}

func readIfExists(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func normalizeEndpoint(v string) string {
	v = strings.TrimSpace(v)
	for strings.HasSuffix(v, "/") {
		v = strings.TrimSuffix(v, "/")
	}
	return v
}

func upsertTomlValue(content, section, key, valueLiteral string) (string, bool) {
	lines := splitTomlLines(content)
	sectionStart, sectionEnd, hasSection := tomlSectionRange(lines, section)

	if !hasSection {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("[%s]", section))
		lines = append(lines, fmt.Sprintf("%s = %s", key, valueLiteral))
		return strings.Join(lines, "\n"), true
	}

	for i := sectionStart; i < sectionEnd; i++ {
		k, v, ok := tomlKeyValue(lines[i])
		if !ok || k != key {
			continue
		}
		if tomlValueEqual(v, valueLiteral) {
			return strings.Join(lines, "\n"), false
		}
		lines[i] = fmt.Sprintf("%s = %s", key, valueLiteral)
		return strings.Join(lines, "\n"), true
	}

	line := fmt.Sprintf("%s = %s", key, valueLiteral)
	lines = append(lines[:sectionEnd], append([]string{line}, lines[sectionEnd:]...)...)
	return strings.Join(lines, "\n"), true
}

func tomlStringValue(content, section, key string) (string, bool) {
	lines := splitTomlLines(content)
	sectionStart, sectionEnd, hasSection := tomlSectionRange(lines, section)
	if !hasSection {
		return "", false
	}

	for i := sectionStart; i < sectionEnd; i++ {
		k, v, ok := tomlKeyValue(lines[i])
		if !ok || k != key {
			continue
		}
		parsed, ok := parseSimpleTomlValue(v)
		if !ok || parsed.kind != tomlKindString {
			return "", false
		}
		return parsed.s, true
	}
	return "", false
}

func splitTomlLines(content string) []string {
	if content == "" {
		return []string{}
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func tomlSectionRange(lines []string, section string) (int, int, bool) {
	start := -1
	end := len(lines)

	for i, line := range lines {
		header, ok := tomlSectionHeader(line)
		if !ok {
			continue
		}

		if start >= 0 {
			end = i
			break
		}
		if header == section {
			start = i + 1
		}
	}

	if start < 0 {
		return 0, 0, false
	}
	return start, end, true
}

func tomlSectionHeader(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}

	end := strings.Index(trimmed, "]")
	if end <= 1 {
		return "", false
	}
	return strings.TrimSpace(trimmed[1:end]), true
}

func tomlKeyValue(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
		return "", "", false
	}

	eq := strings.Index(trimmed, "=")
	if eq <= 0 {
		return "", "", false
	}

	key := strings.TrimSpace(trimmed[:eq])
	if key == "" {
		return "", "", false
	}

	value := strings.TrimSpace(trimmed[eq+1:])
	return key, value, true
}

func tomlValueEqual(existing, expected string) bool {
	e, okE := parseSimpleTomlValue(existing)
	x, okX := parseSimpleTomlValue(expected)
	if okE && okX && e.kind == x.kind {
		switch e.kind {
		case tomlKindString:
			return e.s == x.s
		case tomlKindBool:
			return e.b == x.b
		}
	}
	return strings.TrimSpace(existing) == strings.TrimSpace(expected)
}

type tomlSimpleKind int

const (
	tomlKindUnknown tomlSimpleKind = iota
	tomlKindString
	tomlKindBool
)

type tomlSimpleValue struct {
	kind tomlSimpleKind
	s    string
	b    bool
}

func parseSimpleTomlValue(value string) (tomlSimpleValue, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return tomlSimpleValue{}, false
	}

	if value[0] == '"' || value[0] == '\'' {
		s, ok := parseTomlString(value)
		if !ok {
			return tomlSimpleValue{}, false
		}
		return tomlSimpleValue{kind: tomlKindString, s: s}, true
	}

	token := value
	if idx := strings.IndexAny(token, " \t#"); idx >= 0 {
		token = token[:idx]
	}
	switch strings.ToLower(token) {
	case "true":
		return tomlSimpleValue{kind: tomlKindBool, b: true}, true
	case "false":
		return tomlSimpleValue{kind: tomlKindBool, b: false}, true
	default:
		return tomlSimpleValue{}, false
	}
}

func parseTomlString(value string) (string, bool) {
	if value == "" {
		return "", false
	}

	switch value[0] {
	case '"':
		return parseTomlDoubleQuotedString(value)
	case '\'':
		return parseTomlSingleQuotedString(value)
	default:
		return "", false
	}
}

func parseTomlDoubleQuotedString(value string) (string, bool) {
	escaped := false
	for i := 1; i < len(value); i++ {
		ch := value[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			quoted := value[:i+1]
			unquoted, err := strconv.Unquote(quoted)
			if err != nil {
				return "", false
			}
			return unquoted, true
		}
	}
	return "", false
}

func parseTomlSingleQuotedString(value string) (string, bool) {
	end := strings.Index(value[1:], "'")
	if end < 0 {
		return "", false
	}
	return value[1 : 1+end], true
}
