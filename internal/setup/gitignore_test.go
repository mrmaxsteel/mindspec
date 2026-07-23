package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupVerbs enumerates the three onboarding entry points that must all
// converge on the same gitignore-ensure behavior (spec 123 R4b) — a shared
// table so a new verb can't be added without picking up the same coverage.
func setupVerbs() []struct {
	name string
	run  func(root string, check bool) (*Result, error)
} {
	return []struct {
		name string
		run  func(root string, check bool) (*Result, error)
	}{
		{"claude", RunClaude},
		{"codex", RunCodex},
		{"copilot", RunCopilot},
	}
}

// TestSetupVerbs_EnsureGitignoreRuntimeEntries pins AC-6: a repo that never
// ran `mindspec init` (no pre-existing .gitignore) gets both runtime
// entries gitignored by each of `setup claude`, `setup codex`, and `setup
// copilot`; a re-run is byte-idempotent. RED on pre-spec-123 main: none of
// the three setup verbs touched .gitignore at all.
func TestSetupVerbs_EnsureGitignoreRuntimeEntries(t *testing.T) {
	for _, v := range setupVerbs() {
		v := v
		t.Run(v.name, func(t *testing.T) {
			root := t.TempDir()

			if _, err := v.run(root, false); err != nil {
				t.Fatalf("%s: %v", v.name, err)
			}

			data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatalf("%s: reading .gitignore: %v", v.name, err)
			}
			content := string(data)
			for _, entry := range []string{".mindspec/session.json", ".mindspec/focus"} {
				if !strings.Contains(content, entry) {
					t.Errorf("%s: .gitignore missing %q; got:\n%s", v.name, entry, content)
				}
			}

			// Idempotent re-run: byte-identical.
			if _, err := v.run(root, false); err != nil {
				t.Fatalf("%s: second run: %v", v.name, err)
			}
			data2, err := os.ReadFile(filepath.Join(root, ".gitignore"))
			if err != nil {
				t.Fatalf("%s: re-reading .gitignore: %v", v.name, err)
			}
			if string(data2) != content {
				t.Errorf("%s: second run changed .gitignore; before:\n%s\nafter:\n%s", v.name, content, data2)
			}
		})
	}
}

// TestSetupVerbs_CheckModeWritesNoGitignore pins the `--check` half of
// AC-6: check mode reports without writing .gitignore at all.
func TestSetupVerbs_CheckModeWritesNoGitignore(t *testing.T) {
	for _, v := range setupVerbs() {
		v := v
		t.Run(v.name, func(t *testing.T) {
			root := t.TempDir()

			if _, err := v.run(root, true); err != nil {
				t.Fatalf("%s(check=true): %v", v.name, err)
			}

			if _, err := os.Stat(filepath.Join(root, ".gitignore")); err == nil {
				t.Errorf("%s: --check mode wrote .gitignore", v.name)
			} else if !os.IsNotExist(err) {
				t.Fatalf("%s: stat .gitignore: %v", v.name, err)
			}
		})
	}
}
