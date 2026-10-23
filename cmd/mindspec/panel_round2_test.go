package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/instruct"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestPanelDefer_ApproveResubmittedRequestChanges(t *testing.T) {
	for _, finding := range []string{"ccr[0]", ""} {
		t.Run("finding="+finding, func(t *testing.T) {
			f := newCLIV2(t)
			fake := fakeDeferralBD(t)
			rebindWrite(t, fake, "epics.json", `[{"id":"epic-x","metadata":{"spec_num":900,"spec_title":"test"}}]`)
			rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"open","description":"minor nit","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`)
			v := f.verdict()
			v.Findings = []string{"minor nit"}
			requirePanelVerb(t, f.args(f.file(t, v))...)
			// Supply the other approval so the resubmitted finding alone blocks.
			other := f.verdict()
			other.Slot = "R2"
			f.submit(t, other)
			args := append(deferArgs(), "--spec", "900-test")
			if finding != "" {
				args = append(args, "--finding", finding)
			}
			requirePanelVerb(t, args...)
			v.Verdict = panel.VerdictRequestChanges
			v.Findings = nil
			v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "minor nit", Basis: "traced"}}
			requirePanelVerb(t, f.args(f.file(t, v))...)
			out, tallyErr := runPanelVerbCmd("tally", "p", "--spec", "900-test")
			res, err := panel.Tally(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			if tallyErr == nil || len(res.EffectiveDeferrals()) != 0 || res.ResolvedCount() != 0 || res.PassRule(true) || !strings.Contains(out, "1 unresolved (R1 (ccr[0]))") {
				t.Fatalf("APPROVE deferral cleared a resubmitted blocking finding: %s", out)
			}
			if len(res.Panel.Deferrals) != 1 || len(res.DeferralTexts(res.Panel.Deferrals[0])) != 0 {
				t.Fatal("audit-only deferral became a completion obligation")
			}
			manifest, err := json.Marshal(panel.DeriveCoverageManifest(res))
			if err != nil {
				t.Fatal(err)
			}
			if err := panel.AppendRecords(f.dir, manifest); err != nil {
				t.Fatal(err)
			}
			if err := panel.CheckCompletenessAt(f.dir, "p"); err == nil {
				t.Fatal("minor deferral covered the resubmitted blocking verdict")
			}
			lines, err := panel.ReadLedger(f.dir)
			if err != nil {
				t.Fatal(err)
			}
			sawMinor := false
			for _, line := range lines {
				var row panel.DispositionRow
				if err := json.Unmarshal(line, &row); err != nil {
					t.Fatal(err)
				}
				if row.Record == panel.RecordDisposition && row.Disposition == "confirmed-deferred" {
					if row.Severity != "minor" {
						t.Fatalf("audit row changed: %+v", row)
					}
					sawMinor = true
				}
			}
			if !sawMinor {
				t.Fatal("missing minor audit row")
			}
		})
	}
}

func TestPanelVerify_HexNamedTargetMoves(t *testing.T) {
	for _, kind := range []string{"branch", "tag"} {
		t.Run(kind, func(t *testing.T) {
			root, dir, from := setupRebind(t, "hex-"+kind)
			out, err := runPanelVerbCmd("verify", "demo")
			if err != nil || strings.Contains(out, "BLOCK:") {
				t.Fatalf("initial target must be fresh: %v %s", err, out)
			}
			rebindGit(t, root, "checkout", "-qb", "target-change")
			name := ".mindspec/specs/900-demo/plan.md"
			rebindWrite(t, root, name, "# changed target subject\n")
			rebindGit(t, root, "add", name)
			rebindGit(t, root, "commit", "-qm", "target edit")
			tip := rebindGit(t, root, "rev-parse", "HEAD")
			if kind == "branch" {
				rebindGit(t, root, "branch", "-f", "cafe", tip)
			} else {
				rebindGit(t, root, "tag", "-f", "cafe", tip)
			}
			rebindGit(t, root, "checkout", "-q", "main")
			if spec := rebindGit(t, root, "rev-parse", "spec/900-demo"); spec != from {
				t.Fatalf("spec moved: %s", spec)
			}
			if got := rebindGit(t, root, "rev-parse", "cafe"); got != tip {
				t.Fatalf("target did not move: %s", got)
			}
			out, err = runPanelVerbCmd("verify", "demo")
			if err != nil || !strings.Contains(out, "BLOCK:") || !strings.Contains(out, "commits landed after review") {
				t.Fatalf("hex %s must be stale: %v %s", kind, err, out)
			}
			if out := instruct.PanelState(root); !strings.Contains(out, "gate would BLOCK") || !strings.Contains(out, "commits landed after review") {
				t.Fatalf("instruct must read stale: %s", out)
			}
			if d := rebindGate(t, root, dir); d.Action != panel.Block {
				t.Fatalf("gate allowed changed hex ref: %+v", d)
			}
		})
	}
}

func TestPanelDefer_RequestChangesSubmissionBinding(t *testing.T) {
	f := newCLIV2(t)
	fake := fakeDeferralBD(t)
	rebindWrite(t, fake, "epics.json", `[{"id":"epic-x","metadata":{"spec_num":900,"spec_title":"test"}}]`)
	rebindWrite(t, fake, "follow-up.json", `[{"id":"follow-up","status":"open","description":"blocking finding","dependencies":[{"id":"epic-x","dependency_type":"discovered-from"}]}]`)
	v := f.verdict()
	v.Verdict = panel.VerdictRequestChanges
	v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: "blocking finding", Basis: "traced"}}
	submitted := f.submit(t, v)
	other := f.verdict()
	other.Slot = "R2"
	f.submit(t, other)
	requirePanelVerb(t, append(deferArgs(), "--spec", "900-test", "--finding", "ccr[0]")...)
	res, err := panel.Tally(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.EffectiveDeferrals()) != 1 {
		t.Fatal("current REQUEST_CHANGES deferral did not apply")
	}
	d := res.EffectiveDeferrals()[0]
	if d.SourceVerdict != panel.VerdictRequestChanges || d.SourceFileSHA256 != submitted.FileSHA256 {
		t.Fatalf("missing source binding: %+v", d)
	}
	manifest, err := json.Marshal(panel.DeriveCoverageManifest(res))
	if err != nil {
		t.Fatal(err)
	}
	if err := panel.AppendRecords(f.dir, manifest); err != nil {
		t.Fatal(err)
	}
	if err := panel.CheckCompletenessAt(f.dir, "p"); err != nil {
		t.Fatalf("current deferral must cover its slot: %v", err)
	}
	v.Rationale += " new submission"
	f.submit(t, v)
	res, err = panel.Tally(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.EffectiveDeferrals()) != 0 || res.ResolvedCount() != 0 || res.UnresolvedNames() != "R1 (ccr[0])" {
		t.Fatalf("superseded submission still deferred: %s", panel.RuleLine(res))
	}
	// Currency belongs to tally/gate; the durable floor does not read verdicts.
	if err := panel.CheckCompletenessAt(f.dir, "p"); err != nil {
		t.Fatalf("ledger-only floor must retain recorded coverage: %v", err)
	}
	if err := panel.CheckCore3(panel.Registration{Dir: f.dir, Panel: *res.Panel}); err == nil || !strings.Contains(err.Error(), "no covering disposition row") {
		t.Fatalf("lifecycle coverage check must reject superseded deferral: %v", err)
	}
}

func TestPanelVerify_HexTagTakesPrecedenceOverBranch(t *testing.T) {
	root, _, from := setupRebind(t, "hex-branch")
	rebindGit(t, root, "tag", "cafe", from)
	rebindGit(t, root, "checkout", "-qb", "target-change")
	rebindWrite(t, root, ".mindspec/specs/900-demo/plan.md", "# changed tag subject\n")
	rebindGit(t, root, "add", ".")
	rebindGit(t, root, "commit", "-qm", "tag edit")
	tip := rebindGit(t, root, "rev-parse", "HEAD")
	rebindGit(t, root, "tag", "-f", "cafe", tip)
	rebindGit(t, root, "checkout", "-q", "main")
	if got := rebindGit(t, root, "rev-parse", "--verify", "cafe^{commit}"); !strings.HasSuffix(got, tip) {
		t.Fatalf("git precedence got %s", got)
	}
	out, err := runPanelVerbCmd("verify", "demo")
	if err != nil || !strings.Contains(out, "BLOCK:") {
		t.Fatalf("changed tag bypassed: %v %s", err, out)
	}
}

func TestPanelVerify_FullSHANameRemainsObjectPin(t *testing.T) {
	root, _, from := setupRebind(t, "plan-pinned")
	rebindGit(t, root, "branch", from, from)
	// Git's symbolic output names this branch, but revision resolution uses
	// the full object ID. Pin both behaviors so classification cannot confuse them.
	if got := rebindGit(t, root, "rev-parse", "--symbolic-full-name", from); !strings.HasSuffix(got, "refs/heads/"+from) {
		t.Fatalf("git symbolic full-SHA behavior changed: %s", got)
	}
	if got := rebindGit(t, root, "rev-parse", "--verify", from+"^{commit}"); !strings.HasSuffix(got, from) {
		t.Fatalf("git full-SHA object behavior changed: %s", got)
	}
	rebindGit(t, root, "checkout", "-q", "spec/900-demo")
	rebindWrite(t, root, ".mindspec/specs/900-demo/plan.md", "# unreviewed spec plan\n")
	rebindGit(t, root, "add", ".")
	rebindGit(t, root, "commit", "-qm", "plan edit")
	rebindGit(t, root, "checkout", "-q", "main")
	out, err := runPanelVerbCmd("verify", "demo")
	if err != nil || !strings.Contains(out, "BLOCK:") {
		t.Fatalf("full SHA named branch hides unreviewed pinned spec: %v %s", err, out)
	}
}

func TestPanelVerify_ShortSHARefKeepsOwnTip(t *testing.T) {
	root, _, from := setupRebind(t, "plan-pinned", "6")
	rebindGit(t, root, "branch", from[:6], from)
	rebindGit(t, root, "checkout", "-q", "spec/900-demo")
	rebindWrite(t, root, ".mindspec/specs/900-demo/plan.md", "# spec plan edit\n")
	rebindGit(t, root, "add", ".")
	rebindGit(t, root, "commit", "-qm", "plan edit")
	rebindGit(t, root, "checkout", "-q", "main")
	out, err := runPanelVerbCmd("verify", "demo")
	if err != nil || strings.Contains(out, "BLOCK:") {
		t.Fatalf("named prefix branch should retain its own tip: %v %s", err, out)
	}
}
