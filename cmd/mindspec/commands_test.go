package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestCommandsCmd_Registered pins the `mindspec commands populate`
// subcommand wiring (spec 123 R7c).
func TestCommandsCmd_Registered(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "commands" {
			found = true
			var sub bool
			for _, s := range c.Commands() {
				if s.Name() == "populate" {
					sub = true
				}
			}
			if !sub {
				t.Error("commands command missing populate subcommand")
			}
		}
	}
	if !found {
		t.Fatal("commands command not registered on rootCmd")
	}
}

// TestRunCommandsPopulate_PrintsPromptAndWritesNothing mirrors
// TestRunModelsPopulate_PrintsPromptAndWritesNothing for commands: —
// AC-11(iii)'s commands-scaffolder-parity counterpart: `mindspec
// commands populate` prints an agent prompt and writes NO file (ZFC —
// the framework proposes no build commands, not even its own).
func TestRunCommandsPopulate_PrintsPromptAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	withTestChdir(t, root)

	var buf bytes.Buffer
	if err := runCommandsPopulate(&buf); err != nil {
		t.Fatalf("runCommandsPopulate: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"commands:", ".mindspec/config.yaml", "mindspec doctor"} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt missing %q:\n%s", want, out)
		}
	}
	// The prompt may CAUTION against assuming mindspec's own commands
	// apply (it names them as a "never assume" example), but it must
	// never assert them as the repo's OWN declared values.
	if !strings.Contains(out, "never assume") {
		t.Errorf("prompt must caution against assuming mindspec's own build applies to the onboarded repo: %s", out)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("commands populate must write nothing, found: %v", entries)
	}
}
