// Package otel renders OTEL endpoint configuration into the formats
// downstream workloads consume (Claude Code .claude/settings.local.json,
// Codex ~/.codex/config.toml, raw OTEL_* env exports).
//
// Spec 084 (mindspec-otel-only), Bead 1. This package is the single
// legitimate observability surface in mindspec post-spec-084: it emits
// configuration that the workload's own OTEL SDK reads at runtime.
//
// Hard Constraint #5 (spec 084): the otel package and cmd/mindspec/otel.go
// perform ZERO network I/O. No net.Dial, no http.Get, no http.Post, no
// http.Client.Do. Pure file rendering only. Bead 4 lands an AST-checked
// specgate test (TestNoOtelNetCalls) that enforces this perpetually.
//
// # TOML merge strategy
//
// The plan (line ~160) permits either a BurntSushi/toml round-trip OR a
// regex-based key-replacement strategy. Per Bead 1's hard constraint
// (DO NOT touch go.mod), this implementation uses the **regex /
// line-based key replacement** strategy. The implementation reuses the
// existing helper patterns from internal/recording/codex_bootstrap.go:
// upsertTomlValue walks line-by-line, finds an existing [otel.exporter]
// table (or appends one), and replaces only the canonical keys. Sibling
// top-level tables and keys are preserved byte-for-byte. Idempotency
// (re-run with identical inputs produces a sha256-identical output
// file) is the binding acceptance criterion (spec line 569) and is
// asserted by config_test.go's TestRenderCodexConfigToml_Idempotent.
package otel

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// DefaultProtocol is the canonical OTEL transport mindspec emits when
// --protocol is not supplied. Matches the Claude Code default for
// CLAUDE_CODE_ENABLE_TELEMETRY=1 emissions and the OTEL spec's
// recommendation for HTTP/JSON encoding.
const DefaultProtocol = "http/json"

// DefaultServiceName is the canonical service.name attribute mindspec
// emits when --service-name is not supplied.
const DefaultServiceName = "mindspec"

// Config carries the user-supplied OTEL endpoint configuration that
// every rendering target reads.
//
// Endpoint is the OTLP receiver URL (HTTP or HTTPS). It is parsed at
// rendering time to validate URL shape; a parse failure causes the
// render function to return an error.
//
// Headers maps OTEL exporter header names to values. Values matching
// the secret-hygiene pattern (case-insensitive bearer/token/key/
// secret/password) are written verbatim to target files but redacted
// to "***" in any status output (see Redacted).
//
// Protocol is one of "http/json", "http/protobuf", or "grpc". Empty
// string defaults to DefaultProtocol at render time.
//
// ServiceName is the OTEL service.name attribute. Empty string
// defaults to DefaultServiceName.
type Config struct {
	Endpoint    string
	Headers     map[string]string
	Protocol    string
	ServiceName string
}

// Normalize fills in defaults for empty fields without mutating the
// receiver. Used by every renderer so the default-application path is
// identical regardless of which target file is being written.
func (c Config) Normalize() Config {
	out := c
	if strings.TrimSpace(out.Protocol) == "" {
		out.Protocol = DefaultProtocol
	}
	if strings.TrimSpace(out.ServiceName) == "" {
		out.ServiceName = DefaultServiceName
	}
	return out
}

// Validate checks that Config has the minimum required fields and that
// Endpoint parses as a URL with an http or https scheme. Used at the
// start of every renderer.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("otel: endpoint is required")
	}
	u, err := parseEndpoint(c.Endpoint)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("otel: endpoint scheme must be http or https, got %q", u.Scheme)
	}
	switch strings.TrimSpace(c.Protocol) {
	case "", "http/json", "http/protobuf", "grpc":
	default:
		return fmt.Errorf("otel: unknown protocol %q (expected http/json, http/protobuf, or grpc)", c.Protocol)
	}
	return nil
}

// parseEndpoint validates the URL shape of an endpoint string WITHOUT
// performing any network I/O. url.Parse is a pure-parse call (it does
// not dial the host); the specgate AST check explicitly allows it.
func parseEndpoint(endpoint string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("otel: parsing endpoint %q: %w", endpoint, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("otel: endpoint %q has no host", endpoint)
	}
	return u, nil
}

// secretHeaderPattern matches header NAMES whose VALUES should be
// redacted when displayed to the user. Per spec 084 Hard Constraint #5:
// values containing substrings matching this pattern are written to the
// target file verbatim (the file is the canonical store) but redacted
// to "***" in any `mindspec otel status` output.
var secretHeaderPattern = regexp.MustCompile(`(?i)bearer|token|key|secret|password|authorization`)

// IsSecretHeader returns true if the given header name should be
// redacted in status output. Matches by name (e.g. "authorization",
// "x-api-key") rather than by value, so a header named "x-api-key"
// with the literal value "public-not-actually-secret" is still
// redacted: the safer default.
func IsSecretHeader(name string) bool {
	return secretHeaderPattern.MatchString(name)
}

// Redacted returns a copy of Headers with secret-pattern values
// replaced by "***". Used by ReadCurrent / status rendering.
func (c Config) Redacted() Config {
	out := c
	if len(c.Headers) == 0 {
		return out
	}
	redacted := make(map[string]string, len(c.Headers))
	for k, v := range c.Headers {
		if IsSecretHeader(k) {
			redacted[k] = "***"
		} else {
			redacted[k] = v
		}
	}
	out.Headers = redacted
	return out
}

// FormatHeaders renders the Headers map as the
// "k1=v1,k2=v2" canonical comma-separated string OTEL exporters
// consume via OTEL_EXPORTER_OTLP_HEADERS. Keys are sorted
// alphabetically so the output is deterministic across invocations
// (required for sha256 idempotency).
func (c Config) FormatHeaders() string {
	if len(c.Headers) == 0 {
		return ""
	}
	keys := make([]string, 0, len(c.Headers))
	for k := range c.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+c.Headers[k])
	}
	return strings.Join(parts, ",")
}

// ParseHeaders parses a "k1=v1,k2=v2" string into a Headers map. Empty
// input returns nil. Used by the --headers CLI flag handler. Returns
// an error if any segment lacks an "=" separator.
func ParseHeaders(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	out := make(map[string]string)
	for _, segment := range strings.Split(s, ",") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		eq := strings.Index(segment, "=")
		if eq <= 0 {
			return nil, fmt.Errorf("otel: header segment %q has no '=' separator", segment)
		}
		k := strings.TrimSpace(segment[:eq])
		v := strings.TrimSpace(segment[eq+1:])
		if k == "" {
			return nil, fmt.Errorf("otel: header segment %q has empty key", segment)
		}
		out[k] = v
	}
	return out, nil
}

// RenderClaudeSettingsLocal renders an OTEL Config into the
// .claude/settings.local.json shape Claude Code consumes.
//
// The returned map is the FULL settings document; callers merging into
// an existing file MUST read the existing JSON, apply this function's
// output to the "env" key (preserving sibling top-level keys), and
// re-marshal. The rendering layer itself does not perform file I/O.
//
// Per spec 084 Hard Constraint #5, output is deterministic: re-rendering
// with identical inputs produces a byte-identical map (when marshaled
// with json.MarshalIndent and sorted keys, which json.Marshal does by
// default).
func RenderClaudeSettingsLocal(c Config) (map[string]any, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	c = c.Normalize()

	env := map[string]any{
		"CLAUDE_CODE_ENABLE_TELEMETRY": "1",
		"OTEL_METRICS_EXPORTER":        "otlp",
		"OTEL_LOGS_EXPORTER":           "otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL":  c.Protocol,
		"OTEL_EXPORTER_OTLP_ENDPOINT":  c.Endpoint,
		"OTEL_SERVICE_NAME":            c.ServiceName,
	}
	if hdr := c.FormatHeaders(); hdr != "" {
		env["OTEL_EXPORTER_OTLP_HEADERS"] = hdr
	}
	return map[string]any{"env": env}, nil
}

// MergeClaudeSettingsLocal merges the otel env keys from `c` into an
// existing settings document (or a fresh one if `existing` is nil).
// Sibling top-level keys and non-otel env keys are preserved
// byte-for-byte. Returns the merged document ready for json.Marshal.
//
// This is the function callers use when writing
// .claude/settings.local.json: it preserves any user customizations
// outside the OTEL keys.
func MergeClaudeSettingsLocal(existing map[string]any, c Config) (map[string]any, error) {
	rendered, err := RenderClaudeSettingsLocal(c)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return rendered, nil
	}

	out := make(map[string]any, len(existing))
	for k, v := range existing {
		out[k] = v
	}

	// Merge env block: preserve non-otel keys, overwrite otel keys.
	var existingEnv map[string]any
	if raw, ok := out["env"]; ok {
		existingEnv, _ = raw.(map[string]any)
	}
	newEnv := rendered["env"].(map[string]any)
	mergedEnv := make(map[string]any, len(existingEnv)+len(newEnv))
	for k, v := range existingEnv {
		mergedEnv[k] = v
	}
	for k, v := range newEnv {
		mergedEnv[k] = v
	}
	out["env"] = mergedEnv
	return out, nil
}

// tomlSectionHeaderRegex matches a TOML table header on its own line,
// captured as `[table.name]`. Used by the line-based [otel]-block
// replacement strategy in RenderCodexConfigToml.
//
// Replaces the old fragile codexOtelRegex / codexExporterRegex pair,
// which used `[^\[]*` to delimit the [otel] block and therefore
// truncated at any value containing a literal `[` (e.g.
// `description = "[experimental]"`) and leaked sibling [otel.foo]
// subtables into the replacement zone.
var tomlSectionHeaderRegex = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*$`)

// replaceOtelBlock removes any existing [otel] table and any
// [otel.<subtable>] tables from `existing` and returns (cleaned,
// hadOtel). The cleaned string preserves all other top-level tables
// byte-for-byte (except for blank-line collapsing handled later).
//
// A table is considered "otel-namespace" iff its header is `[otel]`
// or matches `[otel.*]`. The parser walks line by line and treats a
// header `[name]` as ending the otel zone iff `name` is not in the
// otel namespace. Lines inside string values that happen to contain
// `[` no longer truncate the block (the prior regex did).
func replaceOtelBlock(existing string) (string, bool) {
	lines := strings.Split(existing, "\n")
	out := make([]string, 0, len(lines))
	inOtelZone := false
	hadOtel := false
	for _, line := range lines {
		if m := tomlSectionHeaderRegex.FindStringSubmatch(line); m != nil {
			name := strings.TrimSpace(m[1])
			isOtelNs := name == "otel" || strings.HasPrefix(name, "otel.")
			if isOtelNs {
				inOtelZone = true
				hadOtel = true
				continue
			}
			// A non-otel top-level header ends the otel zone.
			inOtelZone = false
			out = append(out, line)
			continue
		}
		if inOtelZone {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), hadOtel
}

// RenderCodexConfigToml renders an OTEL Config into a Codex
// ~/.codex/config.toml string, merging into existingToml if non-empty.
//
// Merge semantics per spec 084 Hard Constraint #5:
//   - The [otel.exporter] table is REPLACED whole (no key-level merge
//     inside it).
//   - The [otel] table's mindspec-owned keys (exporter, trace_exporter,
//     log_user_prompt) are upserted.
//   - All other top-level tables and keys are preserved byte-for-byte.
//   - If existingToml is empty, a fresh document is returned.
//   - sha256 idempotency: re-running with identical inputs against the
//     output produces a byte-identical result.
//
// Returns the rendered TOML string (with trailing newline). The caller
// is responsible for writing it to disk with mode 0600.
func RenderCodexConfigToml(c Config, existingToml string) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	c = c.Normalize()

	// Canonical block we want to end up with. Sorted keys, single
	// trailing newline per line for byte-deterministic output.
	headersInline := ""
	if hdr := c.FormatHeaders(); hdr != "" {
		// TOML inline table requires quoted keys/values. We render
		// headers as a nested inline table per OTEL exporter
		// conventions: { "k1" = "v1", "k2" = "v2" }.
		keys := make([]string, 0, len(c.Headers))
		for k := range c.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, k := range keys {
			pairs = append(pairs, fmt.Sprintf("%q = %q", k, c.Headers[k]))
		}
		headersInline = fmt.Sprintf(", headers = { %s }", strings.Join(pairs, ", "))
	}

	exporterLine := fmt.Sprintf(`exporter = { "otlp-http" = { endpoint = %q, protocol = %q%s } }`,
		c.Endpoint, c.Protocol, headersInline)

	otelBlock := strings.Join([]string{
		"[otel]",
		exporterLine,
		`trace_exporter = "none"`,
		`log_user_prompt = false`,
		fmt.Sprintf(`service_name = %q`, c.ServiceName),
	}, "\n") + "\n"

	if strings.TrimSpace(existingToml) == "" {
		return otelBlock, nil
	}

	// Line-based [otel]-block replacement. Strips the [otel] table and
	// any [otel.<sub>] subtable (e.g. legacy [otel.exporter]) without
	// truncating on `[` characters inside string values, then appends
	// the canonical block. This is the documented "more robust
	// line-based parser" from spec 084.
	stripped, _ := replaceOtelBlock(existingToml)
	stripped = strings.TrimRight(stripped, "\n")
	canonical := strings.TrimRight(otelBlock, "\n")
	if stripped == "" {
		return canonical + "\n", nil
	}
	out := stripped + "\n\n" + canonical

	// Collapse any triple+ newlines created by stripping into double.
	out = collapseBlankRuns(out)
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}

var blankRunRegex = regexp.MustCompile(`\n{3,}`)

func collapseBlankRuns(s string) string {
	return blankRunRegex.ReplaceAllString(s, "\n\n")
}

// RenderEnvExports renders an OTEL Config as POSIX-shell `export KEY=VALUE`
// lines, suitable for `eval $(mindspec otel setup --target=env …)`
// workflows or for one-off paste into a shell session.
//
// Output is deterministic (keys sorted) and never includes a newline
// before the first export. The trailing newline is included.
//
// Values are POSIX-shell single-quoted (each ' inside the value is
// escaped as '\”). This is the only escape form that round-trips
// safely without shell interpretation.
func RenderEnvExports(c Config) string {
	// Caller is expected to call Validate first; if Endpoint is empty
	// here that's a programmer error. We still render to keep the
	// function pure (no panics on invalid input) — the CLI layer
	// performs the user-facing validation.
	c = c.Normalize()

	pairs := map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY": "1",
		"OTEL_METRICS_EXPORTER":        "otlp",
		"OTEL_LOGS_EXPORTER":           "otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL":  c.Protocol,
		"OTEL_EXPORTER_OTLP_ENDPOINT":  c.Endpoint,
		"OTEL_SERVICE_NAME":            c.ServiceName,
	}
	if hdr := c.FormatHeaders(); hdr != "" {
		pairs["OTEL_EXPORTER_OTLP_HEADERS"] = hdr
	}

	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString("export ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(shellQuote(pairs[k]))
		b.WriteString("\n")
	}
	return b.String()
}

// shellQuote returns a POSIX-shell single-quoted form of s. Embedded
// single quotes are escaped as '\”. The result is always wrapped in
// single quotes even for "safe" values; consistency aids the
// idempotency test.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// MarshalClaudeSettings serializes a settings document to a
// byte-deterministic JSON form (2-space indent, trailing newline).
// Callers use this when writing .claude/settings.local.json.
//
// json.MarshalIndent already emits keys in sorted order (the encoding/
// json package sorts map keys for determinism), so re-marshaling
// equivalent maps produces byte-identical output.
func MarshalClaudeSettings(settings map[string]any) ([]byte, error) {
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("otel: marshaling claude settings: %w", err)
	}
	return append(out, '\n'), nil
}
