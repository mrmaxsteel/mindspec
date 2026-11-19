package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// s134Load captures the loader's warnings, including a cached second read.
func s134Load(t *testing.T, content string) (*Config, string) {
	t.Helper()
	ResetCache()
	t.Cleanup(ResetCache)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	capture, err := os.CreateTemp(t.TempDir(), "warnings")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = capture
	defer func() { os.Stderr = original; capture.Close() }()
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Load(root)
	if err != nil || again != cfg {
		t.Fatalf("cached read: %p %p %v", cfg, again, err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	viaAlias, err := Load(alias)
	if err != nil || viaAlias != cfg {
		t.Fatalf("root alias bypassed cache: %p %p %v", cfg, viaAlias, err)
	}
	if err := capture.Sync(); err != nil {
		t.Fatal(err)
	}
	warnings, err := os.ReadFile(capture.Name())
	if err != nil {
		t.Fatal(err)
	}
	return cfg, string(warnings)
}

func TestS134Config_RemovedKeysWarnOnceAndIgnore(t *testing.T) {
	cfg, warnings := s134Load(t, `loop:
  gate_authority: {panel_skip: robot, bead_merge: robot}
  halt:
    on_reject: fix
    panel_deadlock_rounds: 17
    max_consecutive_impl_failures: 19
`)
	keys := []string{"gate_authority", "halt.max_consecutive_impl_failures", "halt.on_reject", "halt.panel_deadlock_rounds"}
	if !reflect.DeepEqual(cfg.RemovedLoopKeys(), keys) {
		t.Fatalf("removed keys: %v", cfg.RemovedLoopKeys())
	}
	for _, key := range keys {
		want := "warning: loop." + key + " is removed and ignored"
		if strings.Count(warnings, want) != 1 {
			t.Fatalf("need exactly one %q: %q", want, warnings)
		}
	}
	if strings.Count(warnings, "warning:") != len(keys) {
		t.Fatalf("extra warnings: %q", warnings)
	}
	if cfg.RemovedLoopValue("halt.on_reject") != "fix" {
		t.Fatal("loader forced a reject policy")
	}
	if cfg.RoundLimit() != 5 || cfg.Panel.LeaseTTL != "2h" {
		t.Fatalf("removed settings changed policy: %+v", cfg.Panel)
	}
}

func TestS134Config_AllRemovedKeysNonStrict(t *testing.T) {
	cfg, warnings := s134Load(t, `loop:
  enabled: strange
  gate_authority: {anything: [null, 9]}
  halt: {on_reject: [fix], panel_deadlock_rounds: null, max_consecutive_impl_failures: {x: y}}
  budget: {max_beads_per_wake: true, token_budget: unbounded}
  context: {controller_handoff: never}
  handoff_log: [no, path]
`)
	if len(cfg.RemovedLoopKeys()) != 9 || strings.Count(warnings, "warning:") != 9 {
		t.Fatalf("keys %v warnings %q", cfg.RemovedLoopKeys(), warnings)
	}
}

func TestS134Config_RoundLimitAlias(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		want          int
	}{
		{"default", "", 5},
		{"alias", "loop:\n  halt:\n    max_rounds_per_bead: 4\n", 4},
		{"precedence", "panel:\n  max_rounds: 6\nloop:\n  halt:\n    max_rounds_per_bead: 4\n", 6},
		{"explicit-default", "panel:\n  max_rounds: 5\nloop:\n  halt:\n    max_rounds_per_bead: 4\n", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, warnings := s134Load(t, tc.content)
			if cfg.RoundLimit() != tc.want {
				t.Fatalf("RoundLimit = %d, want %d", cfg.RoundLimit(), tc.want)
			}
			if tc.content != "" && (strings.Count(warnings, "warning:") != 1 || !strings.Contains(warnings, "value 4")) {
				t.Fatalf("alias warning: %q", warnings)
			}
		})
	}
	if (&Config{}).RoundLimit() != 5 {
		t.Fatal("zero config default")
	}
}

func TestS134Config_PanelPolicy(t *testing.T) {
	cfg, _ := s134Load(t, "panel:\n  max_rounds: 8\n  lease_ttl: 10m\n")
	if cfg.RoundLimit() != 8 || cfg.Panel.LeaseTTL != "10m" {
		t.Fatalf("policy: %+v", cfg.Panel)
	}
	for _, content := range []string{"panel: {max_rounds: 0}", "panel: {max_rounds: -1}", "panel: {lease_ttl: forever}", "panel: {lease_ttl: 0h}"} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".mindspec/config.yaml"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root); err == nil {
			t.Fatalf("accepted invalid policy %q", content)
		}
	}
}
