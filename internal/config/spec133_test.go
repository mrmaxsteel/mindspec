package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestS133Config_Presence(t *testing.T) {
	for _, value := range []string{"", "warn", "enforce", "maybe", "''", "null"} {
		t.Run(value, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
				t.Fatal(err)
			}
			text := ""
			if value != "" {
				text = "enforcement:\n  panel_presence: " + value + "\n"
			}
			if err := os.WriteFile(filepath.Join(root, ".mindspec/config.yaml"), []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(root)
			if value == "maybe" || value == "''" || value == "null" {
				if err == nil || !strings.Contains(err.Error(), "enforcement.panel_presence") {
					t.Fatalf("expected named validation error, got %v", err)
				}
				return
			}
			want := value
			if want == "" {
				want = "warn"
			}
			if err != nil || cfg.Enforcement.PanelPresence != want {
				t.Fatalf("cfg=%+v err=%v want %s", cfg, err, want)
			}
		})
	}
}

func TestS133Config_RepositoryEnforces(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enforcement.PanelPresence != "enforce" {
		t.Fatalf("repository policy=%q, want enforce", cfg.Enforcement.PanelPresence)
	}
}
