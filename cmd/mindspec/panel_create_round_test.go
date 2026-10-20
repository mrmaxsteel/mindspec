package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func readCreatedPanel(t *testing.T, root string) (panel.Panel, map[string]json.RawMessage) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "review", "p", panel.FileName))
	if err != nil {
		t.Fatal(err)
	}
	var p panel.Panel
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return p, raw
}
func TestPanelCreate_ReviewersFloor(t *testing.T) {
	for _, n := range []string{"1", "0", "-1"} {
		t.Run(n, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			before := panelTreeBytes(t, root)
			out, err := runPanelVerbCmd("create", "p", "--spec", "130-test", "--gate", "bead", "--target", "bead/probe-x.1", "--reviewers", n)
			if err == nil {
				t.Fatalf("--reviewers %s accepted: %s", n, out)
			}
			if !strings.Contains(err.Error(), "at least two") {
				t.Fatalf("wrong refusal: %v", err)
			}
			if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal wrote files: %+v", after)
			}
		})
	}
}
func TestPanelCreate_ReviewersN(t *testing.T) {
	for _, tc := range []struct {
		name, config, n string
		models          []string
	}{
		{"defaults", "", "3", []string{"claude", "codex", "claude"}},
		{"explicit global", "panel:\n  reviewers:\n    - family: claude\n      count: 3\n", "4", []string{"claude", "claude", "claude", "claude"}},
		{"gate entry order", "panel:\n  gates:\n    bead:\n      reviewers:\n        - family: codex\n          count: 3\n          lens: security\n        - family: claude\n          count: 1\n", "3", []string{"codex", "claude", "codex"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			if tc.config != "" {
				if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(tc.config), 0644); err != nil {
					t.Fatal(err)
				}
				config.ResetCache()
			}
			out, err := runPanelVerbCmd("create", "p", "--spec", "130-test", "--gate", "bead", "--target", "bead/probe-x.1", "--reviewers", tc.n)
			if err != nil {
				t.Fatalf("create: %v %s", err, out)
			}
			p, _ := readCreatedPanel(t, root)
			if p.ExpectedReviewers != len(tc.models) || len(p.Roster) != len(tc.models) {
				t.Fatalf("wrong size: %+v", p)
			}
			for i, s := range p.Roster {
				if s.Model != tc.models[i] || s.Lens == "" {
					t.Fatalf("wrong slot %d: %+v", i, s)
				}
			}
			if tc.name == "gate entry order" && (p.Roster[0].Lens != "security" || p.Roster[2].Lens != "security") {
				t.Fatalf("lost explicit lens: %+v", p.Roster)
			}
			lines, err := panel.ReadLedger(filepath.Join(root, "review", "p"))
			if err != nil {
				t.Fatal(err)
			}
			if len(lines) != 1 {
				t.Fatalf("missing round event: %s", lines)
			}
			var event panel.RoundRecord
			if err := json.Unmarshal(lines[0], &event); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(event.Roster, p.Roster) || event.ExpectedReviewers != p.ExpectedReviewers || event.Telemetry != "required" || p.Telemetry != "required" {
				t.Fatalf("snapshot and ledger disagree: %+v %+v", p, event)
			}
		})
	}
}
func TestPanelCreate_NoApproveThreshold(t *testing.T) {
	root := setupCreateIntegrity(t)
	cfg := "panel:\n  approve_threshold: '3'\n  gates:\n    bead:\n      approve_threshold: '2'\n"
	if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	config.ResetCache()
	for _, round := range []string{"1", "2"} {
		out, err := runPanelVerbCmd("create", "p", "--spec", "130-test", "--gate", "bead", "--target", "bead/probe-x.1", "--round", round)
		if err != nil {
			t.Fatalf("create: %v %s", err, out)
		}
		p, raw := readCreatedPanel(t, root)
		if _, exists := raw["approve_threshold"]; exists || p.ApproveThresholdExpr != "" {
			t.Fatalf("approve_threshold written: %s", raw["approve_threshold"])
		}
	}
}
func TestPanelCreate_ReviewersOverridesCarriedRoster(t *testing.T) {
	root := setupCreateIntegrity(t)
	if out, err := runPanelVerbCmd("create", "p", "--spec", "130-test", "--gate", "bead", "--target", "bead/probe-x.1"); err != nil {
		t.Fatalf("create: %v %s", err, out)
	}
	if out, err := runPanelVerbCmd("create", "p", "--spec", "130-test", "--gate", "bead", "--target", "bead/probe-x.1", "--round", "2", "--reviewers", "3"); err != nil {
		t.Fatalf("bump: %v %s", err, out)
	}
	p, _ := readCreatedPanel(t, root)
	if len(p.Roster) != 3 || p.ExpectedReviewers != 3 {
		t.Fatalf("override ignored: %+v", p)
	}
}

func TestPanelCreate_LegacyPathRecovery(t *testing.T) {
	root := setupCreateIntegrity(t)
	dir := filepath.Join(root, "review", "p")
	p := panel.Panel{Spec: "130-test", Gate: "bead", Target: "bead/probe-x.1", Round: 1,
		Refutations: []panel.Refutation{{Slot: "F1", Round: 1, Finding: "ccr[0]", Evidence: "/tmp/probe/result"}}}
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	before := panelTreeBytes(t, root)
	out, err := runPanelVerbCmd("create", "p", "--spec", "130-test", "--gate", "bead", "--target", "bead/probe-x.1", "--round", "2")
	if err == nil {
		t.Fatalf("legacy path accepted: %s", out)
	}
	want := "recovery: remove the absolute path from refutation[0] slot F1 finding ccr[0] in the legacy panel.json, then run create again"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("missing specific recovery: %v", err)
	}
	if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("refusal wrote files: %+v", after)
	}
}
