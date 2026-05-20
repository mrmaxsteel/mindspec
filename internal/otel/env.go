package otel

// env.go: render an OTEL Config into the env-var key/value pairs that
// `mindspec record start` overlays on a child workload's environment.
//
// Spec 084 Bead 2: these helpers used to live in cmd/mindspec/record.go
// but the bead-2 panel (CONSENSUS, required revision #4) moved them
// here so they can be unit-tested in isolation, without spinning up
// the cobra subprocess harness.
//
// Three small functions live here:
//
//   - EnvKeyValues(cfg)            — render Config -> map[string]string
//   - MergeEnv(parent, overrides)  — overlay map onto a KEY=VAL slice
//   - LoadConfigured(root)         — read the user's on-disk config
//   - BuildWorkloadEnv(root,
//                       parent)    — convenience: LoadConfigured +
//                                    EnvKeyValues + MergeEnv, with
//                                    cfg.Validate() surfaced as a real
//                                    error rather than silently
//                                    swallowed (CONSENSUS revision #1).

import (
	"fmt"
	"strings"
)

// EnvKeyValues renders an OTEL Config as the flat KEY=VALUE map that
// mindspec injects into a workload's exec.Cmd.Env. Uses the same key
// set as RenderClaudeSettingsLocal:
//
//	CLAUDE_CODE_ENABLE_TELEMETRY=1
//	OTEL_METRICS_EXPORTER=otlp
//	OTEL_LOGS_EXPORTER=otlp
//	OTEL_EXPORTER_OTLP_PROTOCOL=<protocol>
//	OTEL_EXPORTER_OTLP_ENDPOINT=<endpoint>
//	OTEL_SERVICE_NAME=<service-name>
//	OTEL_EXPORTER_OTLP_HEADERS=<k1=v1,k2=v2>  (only when non-empty)
//
// The Config is Normalized first so callers can pass a freshly parsed
// status entry without re-deriving defaults.
func EnvKeyValues(c Config) map[string]string {
	c = c.Normalize()
	out := map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY": "1",
		"OTEL_METRICS_EXPORTER":        "otlp",
		"OTEL_LOGS_EXPORTER":           "otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL":  c.Protocol,
		"OTEL_EXPORTER_OTLP_ENDPOINT":  c.Endpoint,
		"OTEL_SERVICE_NAME":            c.ServiceName,
	}
	if hdr := c.FormatHeaders(); hdr != "" {
		out["OTEL_EXPORTER_OTLP_HEADERS"] = hdr
	}
	return out
}

// MergeEnv overlays `overrides` onto the `parent` KEY=VALUE slice and
// returns a new slice with no duplicate keys.
//
// Precedence (CONSENSUS revision #2): if a key already exists in
// `parent`, the parent value WINS — the override is dropped. This
// matches POSIX expectation that an explicit `export OTEL_*=...` in
// the caller's shell takes precedence over mindspec-rendered config.
// Overrides whose keys are absent from `parent` are appended.
//
// Lines in `parent` without an '=' are preserved verbatim (this should
// never happen for env vars, but os.Environ() callers occasionally
// see odd entries on some platforms — be permissive).
func MergeEnv(parent []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		// Return a copy so the caller can't mutate os.Environ()'s slice.
		out := make([]string, len(parent))
		copy(out, parent)
		return out
	}
	present := make(map[string]bool, len(parent))
	out := make([]string, 0, len(parent)+len(overrides))
	for _, kv := range parent {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			out = append(out, kv)
			continue
		}
		key := kv[:eq]
		present[key] = true
		out = append(out, kv)
	}
	// Stable order: iterate the canonical key list rather than the
	// map (which has randomized iteration order in Go).
	for _, k := range canonicalEnvOrder {
		v, ok := overrides[k]
		if !ok {
			continue
		}
		if present[k] {
			// CONSENSUS revision #2: caller-set wins.
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}

// canonicalEnvOrder is the iteration order MergeEnv uses to append
// new overrides — fixed so callers (and tests) see deterministic
// output even though Go's map iteration is randomized.
var canonicalEnvOrder = []string{
	"CLAUDE_CODE_ENABLE_TELEMETRY",
	"OTEL_METRICS_EXPORTER",
	"OTEL_LOGS_EXPORTER",
	"OTEL_EXPORTER_OTLP_PROTOCOL",
	"OTEL_EXPORTER_OTLP_ENDPOINT",
	"OTEL_SERVICE_NAME",
	"OTEL_EXPORTER_OTLP_HEADERS",
}

// LoadConfigured reads the OTEL config the user previously wrote via
// `mindspec otel setup`, preferring the Claude target over Codex
// (Claude is the project-local default; Codex is user-global).
//
// Returns (Config{}, false, nil) when nothing is configured.
// Returns (Config{}, false, err) when an on-disk config file existed
// but failed to parse — callers should surface this rather than
// silently degrading (CONSENSUS revision #1).
func LoadConfigured(root string) (Config, bool, error) {
	status, err := ReadCurrent(root)
	if err != nil {
		return Config{}, false, err
	}
	if status.ClaudeParseErr != "" {
		return Config{}, false, fmt.Errorf("malformed Claude OTEL config: %s", status.ClaudeParseErr)
	}
	if status.CodexParseErr != "" && !status.ClaudePresent {
		// Only surface a Codex parse error if Claude was absent —
		// otherwise the Claude config wins and the Codex side is
		// informational.
		return Config{}, false, fmt.Errorf("malformed Codex OTEL config: %s", status.CodexParseErr)
	}
	if status.ClaudePresent {
		return status.Claude, true, nil
	}
	if status.CodexPresent {
		return status.Codex, true, nil
	}
	return Config{}, false, nil
}

// BuildWorkloadEnv is the convenience wrapper `mindspec record start`
// uses: load the user-configured OTEL endpoint, validate it, render
// the canonical env keys, and overlay them on the parent process env.
//
// Error contract (CONSENSUS revision #1): a malformed on-disk config
// file, or one that fails Validate, returns a real error. The caller
// MUST surface that error rather than silently passing the parent env
// through — silent degradation is reserved for the "no config exists"
// case (status.ClaudePresent == false && status.CodexPresent == false),
// not for "config exists but is broken".
//
// When no OTEL endpoint is configured at all, returns (parent, nil) —
// the workload's own OTEL SDK will silently drop events per its own
// no-endpoint contract (spec 084 line 535).
func BuildWorkloadEnv(root string, parent []string) ([]string, error) {
	cfg, present, err := LoadConfigured(root)
	if err != nil {
		return nil, err
	}
	if !present {
		// No config on disk anywhere; pass parent env through unchanged.
		// (Silent degradation is the workload's OTEL SDK contract, not
		// mindspec's.)
		out := make([]string, len(parent))
		copy(out, parent)
		return out, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid OTEL config on disk: %w", err)
	}
	return MergeEnv(parent, EnvKeyValues(cfg)), nil
}
