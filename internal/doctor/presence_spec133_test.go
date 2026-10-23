package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func s133PresenceFixture(t *testing.T, policy string) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	root := t.TempDir()
	writeConfig(t, root, "enforcement:\n  panel_presence: "+policy+"\nsource_globs: [cmd/**]\nmodels: {implementation: test}\ncommands: {test: test}\n")
	if err := os.MkdirAll(filepath.Join(root, ".beads", "embeddeddolt"), 0755); err != nil {
		t.Fatal(err)
	}
	// Every subprocess not covered by the phase seams sees only this local fake.
	fake := filepath.Join(root, "bd")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '[]\\n'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	return root
}

func TestS133DoctorPresence_PolicyInfo(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			root := s133PresenceFixture(t, policy)
			report := RunWithOptions(root, Options{SkipLocalEnv: true})
			count := 0
			for _, check := range report.Checks {
				if check.Name != "Panel presence policy" {
					continue
				}
				count++
				if check.Status != Info || check.FixFunc != nil || !strings.Contains(check.Message, "enforcement.panel_presence") || !strings.Contains(check.Message, "gates without a panel only warn") {
					t.Fatalf("wrong policy diagnostic: %+v", check)
				}
			}
			want := 0
			if policy == "warn" {
				want = 1
			}
			if count != want {
				t.Fatalf("policy info count = %d, want %d", count, want)
			}
		})
	}
}

func TestS133DoctorPresence_InFlightWarn(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			root := s133PresenceFixture(t, policy)
			var epics []phase.EpicInfo
			byID := map[string]string{}
			for _, fixture := range []struct {
				id, mode string
				absent   bool
				status   string
			}{
				{"001-plan", "plan", true, "open"}, {"002-implement", "implement", true, "in_progress"},
				{"003-clean", "plan", false, "open"}, {"004-review", "review", true, "open"},
				{"005-done", "done", true, "closed"}, {"006-spec", "spec", true, "open"},
			} {
				epic := phase.EpicInfo{ID: "epic-" + fixture.id, Status: fixture.status, IssueType: "epic", Metadata: map[string]interface{}{"spec_id": fixture.id, "mindspec_phase": fixture.mode, "panel_absent": fixture.absent}}
				epics = append(epics, epic)
				data, err := json.Marshal(epic)
				if err != nil {
					t.Fatal(err)
				}
				byID[epic.ID] = string(data)
			}
			data, err := json.Marshal(epics)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(stubDryRunFinders(t, string(data), nil, byID))
			report := RunWithOptions(root, Options{SkipLocalEnv: true})
			got := map[string]int{}
			for _, check := range report.Checks {
				if !strings.HasPrefix(check.Name, "Panel absent: ") {
					continue
				}
				got[strings.TrimPrefix(check.Name, "Panel absent: ")]++
				if check.Status != Warn || check.FixFunc != nil || !strings.Contains(check.Message, "panel_absent") {
					t.Fatalf("wrong absence diagnostic: %+v", check)
				}
			}
			if len(got) != 2 || got["001-plan"] != 1 || got["002-implement"] != 1 {
				t.Fatalf("wrong in-flight warnings: %v; checks: %+v", got, report.Checks)
			}
		})
	}
}
