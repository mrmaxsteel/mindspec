package panel_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func artifactFixture(t *testing.T) paneltest.Fixture {
	t.Helper()
	f := paneltest.New(t)
	f.Panel.VerdictSchema = 2
	if err := panel.WriteSnapshot(f.Dir, f.Panel); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestS133ArtifactPath_SchemaTypes(t *testing.T) {
	data, err := panel.VerdictV2Schema()
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if typ, ok := node["type"]; ok {
				switch typ {
				case "object", "array", "string", "integer", "boolean":
				default:
					t.Fatalf("invalid exported schema type %q", typ)
				}
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(root)
}

func TestS133ArtifactPath_SubmitValidation(t *testing.T) {
	for _, name := range []string{"docs/result.txt", "a/../x", "../x", "/x", "x\x00y", "x\ny", "x\x1fy", "x\x7fy", "..", "a/..", "a\\b", ""} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			f := artifactFixture(t)
			v := paneltest.Verdict(panel.VerdictRequestChanges)
			var raw map[string]any
			if err := json.Unmarshal(paneltest.Bytes(t, v), &raw); err != nil {
				t.Fatal(err)
			}
			raw["concrete_changes_required"].([]any)[0].(map[string]any)["artifact_path"] = name
			data, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			before, err := panel.ReadLedger(f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			_, err = panel.Submit(f.Target(), data, paneltest.Resolve)
			if name == "docs/result.txt" {
				if err != nil {
					t.Fatalf("valid path refused: %v", err)
				}
				return
			}
			var typed *panel.VerdictError
			if !errors.As(err, &typed) || typed.Pointer != "/concrete_changes_required/0/artifact_path" || strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("path must fail its own validation with its pointer: %v", err)
			}
			after, readErr := panel.ReadLedger(f.Dir)
			if readErr != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("invalid submission changed ledger: %v", readErr)
			}
		})
	}
}

func TestS133ArtifactPath_RoundTreeAndDischarges(t *testing.T) {
	f := artifactFixture(t)
	v := paneltest.Verdict(panel.VerdictRequestChanges)
	v.ConcreteChangesRequired[0].ArtifactPath = "evidence/result.txt"
	if _, err := panel.Submit(f.Target(), paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
		t.Fatal(err)
	}
	v = paneltest.Verdict(panel.VerdictApprove)
	v.Slot = "R2"
	target := f.Target()
	target.Slot = "R2"
	if _, err := panel.Submit(target, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
		t.Fatal(err)
	}
	r, err := panel.Tally(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.RoundReviewedHeadSHA != paneltest.ReviewedSHA {
		t.Fatalf("round commit lost: %q", r.RoundReviewedHeadSHA)
	}
	r.Panel.ReviewedHeadSHA = strings.Repeat("b", 40)
	reads := 0
	reader := func(_, sha string) (string, error) {
		reads++
		if sha != paneltest.ReviewedSHA {
			t.Fatalf("read rebound commit %q instead of round commit", sha)
		}
		return "100644 blob " + strings.Repeat("c", 40) + "\tREADME.md\n", nil
	}
	details := panel.ArtifactPathDetails(r, "unused", reader)
	for _, want := range []string{"absent from round tree", "R1", "ccr[0]", "evidence/result.txt"} {
		if !strings.Contains(details, want) {
			t.Fatalf("missing %q: %s", want, details)
		}
	}
	for _, discharge := range []string{"refutation", "deferral", "plan deferral"} {
		t.Run(discharge, func(t *testing.T) {
			copy := *r
			p := *r.Panel
			copy.Panel = &p
			finding := r.Verdicts[0].Findings[0]
			switch discharge {
			case "refutation":
				p.Refutations = []panel.Refutation{{Slot: "R1", Round: 1, Finding: finding.ID, FindingSHA256: finding.SHA256}}
			case "deferral":
				p.Deferrals = []panel.Deferral{{Slot: "R1", Round: 1, Finding: finding.ID, FindingSHA256: finding.SHA256, FindingText: finding.Text, Bead: "follow-up", By: "operator", CreatedAt: paneltest.CreatedAt, SourceVerdict: panel.VerdictRequestChanges, SourceFileSHA256: r.Verdicts[0].FileSHA256}}
			case "plan deferral":
				// Plan deferrals must be ledger-backed, even for the annotation.
				p.Gate = "spec_approve"
				entry := panel.PlanDeferral{ID: "plan-test", Slot: "R1", Round: 1, Finding: finding.ID, FindingSHA256: finding.SHA256, FindingText: finding.Text, By: "operator", CreatedAt: paneltest.CreatedAt}
				p.PlanDeferrals = []panel.PlanDeferral{entry}
				paneltest.Seed(t, f.Dir, panel.PlanDeferralRecord{Record: panel.RecordDeferredToPlan, Spec: p.Spec, Panel: "p", PlanDeferral: entry})
			}
			before := reads
			if got := panel.ArtifactPathDetails(&copy, "unused", reader); got != "" || reads != before {
				t.Fatalf("discharged item read tree or still blocked: %q, reads %d -> %d", got, before, reads)
			}
		})
	}
}

func TestS133ArtifactPath_ExistingAndUnverifiedPaths(t *testing.T) {
	r := &panel.Result{Panel: &panel.Panel{}, LatestRound: 1, RoundReviewedHeadSHA: paneltest.ReviewedSHA,
		Verdicts: []panel.Verdict{{Slot: "R1", Round: 1, Verdict: panel.VerdictRequestChanges, Findings: []panel.Finding{{ID: "ccr[0]"}}, ArtifactPaths: map[string]string{"ccr[0]": "docs/résult.txt"}}}}
	for _, tc := range []struct {
		name, listing, want string
		err                 error
	}{
		{"present", "100644 blob abc\t\"docs/r\\303\\251sult.txt\"\n", "", nil},
		{"absent", "", "absent from round tree", nil},
		{"read error", "", "not verified", errors.New("unreadable commit")},
		{"bad listing", "broken", "not verified", nil},
		{"bad quoting", "100644 blob abc\t\"unterminated\n", "not verified", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := panel.ArtifactPathDetails(r, "unused", func(string, string) (string, error) { return tc.listing, tc.err })
			if (tc.want == "" && got != "") || (tc.want != "" && !strings.Contains(got, tc.want)) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestS133ArtifactPath_PublicMessages(t *testing.T) {
	internalRef := regexp.MustCompile(`(?i)ADR[- ]?[0-9]+|spec[ -]+[0-9]+|\b(?:AC|R)[0-9]+(?:\.[0-9]+)?\b|mindspec-[a-z0-9.-]+|bead[ -]+[0-9]+|issue[ -]+[0-9]+|#[0-9]+`)
	f := artifactFixture(t)
	for _, name := range []string{"../x", "/x", "x\x01y"} {
		v := paneltest.Verdict(panel.VerdictRequestChanges)
		v.ConcreteChangesRequired[0].ArtifactPath = name
		_, err := panel.Submit(f.Target(), paneltest.Bytes(t, v), paneltest.Resolve)
		if err == nil || internalRef.MatchString(err.Error()) {
			t.Fatalf("validation message contains an internal reference or is absent: %v", err)
		}
	}
	r := &panel.Result{Panel: &panel.Panel{}, LatestRound: 1, RoundReviewedHeadSHA: paneltest.ReviewedSHA,
		Verdicts: []panel.Verdict{{Slot: "reviewer", Round: 1, Verdict: panel.VerdictRequestChanges, Findings: []panel.Finding{{ID: "ccr[0]"}}, ArtifactPaths: map[string]string{"ccr[0]": "evidence/result.txt"}}}}
	for _, reader := range []func(string, string) (string, error){nil,
		func(string, string) (string, error) { return "", nil },
		func(string, string) (string, error) { return "broken", nil },
	} {
		message := panel.ArtifactPathDetails(r, "unused", reader)
		if message == "" || internalRef.MatchString(message) {
			t.Fatalf("artifact message contains an internal reference or is absent: %q", message)
		}
	}
}

func TestS133ArtifactPath_NoFieldKeepsDecision(t *testing.T) {
	f := artifactFixture(t)
	for _, verdict := range []string{panel.VerdictApprove, panel.VerdictRequestChanges} {
		t.Run(verdict, func(t *testing.T) {
			for _, slot := range []string{"R1", "R2"} {
				v := paneltest.Verdict(verdict)
				v.Slot = slot
				target := f.Target()
				target.Slot = slot
				if _, err := panel.Submit(target, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
					t.Fatal(err)
				}
			}
			r, err := panel.Tally(f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			facts := panel.GateFacts{Reg: &panel.Registration{Dir: f.Dir, Panel: *r.Panel}, Res: r, HeadSHA: paneltest.ReviewedSHA, WorktreeAbsent: true}
			before := panel.PanelGateDecision(facts)
			facts.ArtifactDetails = panel.ArtifactPathDetails(r, "unused", func(string, string) (string, error) { t.Fatal("no artifact path must make no read"); return "", nil })
			after := panel.PanelGateDecision(facts)
			want := panel.Allow
			if verdict == panel.VerdictRequestChanges {
				want = panel.Block
			}
			if !reflect.DeepEqual(before, after) || after.Action != want {
				t.Fatalf("no-field behavior changed: before=%+v after=%+v want=%v", before, after, want)
			}
		})
	}
}
