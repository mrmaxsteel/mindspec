package otel

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteClaudeSettingsLocal_FreshRepo(t *testing.T) {
	root := t.TempDir()
	c := Config{Endpoint: "http://collector.example:4318"}

	res, err := WriteClaudeSettingsLocal(root, c)
	if err != nil {
		t.Fatalf("WriteClaudeSettingsLocal: %v", err)
	}
	if !res.Written {
		t.Errorf("expected Written=true on fresh write, got %+v", res)
	}
	data, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	env := doc["env"].(map[string]any)
	if env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://collector.example:4318" {
		t.Errorf("endpoint mismatch in written file")
	}
}

func TestWriteClaudeSettingsLocal_Idempotent(t *testing.T) {
	root := t.TempDir()
	c := Config{Endpoint: "http://collector.example:4318"}

	res1, err := WriteClaudeSettingsLocal(root, c)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if !res1.Written {
		t.Fatalf("first write should have written")
	}
	bytes1, _ := os.ReadFile(res1.Path)
	h1 := sha256.Sum256(bytes1)

	res2, err := WriteClaudeSettingsLocal(root, c)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if !res2.NoOp {
		t.Errorf("second identical write should be NoOp, got Written=%v", res2.Written)
	}
	bytes2, _ := os.ReadFile(res2.Path)
	h2 := sha256.Sum256(bytes2)
	if h1 != h2 {
		t.Errorf("idempotent re-run produced different file content")
	}
}

func TestWriteClaudeSettingsLocal_PreservesSiblings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	pre := `{
  "env": {
    "MY_USER_VAR": "hello"
  },
  "permissions": {
    "allow": ["Bash(ls)"]
  }
}
`
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.local.json"), []byte(pre), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := WriteClaudeSettingsLocal(root, Config{Endpoint: "http://x:4318"})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !res.Written {
		t.Errorf("expected Written=true")
	}

	data, _ := os.ReadFile(res.Path)
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := doc["permissions"]; !ok {
		t.Errorf("permissions sibling lost")
	}
	env := doc["env"].(map[string]any)
	if env["MY_USER_VAR"] != "hello" {
		t.Errorf("MY_USER_VAR sibling lost: %v", env["MY_USER_VAR"])
	}
	if env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://x:4318" {
		t.Errorf("OTEL endpoint not written")
	}
}

func TestWriteCodexConfig_FreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.toml")
	c := Config{Endpoint: "http://collector:4318"}

	res, err := WriteCodexConfig(path, c)
	if err != nil {
		t.Fatalf("WriteCodexConfig: %v", err)
	}
	if !res.Written {
		t.Errorf("expected Written=true")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
	}
	parentInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat parent dir: %v", err)
	}
	if parentInfo.Mode().Perm() != 0o700 {
		t.Errorf("parent dir mode = %o, want 0700", parentInfo.Mode().Perm())
	}
}

func TestWriteCodexConfig_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	c := Config{Endpoint: "http://collector:4318", ServiceName: "myapp"}

	res1, err := WriteCodexConfig(path, c)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if !res1.Written {
		t.Fatalf("first write should write")
	}
	bytes1, _ := os.ReadFile(path)
	h1 := sha256.Sum256(bytes1)

	res2, err := WriteCodexConfig(path, c)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !res2.NoOp {
		t.Errorf("second identical run should be NoOp, got Written=%v", res2.Written)
	}
	bytes2, _ := os.ReadFile(path)
	h2 := sha256.Sum256(bytes2)
	if h1 != h2 {
		t.Errorf("sha256 changed across idempotent runs")
	}
}

func TestWriteCodexConfig_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	pre := `[profile]
name = "default"
`
	if err := os.WriteFile(path, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := WriteCodexConfig(path, Config{Endpoint: "http://x:4318"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !contains(string(data), `[profile]`) {
		t.Errorf("[profile] table lost:\n%s", data)
	}
	if !contains(string(data), `name = "default"`) {
		t.Errorf("profile.name lost:\n%s", data)
	}
	if !contains(string(data), `[otel]`) {
		t.Errorf("[otel] table not appended:\n%s", data)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || stringIndex(haystack, needle) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
