package panel

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestS133GateText_ProductStringLiterals(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)\bADR-[0-9]+|\bspec[ -][0-9]+|\bR[0-9]+(?:\.[0-9]+)?\b|\bAC[0-9]+\b|(?:^|[ \t\n(])mindspec-[a-z0-9]+|#[0-9]+`)
	for _, known := range []string{"ADR-9999", "spec 999", "R7.3", "AC9", "mindspec-issue", "#123"} {
		if !pattern.MatchString(known) {
			t.Fatalf("detector missed %s", known)
		}
	}
	for _, name := range []string{"document_subject.go", "plan_scaffold.go", "placeholder_template.go", "subject_head.go", "gate.go", "create.go", "../approve/lifecycle_gate.go", "../approve/spec.go", "../config/config.go", "../../cmd/mindspec/panel.go", "../../cmd/mindspec/panel_rebind.go", "../instruct/panelstate.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			value = strings.ReplaceAll(value, "R1..Rn", "<roster slots>")
			if pattern.MatchString(value) {
				t.Errorf("%s has an internal reference in a product string: %q", name, value)
			}
			return true
		})
	}
}

func TestS133GateText_EveryLeg(t *testing.T) {
	for _, gate := range []string{"spec_approve", "plan_approve", "final_review"} {
		for _, leg := range []string{"registration", "integrity", "round", "stale", "subject error", "dirty", "incomplete", "reject", "hard block", "unresolved", "git error", "missing target"} {
			t.Run(gate+"/"+leg, func(t *testing.T) {
				p := &Panel{Spec: "example", Gate: gate, Target: "spec/example", Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: strings.Repeat("a", 40), SubjectDigest: "old"}
				reg := Registration{Dir: "/review/p", Panel: *p}
				r := result(p, 2, 0, 1, nil, nil)
				f := GateFacts{Reg: &reg, Res: r, HeadSHA: p.ReviewedHeadSHA, WorktreeAbsent: true}
				switch leg {
				case "registration":
					r.Panel = nil
					r.PanelErr = errors.New("broken")
				case "integrity":
					r.IntegrityErr = errors.New("changed")
				case "round":
					r.RoundMismatch = true
					r.LatestRound = 2
				case "stale":
					f.HeadSHA = strings.Repeat("b", 40)
					f.SubjectDigest = "new"
				case "subject error":
					f.SubjectErr = errors.New("unreadable subject")
				case "dirty":
					f.WorktreeAbsent = false
					f.UserDirt = []string{"source.go"}
					f.WorktreePath = "/repo"
				case "incomplete":
					r.Verdicts = r.Verdicts[:1]
				case "reject":
					r.Rejects = 1
				case "hard block":
					r.HardBlocks = []string{"R1"}
				case "unresolved":
					r.Verdicts[0].Verdict = VerdictRequestChanges
				case "git error":
					f.GitErr = errors.New("lookup failed")
				case "missing target":
					f.MissingRef = true
				}
				d := PanelGateDecision(f)
				wantAction := Block
				if leg == "git error" || leg == "missing target" {
					wantAction = Warn
				}
				if d.Action != wantAction {
					t.Fatalf("leg %s not blocked: %+v", leg, d)
				}
				for _, want := range []string{"gate " + gate, "panel p", "recovery:"} {
					if !strings.Contains(d.Message, want) {
						t.Errorf("missing %s: %s", want, d.Message)
					}
				}
				for _, bad := range []string{"Do NOT bypass", "raw `git merge", "/ms-bead-fix", SkipPanelEnv} {
					if strings.Contains(d.Message, bad) {
						t.Errorf("forbidden %s: %s", bad, d.Message)
					}
				}
				if strings.Count(d.Message, "recovery:") != 1 {
					t.Errorf("repeated recovery: %s", d.Message)
				}
				if regexp.MustCompile(`(?i)ADR-[0-9]+|spec[ -][0-9]+|\bR[0-9]+\.[0-9]+|mindspec-[a-z0-9]+|#[0-9]+`).MatchString(d.Message) {
					t.Errorf("internal reference: %s", d.Message)
				}
			})
		}
	}
}
