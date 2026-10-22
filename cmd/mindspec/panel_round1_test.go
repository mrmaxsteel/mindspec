package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/instruct"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestPanelDefer_V2ApproveMinorSubmission(t *testing.T) {
	for _, finding := range []string{"ccr[0]", ""} {
		t.Run("finding="+finding, func(t *testing.T) {
			f := newCLIV2(t)
			fake := fakeDeferralBD(t)
			rebindWrite(t, fake, "epics.json", `[{"id":"epic-x","metadata":{"spec_num":900,"spec_title":"test"}}]`)
			rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"open","description":"minor nit","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`)
			v := f.verdict()
			v.Findings = []string{"minor nit"}
			requirePanelVerb(t, f.args(f.file(t, v))...)
			before, err := panel.Tally(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			args := append(deferArgs(), "--spec", "900-test")
			if finding != "" {
				args = append(args, "--finding", finding)
			}
			requirePanelVerb(t, args...)
			lines, err := panel.ReadLedger(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			var event panel.DeferralRecord
			var row panel.DispositionRow
			for _, line := range lines {
				var kind struct{ Record string }
				if err := json.Unmarshal(line, &kind); err != nil {
					t.Fatal(err)
				}
				switch kind.Record {
				case panel.RecordDeferral:
					if err := json.Unmarshal(line, &event); err != nil {
						t.Fatal(err)
					}
				case panel.RecordDisposition:
					if err := json.Unmarshal(line, &row); err != nil {
						t.Fatal(err)
					}
				}
			}
			wantID := finding
			if wantID == "" {
				wantID = "slot"
			}
			if event.Record != panel.RecordDeferral || event.Slot != "R1" || event.Round != 1 || event.Finding != wantID || event.FindingText != "minor nit" || event.Bead != "follow-up" || event.FindingSHA256 == "" {
				t.Fatalf("missing APPROVE audit event: %+v", event)
			}
			if row.Disposition != "confirmed-deferred" || row.Severity != "minor" || row.Reviewer != "R1" || row.Model != v.Model {
				t.Fatalf("bad minor row: %+v", row)
			}
			after, err := panel.Tally(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			if after.ResolvedCount() != 0 || after.ResolvedCount() != before.ResolvedCount() || after.Approves != before.Approves {
				t.Fatalf("APPROVE deferral affected gate counts: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestPanelRebind_ShortSHASpecAndPlan(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		for _, length := range []int{4, 5, 6} {
			t.Run(fmt.Sprintf("%s/%d", gate, length), func(t *testing.T) {
				root, dir, from := setupRebind(t, strings.TrimSuffix(gate, "_approve")+"-pinned", fmt.Sprint(length))
				p := rebindRegistration(t, dir).Panel
				if p.Gate != gate || p.Target != from[:length] {
					t.Fatalf("wrong fixture: %+v", p)
				}
				if got := rebindGit(t, root, "rev-parse", p.Target); got != from {
					t.Fatalf("short SHA %s resolves to %s", p.Target, got)
				}
				rebindGit(t, root, "checkout", "-q", "spec/900-demo")
				name := ".mindspec/specs/900-demo/plan.md"
				if gate == "spec_approve" {
					name = ".mindspec/specs/900-demo/spec.md"
				}
				rebindWrite(t, root, name, "# changed subject\n")
				rebindGit(t, root, "add", name)
				rebindGit(t, root, "commit", "-qm", "subject edit")
				tip := rebindGit(t, root, "rev-parse", "HEAD")
				rebindGit(t, root, "checkout", "-q", "main")
				out, err := runPanelVerbCmd("verify", "demo")
				if err != nil || !strings.Contains(out, "BLOCK:") || !strings.Contains(out, "commits landed after review") {
					t.Fatalf("short SHA verify must be stale: %v\n%s", err, out)
				}
				if out := instruct.PanelState(root); !strings.Contains(out, "gate would BLOCK") || !strings.Contains(out, "commits landed after review") {
					t.Fatalf("instruct must report stale: %s", out)
				}
				if out, err := runPanelVerbCmd("rebind", "demo", "--reason", "max: subject edit"); err != nil || strings.Contains(out, "already fresh") {
					t.Fatalf("rebind failed: %v %s", err, out)
				}
				p = rebindRegistration(t, dir).Panel
				if p.ReviewedHeadSHA != tip || len(p.Rebinds) != 1 || p.Rebinds[0].FromSHA != from || p.Rebinds[0].ToSHA != tip {
					t.Fatalf("missing short SHA rebind: %+v", p)
				}
				lines, err := panel.ReadLedger(dir)
				if err != nil {
					t.Fatal(err)
				}
				var recorded bool
				for _, line := range lines {
					var event panel.RebindRecord
					if err := json.Unmarshal(line, &event); err != nil {
						t.Fatal(err)
					}
					if event.Record == panel.RecordRebind {
						recorded = true
					}
				}
				if !recorded {
					t.Fatal("no rebind audit event")
				}
			})
		}
	}
}

func TestPanelDispositionAppend_RefusesSpecMismatch(t *testing.T) {
	const requested = "117-panel-review-telemetry"
	const other = "118-other"
	root := mkDispositionRoot(t, requested)
	dir := filepath.Join(root, ".mindspec", "specs", other, "reviews", "p-mismatch")
	if err := panel.WriteSnapshot(dir, panel.Panel{Spec: other, Gate: "bead", Round: 1}); err != nil {
		t.Fatal(err)
	}
	err := runDispositionAppend(t, requested, "p-mismatch", `{"reviewer":"R1","model":"opus","severity":"minor","summary":"rename X","disposition":"confirmed-fixed"}`)
	if err == nil {
		t.Fatal("append must refuse a panel registered to another spec")
	}
	if _, err := os.Stat(filepath.Join(root, ".mindspec", "specs", requested, "reviews", "p-mismatch", "dispositions.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("mismatched append wrote a ledger: %v", err)
	}
}
