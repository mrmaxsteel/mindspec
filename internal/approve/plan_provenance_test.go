package approve

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// TestCheckExistingBeadsSafety_ClosedChildProvenanceTable is spec 127
// AC-6: a PURE, hermetic table test over the preflight-resolved
// provenance VALUE (S3-r2-5) — no bd, no git, no identity. Every child
// fixture is constructed directly with its childProvenanceEvidence
// already set. THIS test installs no seam override of its own — a
// SIBLING test in this same file (TestResolveChildProvenance_LegsThroughSeams)
// does override planBranchExistsInFn/planFindLandedMergeFn, but for
// evaluateChildProvenance/resolveChildProvenance, functions this test
// never calls. checkExistingBeadsSafety itself declares no seam
// parameter and calls no lifecycle/bd function in its body (checked by
// reading it, not enforced here): a hidden I/O call smuggled into it
// would run against whatever planBranchExistsInFn/planFindLandedMergeFn
// are CURRENTLY bound to — their live production defaults throughout
// this test, since nothing here touches them — not panic on a nil
// dependency (the production seams are never nil).
//
// Legs map onto AC-6 exactly:
//   - (i) completed-work evidence -> preserve, no `bd delete`.
//   - (ii) ambiguous/unavailable evidence (including the zero value, the
//     shape a caller with no root to resolve evidence against — e.g.
//     handleExistingBeads — always produces) -> preserve, no `bd delete`,
//     identically to (i).
//   - (iii) positive partial/interrupted provenance -> deletion hint
//     PERMITTED, provenance named in the message, emitted via the
//     constructor (guard.NewDestructiveCommand — proven by the emitted
//     line actually matching the reviewed floor family, AC-11(b)'s
//     single source of truth).
func TestCheckExistingBeadsSafety_ClosedChildProvenanceTable(t *testing.T) {
	tests := []struct {
		name       string
		prov       childProvenanceEvidence
		wantDelete bool
		wantWords  []string
	}{
		{
			name:       "leg i: completed-work evidence preserves the record",
			prov:       provenanceCompletedWork,
			wantDelete: false,
			wantWords:  []string{"completed work", "preserved"},
		},
		{
			name:       "leg ii: ambiguous evidence preserves the record",
			prov:       provenanceAmbiguous,
			wantDelete: false,
			wantWords:  []string{"ambiguous or could not be resolved", "preserved"},
		},
		{
			name:       "leg ii: unresolved (zero value) preserves identically to ambiguous",
			prov:       provenanceUnresolved,
			wantDelete: false,
			wantWords:  []string{"ambiguous or could not be resolved", "preserved"},
		},
		{
			name:       "leg iii: positive partial/interrupted provenance permits the deletion hint",
			prov:       provenancePartialInterrupted,
			wantDelete: true,
			wantWords:  []string{"positively establishes", "partial", "interrupted"},
		},
	}

	// Exhaustiveness guard: every childProvenanceEvidence value this
	// bead defines must be exercised by name above — a new value added
	// later without a fixture here would otherwise silently fall
	// through untested.
	covered := map[childProvenanceEvidence]bool{}
	for _, tt := range tests {
		covered[tt.prov] = true
	}
	for _, v := range []childProvenanceEvidence{provenanceUnresolved, provenanceCompletedWork, provenanceAmbiguous, provenancePartialInterrupted} {
		if !covered[v] {
			t.Fatalf("childProvenanceEvidence value %d has no fixture in this table", v)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			children := []existingChildBead{{ID: "bead-x", Status: "closed", Provenance: tt.prov}}
			err := checkExistingBeadsSafety(children)
			if err == nil {
				t.Fatal("expected a refusal for a closed child")
			}
			msg := err.Error()

			hasDelete := strings.Contains(msg, "bd delete")
			if hasDelete != tt.wantDelete {
				t.Errorf("bd-delete presence = %v, want %v; msg:\n%s", hasDelete, tt.wantDelete, msg)
			}
			for _, want := range tt.wantWords {
				if !strings.Contains(msg, want) {
					t.Errorf("expected message to contain %q; got:\n%s", want, msg)
				}
			}
			if tt.wantDelete {
				if !strings.Contains(msg, "bd delete bead-x --force") {
					t.Errorf("expected the exact constructor-produced form; got:\n%s", msg)
				}
			} else if strings.Contains(msg, "bd delete") {
				t.Errorf("a preserve refusal must never mention bd delete at all; got:\n%s", msg)
			}
		})
	}
}

// TestCheckExistingBeadsSafety_InProgressUnaffected pins that R3c's
// provenance gating touches ONLY the closed-child leg — the pre-existing
// in_progress refusal (untouched by this bead) still fires unconditionally
// and still names `mindspec complete`, regardless of Provenance (which is
// never even read for a non-closed child).
func TestCheckExistingBeadsSafety_InProgressUnaffected(t *testing.T) {
	children := []existingChildBead{{ID: "bead-y", Status: "in_progress", Provenance: provenancePartialInterrupted}}
	err := checkExistingBeadsSafety(children)
	if err == nil {
		t.Fatal("expected a refusal for an in_progress child")
	}
	msg := err.Error()
	if !strings.Contains(msg, "mindspec complete bead-y") {
		t.Errorf("in_progress refusal must be unchanged; got:\n%s", msg)
	}
	if strings.Contains(msg, "bd delete") {
		t.Errorf("in_progress must never draw a bd-delete hint, even with partial/interrupted Provenance set; got:\n%s", msg)
	}
}

// TestResolveChildProvenance_LegsThroughSeams is AC-4's sibling
// discipline applied to R3c's own resolution half (resolveChildProvenance):
// each of the three provenance legs, forced through the planBranchExistsInFn/
// planFindLandedMergeFn seams (no real git), resolves to the outcome
// childProvenanceEvidence's own doc comment claims.
func TestResolveChildProvenance_LegsThroughSeams(t *testing.T) {
	origExists := planBranchExistsInFn
	origLanded := planFindLandedMergeFn
	t.Cleanup(func() {
		planBranchExistsInFn = origExists
		planFindLandedMergeFn = origLanded
	})

	errBoom := fmt.Errorf("simulated infra failure")

	// Case: branch survives -> ambiguous (never guessed either way),
	// regardless of what FindLandedMerge would say (never even reached).
	t.Run("surviving branch is ambiguous", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return true, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			t.Fatal("FindLandedMerge must not be consulted when the branch survives")
			return nil, nil
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenanceAmbiguous {
			t.Errorf("got %v, want provenanceAmbiguous", got)
		}
	})

	t.Run("branch-existence probe error is ambiguous", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, errBoom }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			t.Fatal("FindLandedMerge must not be consulted when the existence probe errors")
			return nil, nil
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenanceAmbiguous {
			t.Errorf("got %v, want provenanceAmbiguous", got)
		}
	})

	t.Run("no branch, landed merge found is completed work", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			return &lifecycle.LandedMerge{}, nil
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenanceCompletedWork {
			t.Errorf("got %v, want provenanceCompletedWork", got)
		}
	})

	t.Run("no branch, genuinely zero candidate merges is partial/interrupted", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			return nil, fmt.Errorf("wrapped: %w", lifecycle.ErrLandedMergeNoCandidate)
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenancePartialInterrupted {
			t.Errorf("got %v, want provenancePartialInterrupted", got)
		}
	})

	// Bead-5 fix round 1, RULING 1 (G1/O1, both BLOCKING): before the
	// fix, evaluateChildProvenance matched the BROAD
	// errors.Is(landedErr, lifecycle.ErrLandedMergeNotFound) instead of
	// the narrow ErrLandedMergeNoCandidate — which also matches every
	// case below, collapsing them all into the same
	// provenancePartialInterrupted (destructive-licensing) outcome.
	// Restoring that bare errors.Is check reds every one of these.

	t.Run("no branch, uncorroborated owned candidate (LandedMergeNoEvidence) is ambiguous, never partial/interrupted", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			return nil, &lifecycle.LandedMergeNoEvidence{
				BeadID: beadID, SpecBranch: specBranch,
				MergeSHA: "aaaaaaa", SecondParent: "bbbbbbb",
			}
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenanceAmbiguous {
			t.Errorf("got %v, want provenanceAmbiguous — an owned-but-uncorroborated candidate must never license deletion", got)
		}
	})

	t.Run("no branch, owned candidates disagree on second parent (LandedMergeNoEvidence, conflicting) is ambiguous", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			return nil, &lifecycle.LandedMergeNoEvidence{
				BeadID: beadID, SpecBranch: specBranch,
				MergeSHA: "aaaaaaa", SecondParent: "bbbbbbb",
				ConflictingSecondParent: "ccccccc",
			}
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenanceAmbiguous {
			t.Errorf("got %v, want provenanceAmbiguous — genuine ambiguity about which landing is this bead's tip must never license deletion", got)
		}
	})

	t.Run("no branch, a corroboration-leg contradiction is ambiguous, not partial/interrupted", func(t *testing.T) {
		// Mirrors the SHAPE of landed.go's reviewed_head_sha/branch-tip/
		// landed-binding contradiction returns: a plain fmt.Errorf
		// wrapping the BROAD ErrLandedMergeNotFound sentinel (not the
		// narrow ErrLandedMergeNoCandidate) — a real owned candidate
		// exists, but a corroboration datum disagrees with it.
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			return nil, fmt.Errorf("%w: %s on %s (surviving branch tip contradicts merge's second parent)",
				lifecycle.ErrLandedMergeNotFound, beadID, specBranch)
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenanceAmbiguous {
			t.Errorf("got %v, want provenanceAmbiguous — a corroboration contradiction is not a definitive absence", got)
		}
	})

	t.Run("no branch, a positively-identified-then-reverted landing is ambiguous, not partial/interrupted", func(t *testing.T) {
		// Mirrors landed.go's revert-shape return: a positively
		// corroborated candidate whose content is no longer present at
		// the tip. Still a real, owned, once-landed merge — never the
		// zero-candidate absence provenancePartialInterrupted requires.
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			return nil, fmt.Errorf("%w: %s on %s (merge's content is no longer present at the current tip — it was reverted or cleanly removed after landing)",
				lifecycle.ErrLandedMergeNotFound, beadID, specBranch)
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenanceAmbiguous {
			t.Errorf("got %v, want provenanceAmbiguous — a reverted-after-landing signature is not a definitive never-landed absence", got)
		}
	})

	t.Run("no branch, landed-merge lookup errors (not the not-found sentinel) is ambiguous", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			return nil, errBoom
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenanceAmbiguous {
			t.Errorf("got %v, want provenanceAmbiguous", got)
		}
	})

	t.Run("malformed bead id is ambiguous, never derives a branch", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) {
			t.Fatal("must never derive/probe a branch for a malformed id")
			return false, nil
		}
		got := evaluateChildProvenance("/root", "spec/x", "--not-a-valid-id")
		if got != provenanceAmbiguous {
			t.Errorf("got %v, want provenanceAmbiguous", got)
		}
	})
}

// TestResolveChildProvenance_NonClosedLeftAtZeroValue pins that
// resolveChildProvenance never resolves evidence for a non-closed
// child — it stays at provenanceUnresolved, and (proven by never
// wiring the seams here) never even calls the git-touching functions.
func TestResolveChildProvenance_NonClosedLeftAtZeroValue(t *testing.T) {
	origExists := planBranchExistsInFn
	origLanded := planFindLandedMergeFn
	t.Cleanup(func() {
		planBranchExistsInFn = origExists
		planFindLandedMergeFn = origLanded
	})
	planBranchExistsInFn = func(workdir, branch string) (bool, error) {
		t.Fatal("must never probe branch existence for a non-closed child")
		return false, nil
	}
	planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
		t.Fatal("must never look up a landed merge for a non-closed child")
		return nil, nil
	}

	children := []existingChildBead{{ID: "bead-open", Status: "open"}}
	out := resolveChildProvenance("/root", "spec/x", children)
	if len(out) != 1 || out[0].Provenance != provenanceUnresolved {
		t.Fatalf("expected the open child left at provenanceUnresolved, got %+v", out)
	}
}

// TestAdversaryLandedMergeAmbiguityNeverLicensesDeletion is the G1
// adversarial finding from the bead-5 panel (RULING 1, BLOCKING): with
// the bead branch absent and owned candidate merges disagreeing on
// their second parent — a real, ambiguous merge history, not a
// never-landed bead — the end-to-end path from evaluateChildProvenance
// through checkExistingBeadsSafety's rendered refusal must never reach
// `bd delete`. Restoring the pre-fix bare
// errors.Is(landedErr, lifecycle.ErrLandedMergeNotFound) check in
// evaluateChildProvenance reds this test twice: provenance resolves to
// provenancePartialInterrupted, and the rendered refusal ends in
// `bd delete bead-1 --force`.
func TestAdversaryLandedMergeAmbiguityNeverLicensesDeletion(t *testing.T) {
	origExists := planBranchExistsInFn
	origLanded := planFindLandedMergeFn
	t.Cleanup(func() {
		planBranchExistsInFn = origExists
		planFindLandedMergeFn = origLanded
	})

	planBranchExistsInFn = func(string, string) (bool, error) {
		return false, nil
	}
	planFindLandedMergeFn = func(string, string, string) (*lifecycle.LandedMerge, error) {
		return nil, &lifecycle.LandedMergeNoEvidence{
			BeadID: "bead-1", SpecBranch: "spec/x",
			MergeSHA: "aaaaaaaa", SecondParent: "bbbbbbbb",
			ConflictingSecondParent: "cccccccc",
		}
	}

	prov := evaluateChildProvenance("/repo", "spec/x", "bead-1")
	if prov != provenanceAmbiguous {
		t.Fatalf("ambiguous owned merge evidence classified as %v; want provenanceAmbiguous", prov)
	}
	err := checkExistingBeadsSafety([]existingChildBead{{
		ID: "bead-1", Status: "closed", Provenance: prov,
	}})
	if err == nil {
		t.Fatal("expected closed-child refusal")
	}
	if strings.Contains(err.Error(), "bd delete") {
		t.Fatalf("ambiguous owned merge evidence emitted destructive deletion command:\n%s", err)
	}
}

// TestPlanBranchExistsInFnDefaultsToLifecycleBranchExistsIn is O2-1's
// fix: the seam-var doc comment above planBranchExistsInFn/
// planFindLandedMergeFn claims both are "pointer-pinned in
// plan_provenance_test.go" — every OTHER test in this file only
// overrides the seam and restores it via t.Cleanup, which proves
// nothing about the DEFAULT. This test (and its sibling below) is the
// pin the comment was already claiming existed.
func TestPlanBranchExistsInFnDefaultsToLifecycleBranchExistsIn(t *testing.T) {
	if reflect.ValueOf(planBranchExistsInFn).Pointer() != reflect.ValueOf(lifecycle.BranchExistsIn).Pointer() {
		t.Fatal("planBranchExistsInFn must default to lifecycle.BranchExistsIn (spec 127 R3c)")
	}
}

// TestPlanFindLandedMergeFnDefaultsToLifecycleFindLandedMerge is
// TestPlanBranchExistsInFnDefaultsToLifecycleBranchExistsIn's sibling
// pin for the other R3c seam.
func TestPlanFindLandedMergeFnDefaultsToLifecycleFindLandedMerge(t *testing.T) {
	if reflect.ValueOf(planFindLandedMergeFn).Pointer() != reflect.ValueOf(lifecycle.FindLandedMerge).Pointer() {
		t.Fatal("planFindLandedMergeFn must default to lifecycle.FindLandedMerge (spec 127 R3c)")
	}
}
