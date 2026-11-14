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
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestPanelCreateReviewerOverrides(t *testing.T) {
	for _, choice := range []string{"R1=actual-model", "R1=harness:actual-model", "R1=worker:actual-model"} {
		t.Run(choice, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("panel:\n  runners: [{family: worker, kind: harness}]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			config.ResetCache()
			cfg, err := config.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			defaults, err := cfg.PanelGateReviewerSlots("adhoc", 2)
			if err != nil {
				t.Fatal(err)
			}
			stubNonBeadRevParse(t, func(string, string) (string, error) { return paneltest.ReviewedSHA, nil })
			args := []string{"create", "actual", "--gate", "adhoc", "--target", "HEAD", "--reviewers", "2", "--reviewer", choice, "--reviewer", "R2=second-model"}
			if out, err := runPanelVerbCmd(args...); err != nil {
				t.Fatalf("create: %v %s", err, out)
			}
			dir := filepath.Join(root, "review", "actual")
			res, err := panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			wantFamily := defaults[0].Family
			if strings.Contains(choice, "worker:") {
				wantFamily = "worker"
			}
			if res.Panel.Roster[0].Family != wantFamily {
				t.Fatalf("override family=%s, want %s", res.Panel.Roster[0].Family, wantFamily)
			}
			if res.Panel.Roster[0].Model != "actual-model" || res.Panel.Roster[1].Model != "second-model" {
				t.Fatalf("override roster: %+v", res.Panel.Roster)
			}
			for i, slot := range res.Panel.Roster {
				if slot.Lens != defaults[i].Lens {
					t.Fatalf("override changed slot %s lens: got %q, want %q", slot.Slot, slot.Lens, defaults[i].Lens)
				}
			}
			if field, err := panel.CheckHandEdits(dir, *res.Panel); err != nil || field != "" {
				t.Fatalf("hand edit: %s %v", field, err)
			}
			resolve := func(p panel.Panel, s panel.RosterSlot) (panel.SubmitResolution, error) {
				return panel.SubmitResolution{Family: s.Family, Model: s.Model, RunnerKind: "harness", RunnerSource: "default"}, nil
			}
			for _, s := range res.Panel.Roster {
				v := paneltest.Verdict(panel.VerdictApprove)
				v.Slot = s.Slot
				v.Model = s.Model
				v.ReviewedSHA = res.Panel.ReviewedHeadSHA
				target := panel.SubmitTarget{Dir: dir, Slot: s.Slot, Round: 1}
				if s.Slot == "R1" {
					wrong := v
					wrong.Model = defaults[0].Model
					if _, err := panel.Submit(target, paneltest.Bytes(t, wrong), resolve); err == nil || !strings.Contains(err.Error(), "/model") {
						t.Fatalf("old model accepted: %v", err)
					}
				}
				if _, err := panel.Submit(target, paneltest.Bytes(t, v), resolve); err != nil {
					t.Fatal(err)
				}
			}
			res, err = panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			manifest := panel.DeriveCoverageManifest(res)
			if len(manifest.Slots) != 2 || manifest.Slots[0].Model != "actual-model" || manifest.Slots[1].Model != "second-model" {
				t.Fatalf("manifest: %+v", manifest)
			}
			if out, err := runPanelVerbCmd("tally", "actual"); err != nil {
				t.Fatalf("tally: %v %s", err, out)
			}
			lines, err := panel.ReadLedger(dir)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range lines {
				var saved panel.CoverageManifest
				if err := json.Unmarshal(line, &saved); err != nil {
					t.Fatal(err)
				}
				if saved.Record == panel.RecordPanelManifest {
					found = true
					if len(saved.Slots) != 2 || saved.Slots[0].Model != "actual-model" || saved.Slots[1].Model != "second-model" {
						t.Fatalf("stored manifest: %+v", saved)
					}
				}
			}
			if !found {
				t.Fatal("tally wrote no manifest")
			}
			roster := res.Panel.Roster
			resetPanelCreateFlags(t)
			if out, err := runPanelVerbCmd("create", "actual", "--gate", "adhoc", "--target", "HEAD", "--round", "2"); err != nil {
				t.Fatalf("round bump: %v %s", err, out)
			}
			res, err = panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Panel.Roster, roster) || res.Panel.ReviewerCountSource != "explicit" {
				t.Fatalf("round lost explicit roster: %+v", res.Panel)
			}
			for i, slot := range res.Panel.Roster {
				if slot.Lens != defaults[i].Lens {
					t.Fatalf("round bump changed slot %s lens: got %q, want %q", slot.Slot, slot.Lens, defaults[i].Lens)
				}
			}
		})
	}
}

func TestPanelCreateReviewerOverrideRefusals(t *testing.T) {
	for _, choice := range []string{"R99=model", "R1=unknown:model", "R1=", "R1=-option", "R1=two words", "R1=harness:", "R1", "R1=model\n"} {
		t.Run(choice, func(t *testing.T) {
			root := setupCreateIntegrity(t)
			before := panelTreeBytes(t, root)
			out, err := runPanelVerbCmd("create", "bad", "--gate", "adhoc", "--target", "HEAD", "--reviewer", choice)
			if err == nil {
				t.Fatalf("invalid override accepted: %q %s", choice, out)
			}
			if strings.Contains(err.Error(), "unknown flag") {
				t.Fatalf("override flag missing: %v", err)
			}
			if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("invalid override wrote files")
			}
		})
	}
}

func TestPanelCreateSlotOverrideKeepsDefaultCountSource(t *testing.T) {
	root := setupCreateIntegrity(t)
	if out, err := runPanelVerbCmd("create", "actual", "--gate", "adhoc", "--target", "HEAD", "--reviewer", "R1=actual-model"); err != nil {
		t.Fatalf("create: %v %s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(root, "review", "actual", panel.FileName))
	if err != nil {
		t.Fatal(err)
	}
	var p panel.Panel
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if p.ReviewerCountSource != "default" {
		t.Fatalf("override source: %q", p.ReviewerCountSource)
	}
	resetPanelCreateFlags(t)
	if out, err := runPanelVerbCmd("create", "actual", "--gate", "adhoc", "--target", "HEAD", "--round", "2"); err != nil {
		t.Fatalf("round bump: %v %s", err, out)
	}
	res, err := panel.Tally(filepath.Join(root, "review", "actual"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Panel.ReviewerCountSource != "default" || !reflect.DeepEqual(res.Panel.Roster, p.Roster) {
		t.Fatalf("round lost default count source or override roster: %+v", res.Panel)
	}
}

func TestPanelCreateDuplicateReviewerOverrideRefused(t *testing.T) {
	root := setupCreateIntegrity(t)
	before := panelTreeBytes(t, root)
	out, err := runPanelVerbCmd("create", "bad", "--gate", "adhoc", "--target", "HEAD", "--reviewer", "R1=a", "--reviewer", "R1=b")
	if err == nil || !strings.Contains(err.Error(), "duplicate reviewer override") {
		t.Fatalf("duplicate override must be refused: %v %s", err, out)
	}
	if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("duplicate override wrote files")
	}
}
