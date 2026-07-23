package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestModelsCmd_Registered pins the `mindspec models populate` subcommand
// wiring (spec 123 R6b).
func TestModelsCmd_Registered(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "models" {
			found = true
			var sub bool
			for _, s := range c.Commands() {
				if s.Name() == "populate" {
					sub = true
				}
			}
			if !sub {
				t.Error("models command missing populate subcommand")
			}
		}
	}
	if !found {
		t.Fatal("models command not registered on rootCmd")
	}
}

// TestRunModelsPopulate_PrintsPromptAndWritesNothing pins AC-11(iii):
// `mindspec models populate` prints an agent prompt naming models: and
// writes NO file to disk (ZFC — the framework proposes no model ids).
func TestRunModelsPopulate_PrintsPromptAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	withTestChdir(t, root)

	var buf bytes.Buffer
	if err := runModelsPopulate(&buf); err != nil {
		t.Fatalf("runModelsPopulate: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"models:", ".mindspec/config.yaml", "mindspec doctor"} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "make build") || strings.Contains(out, "make test") {
		t.Errorf("prompt must not pre-fill any model/command values: %s", out)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("models populate must write nothing, found: %v", entries)
	}
}
