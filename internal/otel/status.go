package otel

// status.go: read-only inspection of the currently configured OTEL
// endpoint. Powers `mindspec otel status`. Spec 084 Bead 1.
//
// Per spec 084 Hard Constraint #5 ("mindspec otel status performs zero
// network I/O"), this file uses ONLY os.ReadFile + parsers. No
// net.Dial, no http.Get. Bead 4 enforces this AST-statically via the
// permanent specgate TestNoOtelNetCalls.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CurrentStatus aggregates what mindspec can observe about the
// currently-configured OTEL endpoint by reading the two canonical
// target files (.claude/settings.local.json and ~/.codex/config.toml).
type CurrentStatus struct {
	// Claude is the OTEL config inferred from the project's
	// .claude/settings.local.json. ClaudePresent indicates whether
	// the file existed and parsed successfully. ClaudeParseErr is
	// non-empty if the file existed but failed to parse — callers
	// surface this in the status report and exit non-zero.
	Claude         Config
	ClaudePath     string
	ClaudePresent  bool
	ClaudeParseErr string

	// Codex is the OTEL config inferred from ~/.codex/config.toml
	// (or the path supplied to ReadCurrentWithCodexPath). Same
	// presence/parse-error semantics as Claude.
	Codex         Config
	CodexPath     string
	CodexPresent  bool
	CodexParseErr string
}

// ReadCurrent reads the OTEL configuration from a fresh project root,
// using the user's default Codex config location (~/.codex/config.toml).
//
// This is the convenience wrapper most callers want. Tests use
// ReadCurrentWithCodexPath to supply a temp-directory path.
func ReadCurrent(root string) (CurrentStatus, error) {
	home, err := os.UserHomeDir()
	codexPath := ""
	if err == nil {
		codexPath = filepath.Join(home, ".codex", "config.toml")
	}
	return ReadCurrentWithCodexPath(root, codexPath)
}

// ReadCurrentWithCodexPath is ReadCurrent with an explicit Codex
// config path, enabling hermetic tests.
func ReadCurrentWithCodexPath(root, codexPath string) (CurrentStatus, error) {
	status := CurrentStatus{
		ClaudePath: filepath.Join(root, ".claude", "settings.local.json"),
		CodexPath:  codexPath,
	}

	// Claude side.
	if c, present, parseErr := readClaudeConfig(status.ClaudePath); parseErr != "" {
		status.ClaudeParseErr = parseErr
	} else if present {
		status.Claude = c
		status.ClaudePresent = true
	}

	// Codex side.
	if codexPath != "" {
		if c, present, parseErr := readCodexConfig(codexPath); parseErr != "" {
			status.CodexParseErr = parseErr
		} else if present {
			status.Codex = c
			status.CodexPresent = true
		}
	}

	return status, nil
}

// readClaudeConfig extracts an OTEL Config from a Claude settings file.
// Returns (Config, present, parseErr). parseErr is non-empty only when
// the file existed but failed to parse as JSON.
func readClaudeConfig(path string) (Config, bool, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, ""
		}
		return Config{}, false, fmt.Sprintf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Config{}, false, fmt.Sprintf("parse %s: %v", path, err)
	}
	envRaw, _ := doc["env"].(map[string]any)
	if envRaw == nil {
		return Config{}, false, ""
	}
	endpoint, _ := envRaw["OTEL_EXPORTER_OTLP_ENDPOINT"].(string)
	if endpoint == "" {
		return Config{}, false, ""
	}
	protocol, _ := envRaw["OTEL_EXPORTER_OTLP_PROTOCOL"].(string)
	service, _ := envRaw["OTEL_SERVICE_NAME"].(string)
	headers := map[string]string{}
	if hdrStr, ok := envRaw["OTEL_EXPORTER_OTLP_HEADERS"].(string); ok && hdrStr != "" {
		parsed, _ := ParseHeaders(hdrStr)
		headers = parsed
	}
	return Config{
		Endpoint:    endpoint,
		Protocol:    protocol,
		ServiceName: service,
		Headers:     headers,
	}, true, ""
}

// codexEndpointRegex extracts the endpoint URL from the inline-exporter
// form mindspec emits: `exporter = { "otlp-http" = { endpoint = "...", ... } }`.
// Used AFTER scoping to the [otel]-namespace lines.
var codexEndpointRegex = regexp.MustCompile(`endpoint\s*=\s*"([^"]+)"`)

// codexProtocolRegex extracts the protocol value from the same line.
var codexProtocolRegex = regexp.MustCompile(`protocol\s*=\s*"([^"]+)"`)

// codexServiceNameRegex extracts service_name from the [otel] table.
var codexServiceNameRegex = regexp.MustCompile(`(?m)^\s*service_name\s*=\s*"([^"]+)"`)

// codexHeadersBlockRegex / codexHeaderPairRegex extract each
// "k" = "v" pair from a headers inline table such as
// `headers = { "k1" = "v1", "k2" = "v2" }`.
var codexHeadersBlockRegex = regexp.MustCompile(`headers\s*=\s*\{([^}]*)\}`)
var codexHeaderPairRegex = regexp.MustCompile(`"([^"]+)"\s*=\s*"([^"]*)"`)

// readCodexConfig extracts an OTEL Config from a Codex TOML file.
// Uses a line-based scoping pass to restrict the value-extraction
// regexes to the [otel] table (and any [otel.*] subtables) — sibling
// top-level tables that happen to contain an `endpoint = "..."` key
// no longer leak into the status reader. A user-supplied Codex config
// with a different exporter shape will report no endpoint, which is
// the correct behavior: status reports the mindspec-managed surface
// only.
func readCodexConfig(path string) (Config, bool, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, ""
		}
		return Config{}, false, fmt.Sprintf("read %s: %v", path, err)
	}
	otelScope := extractOtelScope(string(raw))
	if otelScope == "" {
		return Config{}, false, ""
	}

	endpointMatch := codexEndpointRegex.FindStringSubmatch(otelScope)
	if len(endpointMatch) < 2 {
		return Config{}, false, ""
	}
	cfg := Config{Endpoint: endpointMatch[1]}
	if m := codexProtocolRegex.FindStringSubmatch(otelScope); len(m) >= 2 {
		cfg.Protocol = m[1]
	}
	if m := codexServiceNameRegex.FindStringSubmatch(otelScope); len(m) >= 2 {
		cfg.ServiceName = m[1]
	}
	if hm := codexHeadersBlockRegex.FindStringSubmatch(otelScope); len(hm) >= 2 {
		headers := map[string]string{}
		for _, pair := range codexHeaderPairRegex.FindAllStringSubmatch(hm[1], -1) {
			if len(pair) >= 3 {
				headers[pair[1]] = pair[2]
			}
		}
		if len(headers) > 0 {
			cfg.Headers = headers
		}
	}
	return cfg, true, ""
}

// extractOtelScope returns the substring of `content` consisting only
// of the [otel] table and any [otel.<sub>] subtable bodies, joined
// with newlines. Returns "" when no otel-namespace table is present.
// Uses the same line-based parser strategy as replaceOtelBlock in
// config.go.
func extractOtelScope(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inOtelZone := false
	for _, line := range lines {
		if m := tomlSectionHeaderRegex.FindStringSubmatch(line); m != nil {
			name := strings.TrimSpace(m[1])
			if name == "otel" || strings.HasPrefix(name, "otel.") {
				inOtelZone = true
				continue
			}
			inOtelZone = false
			continue
		}
		if inOtelZone {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n")
}
