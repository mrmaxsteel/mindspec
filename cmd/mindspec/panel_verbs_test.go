package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/spf13/cobra"
)

func resetDischargeFlags(t *testing.T) {
	t.Helper()
	for _, cmd := range []*cobra.Command{panelRefuteCmd, panelAbandonCmd} {
		reset := func() {
			for _, name := range []string{"spec", "slot", "finding", "ground", "ac", "reason", "evidence", "by", "round"} {
				flag := cmd.Flags().Lookup(name)
				if flag != nil {
					if err := flag.Value.Set(flag.DefValue); err != nil {
						t.Fatal(err)
					}
					flag.Changed = false
				}
			}
		}
		reset()
		t.Cleanup(reset)
	}
}
func setupDischarge(t *testing.T) (string, string) {
	t.Helper()
	root := setupCreateIntegrity(t)
	resetDischargeFlags(t)
	dir := filepath.Join(root, "review", "p")
	roster := []panel.RosterSlot{{Slot: "R1", Model: "claude", Lens: "correctness"}, {Slot: "R2", Model: "codex", Lens: "tests"}}
	if err := panel.Create(dir, panel.CreateInput{Spec: "900-demo", Target: "bead/probe-x.1", Gate: "bead", Round: 1, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:abc", Roster: roster, ExpectedReviewers: 2}); err != nil {
		t.Fatal(err)
	}
	writeDischargeVerdict(t, dir, "R1", panel.FindingVerdict{Verdict: panel.VerdictRequestChanges, ConcreteChangesRequired: []string{"missing test", "second ask"}})
	writeDischargeVerdict(t, dir, "R2", panel.FindingVerdict{Verdict: panel.VerdictApprove})
	return root, dir
}
func writeDischargeVerdict(t *testing.T, dir, slot string, v panel.FindingVerdict) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, slot+"-round-1.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
}
func refuteArgs() []string {
	return []string{"refute", "p", "--slot", "R1", "--round", "1", "--ground", "disproven", "--reason", "covered by test", "--evidence", "https://example.com/proof", "--by", "max"}
}
func replaceRefuteArg(args []string, flag, value string) []string {
	out := append([]string{}, args...)
	for i := range out {
		if out[i] == flag {
			out[i+1] = value
			return out
		}
	}
	return append(out, flag, value)
}
func requirePanelVerb(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runPanelVerbCmd(args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	if !strings.Contains(out, "rule: PASS iff fresh") {
		t.Fatalf("missing shared rule line: %s", out)
	}
	return out
}
func TestPanelRefute_OneOperationIdempotent(t *testing.T) {
	root, dir := setupDischarge(t)
	requirePanelVerb(t, refuteArgs()...)
	first, err := panel.ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 {
		t.Fatalf("want round, refutation, disposition in one batch; got %s", first)
	}
	var event panel.RefutationRecord
	if err := json.Unmarshal(first[1], &event); err != nil {
		t.Fatal(err)
	}
	var row panel.DispositionRow
	if err := json.Unmarshal(first[2], &row); err != nil {
		t.Fatal(err)
	}
	if event.Record != "refutation" || row.Disposition != "audited-refuted" || row.Reviewer != "R1" || row.Model != "claude" || row.Severity != "blocking" || row.Spec != "900-demo" || row.Gate != "bead" || row.Summary != `"missing test\nsecond ask"` || row.ID != "p:R1:r1:slot:"+event.FindingSHA256[:12]+":refutation" {
		t.Fatalf("wrong records: %+v %+v", event, row)
	}
	before := panelTreeBytes(t, root)
	requirePanelVerb(t, refuteArgs()...)
	if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("rerun changed panel/ledger: before=%v after=%v", before, after)
	}
	res, err := panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Panel.Refutations) != 1 || res.IntegrityErr != nil || len(res.AppliedRefutations()) != 1 {
		t.Fatalf("refutation not applied: %+v", res)
	}
	// The event/row can land before a crash interrupts snapshot rename.
	p := *res.Panel
	p.Refutations = nil
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	requirePanelVerb(t, refuteArgs()...)
	lines, err := panel.ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lines, first) {
		t.Fatalf("crash retry duplicated ledger: %s", lines)
	}
	res, err = panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Panel.Refutations) != 1 || res.IntegrityErr != nil {
		t.Fatalf("crash retry did not repair snapshot: %+v", res)
	}
}

// Pin the actual batch call as well as its persisted result: separate appends
// would leave a crash window between the event and its disposition row.
func TestPanelRefute_BatchLock(t *testing.T) {
	tree, err := parser.ParseFile(token.NewFileSet(), "panel_refute.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	ast.Inspect(tree, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "panel" {
			return true
		}
		if sel.Sel.Name == "AppendRecords" {
			t.Error("refute must append the event and row in one checked lock hold")
		}
		if sel.Sel.Name == "AppendRecordsWithCheck" {
			calls++
			if len(call.Args) != 4 {
				t.Error("one checked batch must carry both event and row")
			}
		}
		return true
	})
	if calls != 1 {
		t.Fatalf("want exactly one batch append, got %d", calls)
	}
}
func TestPanelRefute_ACResolved(t *testing.T) {
	for _, ac := range []string{"", "AC-99", "AC-1"} {
		t.Run(ac, func(t *testing.T) {
			root, _ := setupDischarge(t)
			path := filepath.Join(root, ".mindspec", "docs", "specs", "900-demo")
			// mkPanelTestRoot uses the canonical spec layout.
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "spec.md"), []byte("# Spec\n- **AC-1**: supported criterion\n"), 0644); err != nil {
				t.Fatal(err)
			}
			args := replaceRefuteArg(refuteArgs(), "--ground", "out_of_acceptance_scope")
			args = append(args, "--ac", ac)
			if ac == "AC-1" {
				requirePanelVerb(t, args...)
			} else {
				assertPanelRefusal(t, root, args...)
			}
		})
	}
}
func TestPanelRefute_Refusals(t *testing.T) {
	cases := []struct {
		name        string
		verdict     *panel.FindingVerdict
		flag, value string
	}{
		{name: "APPROVE", verdict: &panel.FindingVerdict{Verdict: "APPROVE"}},
		{name: "REJECT", verdict: &panel.FindingVerdict{Verdict: "REJECT"}},
		{name: "hard_block", verdict: &panel.FindingVerdict{Verdict: "REQUEST_CHANGES", HardBlock: true, Rationale: "blocked"}},
		{name: "INVALID", verdict: &panel.FindingVerdict{Verdict: "INVALID"}},
		{name: "empty RC", verdict: &panel.FindingVerdict{Verdict: "REQUEST_CHANGES"}},
		{name: "absent slot", flag: "--slot", value: "R3"},
		{name: "future round", flag: "--round", value: "2"},
		{name: "unknown finding", flag: "--finding", value: "ccr[99]"},
		{name: "empty by", flag: "--by", value: " "},
		{name: "unresolved evidence", flag: "--evidence", value: "does-not-exist"},
		{name: "empty evidence", flag: "--evidence", value: ""},
		{name: "empty reason", flag: "--reason", value: ""},
		{name: "unknown ground", flag: "--ground", value: "wrong"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, dir := setupDischarge(t)
			if tc.verdict != nil {
				writeDischargeVerdict(t, dir, "R1", *tc.verdict)
			}
			args := refuteArgs()
			if tc.flag != "" {
				args = replaceRefuteArg(args, tc.flag, tc.value)
			}
			assertPanelRefusal(t, root, args...)
		})
	}
	t.Run("stale round", func(t *testing.T) {
		root, dir := setupDischarge(t)
		roster := []panel.RosterSlot{{Slot: "R1", Model: "claude", Lens: "correctness"}, {Slot: "R2", Model: "codex", Lens: "tests"}}
		if err := panel.Create(dir, panel.CreateInput{Spec: "900-demo", Target: "bead/probe-x.1", Gate: "bead", Round: 2, ReviewedHeadSHA: "abc123", SubjectDigest: "sha256:abc", Roster: roster, ExpectedReviewers: 2}); err != nil {
			t.Fatal(err)
		}
		for _, slot := range []string{"R1", "R2"} {
			raw, err := os.ReadFile(filepath.Join(dir, slot+"-round-1.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, slot+"-round-2.json"), raw, 0644); err != nil {
				t.Fatal(err)
			}
		}
		assertPanelRefusal(t, root, refuteArgs()...)
		requirePanelVerb(t, replaceRefuteArg(refuteArgs(), "--round", "2")...)
	})
}
func TestPanelRefute_RedactsPaths(t *testing.T) {
	_, dir := setupDischarge(t)
	writeDischargeVerdict(t, dir, "R1", panel.FindingVerdict{Verdict: "REQUEST_CHANGES", ConcreteChangesRequired: []string{"inspect /tmp/x and /Users/max/private"}})
	args := append(refuteArgs(), "--finding", "ccr[0]")
	requirePanelVerb(t, args...)
	lines, err := panel.ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines[1:] {
		if bytes.Contains(line, []byte("/tmp/x")) || bytes.Contains(line, []byte("/Users/")) {
			t.Fatalf("path leaked: %s", line)
		}
	}
	var row panel.DispositionRow
	if err := json.Unmarshal(lines[2], &row); err != nil {
		t.Fatal(err)
	}
	if row.Summary != "inspect <path> and <path>" {
		t.Fatalf("wrong summary: %q", row.Summary)
	}
	res, err := panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.IntegrityErr != nil || len(res.AppliedRefutations()) != 1 || res.Panel.Refutations[0].FindingText != row.Summary {
		t.Fatalf("redacted refutation not applied: %+v", res)
	}
}
func TestPanelRefute_Evidence(t *testing.T) {
	for _, evidence := range []string{".mindspec", "https://example.com/proof", "HEAD", "absolute repo path"} {
		t.Run(evidence, func(t *testing.T) {
			root, dir := setupDischarge(t)
			absolute := evidence == "absolute repo path"
			if absolute {
				evidence = filepath.Join(root, ".mindspec")
			}
			if evidence == "HEAD" {
				subjectCLIGit(t, root, "init")
				subjectCLIGit(t, root, "commit", "--allow-empty", "-m", "evidence")
			}
			requirePanelVerb(t, replaceRefuteArg(refuteArgs(), "--evidence", evidence)...)
			if absolute {
				res, err := panel.Tally(dir)
				if err != nil {
					t.Fatal(err)
				}
				if res.Panel.Refutations[0].Evidence != ".mindspec" {
					t.Fatal("absolute repo evidence must be stored relatively")
				}
			}
		})
	}
	for _, absolute := range []bool{false, true} {
		name := "symlink relative"
		if absolute {
			name = "symlink absolute"
		}
		t.Run(name, func(t *testing.T) {
			root, dir := setupDischarge(t)
			link := filepath.Join(t.TempDir(), "repo")
			if err := os.Symlink(root, link); err != nil {
				t.Fatal(err)
			}
			withTestChdir(t, link)
			t.Setenv("PWD", link)
			if got, err := findRoot(); err != nil || got != link {
				t.Fatalf("symlink root not exercised: got %q, %v", got, err)
			}
			evidence := ".mindspec"
			if absolute {
				evidence = filepath.Join(link, evidence)
			}
			requirePanelVerb(t, replaceRefuteArg(refuteArgs(), "--evidence", evidence)...)
			res, err := panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Panel.Refutations) != 1 || res.Panel.Refutations[0].Evidence != ".mindspec" {
				t.Fatalf("symlink evidence must be stored relatively: %+v", res.Panel.Refutations)
			}
		})
	}
	for _, name := range []string{"/etc/hosts", "../", "outside file", "blob", "tree"} {
		t.Run(name, func(t *testing.T) {
			root, _ := setupDischarge(t)
			evidence := name
			switch name {
			case "outside file":
				evidence = filepath.Join(t.TempDir(), "proof.txt")
				if err := os.WriteFile(evidence, []byte("outside proof"), 0644); err != nil {
					t.Fatal(err)
				}
			case "blob", "tree":
				subjectCLIGit(t, root, "init")
				subjectCLIWrite(t, root, "proof.txt", "proof")
				subjectCLIGit(t, root, "add", "proof.txt")
				subjectCLIGit(t, root, "commit", "-m", "evidence")
				object := "HEAD:proof.txt"
				if name == "tree" {
					object = "HEAD^{tree}"
				}
				evidence = subjectCLIGit(t, root, "rev-parse", object)
			}
			if name != "blob" && name != "tree" {
				if _, err := os.Stat(evidence); err != nil {
					t.Fatalf("outside evidence must exist: %v", err)
				}
			}
			assertPanelRefusal(t, root, replaceRefuteArg(refuteArgs(), "--evidence", evidence)...)
		})
	}
}
func TestPanelAbandon_Verb(t *testing.T) {
	for _, tc := range []struct {
		reason, by string
		ok         bool
	}{{"max: rework", "max", true}, {"", "max", false}, {"max: rework", "", false}} {
		t.Run(tc.reason+tc.by, func(t *testing.T) {
			root, dir := setupDischarge(t)
			args := []string{"abandon", "p", "--reason", tc.reason, "--by", tc.by}
			if !tc.ok {
				assertPanelRefusal(t, root, args...)
				return
			}
			requirePanelVerb(t, args...)
			res, err := panel.Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !res.Panel.Abandoned || res.Panel.AbandonReason != tc.reason || res.IntegrityErr != nil {
				t.Fatalf("abandon snapshot: %+v", res)
			}
			lines, err := panel.ReadLedger(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(lines) != 2 {
				t.Fatalf("abandon must write one event and no manifest: %s", lines)
			}
			var event panel.AbandonRecord
			if err := json.Unmarshal(lines[1], &event); err != nil {
				t.Fatal(err)
			}
			if event.Record != "abandon" || event.By != tc.by || event.Reason != tc.reason {
				t.Fatalf("wrong event: %+v", event)
			}
		})
	}
}

func TestPanelAbandon_VerbRetryAndIncompleteRound(t *testing.T) {
	root, dir := setupDischarge(t)
	if err := os.WriteFile(filepath.Join(dir, "R1-round-2.json"), []byte(`{"verdict":"APPROVE"}`), 0644); err != nil {
		t.Fatal(err)
	}
	args := []string{"abandon", "p", "--reason", "max: rework", "--by", "max"}
	requirePanelVerb(t, args...)
	before := panelTreeBytes(t, root)
	requirePanelVerb(t, args...)
	if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("abandon rerun changed ledger or snapshot")
	}
	res, err := panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := *res.Panel
	p.Abandoned = false
	p.AbandonReason = ""
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	requirePanelVerb(t, args...)
	if after := panelTreeBytes(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("abandon crash retry did not converge")
	}
}
func TestPanelAbandon_KeepsManifest(t *testing.T) {
	_, dir := setupDischarge(t)
	manifest := panel.CoverageManifest{Record: "panel", Spec: "900-demo", Gate: "bead", Panel: "p", Round: 1, Slots: []panel.ManifestSlot{{Slot: "R1", Model: "claude", Verdict: "APPROVE"}, {Slot: "R2", Model: "codex", Verdict: "APPROVE"}}, CreatedAt: "2026-10-20T00:00:00Z"}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := panel.AppendRecords(dir, raw); err != nil {
		t.Fatal(err)
	}
	before, err := panel.ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	requirePanelVerb(t, "abandon", "p", "--reason", "max: rework", "--by", "max")
	after, err := panel.ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)+1 || !bytes.Equal(after[1], before[1]) {
		t.Fatalf("manifest was removed or rewritten: %s", after)
	}
	var retained panel.CoverageManifest
	if err := json.Unmarshal(after[1], &retained); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(panel.ComputeQ4(nil, []panel.CoverageManifest{manifest}), panel.ComputeQ4(nil, []panel.CoverageManifest{retained})) {
		t.Fatal("abandon changed Q4 coverage")
	}
	if len(retained.Slots) != 2 {
		t.Fatal("Q4 denominator lost retained slots")
	}
}
func TestPanelRefute_BatchConflictWritesNothing(t *testing.T) {
	root, dir := setupDischarge(t)
	requirePanelVerb(t, refuteArgs()...)
	assertPanelRefusal(t, root, replaceRefuteArg(refuteArgs(), "--reason", "conflicting reason")...)
	res, err := panel.Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Panel.Refutations) != 1 {
		t.Fatal("conflict changed snapshot")
	}
}
