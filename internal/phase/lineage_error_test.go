package phase

// Spec 119 final-review finding A: FindEpicForBead(WithCache)'s error
// contract. A genuinely-no-lineage result (every lookup succeeded; the bead
// has no discoverable epic lineage) wraps the typed ErrNoEpicLineage
// sentinel; every REAL lookup failure — the bead `bd show`, its JSON
// decode and the parent-epic `bd show` —
// propagates as a plain error, never reclassified as "no lineage".
// `mindspec complete` fails closed on the plain-error class and falls back
// to legacy cwd resolution ONLY on the sentinel class.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// stubBeadShow installs runBDFn/listJSONFn stubs for one FindEpicForBead
// scenario. showResponses maps a shown ID ("bead-1", "epic-x") to its JSON
// payload or an error. epicListJSON and epicListErr drive the scoped epic scan.
func stubBeadShow(t *testing.T, showResponses map[string]string, showErrs map[string]error, epicListJSON string, epicListErr error) {
	t.Helper()
	restoreRun := SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			id := args[1]
			if err, ok := showErrs[id]; ok {
				return nil, err
			}
			if payload, ok := showResponses[id]; ok {
				return []byte(payload), nil
			}
			return nil, fmt.Errorf("unexpected bd show %s", id)
		}
		return nil, fmt.Errorf("unexpected bd invocation %v", args)
	})
	t.Cleanup(restoreRun)
	restoreList := SetListJSONForTest(func(args ...string) ([]byte, error) {
		if epicListErr != nil {
			return nil, epicListErr
		}
		return []byte(epicListJSON), nil
	})
	t.Cleanup(restoreList)
}

// A failed bead `bd show` is a real error, not a no-lineage result.
func TestFindEpicForBead_ShowErrorIsRealError(t *testing.T) {
	stubBeadShow(t, nil, map[string]error{"bead-1": errors.New("dolt lock contention")}, "[]", nil)

	_, _, err := FindEpicForBead("bead-1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrNoEpicLineage) {
		t.Errorf("a bd show failure must NOT read as ErrNoEpicLineage, got: %v", err)
	}
}

// A successful show with zero items ("no such bead") IS a genuine
// no-lineage result: the sentinel, so legacy fallback stays legitimate.
func TestFindEpicForBead_BeadNotFoundIsNoLineage(t *testing.T) {
	stubBeadShow(t, map[string]string{"bead-1": "[]"}, nil, "[]", nil)

	_, _, err := FindEpicForBead("bead-1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrNoEpicLineage) {
		t.Errorf("a definitively absent bead must wrap ErrNoEpicLineage, got: %v", err)
	}
}

// The parent-epic lookup failing (cache.FindEpic → bd show epic-x) must
// PROPAGATE — this was the pre-final-review inner swallow that let a
// transient bd failure masquerade as an epic-less bead.
func TestFindEpicForBead_DependentEpicLookupErrorPropagates(t *testing.T) {
	beadJSON := `[{"title":"[119] some bead","dependencies":[{"id":"epic-x","issue_type":"epic","dependency_type":"parent-child"}]}]`
	lookupErr := errors.New("connection refused")
	stubBeadShow(t,
		map[string]string{"bead-1": beadJSON},
		map[string]error{"epic-x": lookupErr},
		"[]", nil)

	_, _, err := FindEpicForBead("bead-1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrNoEpicLineage) {
		t.Errorf("a parent-epic lookup failure must NOT read as ErrNoEpicLineage, got: %v", err)
	}
	if !errors.Is(err, lookupErr) {
		t.Fatalf("parent lookup cause lost: %v", err)
	}
	if !strings.Contains(err.Error(), "epic-x") {
		t.Errorf("error should name the failing epic, got: %v", err)
	}
}

// Every lookup succeeded, nothing matched → the genuine no-lineage sentinel.
func TestFindEpicForBead_NoMatchIsNoLineage(t *testing.T) {
	beadJSON := `[{"title":"untitled bead","dependencies":[]}]`
	stubBeadShow(t, map[string]string{"bead-1": beadJSON}, nil, "[]", nil)

	_, _, err := FindEpicForBead("bead-1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrNoEpicLineage) {
		t.Errorf("a fully-searched epic-less bead must wrap ErrNoEpicLineage, got: %v", err)
	}
}

// The happy path is unchanged: an epic-typed dependency with spec metadata
// resolves the lineage.
func TestFindEpicForBead_ResolvesViaEpicDependency(t *testing.T) {
	beadJSON := `[{"title":"[119] some bead","dependencies":[{"id":"epic-x","issue_type":"epic","dependency_type":"parent-child"}]}]`
	epicJSON := `[{"id":"epic-x","title":"[SPEC 119-test] epic","status":"open","issue_type":"epic","dependency_type":"parent-child","metadata":{"spec_num":119,"spec_title":"test"}}]`
	stubBeadShow(t, map[string]string{"bead-1": beadJSON, "epic-x": epicJSON}, nil, "[]", nil)

	epicID, specID, err := FindEpicForBead("bead-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if epicID != "epic-x" || specID != "119-test" {
		t.Errorf("got (%q, %q), want (epic-x, 119-test)", epicID, specID)
	}
}

func TestFindEpicForBead_AllEpicScanFailureIsRealError(t *testing.T) {
	cause := errors.New("all-epic read failed")
	stubBeadShow(t, map[string]string{
		"bead-1": `[{"dependencies":[{"id":"epic-x","issue_type":"epic","dependency_type":"parent-child"}]}]`,
		"epic-x": `[{"id":"epic-x","metadata":{"spec_id":"119-test"}}]`,
	}, nil, "", cause)
	epic, spec, err := FindEpicForBead("bead-1")
	if epic != "epic-x" || spec != "119-test" || !errors.Is(err, cause) || errors.Is(err, ErrNoEpicLineage) {
		t.Fatalf("all-epic scan must preserve cause and refuse fallback: %s %s %v", epic, spec, err)
	}
}

func TestFindEpicForBead_DuplicateBindingIsRealError(t *testing.T) {
	stubBeadShow(t, map[string]string{
		"bead-1": `[{"dependencies":[{"id":"epic-x","issue_type":"epic","dependency_type":"parent-child"}]}]`,
		"epic-x": `[{"id":"epic-x","metadata":{"spec_id":"119-test"}}]`,
	}, nil, `[{"id":"epic-x","metadata":{"spec_id":"119-test"}},{"id":"epic-y","status":"closed","metadata":{"spec_id":"119-test"}}]`, nil)
	epic, spec, _, err := FindEpicLineageWithCache(NewCache(), "bead-1")
	if epic != "epic-x" || spec != "119-test" || err == nil || errors.Is(err, ErrNoEpicLineage) || !strings.Contains(err.Error(), "epic-x, epic-y") {
		t.Fatalf("duplicate binding must refuse fallback: %s %s %v", epic, spec, err)
	}
}
