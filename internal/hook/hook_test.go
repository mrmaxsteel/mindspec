package hook

import (
	"strings"
	"testing"
)

func TestParseInput_ClaudeProtocol(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`{"tool_input":{"file_path":"internal/foo.go","command":"go build"}}`)
	inp, proto, err := ParseInput(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proto != ProtocolClaude {
		t.Errorf("expected ProtocolClaude, got %d", proto)
	}
	if inp.FilePath != "internal/foo.go" {
		t.Errorf("expected file_path=internal/foo.go, got %q", inp.FilePath)
	}
	if inp.Command != "go build" {
		t.Errorf("expected command=go build, got %q", inp.Command)
	}
}

func TestParseInput_CopilotProtocol(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`{"toolName":"edit","toolArgs":{"file_path":"cmd/main.go"}}`)
	inp, proto, err := ParseInput(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proto != ProtocolCopilot {
		t.Errorf("expected ProtocolCopilot, got %d", proto)
	}
	if inp.FilePath != "cmd/main.go" {
		t.Errorf("expected file_path=cmd/main.go, got %q", inp.FilePath)
	}
}

func TestParseInput_CopilotPathFallback(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`{"toolName":"write","toolArgs":{"path":"internal/x.go"}}`)
	inp, proto, err := ParseInput(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proto != ProtocolCopilot {
		t.Errorf("expected ProtocolCopilot, got %d", proto)
	}
	if inp.FilePath != "internal/x.go" {
		t.Errorf("expected file_path=internal/x.go, got %q", inp.FilePath)
	}
}

func TestParseInput_EmptyStdin(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("")
	inp, proto, err := ParseInput(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proto != ProtocolClaude {
		t.Errorf("expected default ProtocolClaude for empty stdin, got %d", proto)
	}
	if inp.FilePath != "" || inp.Command != "" {
		t.Errorf("expected empty fields for empty stdin")
	}
}

func TestParseInput_InvalidJSON(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("not json")
	inp, _, err := ParseInput(r)
	if err != nil {
		t.Fatalf("should not error on invalid JSON: %v", err)
	}
	if inp.FilePath != "" {
		t.Errorf("expected empty fields for invalid JSON")
	}
}

func TestParseInput_EmptyJSON(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("{}")
	_, proto, err := ParseInput(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proto != ProtocolClaude {
		t.Errorf("expected default ProtocolClaude for empty object, got %d", proto)
	}
}

func TestEmit_Pass(t *testing.T) {
	t.Parallel()
	code := Emit(Result{Action: Pass}, ProtocolClaude)
	if code != 0 {
		t.Errorf("Pass should return exit code 0, got %d", code)
	}
	code = Emit(Result{Action: Pass}, ProtocolCopilot)
	if code != 0 {
		t.Errorf("Pass should return exit code 0, got %d", code)
	}
}

func TestEmitClaude_Block(t *testing.T) {
	t.Parallel()
	r := Result{Action: Block, Message: "blocked"}
	code := emitClaude(r)
	if code != 2 {
		t.Errorf("Claude block should return exit code 2, got %d", code)
	}
}

func TestEmitClaude_Warn(t *testing.T) {
	t.Parallel()
	r := Result{Action: Warn, Message: "warning text"}
	code := emitClaude(r)
	if code != 0 {
		t.Errorf("Claude warn should return exit code 0, got %d", code)
	}
}

func TestEmitCopilot_Block(t *testing.T) {
	t.Parallel()
	r := Result{Action: Block, Message: "blocked"}
	code := emitCopilot(r)
	if code != 0 {
		t.Errorf("Copilot block should return exit code 0 (JSON response), got %d", code)
	}
}

func TestEmitCopilot_Warn(t *testing.T) {
	t.Parallel()
	r := Result{Action: Warn, Message: "warning text"}
	code := emitCopilot(r)
	if code != 0 {
		t.Errorf("Copilot warn should return exit code 0 (JSON response), got %d", code)
	}
}

func TestNames_Sorted(t *testing.T) {
	t.Parallel()
	for i := 1; i < len(Names); i++ {
		if Names[i] < Names[i-1] {
			t.Errorf("Names not sorted: %q before %q", Names[i-1], Names[i])
		}
	}
}
