package recording

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCodexOTLPFreshAndIdempotent(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".codex", "config.toml")

	result, err := EnsureCodexOTLP(configPath, false)
	if err != nil {
		t.Fatalf("EnsureCodexOTLP first call: %v", err)
	}
	if !result.Changed {
		t.Fatal("expected first call to write codex config")
	}
	if result.Conflict {
		t.Fatal("unexpected conflict on fresh config")
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(raw)

	assertContainsLine(t, got, `[otel]`)
	assertContainsLine(t, got, `exporter = { "otlp-http" = { endpoint = "http://localhost:4318/v1/logs", protocol = "json" } }`)
	assertContainsLine(t, got, `trace_exporter = "none"`)
	assertContainsLine(t, got, `log_user_prompt = false`)
	assertNotContainsLine(t, got, `[otel.exporter."otlp-http"]`)

	result, err = EnsureCodexOTLP(configPath, false)
	if err != nil {
		t.Fatalf("EnsureCodexOTLP second call: %v", err)
	}
	if result.Changed {
		t.Fatal("expected second call to be idempotent")
	}
	if result.Conflict {
		t.Fatal("unexpected conflict on already-configured endpoint")
	}
}

func TestEnsureCodexOTLPMergesExistingConfig(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".codex", "config.toml")
	existing := strings.Join([]string{
		`[model]`,
		`provider = "openai"`,
		``,
		`[otel]`,
		`environment = "prod"`,
		``,
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := EnsureCodexOTLP(configPath, false)
	if err != nil {
		t.Fatalf("EnsureCodexOTLP: %v", err)
	}
	if !result.Changed {
		t.Fatal("expected config merge to write updates")
	}
	if result.Conflict {
		t.Fatal("unexpected conflict when no endpoint is set")
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	assertContainsLine(t, got, `[model]`)
	assertContainsLine(t, got, `provider = "openai"`)
	assertContainsLine(t, got, `environment = "prod"`)
	assertContainsLine(t, got, `exporter = { "otlp-http" = { endpoint = "http://localhost:4318/v1/logs", protocol = "json" } }`)
	assertContainsLine(t, got, `trace_exporter = "none"`)
	assertContainsLine(t, got, `log_user_prompt = false`)
}

func TestEnsureCodexOTLPConflictNoForce(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".codex", "config.toml")
	existing := strings.Join([]string{
		`[otel]`,
		`exporter = { "otlp-http" = { endpoint = "https://otel.example.com:4318", protocol = "json" } }`,
		``,
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := EnsureCodexOTLP(configPath, false)
	if err != nil {
		t.Fatalf("EnsureCodexOTLP: %v", err)
	}
	if result.Changed {
		t.Fatal("expected no write when endpoint conflict exists without --force")
	}
	if !result.Conflict {
		t.Fatal("expected conflict result")
	}
	if result.ExistingEndpoint != "https://otel.example.com:4318" {
		t.Fatalf("existing endpoint = %q", result.ExistingEndpoint)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != existing {
		t.Fatal("expected conflict path to leave config unchanged")
	}
}

func TestEnsureCodexOTLPConflictWithForce(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".codex", "config.toml")
	existing := strings.Join([]string{
		`[otel]`,
		`exporter = { "otlp-http" = { endpoint = "https://otel.example.com:4318", protocol = "json" } }`,
		``,
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := EnsureCodexOTLP(configPath, true)
	if err != nil {
		t.Fatalf("EnsureCodexOTLP: %v", err)
	}
	if !result.Changed {
		t.Fatal("expected force path to rewrite endpoint")
	}
	if result.Conflict {
		t.Fatal("expected no conflict when force is true")
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	assertContainsLine(t, string(raw), `exporter = { "otlp-http" = { endpoint = "http://localhost:4318/v1/logs", protocol = "json" } }`)
}

func TestEnsureCodexOTLPMigratesLegacyExporterSection(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".codex", "config.toml")
	legacy := strings.Join([]string{
		`[otel]`,
		`exporter = "otlp-http"`,
		`trace_exporter = "none"`,
		`log_user_prompt = false`,
		``,
		`[otel.exporter."otlp-http"]`,
		`endpoint = "http://localhost:4318/v1/logs"`,
		``,
	}, "\n")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := EnsureCodexOTLP(configPath, false)
	if err != nil {
		t.Fatalf("EnsureCodexOTLP: %v", err)
	}
	if !result.Changed {
		t.Fatal("expected migration to rewrite legacy exporter format")
	}
	if result.Conflict {
		t.Fatal("unexpected conflict during legacy migration")
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	assertContainsLine(t, got, `exporter = { "otlp-http" = { endpoint = "http://localhost:4318/v1/logs", protocol = "json" } }`)
	assertNotContainsLine(t, got, `[otel.exporter."otlp-http"]`)
}

func assertContainsLine(t *testing.T, s, needle string) {
	t.Helper()
	if !strings.Contains(s, needle) {
		t.Fatalf("expected config to contain %q\n\n%s", needle, s)
	}
}

func assertNotContainsLine(t *testing.T, s, needle string) {
	t.Helper()
	if strings.Contains(s, needle) {
		t.Fatalf("expected config to NOT contain %q\n\n%s", needle, s)
	}
}
