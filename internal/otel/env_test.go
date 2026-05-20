package otel

// env_test.go: unit tests for the env-merging helpers moved out of
// cmd/mindspec/record.go per spec 084 Bead 2 CONSENSUS revision #4.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvKeyValues_BasicKeysAndHeaders(t *testing.T) {
	t.Parallel()
	got := EnvKeyValues(Config{
		Endpoint:    "http://localhost:4318",
		Protocol:    "http/protobuf",
		ServiceName: "test-svc",
		Headers:     map[string]string{"x-tenant": "acme"},
	})
	wantKeys := []string{
		"CLAUDE_CODE_ENABLE_TELEMETRY",
		"OTEL_METRICS_EXPORTER",
		"OTEL_LOGS_EXPORTER",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_SERVICE_NAME",
		"OTEL_EXPORTER_OTLP_HEADERS",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in EnvKeyValues output: %v", k, got)
		}
	}
	if got["CLAUDE_CODE_ENABLE_TELEMETRY"] != "1" {
		t.Errorf("CLAUDE_CODE_ENABLE_TELEMETRY = %q; want %q", got["CLAUDE_CODE_ENABLE_TELEMETRY"], "1")
	}
	if got["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://localhost:4318" {
		t.Errorf("endpoint = %q; want http://localhost:4318", got["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	if got["OTEL_EXPORTER_OTLP_HEADERS"] == "" {
		t.Errorf("expected non-empty OTEL_EXPORTER_OTLP_HEADERS for non-empty Headers map")
	}
}

func TestEnvKeyValues_NoHeadersOmitted(t *testing.T) {
	t.Parallel()
	got := EnvKeyValues(Config{
		Endpoint: "http://localhost:4318",
	})
	if _, ok := got["OTEL_EXPORTER_OTLP_HEADERS"]; ok {
		t.Errorf("OTEL_EXPORTER_OTLP_HEADERS should be absent when Headers is empty")
	}
}

func TestMergeEnv_ParentSetWins(t *testing.T) {
	// CONSENSUS revision #2: caller-set OTEL_* env wins over
	// mindspec-rendered overrides.
	t.Parallel()
	parent := []string{
		"PATH=/usr/bin",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://caller-set:9999",
	}
	overrides := map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://mindspec:4318",
		"OTEL_SERVICE_NAME":           "from-mindspec",
	}
	got := MergeEnv(parent, overrides)
	gotMap := envSliceToMap(got)
	if gotMap["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://caller-set:9999" {
		t.Errorf("caller-set OTEL_EXPORTER_OTLP_ENDPOINT must win; got %q",
			gotMap["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	if gotMap["OTEL_SERVICE_NAME"] != "from-mindspec" {
		t.Errorf("absent-from-parent OTEL_SERVICE_NAME should be injected; got %q",
			gotMap["OTEL_SERVICE_NAME"])
	}
	if gotMap["PATH"] != "/usr/bin" {
		t.Errorf("unrelated PATH should be preserved; got %q", gotMap["PATH"])
	}
}

func TestMergeEnv_NoDuplicateKeys(t *testing.T) {
	t.Parallel()
	parent := []string{"FOO=1", "BAR=2"}
	overrides := map[string]string{"BAR": "ignored-because-parent-wins", "BAZ": "3"}
	got := MergeEnv(parent, overrides)

	counts := make(map[string]int)
	for _, kv := range got {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		counts[kv[:eq]]++
	}
	for k, n := range counts {
		if n > 1 {
			t.Errorf("key %q appears %d times; expected 1", k, n)
		}
	}
}

func TestMergeEnv_EmptyOverridesCopiesParent(t *testing.T) {
	t.Parallel()
	parent := []string{"FOO=1"}
	got := MergeEnv(parent, nil)
	if len(got) != 1 || got[0] != "FOO=1" {
		t.Errorf("MergeEnv with nil overrides should pass parent through; got %v", got)
	}
}

func TestBuildWorkloadEnv_NoConfig_PassesParentThrough(t *testing.T) {
	// NOTE: cannot use t.Parallel because t.Setenv is incompatible.
	// HOME is redirected so ReadCurrent does not pick up the
	// developer's real ~/.codex/config.toml.
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	parent := []string{"PATH=/usr/bin", "FOO=bar"}
	got, err := BuildWorkloadEnv(root, parent)
	if err != nil {
		t.Fatalf("unexpected error with no config: %v", err)
	}
	gotMap := envSliceToMap(got)
	if gotMap["PATH"] != "/usr/bin" || gotMap["FOO"] != "bar" {
		t.Errorf("expected parent env passed through; got %v", gotMap)
	}
	if _, ok := gotMap["OTEL_EXPORTER_OTLP_ENDPOINT"]; ok {
		t.Errorf("no config on disk should NOT synthesize OTEL keys; got %v", gotMap)
	}
}

func TestBuildWorkloadEnv_MalformedClaudeConfig_ReturnsError(t *testing.T) {
	// CONSENSUS revision #1: malformed on-disk config surfaces as a
	// real error instead of silently degrading.
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write malformed JSON.
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.local.json"),
		[]byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := BuildWorkloadEnv(root, []string{"PATH=/usr/bin"})
	if err == nil {
		t.Fatalf("expected error for malformed Claude config; got nil")
	}
	if !strings.Contains(err.Error(), "malformed Claude OTEL config") {
		t.Errorf("expected error to mention malformed Claude OTEL config; got %v", err)
	}
}

func TestBuildWorkloadEnv_InvalidValidate_ReturnsError(t *testing.T) {
	// CONSENSUS revision #1: a parseable config that fails
	// Validate() (e.g. blank endpoint) is also surfaced as an error.
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Endpoint is present in the env block but is a relative URL —
	// Validate() rejects non-URL endpoints.
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.local.json"),
		[]byte(`{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"not-a-url"}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := BuildWorkloadEnv(root, []string{"PATH=/usr/bin"})
	if err == nil {
		t.Fatalf("expected validate error; got nil")
	}
	if !strings.Contains(err.Error(), "invalid OTEL config") {
		t.Errorf("expected error to mention invalid OTEL config; got %v", err)
	}
}

func TestBuildWorkloadEnv_HappyPath_InjectsAbsentKeysOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.local.json"),
		[]byte(`{"env":{"OTEL_EXPORTER_OTLP_ENDPOINT":"http://disk:4318","OTEL_EXPORTER_OTLP_PROTOCOL":"http/json","OTEL_SERVICE_NAME":"disk-svc"}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	parent := []string{
		"PATH=/usr/bin",
		// Caller pre-set OTEL_SERVICE_NAME — must win per revision #2.
		"OTEL_SERVICE_NAME=caller-svc",
	}
	got, err := BuildWorkloadEnv(root, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotMap := envSliceToMap(got)
	if gotMap["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://disk:4318" {
		t.Errorf("absent OTEL_EXPORTER_OTLP_ENDPOINT should be injected from disk; got %q",
			gotMap["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	if gotMap["OTEL_SERVICE_NAME"] != "caller-svc" {
		t.Errorf("caller-set OTEL_SERVICE_NAME must win; got %q",
			gotMap["OTEL_SERVICE_NAME"])
	}
}

func envSliceToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		out[kv[:eq]] = kv[eq+1:]
	}
	return out
}
