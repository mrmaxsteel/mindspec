package approve

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
)

// TestCheckExistingBeadsSafety_ClosedChildProvenanceTable is spec 127
// AC-6: a PURE, hermetic table test over the preflight-resolved
// provenance VALUE (S3-r2-5) — no bd, no git, no identity. Every child
// fixture is constructed directly with its childProvenanceEvidence
// already set; checkExistingBeadsSafety is called with NOTHING else
// stubbed (no seam override anywhere in this file), which is itself the
// proof that the check performs no I/O of its own — a hidden git/bd
// call would need a live seam to avoid panicking on a nil dependency,
// and none exists here.
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

	t.Run("no branch, landed merge NOT found is partial/interrupted", func(t *testing.T) {
		planBranchExistsInFn = func(workdir, branch string) (bool, error) { return false, nil }
		planFindLandedMergeFn = func(workdir, specBranch, beadID string) (*lifecycle.LandedMerge, error) {
			return nil, fmt.Errorf("wrapped: %w", lifecycle.ErrLandedMergeNotFound)
		}
		got := evaluateChildProvenance("/root", "spec/x", "bead-1")
		if got != provenancePartialInterrupted {
			t.Errorf("got %v, want provenancePartialInterrupted", got)
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
