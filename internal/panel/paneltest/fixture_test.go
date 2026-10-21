package paneltest_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestLedger_ExecutionKinds(t *testing.T) {
	f := paneltest.New(t)
	r, err := panel.Submit(f.Target(), paneltest.Bytes(t, paneltest.Verdict("APPROVE")), paneltest.Resolve)
	if err != nil {
		t.Fatal(err)
	}
	started := f.Launch("started", 1, 1)
	started.RunnerKind = "process"
	paneltest.Seed(t, f.Dir, started)
	sub := panel.SubstitutionRecord{Record: panel.RecordSubstitution, Spec: f.Panel.Spec, Panel: "p", Slot: "R1", Round: 1, FromFamily: "harness", ToFamily: "other", Model: "m2", CreatedAt: paneltest.CreatedAt}
	paneltest.Seed(t, f.Dir, sub)
	for _, value := range []any{r, started, sub} {
		raw, _ := json.Marshal(value)
		if err := panel.Validate(raw); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if !panel.IsVerbRecord(m["record"].(string)) {
			t.Fatal("event is not reserved")
		}
		if err := panel.AppendRecords(f.Dir, raw); err != nil {
			t.Fatal("same-key retry refused", err)
		}
		m["model"] = "different"
		raw, _ = json.Marshal(m)
		if err := panel.AppendRecords(f.Dir, raw); err == nil {
			t.Fatal("same-key conflicting payload accepted")
		}
	}
	// Each key's additional coordinate must permit a distinct append.
	r.Seq = 2
	paneltest.Seed(t, f.Dir, r)
	started.Status = "launch_failed"
	paneltest.Seed(t, f.Dir, started)
	sub.Round = 2
	paneltest.Seed(t, f.Dir, sub)
	rows, manifests, err := panel.LoadStore(filepath.Join(f.Dir, "dispositions.jsonl"))
	if err != nil || len(rows) != 0 || len(manifests) != 0 {
		t.Fatalf("query rejected execution events: %v", err)
	}
	paneltest.Seed(t, f.Dir, panel.CoverageManifest{Record: panel.RecordPanelManifest, Spec: f.Panel.Spec, Panel: "p", Gate: "adhoc", Round: 1, Slots: []panel.ManifestSlot{{Slot: "R1", Model: "m", Verdict: "APPROVE"}}, CreatedAt: paneltest.CreatedAt})
	if err := panel.CheckCompleteness(filepath.Dir(filepath.Dir(f.Dir)), "p"); err != nil {
		t.Fatal("check rejected execution events", err)
	}
	for _, tc := range []struct {
		field string
		value any
	}{{"seq", 0}, {"seq", 1.5}, {"attempt", -1}, {"status", "unknown"}, {"via", "other"}, {"runner_kind", "other"}, {"runner_source", "environment"}, {"file_sha256", strings.Repeat("A", 64)}} {
		raw, _ := json.Marshal(r)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		m[tc.field] = tc.value
		raw, _ = json.Marshal(m)
		if err := panel.Validate(raw); err == nil {
			t.Fatalf("invalid %s accepted", tc.field)
		}
	}
}
func TestSlotLayoutAndFixture(t *testing.T) {
	root := t.TempDir()
	layout, err := panel.NewSlotLayout(root, "scratch", "", "p", 2, "R1")
	if err != nil {
		t.Fatal(err)
	}
	if layout.Dir != filepath.Join(root, "scratch", "adhoc", "p", "r2", "R1") || layout.Lock != filepath.Join(root, "scratch", ".locks", "adhoc", "p", "r2", "R1.lock") || layout.Tree != filepath.Join(layout.Dir, "tree") {
		t.Fatalf("wrong layout %+v", layout)
	}
	if err := panel.WriteSlotMetadata(layout, panel.SlotMetadata{Slug: "p", Round: 2, Slot: "R1", MainRoot: root}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(layout.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	var meta panel.SlotMetadata
	if err := json.Unmarshal(raw, &meta); err != nil || meta.Spec != "adhoc" {
		t.Fatalf("wrong metadata %+v %v", meta, err)
	}
	for _, file := range []string{filepath.Join(layout.AttemptDir(4), "verdict.json"), filepath.Join(layout.AttemptDir(4), "nested", "v.json")} {
		k, err := panel.AttemptFromPath(layout.Dir, file)
		if err != nil || k != 4 {
			t.Fatalf("attempt %d %v", k, err)
		}
	}
	if k, err := panel.AttemptFromPath(layout.Dir, filepath.Join(root, "attempt-9", "v.json")); err != nil || k != 0 {
		t.Fatalf("outside attempt binds: %d %v", k, err)
	}
	if k, err := panel.AttemptFromPath("", filepath.Join(root, "attempt-9", "v.json")); err != nil || k != 0 {
		t.Fatalf("unowned attempt binds: %d %v", k, err)
	}
	f := paneltest.NewLegacy(t)
	lines, err := panel.ReadLedger(f.Dir)
	if err != nil || len(lines) != 1 {
		t.Fatalf("legacy round: %v %v", lines, err)
	}
	if bytes.Contains(lines[0], []byte("verdict_schema")) {
		t.Fatal("legacy fixture pins v2")
	}
}
