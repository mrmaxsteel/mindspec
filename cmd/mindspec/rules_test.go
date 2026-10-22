package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/rules"
)

func TestRulesCmd(t *testing.T) {
	root := mkFlatPanelTestRoot(t)
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	if registered, _, err := rootCmd.Find([]string{"rules"}); err != nil || registered != rulesCmd {
		t.Fatalf("rules command is not registered: %v", err)
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := newRulesCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	want := []string{"panel.verbs-only", "panel.submit", "panel.isolated-checkout", "panel.scratch-in-slot", "review.basis", "review.property", "review.mutation", "review.stated-limit", "review.ci-parity", "review.no-vote"}
	text, err := run("--scope", "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		id, rest, ok := strings.Cut(line, ": ")
		if !ok || !strings.Contains(rest, " — ") {
			t.Fatalf("invalid rule line %q", line)
		}
		ids = append(ids, id)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("reviewer text ids = %v, want %v", ids, want)
	}
	text, err = run("--scope", "reviewer", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got []rules.Rule
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	ids = nil
	for _, r := range got {
		ids = append(ids, r.ID)
		if r.Prohibition == "" || r.Replacement == "" || r.EnforcedBy == "" {
			t.Fatalf("JSON lost rule fields: %+v", r)
		}
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("reviewer JSON ids = %v, want %v", ids, want)
	}
	text, err = run("--json")
	if err != nil {
		t.Fatal(err)
	}
	var objects []map[string]any
	if err := json.Unmarshal([]byte(text), &objects); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		keys []string
	}{
		{"panel.submit", []string{"enforced_by", "id", "prohibition", "replacement", "scope"}},
		{"review.ci-parity", []string{"enforced_by", "gates", "id", "prohibition", "replacement", "scope"}},
		{"sequence.bead-impl-prompt", []string{"enforced_by", "id", "prohibition", "replacement", "scope", "target"}},
	} {
		t.Run("json-keys/"+tc.id, func(t *testing.T) {
			for _, object := range objects {
				if object["id"] != tc.id {
					continue
				}
				var keys []string
				for key := range object {
					keys = append(keys, key)
				}
				slices.Sort(keys)
				if !reflect.DeepEqual(keys, tc.keys) {
					t.Fatalf("JSON keys = %v, want %v", keys, tc.keys)
				}
				return
			}
			t.Fatalf("JSON missing rule %q", tc.id)
		})
	}
	got = nil
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	all, err := rules.Load()
	if err != nil || !reflect.DeepEqual(got, all) {
		t.Fatalf("unfiltered JSON differs from table: %v", err)
	}
	for _, tc := range []struct {
		id, fragment string
	}{
		{"panel.submit", "enforced_by: panel submit"},
		{"review.ci-parity", "gates: final_review"},
		{"sequence.bead-impl-prompt", "target: next-action"},
	} {
		out, err := run("show", tc.id)
		if err != nil || !strings.HasPrefix(out, tc.id+": ") || !strings.Contains(out, tc.fragment) {
			t.Errorf("show %s: %q, %v", tc.id, out, err)
		}
	}
	for _, args := range [][]string{{"show", "nope"}, {"--scope", "nope"}} {
		if _, err := run(args...); err == nil || !strings.Contains(err.Error(), "recovery:") {
			t.Errorf("%v must refuse with recovery: %v", args, err)
		}
	}
}

func TestRulesGateKeys_MatchConfig(t *testing.T) {
	got := rules.GateKeys()
	want := slices.Clone(config.PanelGateKeys)
	slices.Sort(got)
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rules gate keys = %v, want config.PanelGateKeys %v", got, want)
	}
}
