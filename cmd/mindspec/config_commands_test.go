package main

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

// config_commands_test.go — spec 123 R7b/AC-11(iv) parity: `mindspec
// config show` renders commands: beside models:, UNANNOTATED inert
// (unlike models/loop/runner) because a populated commands: key changes
// generated AGENTS.md content today.

func TestConfigShow_CommandsEmpty(t *testing.T) {
	out, err := renderConfig(config.DefaultConfig())
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}
	if !strings.Contains(out, "commands: {}") {
		t.Errorf("expected an empty commands block, got:\n%s", out)
	}
	// commands: must NOT carry the inert annotation — unlike its
	// models:/loop:/runner: siblings, it drives generated content today.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "commands:") && strings.Contains(line, "declared, not yet enforced") {
			t.Errorf("commands: must not carry the inert annotation, got line: %q", line)
		}
	}
}

func TestConfigShow_CommandsPopulated(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Commands = map[string]string{"build": "make build", "test": "make test"}
	out, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}
	if !strings.Contains(out, "commands:") {
		t.Errorf("expected a commands: block, got:\n%s", out)
	}
	if !strings.Contains(out, "build: make build") || !strings.Contains(out, "test: make test") {
		t.Errorf("expected both declared commands rendered, got:\n%s", out)
	}
}

// TestConfigShow_CommandsEscapesHostileValue mirrors the existing
// models/panel hostile-string coverage (spec 116 AC6): a commands: value
// carrying control bytes must render through termsafe, never raw.
func TestConfigShow_CommandsEscapesHostileValue(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Commands = map[string]string{"build": "echo hi\x1b[2Jinjected\ninjected_key: forged"}
	out, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig: %v", err)
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("rendered config must not contain a raw ESC byte:\n%q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "injected_key:") {
			t.Errorf("hostile commands value forged an extra config line:\n%s", out)
		}
	}
}
