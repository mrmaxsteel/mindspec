package harness

import (
	"context"
	"testing"
	"time"
)

// mockAgent is a test double that satisfies the Agent interface.
type mockAgent struct {
	name   string
	output string
}

func (m *mockAgent) Name() string { return m.name }

func (m *mockAgent) Run(_ context.Context, _ *Sandbox, prompt string, _ RunOpts) (*SessionResult, error) {
	return &SessionResult{
		Output:   m.output + " (prompt: " + prompt + ")",
		Duration: 100 * time.Millisecond,
		ExitCode: 0,
	}, nil
}

func TestMockAgentSatisfiesInterface(t *testing.T) {
	var agent Agent = &mockAgent{name: "test-agent", output: "done"}

	if agent.Name() != "test-agent" {
		t.Errorf("Name() = %q, want test-agent", agent.Name())
	}

	result, err := agent.Run(context.Background(), nil, "do something", RunOpts{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Output == "" {
		t.Error("Output is empty")
	}
}

func TestResolveAgentClaudeCode(t *testing.T) {
	agent, err := ResolveAgent("claude-code")
	if err != nil {
		t.Fatalf("ResolveAgent: %v", err)
	}
	if agent.Name() != "claude-code" {
		t.Errorf("Name() = %q, want claude-code", agent.Name())
	}
}

func TestResolveAgentDefault(t *testing.T) {
	agent, err := ResolveAgent("")
	if err != nil {
		t.Fatalf("ResolveAgent empty: %v", err)
	}
	if agent.Name() != "claude-code" {
		t.Errorf("default agent = %q, want claude-code", agent.Name())
	}
}

func TestResolveAgentUnknown(t *testing.T) {
	_, err := ResolveAgent("unknown-agent")
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestDefaultAgentName(t *testing.T) {
	name := DefaultAgentName()
	// Should return "claude-code" unless BENCH_AGENT is set
	if name == "" {
		t.Error("DefaultAgentName returned empty string")
	}
}
