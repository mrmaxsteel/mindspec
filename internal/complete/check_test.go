package complete

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

func s133CheckFixture(t *testing.T, policy, kind string) (*windowFixture, *bytes.Buffer) {
	t.Helper()
	f, out := s133PresenceFixture(t, policy)
	writeFile(t, f.specWT, ".mindspec/domains/widget/OWNERSHIP.yaml", "paths:\n  - feature.txt\n  - internal/widget/**\n")
	writeFile(t, f.specWT, ".mindspec/domains/widget/architecture.md", "Widget documentation.\n")
	writeFile(t, f.specWT, ".mindspec/adr/ADR-9001.md", "# ADR-9001: Widget\n\n- **Status**: Accepted\n- **Domain(s)**: widget\n\n## Decision\nOwn widget.\n")
	citations := "  - ADR-9001\n"
	if kind == "adr" || kind == "all" || kind == "both" {
		citations = "  []\n"
	}
	writeFile(t, f.specWT, ".mindspec/specs/"+closingSpec+"/plan.md", "---\nstatus: Approved\nadr_citations:\n"+citations+"---\n")
	gitRun(t, f.specWT, "add", "-A")
	gitRun(t, f.specWT, "commit", "-qm", "content gate context")
	writeFile(t, f.beadWT, "internal/widget/widget.go", "package widget\n\nconst Value = 1\n")
	if kind != "doc" && kind != "all" && kind != "both" {
		writeFile(t, f.beadWT, ".mindspec/domains/widget/interfaces.md", "Widget documentation with the change.\n")
	}
	gitRun(t, f.beadWT, "add", "-A")
	gitRun(t, f.beadWT, "commit", "-qm", "content under test")
	f.tip = gateRevParse(t, f.root, "bead/"+closingBead)
	// Local fake isolates report aggregation from the work-destruction evaluator.
	completeWorkDestructionPreflightFn = func(root, branch, target, reason, rerun string) error {
		if kind == "destruction" || kind == "all" {
			return guard.NewFailure("work-destruction preflight: stale deletion", "inspect the branch, then re-run "+rerun)
		}
		return nil
	}
	return f, out
}

func s133CheckState(t *testing.T, f *windowFixture) string {
	t.Helper()
	var b strings.Builder
	for _, dir := range []string{f.root, f.specWT, f.beadWT} {
		fmt.Fprintln(&b, gitStatusPorcelain(t, dir), gateRevParse(t, dir, "HEAD"))
		data, err := os.ReadFile(filepath.Join(dir, ".git", "index"))
		if err != nil { // Linked worktrees keep the index beside their gitdir file.
			gitdir, err := os.ReadFile(filepath.Join(dir, ".git"))
			if err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(filepath.Join(strings.TrimSpace(strings.TrimPrefix(string(gitdir), "gitdir:")), "index"))
			if err != nil {
				t.Fatal(err)
			}
		}
		fmt.Fprintf(&b, "index:%x\n", data)
	}
	data, err := json.Marshal(f.meta)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&b, "closed:%v closes:%d metadata:%s", f.closed, f.closeCalls, data)
	return b.String()
}

func s133GateSet(err error) []string {
	var set []string
	if err == nil {
		return set
	}
	for _, gate := range []string{"doc-sync:", "adr-divergence:", "work-destruction preflight:"} {
		if strings.Contains(err.Error(), gate) {
			set = append(set, gate)
		}
	}
	return set
}

func TestS133CompleteCheck_ReportParity(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, kind := range []string{"pass", "doc", "adr", "destruction", "all"} {
			t.Run(policy+"/"+kind, func(t *testing.T) {
				f, out := s133CheckFixture(t, policy, kind)
				before := s133CheckState(t, f)
				r, checkErr := f.run(CompleteOpts{Check: true})
				if s133CheckState(t, f) != before {
					t.Fatal("--check changed git, index, status or metadata")
				}
				if strings.Contains(out.String(), "gate bead") || f.meta["panel_absent"] != nil {
					t.Fatalf("check ran panel presence: %s %v", out, f.meta)
				}
				want := map[string][]string{"pass": nil, "doc": {"doc-sync:"}, "adr": {"adr-divergence:"}, "destruction": {"work-destruction preflight:"}, "all": {"doc-sync:", "adr-divergence:", "work-destruction preflight:"}}[kind]
				if !reflect.DeepEqual(s133GateSet(checkErr), want) {
					t.Fatalf("check gates = %v want %v: %v", s133GateSet(checkErr), want, checkErr)
				}
				if (checkErr == nil) != (kind == "pass") || (checkErr == nil && (r == nil || !r.Check)) {
					t.Fatalf("check outcome: %+v %v", r, checkErr)
				}
				// Only plain completion gets a passing panel.
				f.registerLedger()
				_, plainErr := f.run(CompleteOpts{})
				if !reflect.DeepEqual(s133GateSet(plainErr), want) || (plainErr == nil) != (checkErr == nil) {
					t.Fatalf("plain/check differ: plain=%v check=%v", plainErr, checkErr)
				}
				if kind != "pass" && f.closed {
					t.Fatal("content refusal closed bead")
				}
			})
		}
	}
}

func TestS133CompleteCheck_ReadOnlyAndPanelControl(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, kind := range []string{"pass", "both", "closed"} {
			t.Run(policy+"/"+kind, func(t *testing.T) {
				f, out := s133CheckFixture(t, policy, kind)
				if kind == "closed" {
					f.closed = true
				}
				// A foreign marker must not be cleared by this read-only command.
				if err := f.ex.PublishStageMarker(closingBead, f.tip, ""); err != nil {
					t.Fatal(err)
				}
				marker, stage, present, err := f.ex.ReadStageMarker(closingBead)
				if err != nil || !present {
					t.Fatalf("marker setup: %v", err)
				}
				before := s133CheckState(t, f)
				r, err := f.run(CompleteOpts{Check: true})
				if before != s133CheckState(t, f) {
					t.Fatal("check wrote state")
				}
				after, afterStage, afterPresent, readErr := f.ex.ReadStageMarker(closingBead)
				if readErr != nil || after != marker || stage != afterStage || present != afterPresent {
					t.Fatal("check changed stage marker")
				}
				if kind == "both" && (err == nil || len(s133GateSet(err)) != 2) {
					t.Fatalf("must name doc and ADR failures: %v", err)
				}
				if kind == "closed" && (err == nil || !strings.Contains(err.Error(), "closed")) {
					t.Fatalf("closed state must refuse: %v", err)
				}
				if kind == "pass" && (err != nil || r == nil || !r.Check) {
					t.Fatalf("passing check: %+v %v", r, err)
				}
				if out.Len() != 0 {
					t.Fatalf("check printed panel advisory: %s", out)
				}
				if policy == "enforce" && kind == "pass" {
					_, err = f.run(CompleteOpts{})
					if err == nil || !strings.Contains(err.Error(), "gate bead") {
						t.Fatalf("plain complete must enforce missing panel: %v", err)
					}
				}
			})
		}
	}
}

func TestS133CompleteCheck_RefusedFlagsAndStates(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, flag := range []string{"merge-only", "resolve-merge", "override-adr", "supersede-adr", "allow-net-deletion", "pending", "branchless"} {
			t.Run(policy+"/"+flag, func(t *testing.T) {
				f, _ := s133CheckFixture(t, policy, "pass")
				opts := CompleteOpts{Check: true}
				switch flag {
				case "merge-only":
					opts.MergeOnly = true
				case "resolve-merge":
					opts.ResolveMerge = true
				case "override-adr":
					opts.OverrideADR = "reason"
				case "supersede-adr":
					opts.SupersedeADR = "ADR-9002"
				case "allow-net-deletion":
					opts.AllowNetDeletion = "reason"
				case "pending":
					f.meta[mergeOnlyPendingKey] = "pending"
				case "branchless":
					gitRun(t, f.beadWT, "checkout", "--detach")
					gitRun(t, f.root, "branch", "-D", "bead/"+closingBead)
				}
				before := s133CheckState(t, f)
				_, err := f.run(opts)
				if err == nil {
					t.Fatal("check accepted incompatible flags/state")
				}
				if before != s133CheckState(t, f) {
					t.Fatal("refusal changed state")
				}
			})
		}
	}
}

func TestS133CompleteCheck_ClosingAndCommittedPaths(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, path := range []string{"closing", "committed"} {
			t.Run(policy+"/"+path, func(t *testing.T) {
				f, _ := s133CheckFixture(t, policy, "pass")
				if path == "closing" {
					f.merge()
				} else {
					worktreeListFn = func() ([]bead.WorktreeListEntry, error) { return nil, nil }
				}
				before := s133CheckState(t, f)
				r, err := f.run(CompleteOpts{Check: true})
				if err != nil || r == nil || !r.Check {
					t.Fatalf("check path: %+v %v", r, err)
				}
				if before != s133CheckState(t, f) {
					t.Fatal("check changed state")
				}
			})
		}
	}
}

func TestS133CompleteCheck_UnownedCompletionCases(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, kind := range []string{"unowned", "undeclared", "attribute", "uncovered"} {
			t.Run(policy+"/"+kind, func(t *testing.T) {
				f, _ := s133CheckFixture(t, policy, "pass")
				switch kind {
				case "unowned":
					writeFile(t, f.beadWT, "CHANGELOG.md", "Release notes.\n")
				case "undeclared":
					writeFile(t, f.specWT, ".mindspec/domains/other/OWNERSHIP.yaml", "paths:\n  - internal/other/**\n")
					writeFile(t, f.beadWT, "internal/other/file.go", "package other\n")
				case "attribute":
					writeFile(t, f.specWT, ".mindspec/domains/widget/OWNERSHIP.yaml", "paths: [ broken\n")
				case "uncovered":
					writeFile(t, f.specWT, ".mindspec/specs/"+closingSpec+"/plan.md", "---\nstatus: Approved\nadr_citations: []\n---\n")
				}
				if kind != "unowned" {
					gitRun(t, f.specWT, "add", "-A")
					gitRun(t, f.specWT, "commit", "-qm", "gate case")
				}
				gitRun(t, f.beadWT, "add", "-A")
				gitRun(t, f.beadWT, "commit", "--allow-empty", "-qm", "case content")
				f.tip = gateRevParse(t, f.root, "bead/"+closingBead)
				f.registerLedger()
				old := warnWriter
				warnings := new(bytes.Buffer)
				warnWriter = warnings
				t.Cleanup(func() { warnWriter = old })
				_, err := f.run(CompleteOpts{})
				if kind == "unowned" {
					if err != nil || !f.closed {
						t.Fatalf("unowned completion must pass: %v", err)
					}
					if strings.Count(warnings.String(), "adr-divergence-unowned") != 1 || !strings.Contains(warnings.String(), "CHANGELOG.md") {
						t.Fatalf("need one unowned warning: %s", warnings)
					}
				} else {
					name := map[string]string{"undeclared": "adr-divergence-unowned", "attribute": "adr-divergence-attribute", "uncovered": "adr-divergence-uncovered"}[kind]
					if err == nil || !strings.Contains(err.Error(), name) || f.closed {
						t.Fatalf("%s must refuse with %s: %v", kind, name, err)
					}
					if kind == "undeclared" && !strings.Contains(err.Error(), "not in the spec") {
						t.Fatalf("need scope Error: %v", err)
					}
				}
			})
		}
	}
}

func TestS133CompleteCheck_RecoveryOperands(t *testing.T) {
	if len(guard.FindFloorMatches("git reset --hard HEAD")) == 0 {
		t.Fatal("floor detector did not fire")
	}
	for _, id := range []string{"bead-1", "hostile;id\ngit reset --hard HEAD"} {
		docs := &validate.Result{}
		docs.AddError("internal-docs", "domain docs missing")
		err := reportContentGates("", id, CompleteOpts{}, docs, &validate.Result{}, nil, io.Discard)
		if err == nil || !guard.HasFinalRecoveryLine(err.Error()) {
			t.Fatalf("missing recovery: %v", err)
		}
		for _, line := range strings.Split(err.Error(), "\n") {
			if strings.HasPrefix(line, "recovery:") && len(guard.FindFloorMatches(line)) != 0 {
				t.Fatalf("unsafe recovery: %q", line)
			}
		}
	}
}

func TestS133CompleteCheck_SurfaceHasNoInternalCitations(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)ADR[- ]?\d+|spec[- #]+\d+|\b(?:AC|R)\d+\b|#\d+|mindspec-qjy8(?:\.\d+)?`)
	if !forbidden.MatchString("see ADR-0041") {
		t.Fatal("reference detector did not fire")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("missing source location")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	for _, path := range []string{"internal/complete/check.go", "cmd/mindspec/complete.go", "internal/complete/complete.go", "internal/complete/materialize.go", "internal/validate/divergence.go"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			literal, ok := n.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			message, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if forbidden.MatchString(message) {
				t.Errorf("%s product string cites internal work: %s", path, message)
			}
			return true
		})
	}
}
