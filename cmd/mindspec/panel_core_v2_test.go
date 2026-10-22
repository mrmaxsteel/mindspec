package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/spf13/cobra"
)

func TestPanelSchema(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := panelSchemaCmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(out.Bytes(), &schema); err != nil {
		t.Fatal(err)
	}
	expected, err := panel.VerdictV2Schema()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != string(expected) {
		t.Fatal("CLI did not print canonical schema")
	}
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Fatalf("wrong schema %v", schema)
	}
	found := false
	for _, child := range panelCmd.Commands() {
		if child == panelSchemaCmd {
			found = true
		}
	}
	if !found {
		t.Fatal("schema command not wired")
	}
}
func TestLandedBindingFor(t *testing.T) {
	calls := 0
	read := func(id string) (map[string]interface{}, error) {
		calls++
		if id != "b" {
			t.Fatalf("id %s", id)
		}
		return map[string]interface{}{"mindspec_landed_review_panel": "p@r2"}, nil
	}
	if got, err := landedBindingFor(read, "", "p", 2); err != nil || got || calls != 0 {
		t.Fatal("adhoc read metadata")
	}
	for _, tc := range []struct {
		slug  string
		round int
		want  bool
	}{{"p", 2, true}, {"p", 1, false}, {"q", 2, false}} {
		got, err := landedBindingFor(read, "b", tc.slug, tc.round)
		if err != nil || got != tc.want {
			t.Fatalf("match %+v: %v %v", tc, got, err)
		}
	}
	injected := errors.New("metadata unavailable")
	if _, err := landedBindingFor(func(string) (map[string]interface{}, error) { return nil, injected }, "b", "p", 2); !errors.Is(err, injected) {
		t.Fatalf("swallowed metadata error %v", err)
	}
}
func TestFindPanelRoot_NoSlotFallsBack(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	expected, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	got, err := findPanelRoot(func(string) (string, error) { t.Fatal("no slot should not load config"); return "", nil })
	if err != nil || got != expected {
		t.Fatalf("fallback got %s want %s: %v", got, expected, err)
	}
}
func TestDispositionAppend_RefusesV2Kinds(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".mindspec", "specs", "999-test")
	if err := os.MkdirAll(specDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	for _, tc := range []struct{ kind, verb string }{
		{"round", "create"}, {"refutation", "refute"}, {"deferral", "defer"},
		{"rebind", "rebind"}, {"abandon", "abandon"}, {"verdict", "submit"},
		{"launch", "launch"}, {"substitution", "launch"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			cmd := panelDispositionAppendCmd
			for key, value := range map[string]string{"spec": "999-test", "panel": "p", "data": `{"record":"` + tc.kind + `"}`} {
				flag := cmd.Flags().Lookup(key)
				t.Cleanup(func() {
					if err := cmd.Flags().Set(flag.Name, flag.DefValue); err != nil {
						t.Error(err)
					}
					flag.Changed = false
				})
				if err := cmd.Flags().Set(key, value); err != nil {
					t.Fatal(err)
				}
			}
			err := cmd.RunE(cmd, nil)
			if err == nil || !strings.Contains(err.Error(), "reserved") {
				t.Fatalf("kind %s not refused as reserved: %v", tc.kind, err)
			}
			lines := strings.Split(err.Error(), "\n")
			if want := "recovery: use panel " + tc.verb; lines[len(lines)-1] != want || strings.Count(err.Error(), "recovery:") != 1 {
				t.Fatalf("kind %s: want one final recovery %q, got %v", tc.kind, want, err)
			}
			if _, err := os.Stat(filepath.Join(specDir, "reviews", "p", "dispositions.jsonl")); !os.IsNotExist(err) {
				t.Fatal("append wrote forbidden record")
			}
		})
	}
}

func TestDispositionReadPaths_ExecutionKinds(t *testing.T) {
	const specID = "999-test"
	root := mkDispositionRoot(t, specID)
	dir := filepath.Join(root, ".mindspec", "specs", specID, "reviews", "p")
	const timestamp = "2026-10-20T10:00:00Z"
	values := []any{
		panel.SubmissionRecord{Record: panel.RecordVerdict, Spec: specID, Panel: "p", Slot: "R1", Round: 1, Seq: 1, Status: "final", Verdict: "APPROVE", Family: "source", Model: "m", RunnerKind: "harness", RunnerSource: "default", Via: "direct", FileSHA256: strings.Repeat("a", 64), CreatedAt: timestamp},
		panel.LaunchRecord{Record: panel.RecordLaunch, Spec: specID, Panel: "p", Slot: "R1", Round: 1, Attempt: 1, Family: "source", Model: "m", Status: "dispatched", RunnerKind: "harness", RunnerSource: "default", CreatedAt: timestamp},
		panel.SubstitutionRecord{Record: panel.RecordSubstitution, Spec: specID, Panel: "p", Slot: "R1", Round: 1, FromFamily: "source", ToFamily: "target", Model: "m", CreatedAt: timestamp},
		panel.CoverageManifest{Record: panel.RecordPanelManifest, Spec: specID, Panel: "p", Gate: "adhoc", Round: 1, Slots: []panel.ManifestSlot{{Slot: "R1", Model: "m", Verdict: "APPROVE"}}},
	}
	var records [][]byte
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, raw)
	}
	if err := panel.AppendRecords(dir, records...); err != nil {
		t.Fatal(err)
	}
	oldDir, oldSpec, oldMetric := queryDir, querySpec, queryMetric
	t.Cleanup(func() { queryDir, querySpec, queryMetric = oldDir, oldSpec, oldMetric })
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
		args []string
		want string
	}{
		{"validate", panelDispositionValidateCmd, []string{filepath.Join(dir, "dispositions.jsonl")}, "all lines valid"},
		{"check", panelDispositionCheckCmd, nil, "completeness floor satisfied"},
		{"query", panelDispositionQueryCmd, nil, "m: 0/0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "check" {
				for key, value := range map[string]string{"spec": specID, "panel": "p"} {
					flag := tc.cmd.Flags().Lookup(key)
					t.Cleanup(func() {
						if err := tc.cmd.Flags().Set(flag.Name, flag.DefValue); err != nil {
							t.Error(err)
						}
						flag.Changed = false
					})
					if err := tc.cmd.Flags().Set(key, value); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tc.name == "query" {
				queryDir, querySpec, queryMetric = filepath.Dir(dir), "", "Q1"
			}
			var out bytes.Buffer
			tc.cmd.SetOut(&out)
			t.Cleanup(func() { tc.cmd.SetOut(nil) })
			if err := tc.cmd.RunE(tc.cmd, tc.args); err != nil || !strings.Contains(out.String(), tc.want) {
				t.Fatalf("%s: %v; output: %s", tc.name, err, out.String())
			}
		})
	}
}
