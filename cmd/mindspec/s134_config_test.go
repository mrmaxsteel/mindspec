package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/journal"
	"github.com/mrmaxsteel/mindspec/internal/redact"
	"github.com/spf13/cobra"
)

func TestS134ConfigShow_TruthfulLabels(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mindspec/config.yaml"), []byte(`loop:
  gate_authority: {bead_merge: panel}
  halt:
    on_reject: fix
    panel_deadlock_rounds: 2
    max_consecutive_impl_failures: 2
    max_rounds_per_bead: 4
`), 0644); err != nil {
		t.Fatal(err)
	}
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"gate_authority", "halt.on_reject", "halt.panel_deadlock_rounds", "halt.max_consecutive_impl_failures"} {
		found := false
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "  "+key+":") {
				found = true
				if !strings.HasSuffix(line, "# ignored") {
					t.Fatalf("false label: %s", line)
				}
			}
		}
		if !found {
			t.Fatalf("missing ignored key %s: %s", key, out)
		}
	}
	for _, want := range []string{"runner: claude-code-skills  # read by a skill", "max_rounds: 4", "lease_ttl: 2h", "# deprecated alias for panel.max_rounds"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "declared, not yet enforced") {
		t.Fatal("stale label")
	}
	// Exercise the command handler as well as its pure renderer.
	resetConfigShowGateFlags(t)
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Error(err)
		}
	})
	var stdout bytes.Buffer
	command := &cobra.Command{}
	command.Flags().String("gate", "", "")
	command.Flags().Bool("json", false, "")
	command.SetOut(&stdout)
	if err := configShowCmd.RunE(command, nil); err != nil {
		t.Fatalf("config show did not exit successfully: %v", err)
	}
	if stdout.String() != out {
		t.Fatalf("command output differs: %s", stdout.String())
	}

}

func TestS134Friction_AllowNotReadySurvivesJournal(t *testing.T) {
	if escapeHatchFlags["allow-not-ready"] != "allow-not-ready" {
		t.Fatal("missing flag registry token")
	}
	if _, ok := redact.EscapeHatchTokens["allow-not-ready"]; !ok {
		t.Fatal("missing redaction registry token")
	}
	cmd := &cobra.Command{Use: "next"}
	cmd.Flags().String("allow-not-ready", "", "reason")
	if err := cmd.Flags().Set("allow-not-ready", "fixture reason"); err != nil {
		t.Fatal(err)
	}
	token, ok := detectFriction(cmd)
	if !ok || token != "allow-not-ready" {
		t.Fatalf("undetected friction: %q %t", token, ok)
	}
	t.Setenv(journal.StateDirEnv, t.TempDir())
	if err := journal.AppendSuccessEvent(journal.Event{Argv0: "mindspec", Command: "next", EscapeHatch: token, Version: "dev", OS: "linux"}); err != nil {
		t.Fatal(err)
	}
	events, err := journal.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EscapeHatch != token || events[0].Identity.EscapeHatch != token {
		t.Fatalf("journal dropped token: %+v", events)
	}
}
