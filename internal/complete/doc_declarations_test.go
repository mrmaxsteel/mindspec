package complete

import (
	"bytes"
	"errors"
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

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/validate"
)

func s133DocRecoveryFloor(t *testing.T, err error) {
	t.Helper()
	if len(guard.FindFloorMatches("git reset --hard HEAD")) == 0 {
		t.Fatal("floor detector did not fire")
	}
	if err == nil || !guard.HasFinalRecoveryLine(err.Error()) {
		t.Fatalf("missing recovery: %v", err)
	}
	for _, line := range strings.Split(err.Error(), "\n") {
		if strings.HasPrefix(line, "recovery:") && len(guard.FindFloorMatches(strings.TrimPrefix(line, "recovery:"))) > 0 {
			t.Fatalf("destructive recovery: %s", line)
		}
	}
}

func s133DocFixture(t *testing.T, policy, kind string, second bool) (*windowFixture, *bytes.Buffer, map[string]map[string]interface{}) {
	t.Helper()
	f, _ := s133CheckFixture(t, policy, kind)
	if second {
		writeFile(t, f.specWT, ".mindspec/domains/other/OWNERSHIP.yaml", "paths:\n  - internal/other/**\n")
		writeFile(t, f.specWT, ".mindspec/domains/other/interfaces.md", "Other docs.\n")
		writeFile(t, f.specWT, ".mindspec/specs/"+closingSpec+"/spec.md", "# Spec\n\n## Impacted Domains\n\n- widget\n- other\n")
		writeFile(t, f.specWT, ".mindspec/adr/ADR-9002.md", "# ADR-9002: Other\n\n- **Status**: Accepted\n- **Domain(s)**: other\n\n## Decision\nOwn other.\n")
		writeFile(t, f.specWT, ".mindspec/specs/"+closingSpec+"/plan.md", "---\nstatus: Approved\nadr_citations:\n  - ADR-9001\n  - ADR-9002\n---\n")
		gitRun(t, f.specWT, "add", "-A")
		gitRun(t, f.specWT, "commit", "-qm", "second domain")
		writeFile(t, f.beadWT, "internal/other/other.go", "package other\n")
		gitRun(t, f.beadWT, "add", "-A")
		gitRun(t, f.beadWT, "commit", "-qm", "second source")
		f.tip = gateRevParse(t, f.root, "bead/"+closingBead)
	}
	out := new(bytes.Buffer)
	old := warnWriter
	warnWriter = out
	t.Cleanup(func() { warnWriter = old })
	stores := map[string]map[string]interface{}{closingBead: f.meta, "doc-owner": {}, "next-owner": {}, "original-source": {}}
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		if m, ok := stores[id]; ok {
			return m, nil
		}
		return nil, fmt.Errorf("unknown fixture bead %s", id)
	}
	completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
		m, ok := stores[id]
		if !ok {
			return errors.New("unknown fixture bead")
		}
		for k, v := range updates {
			m[k] = v
		}
		return nil
	}
	fetchBeadByIDFn = func(id string) (next.BeadInfo, error) {
		status := "open"
		if id == closingBead && f.closed {
			status = "closed"
		}
		if id == "closed-owner" {
			status = "closed"
		}
		return next.BeadInfo{ID: id, Status: status}, nil
	}
	findEpicForBeadFn = func(id string) (string, string, error) {
		if id == "foreign-owner" {
			return "different-epic", "other-spec", nil
		}
		return "epic-window", closingSpec, nil
	}
	f.registerLedger()
	return f, out, stores
}

func TestS133NoDocImpact_CompletionAndScope(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, tc := range []struct {
			name   string
			values []string
			second bool
			want   string
		}{
			{"declared", []string{"widget: test-only change"}, false, ""},
			{"other-domain-still-fails", []string{"widget: test-only change"}, true, `domain "other"`},
			{"both-declared", []string{"widget: internal change", "other: internal change"}, true, ""},
			{"empty-reason", []string{"widget: "}, false, "non-empty reason"},
			{"untouched", []string{"missing: internal change"}, false, "untouched domain"},
		} {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				f, out, _ := s133DocFixture(t, policy, "doc", tc.second)
				_, err := f.run(CompleteOpts{NoDocImpact: tc.values})
				if tc.want != "" {
					requireWindowRefusal(t, err, tc.want)
					if f.closed {
						t.Fatal("refusal closed bead")
					}
					if !strings.Contains(err.Error(), "--no-doc-impact '"+tc.values[0]+"'") {
						t.Fatalf("lost declaration recovery: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				rows, err := validate.ReadDocRecords[validate.DocDeclaration](f.meta, validate.NoDocImpactKey)
				if err != nil || len(rows) != len(tc.values) {
					t.Fatalf("missing audit: %+v %v", rows, err)
				}
				for _, row := range rows {
					if row.By == "" || row.At == "" || row.Reason == "" || !strings.Contains(out.String(), row.Domain) {
						t.Fatalf("incomplete declaration: %+v %s", row, out)
					}
				}
			})
		}
	}
}

func TestS133NoDocImpact_CheckReportsAllDomainsWithoutWrites(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f, out, stores := s133DocFixture(t, policy, "doc", true)
			before := s133CheckState(t, f)
			records := fmt.Sprint(stores)
			r, err := f.run(CompleteOpts{Check: true, NoDocImpact: []string{"widget: no public effect", "other: no public effect"}})
			if err != nil || r == nil || !r.Check {
				t.Fatalf("check: %+v %v", r, err)
			}
			if before != s133CheckState(t, f) || records != fmt.Sprint(stores) {
				t.Fatal("check changed git or tracker")
			}
			for _, d := range []string{"widget", "other"} {
				if !strings.Contains(out.String(), "no doc impact for "+d) {
					t.Fatalf("domain missing: %s", out)
				}
			}
		})
	}
}

func TestS133DocOwner_CheckAndInvalidOwners(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, check := range []bool{false, true} {
			for _, owner := range []string{"", "closed-owner", closingBead, "foreign-owner", "doc-owner"} {
				name := owner
				if name == "" {
					name = "missing-owner"
				}
				t.Run(fmt.Sprintf("%s/check=%t/%s", policy, check, name), func(t *testing.T) {
					f, out, stores := s133DocFixture(t, policy, "doc", true)
					before := s133CheckState(t, f)
					records := fmt.Sprint(stores)
					r, err := f.run(CompleteOpts{Check: check, AllowDocSkew: "docs follow later", DocOwner: owner})
					if owner != "doc-owner" {
						requireWindowRefusal(t, err, "doc-sync")
						s133DocRecoveryFloor(t, err)
						if before != s133CheckState(t, f) || records != fmt.Sprint(stores) {
							t.Fatal("invalid owner wrote state")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					for _, d := range []string{"widget", "other"} {
						if !strings.Contains(out.String(), "covers "+d) {
							t.Fatalf("domain missing: %s", out)
						}
					}
					if check {
						if r == nil || !r.Check || before != s133CheckState(t, f) || records != fmt.Sprint(stores) {
							t.Fatal("check wrote state")
						}
						return
					}
					source, _ := validate.ReadDocRecords[validate.DocDeferral](stores[closingBead], validate.DocDeferralsKey)
					target, _ := validate.ReadDocRecords[validate.DocDeferral](stores[owner], validate.DocDeferralsKey)
					if len(source) != 2 || !reflect.DeepEqual(source, target) {
						t.Fatalf("both beads must carry each deferral: %+v %+v", source, target)
					}
					for _, row := range source {
						if row.ID == "" || row.Reason != "docs follow later" || row.By == "" || row.At == "" {
							t.Fatalf("incomplete audit: %+v", row)
						}
					}
				})
			}
		}
	}
}

func TestS133DocOwner_OwnerMustUpdateDocsOrTransfer(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, action := range []string{"refuse", "declaration", "update", "transfer"} {
			t.Run(policy+"/"+action, func(t *testing.T) {
				kind := "doc"
				if action == "update" {
					kind = "pass"
				}
				f, _, stores := s133DocFixture(t, policy, kind, false)
				d := validate.DocDeferral{ID: "doc-existing", Domains: []string{"widget"}, Source: "original-source", Owner: closingBead, Reason: "earlier deferral", By: "actor", At: "2026-01-01T00:00:00Z"}
				stores[closingBead][validate.DocDeferralsKey] = []validate.DocDeferral{d}
				stores[d.Source][validate.DocDeferralsKey] = []validate.DocDeferral{d}
				opts := CompleteOpts{}
				if action == "declaration" {
					opts.NoDocImpact = []string{"widget: internal only"}
				}
				if action == "transfer" {
					opts.AllowDocSkew = "new doc owner"
					opts.DocOwner = "next-owner"
				}
				_, err := f.run(opts)
				if action == "refuse" || action == "declaration" {
					requireWindowRefusal(t, err, "owner "+closingBead+" must update docs")
					s133DocRecoveryFloor(t, err)
					if f.closed {
						t.Fatal("closed without docs")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if action == "transfer" {
					for _, id := range []string{closingBead, "original-source", "next-owner"} {
						rows, _ := validate.ReadDocRecords[validate.DocDeferral](stores[id], validate.DocDeferralsKey)
						found := false
						for _, r := range rows {
							if r.ID == d.ID {
								found = true
								if r.Owner != "next-owner" || r.Reason != "new doc owner" {
									t.Fatalf("transfer lost: %+v", r)
								}
							}
						}
						if !found {
							t.Fatalf("missing transferred record on %s", id)
						}
					}
				}
			})
		}
	}
}

func TestS133DocOwner_RecoveryKeepsOwnerAndDeclarations(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f, _, _ := s133DocFixture(t, policy, "both", false)
			opts := CompleteOpts{AllowDocSkew: "docs later", DocOwner: "doc-owner", NoDocImpact: []string{"widget: internal only"}}
			_, err := f.run(opts)
			requireWindowRefusal(t, err, "adr-divergence")
			s133DocRecoveryFloor(t, err)
			for _, value := range []string{"--allow-doc-skew 'docs later'", "--doc-owner 'doc-owner'", "--no-doc-impact 'widget: internal only'"} {
				if !strings.Contains(finalRecoveryCommand(t, err.Error()), value) {
					t.Fatalf("lost flag %s: %v", value, err)
				}
			}
			row := f.merge()
			rows, e := namingMerges(f.ex, "spec/"+closingSpec, closingBead)
			if e != nil || len(rows) != 1 || rows[0].SHA != row {
				t.Fatalf("merge fixture: %v %+v", e, rows)
			}
			merged := mergedRangeFailure(f.root, closingBead, "spec/"+closingSpec, f.specWT, rows[0], f.ex, opts, fmt.Errorf("doc-sync: missing docs"))
			if !strings.Contains(merged.Error(), "--doc-owner 'doc-owner'") {
				t.Fatal(merged)
			}
			s133AssertNoInternalReferences(t, err.Error())
			s133AssertNoInternalReferences(t, merged.Error())
		})
	}
}

func TestS133DocOwner_ProductSurface(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)ADR[- ]?\d+|spec[- #]+\d+|\b(?:AC|R)\d+\b|#\d+|mindspec-qjy8(?:\.\d+)?`)
	if !forbidden.MatchString("see ADR-0031") {
		t.Fatal("citation detector did not fire")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("missing source location")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	for _, name := range []string{"internal/complete/doc_declarations.go", "internal/approve/doc_declarations.go", "internal/validate/doc_declarations.go", "cmd/mindspec/complete.go"} {
		tree, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(tree, func(n ast.Node) bool {
			literal, ok := n.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if forbidden.MatchString(value) {
				t.Errorf("%s product string cites internal work: %s", name, value)
			}
			return true
		})
	}
	data, err := os.ReadFile(filepath.Join(root, ".mindspec/domains/workflow/interfaces.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, added, ok := strings.Cut(string(data), "### Domain doc declarations and owners")
	if !ok {
		t.Fatal("missing domain docs")
	}
	if forbidden.MatchString(added) {
		t.Fatalf("domain docs cite internal work: %s", added)
	}
}

func TestS133NoDocImpact_MaterializationResumeKeepsAudit(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f, _, _ := s133DocFixture(t, policy, "doc", false)
			writeFile(t, f.beadWT, ".beads/issues.jsonl", "{\"id\":\"window-1\",\"status\":\"in_progress\",\"title\":\"updated export\"}\n")
			ex := &killAfterExecutor{Executor: f.ex, t: t, killAdvanceAfterCall: 1}
			opts := CompleteOpts{NoDocImpact: []string{"widget: internal change"}}
			if _, err := Run(f.root, closingBead, closingSpec, "", ex, opts); err == nil || !strings.Contains(err.Error(), "materialization") {
				t.Fatalf("fault did not hit materialization: %v", err)
			}
			if f.meta[validate.NoDocImpactKey] != nil {
				t.Fatal("failed run wrote declaration")
			}
			ex.killAdvanceAfterCall = 0
			if _, err := Run(f.root, closingBead, closingSpec, "", ex, opts); err != nil {
				t.Fatal(err)
			}
			rows, err := validate.ReadDocRecords[validate.DocDeclaration](f.meta, validate.NoDocImpactKey)
			if err != nil || len(rows) != 1 || rows[0].Domain != "widget" {
				t.Fatalf("resume lost audit: %+v %v", rows, err)
			}
		})
	}
}

func TestS133DocOwner_ProductionOwnerValidator(t *testing.T) {
	if reflect.ValueOf(docOwnerScopeFn).Pointer() != reflect.ValueOf(validateDocOwner).Pointer() {
		t.Fatal("owner validation must use the production status and epic check")
	}
}

func TestS133DocOwner_DeclarationDoesNotCreateDeferral(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f, _, stores := s133DocFixture(t, policy, "doc", true)
			if _, err := f.run(CompleteOpts{NoDocImpact: []string{"widget: internal only"}, AllowDocSkew: "other docs later", DocOwner: "doc-owner"}); err != nil {
				t.Fatal(err)
			}
			rows, err := validate.ReadDocRecords[validate.DocDeferral](stores[closingBead], validate.DocDeferralsKey)
			if err != nil || len(rows) != 1 || !reflect.DeepEqual(rows[0].Domains, []string{"other"}) {
				t.Fatalf("declaration created spurious doc debt: %+v %v", rows, err)
			}
		})
	}
}

func TestS133NoDocImpact_TestOnlyOwnedChange(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f, _ := s133PresenceFixture(t, policy)
			writeFile(t, f.root, ".mindspec/config.yaml", "enforcement:\n  panel_presence: "+policy+"\n")
			config.ResetCache()
			writeFile(t, f.specWT, ".mindspec/domains/widget/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")
			writeFile(t, f.specWT, ".mindspec/domains/widget/interfaces.md", "Widget docs.\n")
			writeFile(t, f.specWT, ".mindspec/specs/"+closingSpec+"/plan.md", "---\nstatus: Approved\nadr_citations:\n  - ADR-9001\n---\n")
			writeFile(t, f.specWT, ".mindspec/adr/ADR-9001.md", "# ADR-9001: Widget\n\n- **Status**: Accepted\n- **Domain(s)**: widget\n\n## Decision\nOwn widget.\n")
			gitRun(t, f.specWT, "add", "-A")
			gitRun(t, f.specWT, "commit", "-qm", "test ownership")
			writeFile(t, f.beadWT, "internal/widget/widget_test.go", "package widget\n")
			gitRun(t, f.beadWT, "add", "-A")
			gitRun(t, f.beadWT, "commit", "-qm", "test only change")
			f.registerLedger()
			if _, err := f.run(CompleteOpts{NoDocImpact: []string{"widget: test-only change"}}); err != nil {
				t.Fatal(err)
			}
			rows, err := validate.ReadDocRecords[validate.DocDeclaration](f.meta, validate.NoDocImpactKey)
			if err != nil || len(rows) != 1 || rows[0].Domain != "widget" {
				t.Fatalf("missing test-only declaration: %+v %v", rows, err)
			}
		})
	}
}

func TestS133DocOwner_TransferRetryRepairsEveryCopy(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, failID := range []string{closingBead, "next-owner", "original-source"} {
			t.Run(policy+"/"+failID, func(t *testing.T) {
				_, _, stores := s133DocFixture(t, policy, "doc", false)
				d := validate.DocDeferral{ID: "doc-existing", Domains: []string{"widget"}, Source: "original-source", Owner: closingBead, Reason: "old", By: "actor", At: "now"}
				stores[closingBead][validate.DocDeferralsKey] = []validate.DocDeferral{d}
				stores[d.Source][validate.DocDeferralsKey] = []validate.DocDeferral{d}
				realWrite := completeMergeMetadataFn
				failed := false
				completeMergeMetadataFn = func(id string, updates map[string]interface{}) error {
					if id == failID && !failed {
						failed = true
						return fmt.Errorf("injected tracker failure")
					}
					return realWrite(id, updates)
				}
				opts := CompleteOpts{AllowDocSkew: "transfer", DocOwner: "next-owner", docAudit: &docCompletionAudit{}}
				docs := &validate.Result{DocImpact: &validate.DocImpact{Docs: map[string]bool{}}}
				prepare := func() {
					t.Helper()
					opts.docAudit = &docCompletionAudit{}
					if _, err := prepareDocDeclarations(closingBead, opts, docs, io.Discard); err != nil {
						t.Fatal(err)
					}
				}
				prepare()
				if err := recordDocDeclarations(closingBead, opts); err == nil || !failed {
					t.Fatal("expected injected write failure")
				}
				prepare()
				if err := recordDocDeclarations(closingBead, opts); err != nil {
					t.Fatal(err)
				}
				var want []validate.DocDeferral
				for _, id := range []string{closingBead, "next-owner", "original-source"} {
					rows, err := validate.ReadDocRecords[validate.DocDeferral](stores[id], validate.DocDeferralsKey)
					if err != nil || len(rows) != 1 || rows[0].ID != d.ID || rows[0].Owner != "next-owner" || rows[0].Reason != "transfer" || rows[0].By == "" || rows[0].At == "" {
						t.Fatalf("retry did not repair %s: %+v %v", id, rows, err)
					}
					if want == nil {
						want = rows
					} else if !reflect.DeepEqual(want, rows) {
						t.Fatalf("copies differ on %s: want %+v, got %+v", id, want, rows)
					}
				}
			})
		}
	}
}

func TestS133DocOwner_TransferOnlyMissingDomains(t *testing.T) {
	_, _, stores := s133DocFixture(t, "enforce", "doc", false)
	d := validate.DocDeferral{ID: "doc-existing", Domains: []string{"widget", "other"}, Source: "original-source", Owner: closingBead, Reason: "old", By: "actor", At: "now"}
	stores[closingBead][validate.DocDeferralsKey] = []validate.DocDeferral{d}
	opts := CompleteOpts{AllowDocSkew: "transfer", DocOwner: "next-owner", docAudit: &docCompletionAudit{}}
	docs := &validate.Result{DocImpact: &validate.DocImpact{Docs: map[string]bool{"widget": true}}}
	if _, err := prepareDocDeclarations(closingBead, opts, docs, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(opts.docAudit.deferrals) != 1 || !reflect.DeepEqual(opts.docAudit.deferrals[0].Domains, []string{"other"}) {
		t.Fatalf("must transfer only missing docs: %+v", opts.docAudit.deferrals)
	}
}

func TestS133NoDocImpact_MergeOnlyRecordsLandingBinding(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, merged := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/merged=%t", policy, merged), func(t *testing.T) {
				f, _, stores := s133DocFixture(t, policy, "doc", false)
				if merged {
					f.merge()
				}
				r, err := f.run(CompleteOpts{MergeOnly: true, NoDocImpact: []string{"widget: internal only"}})
				if err != nil || r == nil || !r.MergeOnly || f.closed {
					t.Fatalf("merge-only failed: %+v %v", r, err)
				}
				meta := stores[closingBead]
				if meta["mindspec_landed_merge_sha"] != r.MergeSHA || meta["mindspec_landed_second_parent"] != r.MergeParents[1] {
					t.Fatalf("merge-only lost landing binding: %+v", meta)
				}
				rows, err := validate.ReadDocRecords[validate.DocDeclaration](meta, validate.NoDocImpactKey)
				if err != nil || len(rows) != 1 || rows[0].Domain != "widget" {
					t.Fatalf("merge-only lost declaration: %+v %v", rows, err)
				}
			})
		}
	}
}

func TestS133NoDocImpact_ReconcileRecordsLandingBinding(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f, _, stores := s133DocFixture(t, policy, "doc", false)
			merge := f.merge()
			gitRun(t, f.root, "worktree", "remove", "--force", f.beadWT)
			gitRun(t, f.root, "branch", "-D", "bead/"+closingBead)
			mergedUnclosedFn = lifecycle.MergedUnclosed
			r, err := f.run(CompleteOpts{NoDocImpact: []string{"widget: internal only"}})
			if err != nil || r == nil || !r.BeadClosed || !f.closed {
				t.Fatalf("reconcile failed: %+v %v", r, err)
			}
			meta := stores[closingBead]
			if meta["mindspec_landed_merge_sha"] != merge || meta["mindspec_landed_second_parent"] != f.tip {
				t.Fatalf("reconcile lost landing binding: %+v", meta)
			}
			rows, err := validate.ReadDocRecords[validate.DocDeclaration](meta, validate.NoDocImpactKey)
			if err != nil || len(rows) != 1 || rows[0].Domain != "widget" {
				t.Fatalf("reconcile lost declaration: %+v %v", rows, err)
			}
		})
	}
}
