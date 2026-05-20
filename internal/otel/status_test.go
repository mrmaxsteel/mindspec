package otel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCurrent_EmptyRoot(t *testing.T) {
	root := t.TempDir()
	codex := filepath.Join(t.TempDir(), "config.toml")

	status, err := ReadCurrentWithCodexPath(root, codex)
	if err != nil {
		t.Fatalf("ReadCurrent: %v", err)
	}
	if status.ClaudePresent {
		t.Errorf("expected ClaudePresent=false on empty root")
	}
	if status.CodexPresent {
		t.Errorf("expected CodexPresent=false on empty codex path")
	}
	if status.ClaudeParseErr != "" {
		t.Errorf("unexpected ClaudeParseErr: %s", status.ClaudeParseErr)
	}
}

func TestReadCurrent_ClaudeOnly(t *testing.T) {
	root := t.TempDir()
	codex := filepath.Join(t.TempDir(), "config.toml")
	c := Config{Endpoint: "http://collector:4318", ServiceName: "myapp"}
	if _, err := WriteClaudeSettingsLocal(root, c); err != nil {
		t.Fatalf("WriteClaudeSettingsLocal: %v", err)
	}

	status, err := ReadCurrentWithCodexPath(root, codex)
	if err != nil {
		t.Fatalf("ReadCurrent: %v", err)
	}
	if !status.ClaudePresent {
		t.Errorf("expected ClaudePresent=true")
	}
	if status.Claude.Endpoint != "http://collector:4318" {
		t.Errorf("Claude endpoint mismatch: %q", status.Claude.Endpoint)
	}
	if status.Claude.ServiceName != "myapp" {
		t.Errorf("Claude service name mismatch: %q", status.Claude.ServiceName)
	}
	if status.CodexPresent {
		t.Errorf("CodexPresent should be false when file absent")
	}
}

func TestReadCurrent_CodexOnly(t *testing.T) {
	root := t.TempDir()
	dir := t.TempDir()
	codex := filepath.Join(dir, "config.toml")
	c := Config{Endpoint: "http://collector:4318", ServiceName: "myapp",
		Headers: map[string]string{"x-tenant": "acme"}}
	if _, err := WriteCodexConfig(codex, c); err != nil {
		t.Fatalf("WriteCodexConfig: %v", err)
	}

	status, err := ReadCurrentWithCodexPath(root, codex)
	if err != nil {
		t.Fatalf("ReadCurrent: %v", err)
	}
	if !status.CodexPresent {
		t.Errorf("expected CodexPresent=true")
	}
	if status.Codex.Endpoint != "http://collector:4318" {
		t.Errorf("Codex endpoint mismatch: %q", status.Codex.Endpoint)
	}
	if status.Codex.ServiceName != "myapp" {
		t.Errorf("Codex service_name mismatch: %q", status.Codex.ServiceName)
	}
	if status.Codex.Headers["x-tenant"] != "acme" {
		t.Errorf("Codex headers mismatch: %v", status.Codex.Headers)
	}
}

func TestReadCurrent_MalformedClaude(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.local.json"),
		[]byte("not valid json {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := ReadCurrentWithCodexPath(root, "")
	if err != nil {
		t.Fatalf("ReadCurrent should not error on malformed input: %v", err)
	}
	if status.ClaudeParseErr == "" {
		t.Errorf("expected ClaudeParseErr to be populated")
	}
	if status.ClaudePresent {
		t.Errorf("expected ClaudePresent=false on parse error")
	}
}

func TestReadCurrent_NoOtelKeyInValidJson(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Valid JSON without OTEL keys.
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.local.json"),
		[]byte(`{"permissions": {"allow": ["Bash(ls)"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	status, _ := ReadCurrentWithCodexPath(root, "")
	if status.ClaudePresent {
		t.Errorf("ClaudePresent should be false when no OTEL endpoint configured")
	}
	if status.ClaudeParseErr != "" {
		t.Errorf("ClaudeParseErr should be empty when file parsed cleanly: %s", status.ClaudeParseErr)
	}
}
