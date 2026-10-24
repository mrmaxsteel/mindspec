package complete

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func s133PresenceFixture(t *testing.T, policy string) (*windowFixture, *bytes.Buffer) {
	t.Helper()
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	t.Setenv(panel.SkipPanelEnv, "")
	f := newWindowFixture(t)
	writeFile(t, f.root, ".mindspec/config.yaml", "source_globs:\n  - internal/**/*.go\nenforcement:\n  panel_presence: "+policy+"\n")
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	out := new(bytes.Buffer)
	orig := panelAdvisoryOut
	panelAdvisoryOut = out
	t.Cleanup(func() { panelAdvisoryOut = orig })
	return f, out
}

func s133Closed(t *testing.T, f *windowFixture) {
	t.Helper()
	f.closed = true
	stubClosedWindowChild(t)
}

func s133AssertNoInternalReferences(t *testing.T, message string) {
	t.Helper()
	// Operand values are allowed. The message must not cite internal work.
	message = strings.ReplaceAll(strings.ReplaceAll(message, closingSpec, "<spec-id>"), closingBead, "<bead-id>")
	forbidden := regexp.MustCompile(`(?i)ADR[- ]?\d+|spec[- #]+\d+|\b(?:AC|R)\d+\b|#\d+|mindspec-qjy8(?:\.\d+)?`)
	if forbidden.MatchString(message) {
		t.Fatalf("product message cites internal work: %s", message)
	}
}

func s133AssertAbsentAudit(t *testing.T, f *windowFixture) {
	t.Helper()
	if f.meta["panel_absent"] != true || f.meta["panel_absent_gate"] != "bead" {
		t.Fatalf("missing bead absence audit: %v", f.meta)
	}
	stamp, ok := f.meta["panel_absent_at"].(string)
	if _, err := time.Parse(time.RFC3339, stamp); !ok || err != nil {
		t.Fatalf("absence audit needs a time: %v", f.meta)
	}
}

func TestS133BeadPresence_CompletionPaths(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, path := range []struct {
			name                      string
			closed, merged, mergeOnly bool
			pending, resume           bool
		}{
			{name: "open_plain"},
			{name: "open_merge_only", mergeOnly: true},
			{name: "open_pending_merge_only", pending: true},
			{name: "open_closing", merged: true},
			{name: "open_merged_merge_only", merged: true, mergeOnly: true},
			{name: "closed_naming_merge", merged: true, closed: true},
			{name: "closed_unmerged", closed: true},
			{name: "closed_unmerged_resume", closed: true, resume: true},
		} {
			t.Run(policy+"/"+path.name, func(t *testing.T) {
				f, out := s133PresenceFixture(t, policy)
				if path.merged {
					f.merge()
				}
				if path.closed {
					s133Closed(t, f)
				}
				if path.pending {
					f.meta[mergeOnlyPendingKey] = "pending"
				}
				if path.resume {
					gitRun(t, f.beadWT, "commit", "--allow-empty", "-qm", "artifact snapshot\n\nMindspec-Stage: artifact-sync")
					f.tip = gateRevParse(t, f.root, "bead/"+closingBead)
					if err := f.ex.PublishStageMarker(closingBead, f.tip, ""); err != nil {
						t.Fatal(err)
					}
					f.meta[stageMarkerAttestationKey] = f.tip
				}
				before := gateRevParse(t, f.root, "spec/"+closingSpec)
				beforeMeta := fmt.Sprint(f.meta)
				steps := 0
				orig := closingStepHook
				closingStepHook = func(int) error { steps++; return nil }
				t.Cleanup(func() { closingStepHook = orig })
				var result *Result
				var err error
				captureStdout(t, func() { result, err = f.run(CompleteOpts{MergeOnly: path.mergeOnly}) })
				if policy == "enforce" {
					if err == nil || result != nil || !strings.Contains(err.Error(), "gate bead") || !strings.Contains(err.Error(), "no registration") {
						t.Fatalf("missing registration must refuse: result=%+v err=%v", result, err)
					}
					s133AssertNoInternalReferences(t, err.Error())
					if path.closed {
						want := "\nrecovery: run a panel for the bead at its tip\nrecovery: mindspec complete " + closingBead
						if !strings.HasSuffix(err.Error(), want) || strings.Count(err.Error(), "\nrecovery:") != 2 {
							t.Fatalf("closed refusal needs both recovery lines: %v", err)
						}
					} else if !strings.Contains(err.Error(), "--gate bead --target bead/"+closingBead) {
						t.Fatalf("open refusal needs panel creation recovery: %v", err)
					}
					if f.closed != path.closed || f.closeCalls != 0 || steps != 0 || fmt.Sprint(f.meta) != beforeMeta {
						t.Fatalf("refusal changed status or ran later steps: closed=%v closes=%d steps=%d meta=%v", f.closed, f.closeCalls, steps, f.meta)
					}
					if gateRevParse(t, f.root, "spec/"+closingSpec) != before || gateRevParse(t, f.root, "bead/"+closingBead) != f.tip {
						t.Fatal("refusal moved a branch tip")
					}
					if _, err := os.Stat(f.beadWT); err != nil {
						t.Fatalf("refusal removed worktree: %v", err)
					}
					if out.Len() != 0 {
						t.Fatalf("enforcement printed a warning: %s", out)
					}
					return
				}
				if err != nil || result == nil {
					t.Fatalf("warn must complete: result=%+v err=%v", result, err)
				}
				mergeOnly := path.mergeOnly || path.pending
				if f.closed == mergeOnly || result.MergeOnly != mergeOnly || result.BeadClosed == mergeOnly {
					t.Fatalf("warn changed completion semantics: closed=%v result=%+v", f.closed, result)
				}
				if path.closed && f.closeCalls != 0 {
					t.Fatal("closed bead was closed again")
				}
				lines := strings.Split(strings.TrimSpace(out.String()), "\n")
				want := "warning: gate bead panel <slug>: no registration; recovery: mindspec panel create <slug> --spec " + closingSpec + " --gate bead --target bead/" + closingBead
				if len(lines) != 1 || lines[0] != want {
					t.Fatalf("want exactly one absence warning %q, got %q", want, out.String())
				}
				s133AssertNoInternalReferences(t, out.String())
				s133AssertAbsentAudit(t, f)
				rows, err := namingMerges(f.ex, "spec/"+closingSpec, closingBead)
				if err != nil || len(rows) != 1 {
					t.Fatalf("warn did not preserve or make the naming merge: %v %v", rows, err)
				}
			})
		}
	}
}

func TestS133BeadPresence_Path4CleanupGuard(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f, out := s133PresenceFixture(t, policy)
			before := f.merge()
			s133Closed(t, f)
			if err := os.Rename(filepath.Join(f.root, ".mindspec", "specs"), filepath.Join(f.root, ".mindspec", "saved-specs")); err != nil {
				t.Fatal(err)
			}
			makeCanonicalSpecDir(t, f.root, closingSpec)
			gitRun(t, f.root, "worktree", "remove", f.beadWT)
			steps := 0
			orig := closingStepHook
			closingStepHook = func(int) error { steps++; return nil }
			t.Cleanup(func() { closingStepHook = orig })
			_, err := f.run(CompleteOpts{})
			requireWindowRefusal(t, err, "no panel registration survives")
			if !f.closed || f.closeCalls != 0 || steps != 0 || len(f.meta) != 0 || out.Len() != 0 || gateRevParse(t, f.root, "spec/"+closingSpec) != before {
				t.Fatalf("cleanup guard was weakened: closed=%v closes=%d steps=%d metadata=%v out=%q", f.closed, f.closeCalls, steps, f.meta, out.String())
			}
		})
	}
}

func TestS133BeadPresence_BranchlessAndBusyUnchanged(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, path := range []string{"branchless", "busy"} {
			t.Run(policy+"/"+path, func(t *testing.T) {
				f, out := s133PresenceFixture(t, policy)
				m := f.merge()
				if path == "branchless" {
					gitRun(t, f.root, "worktree", "remove", f.beadWT)
					gitRun(t, f.root, "branch", "-D", "bead/"+closingBead)
					f.meta["mindspec_landed_merge_sha"] = m
					f.meta["mindspec_landed_second_parent"] = f.tip
					mergedUnclosedFn = func(string, string, string) (*lifecycle.LandedMerge, bool, error) {
						return &lifecycle.LandedMerge{SHA: m, FirstParent: f.parent, SecondParent: f.tip}, true, nil
					}
					r, err := f.run(CompleteOpts{})
					if err != nil || !r.BeadClosed || f.meta["mindspec_reconcile_landed_merge_sha"] != m {
						t.Fatalf("branchless reconcile changed: %+v %v %v", r, err, f.meta)
					}
				} else {
					writeFile(t, f.specWT, "busy.txt", "busy\n")
					gitRun(t, f.specWT, "add", "busy.txt")
					_, err := f.run(CompleteOpts{})
					requireWindowRefusal(t, err, "spec worktree is busy")
					if f.closed || f.closeCalls != 0 || len(f.meta) != 0 || gateRevParse(t, f.root, "spec/"+closingSpec) != m {
						t.Fatal("busy refusal changed bead state")
					}
				}
				if out.Len() != 0 || f.meta["panel_absent"] != nil {
					t.Fatalf("presence ran on an excluded path: output=%q meta=%v", out.String(), f.meta)
				}
			})
		}
	}
}

func TestS133BeadPresence_HatchesAndRegisteredPanel(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, gate := range []string{"skip", "disabled", "passing", "stale", "abandoned"} {
			t.Run(policy+"/"+gate, func(t *testing.T) {
				f, out := s133PresenceFixture(t, policy)
				switch gate {
				case "skip":
					t.Setenv(panel.SkipPanelEnv, "1")
				case "disabled":
					writeFile(t, f.root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\n  panel_presence: "+policy+"\n")
					config.ResetCache()
				case "passing":
					f.registerLedger()
				case "stale", "abandoned":
					p := panel.Panel{BeadID: windowBeadPointer(), Spec: closingSpec, Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: f.parent}
					p.Abandoned = gate == "abandoned"
					p.AbandonReason = "review replaced"
					specDir, _ := workspace.SpecDir(f.root, closingSpec)
					writeCoLocatedPanel(t, specDir, "presence-panel", p, approveVerdicts(2))
				}
				before := gateRevParse(t, f.root, "spec/"+closingSpec)
				_, err := f.run(CompleteOpts{})
				if gate == "stale" {
					requireWindowRefusal(t, err, "commits landed after review")
					if f.closed || f.closeCalls != 0 || gateRevParse(t, f.root, "spec/"+closingSpec) != before {
						t.Fatal("warn weakened a failing registration")
					}
				} else if err != nil || !f.closed {
					t.Fatalf("hatch or passing panel must complete: closed=%v err=%v", f.closed, err)
				}
				if strings.Contains(out.String(), "no registration") || f.meta["panel_absent"] != nil {
					t.Fatalf("presence applied to a hatch or a real registration: output=%q meta=%v", out.String(), f.meta)
				}
			})
		}
	}
}

func TestS133BeadPresence_NoPanelRefutationObligation(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f, _ := s133PresenceFixture(t, policy)
			t.Setenv(panel.SkipPanelEnv, "1")
			pending := []refutationPendingEntry{{Slot: "R1", Round: 1, Reason: "verified locally", Evidence: "fixture"}}
			f.meta["refutation_pending_entries"] = pending
			_, err := f.run(CompleteOpts{})
			if err != nil || !f.closed {
				t.Fatalf("hatch must retain obligation settlement: closed=%v err=%v", f.closed, err)
			}
			audits, err := decodeRefutationAudits(f.meta["panel_refuted_entries"])
			if err != nil || !reflect.DeepEqual(audits, pending) || f.meta["panel_absent"] != nil {
				t.Fatalf("no-panel obligation was lost: audits=%+v err=%v meta=%v", audits, err, f.meta)
			}
		})
	}
}

func TestS133BeadPresence_ProductMessages(t *testing.T) {
	saveAndRestore(t)
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			var out bytes.Buffer
			p := &beadPanelPresence{policy: policy, specID: "example"}
			err := p.check("sample-1", true, &out)
			if policy == "enforce" {
				if err == nil {
					t.Fatal("missing enforced presence refusal")
				}
				s133AssertNoInternalReferences(t, err.Error())
				s133AssertNoInternalReferences(t, closedBeadPanelFailure("sample-1", true, err).Error())
			} else if err != nil || out.Len() == 0 {
				t.Fatalf("missing absence warning: %v %q", err, out.String())
			}
			s133AssertNoInternalReferences(t, out.String())
		})
	}
	var out bytes.Buffer
	completeMergeMetadataFn = func(string, map[string]interface{}) error { return errors.New("store unavailable") }
	(&beadPanelPresence{absent: true}).audit("sample-1", &out)
	if !strings.Contains(out.String(), "could not record panel_absent") {
		t.Fatal("missing audit write warning")
	}
	s133AssertNoInternalReferences(t, out.String())
}
