package otel

import (
	"crypto/sha256"
	"reflect"
	"strings"
	"testing"
)

func TestParseHeaders(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    map[string]string
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"single", "k=v", map[string]string{"k": "v"}, false},
		{"multi", "k1=v1,k2=v2", map[string]string{"k1": "v1", "k2": "v2"}, false},
		{"spaces", " k1 = v1 , k2 = v2 ", map[string]string{"k1": "v1", "k2": "v2"}, false},
		{"value with =", "k=v=v", map[string]string{"k": "v=v"}, false},
		{"missing =", "k", nil, true},
		{"empty key", "=v", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseHeaders(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseHeaders(%q) error=%v wantErr=%v", tc.in, err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseHeaders(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		c       Config
		wantErr bool
	}{
		{"valid http", Config{Endpoint: "http://localhost:4318"}, false},
		{"valid https", Config{Endpoint: "https://api.example.com"}, false},
		{"missing endpoint", Config{}, true},
		{"bad scheme", Config{Endpoint: "ftp://example.com"}, true},
		{"no host", Config{Endpoint: "http://"}, true},
		{"valid protocol grpc", Config{Endpoint: "http://x", Protocol: "grpc"}, false},
		{"valid protocol http/protobuf", Config{Endpoint: "http://x", Protocol: "http/protobuf"}, false},
		{"bad protocol", Config{Endpoint: "http://x", Protocol: "weird"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestIsSecretHeader(t *testing.T) {
	cases := map[string]bool{
		"authorization": true,
		"Authorization": true,
		"x-api-key":     true,
		"x-api-token":   true,
		"my-secret":     true,
		"password":      true,
		"x-bearer-jwt":  true,
		"content-type":  false,
		"x-trace-id":    false,
	}
	for name, want := range cases {
		if got := IsSecretHeader(name); got != want {
			t.Errorf("IsSecretHeader(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestRedacted(t *testing.T) {
	c := Config{
		Headers: map[string]string{
			"x-api-key":    "sk-prod-12345",
			"x-trace-id":   "abc123",
			"authorization": "Bearer xyz",
		},
	}
	r := c.Redacted()
	if r.Headers["x-api-key"] != "***" {
		t.Errorf("x-api-key not redacted: %q", r.Headers["x-api-key"])
	}
	if r.Headers["authorization"] != "***" {
		t.Errorf("authorization not redacted: %q", r.Headers["authorization"])
	}
	if r.Headers["x-trace-id"] != "abc123" {
		t.Errorf("x-trace-id was unexpectedly redacted: %q", r.Headers["x-trace-id"])
	}
	// Original should be untouched.
	if c.Headers["x-api-key"] != "sk-prod-12345" {
		t.Errorf("Redacted mutated the receiver: %q", c.Headers["x-api-key"])
	}
}

func TestFormatHeaders_Deterministic(t *testing.T) {
	c := Config{Headers: map[string]string{"z": "1", "a": "2", "m": "3"}}
	got1 := c.FormatHeaders()
	got2 := c.FormatHeaders()
	if got1 != got2 {
		t.Errorf("FormatHeaders not deterministic: %q vs %q", got1, got2)
	}
	if got1 != "a=2,m=3,z=1" {
		t.Errorf("FormatHeaders = %q, want sorted form", got1)
	}
}

func TestRenderClaudeSettingsLocal(t *testing.T) {
	c := Config{
		Endpoint:    "http://collector.example:4318",
		ServiceName: "myapp",
		Protocol:    "http/protobuf",
		Headers:     map[string]string{"x-tenant": "acme"},
	}
	got, err := RenderClaudeSettingsLocal(c)
	if err != nil {
		t.Fatalf("RenderClaudeSettingsLocal: %v", err)
	}
	env := got["env"].(map[string]any)
	if env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://collector.example:4318" {
		t.Errorf("endpoint not set: %v", env["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	if env["OTEL_SERVICE_NAME"] != "myapp" {
		t.Errorf("service_name not set: %v", env["OTEL_SERVICE_NAME"])
	}
	if env["OTEL_EXPORTER_OTLP_PROTOCOL"] != "http/protobuf" {
		t.Errorf("protocol not set: %v", env["OTEL_EXPORTER_OTLP_PROTOCOL"])
	}
	if env["OTEL_EXPORTER_OTLP_HEADERS"] != "x-tenant=acme" {
		t.Errorf("headers not set: %v", env["OTEL_EXPORTER_OTLP_HEADERS"])
	}
	if env["CLAUDE_CODE_ENABLE_TELEMETRY"] != "1" {
		t.Errorf("CLAUDE_CODE_ENABLE_TELEMETRY not set: %v", env["CLAUDE_CODE_ENABLE_TELEMETRY"])
	}
}

func TestRenderClaudeSettingsLocal_Defaults(t *testing.T) {
	got, err := RenderClaudeSettingsLocal(Config{Endpoint: "http://x:4318"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	env := got["env"].(map[string]any)
	if env["OTEL_EXPORTER_OTLP_PROTOCOL"] != DefaultProtocol {
		t.Errorf("default protocol not applied: %v", env["OTEL_EXPORTER_OTLP_PROTOCOL"])
	}
	if env["OTEL_SERVICE_NAME"] != DefaultServiceName {
		t.Errorf("default service_name not applied: %v", env["OTEL_SERVICE_NAME"])
	}
}

func TestRenderClaudeSettingsLocal_Idempotent(t *testing.T) {
	c := Config{
		Endpoint:    "http://collector.example:4318",
		ServiceName: "myapp",
		Headers:     map[string]string{"x-tenant": "acme", "x-trace-id": "abc"},
	}
	doc1, _ := RenderClaudeSettingsLocal(c)
	doc2, _ := RenderClaudeSettingsLocal(c)
	b1, _ := MarshalClaudeSettings(doc1)
	b2, _ := MarshalClaudeSettings(doc2)
	h1 := sha256.Sum256(b1)
	h2 := sha256.Sum256(b2)
	if h1 != h2 {
		t.Errorf("Claude settings render not sha256-idempotent:\n%s\nvs\n%s", b1, b2)
	}
}

func TestMergeClaudeSettingsLocal_PreservesSiblings(t *testing.T) {
	existing := map[string]any{
		"permissions": map[string]any{"allow": []any{"Bash(ls)"}},
		"env": map[string]any{
			"CUSTOM_VAR":                  "should-be-preserved",
			"OTEL_EXPORTER_OTLP_ENDPOINT": "http://old-endpoint:1234",
		},
	}
	c := Config{Endpoint: "http://new-endpoint:4318"}
	merged, err := MergeClaudeSettingsLocal(existing, c)
	if err != nil {
		t.Fatalf("MergeClaudeSettingsLocal: %v", err)
	}
	if _, ok := merged["permissions"]; !ok {
		t.Errorf("sibling 'permissions' key was dropped")
	}
	env := merged["env"].(map[string]any)
	if env["CUSTOM_VAR"] != "should-be-preserved" {
		t.Errorf("sibling env CUSTOM_VAR was dropped: %v", env["CUSTOM_VAR"])
	}
	if env["OTEL_EXPORTER_OTLP_ENDPOINT"] != "http://new-endpoint:4318" {
		t.Errorf("OTEL endpoint not overwritten: %v", env["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
}

func TestRenderCodexConfigToml_Fresh(t *testing.T) {
	c := Config{Endpoint: "http://collector.example:4318", ServiceName: "myapp"}
	got, err := RenderCodexConfigToml(c, "")
	if err != nil {
		t.Fatalf("RenderCodexConfigToml: %v", err)
	}
	if !strings.Contains(got, `[otel]`) {
		t.Errorf("missing [otel] table: %s", got)
	}
	if !strings.Contains(got, `endpoint = "http://collector.example:4318"`) {
		t.Errorf("endpoint missing: %s", got)
	}
	if !strings.Contains(got, `service_name = "myapp"`) {
		t.Errorf("service_name missing: %s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("missing trailing newline")
	}
}

func TestRenderCodexConfigToml_Idempotent(t *testing.T) {
	c := Config{
		Endpoint:    "http://collector.example:4318",
		ServiceName: "myapp",
		Headers:     map[string]string{"x-tenant": "acme", "authorization": "Bearer xyz"},
	}
	out1, err := RenderCodexConfigToml(c, "")
	if err != nil {
		t.Fatalf("first render: %v", err)
	}
	out2, err := RenderCodexConfigToml(c, out1)
	if err != nil {
		t.Fatalf("second render: %v", err)
	}
	h1 := sha256.Sum256([]byte(out1))
	h2 := sha256.Sum256([]byte(out2))
	if h1 != h2 {
		t.Errorf("Codex TOML render not sha256-idempotent:\n--- first ---\n%s\n--- second ---\n%s",
			out1, out2)
	}
}

func TestRenderCodexConfigToml_PreservesSiblings(t *testing.T) {
	existing := `[profile]
name = "default"
trust_level = "all"

[server]
port = 1234
`
	c := Config{Endpoint: "http://collector:4318"}
	got, err := RenderCodexConfigToml(c, existing)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(got, `[profile]`) {
		t.Errorf("sibling [profile] table dropped:\n%s", got)
	}
	if !strings.Contains(got, `name = "default"`) {
		t.Errorf("sibling profile.name dropped:\n%s", got)
	}
	if !strings.Contains(got, `[server]`) {
		t.Errorf("sibling [server] table dropped:\n%s", got)
	}
	if !strings.Contains(got, `port = 1234`) {
		t.Errorf("sibling server.port dropped:\n%s", got)
	}
	if !strings.Contains(got, `endpoint = "http://collector:4318"`) {
		t.Errorf("OTEL endpoint not added:\n%s", got)
	}
}

func TestRenderCodexConfigToml_ReplacesExistingOtel(t *testing.T) {
	existing := `[profile]
name = "x"

[otel]
exporter = { "otlp-http" = { endpoint = "http://OLD:4318", protocol = "http/json" } }
trace_exporter = "none"
log_user_prompt = false
service_name = "old"
`
	c := Config{Endpoint: "http://NEW:4318", ServiceName: "new"}
	got, err := RenderCodexConfigToml(c, existing)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(got, "OLD") {
		t.Errorf("old endpoint not replaced:\n%s", got)
	}
	if !strings.Contains(got, `endpoint = "http://NEW:4318"`) {
		t.Errorf("new endpoint not present:\n%s", got)
	}
	if !strings.Contains(got, `service_name = "new"`) {
		t.Errorf("new service_name not present:\n%s", got)
	}
	if !strings.Contains(got, `[profile]`) {
		t.Errorf("sibling table dropped:\n%s", got)
	}
}

func TestRenderEnvExports(t *testing.T) {
	c := Config{
		Endpoint:    "http://collector:4318",
		ServiceName: "myapp",
		Headers:     map[string]string{"x-tenant": "acme"},
	}
	out := RenderEnvExports(c)
	if !strings.Contains(out, `export OTEL_EXPORTER_OTLP_ENDPOINT='http://collector:4318'`) {
		t.Errorf("endpoint export missing:\n%s", out)
	}
	if !strings.Contains(out, `export OTEL_SERVICE_NAME='myapp'`) {
		t.Errorf("service_name export missing:\n%s", out)
	}
	if !strings.Contains(out, `export OTEL_EXPORTER_OTLP_HEADERS='x-tenant=acme'`) {
		t.Errorf("headers export missing:\n%s", out)
	}
}

func TestRenderEnvExports_QuotesEscaped(t *testing.T) {
	c := Config{Endpoint: "http://x:1", Headers: map[string]string{"weird": "val'with'quotes"}}
	out := RenderEnvExports(c)
	// We expect each embedded ' → '\'' inside the shell-single-quoted
	// value. The whole headers string is "weird=val'with'quotes" and
	// gets wrapped, yielding 'weird=val'\''with'\''quotes'.
	if !strings.Contains(out, `'weird=val'\''with'\''quotes'`) {
		t.Errorf("shell quoting did not escape embedded single quotes:\n%s", out)
	}
}

func TestRenderEnvExports_Deterministic(t *testing.T) {
	c := Config{Endpoint: "http://x:1", Headers: map[string]string{"b": "2", "a": "1"}}
	a := RenderEnvExports(c)
	b := RenderEnvExports(c)
	if a != b {
		t.Errorf("RenderEnvExports not deterministic")
	}
	if sha256.Sum256([]byte(a)) != sha256.Sum256([]byte(b)) {
		t.Errorf("RenderEnvExports sha256 differs across runs")
	}
}
