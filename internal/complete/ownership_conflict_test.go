package complete

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

const windowManifest = ".mindspec/domains/widget/OWNERSHIP.yaml"

func ownershipWindow(t *testing.T) *windowFixture {
	t.Helper()
	f := newWindowFixture(t)
	plan := "---\nstatus: Approved\nspec_id: 132-window\nversion: \"1\"\nadr_citations:\n  - ADR-9001\n---\n"
	writeFile(t, f.specWT, ".mindspec/specs/"+closingSpec+"/plan.md", plan)
	writeFile(t, f.specWT, ".mindspec/adr/ADR-9001.md", "# ADR-9001: Widget\n\n- **Status**: Accepted\n- **Domain(s)**: widget\n\n## Decision\nOwn widget.\n")
	writeFile(t, f.specWT, windowManifest, "paths:\n  - feature.txt\n  - internal/widget/**\n  - spec/**\n")
	gitRun(t, f.specWT, "add", "-A")
	gitRun(t, f.specWT, "commit", "-qm", "spec claim")
	writeFile(t, f.beadWT, windowManifest, "paths:\n  - feature.txt\n  - internal/widget/**\n  - bead/**\n")
	writeFile(t, f.beadWT, "internal/widget/widget.go", "package widget\n\nconst Value = 1\n")
	writeFile(t, f.beadWT, ".mindspec/domains/widget/architecture.md", "Widget owns its source.\n")
	gitRun(t, f.beadWT, "add", "-A")
	gitRun(t, f.beadWT, "commit", "-qm", "widget implementation")
	f.tip = gateRevParse(t, f.root, "bead/"+closingBead)
	return f
}
func TestComplete_OwnershipConflictRefusesBeforeClose(t *testing.T) {
	f := ownershipWindow(t)
	before := gateRevParse(t, f.root, "bead/"+closingBead)
	_, err := f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "mindspec complete "+closingBead+" --merge-only")
	if f.closed || gitutil.MergeInProgress(f.specWT) {
		t.Fatal("plain refusal closed or merged")
	}
	if gateRevParse(t, f.root, "bead/"+closingBead) != before {
		t.Fatal("refusal materialized a commit")
	}
}
func TestComplete_OwnershipConflictMergeOnlyLoop(t *testing.T) {
	f := ownershipWindow(t)
	before := gateRevParse(t, f.root, "bead/"+closingBead)
	writeFile(t, f.beadWT, "internal/widget/widget.go", "package widget\n\nconst Value = 2\n")
	_, err := Run(f.root, closingBead, closingSpec, "snapshot before conflict", f.ex, CompleteOpts{MergeOnly: true})
	requireWindowRefusal(t, err, "--merge-only --resolve-merge")
	if gateRevParse(t, f.root, "bead/"+closingBead) == before {
		t.Fatal("ownership deferral lost the materialized snapshot")
	}
	if f.closed || !gitutil.MergeInProgress(f.specWT) || f.meta[mergeOnlyPendingKey] != "pending" {
		t.Fatalf("conflict state: closed=%v metadata=%v", f.closed, f.meta)
	}
	writeFile(t, f.specWT, windowManifest, "paths:\n  - feature.txt\n  - internal/widget/**\n")
	gitRun(t, f.specWT, "add", windowManifest)
	r, err := f.run(CompleteOpts{ResolveMerge: true})
	if err != nil || !r.MergeOnly || f.closed {
		t.Fatalf("resolved pending run closed: %+v %v", r, err)
	}
	r, err = f.run(CompleteOpts{})
	if err != nil || !r.BeadClosed {
		t.Fatalf("closing resolved ownership: %+v %v", r, err)
	}
}
func TestComplete_OwnershipResolutionRevokesClaimRefuses(t *testing.T) {
	f := ownershipWindow(t)
	_, err := f.run(CompleteOpts{MergeOnly: true})
	requireWindowRefusal(t, err, "--resolve-merge")
	writeFile(t, f.specWT, windowManifest, "paths:\n  - feature.txt\n  - internal/old/**\n")
	gitRun(t, f.specWT, "add", windowManifest)
	_, err = f.run(CompleteOpts{MergeOnly: true, ResolveMerge: true})
	requireWindowRefusal(t, err, "adr-divergence-unowned")
	if f.closed || !gitutil.MergeInProgress(f.specWT) {
		t.Fatal("revoked claim did not preserve conflict state")
	}
	// A hand-committed invalid landing must still fail on M. A later fix
	// cannot change M; the scoped amend exit can.
	gitRun(t, f.specWT, "commit", "--no-edit")
	_, err = f.run(CompleteOpts{})
	requireWindowRefusal(t, err, "commit --amend --no-edit")
	requireWindowRefusal(t, err, "A later commit does not change")
	if f.closed {
		t.Fatal("closed an unowned landing")
	}
	writeFile(t, f.specWT, windowManifest, "paths:\n  - feature.txt\n  - internal/widget/**\n")
	gitRun(t, f.specWT, "add", windowManifest)
	gitRun(t, f.specWT, "commit", "--amend", "--no-edit")
	delete(f.meta, mergeOnlySHAKey)
	r, err := f.run(CompleteOpts{})
	if err != nil || !r.MergeOnly || f.closed {
		t.Fatalf("amend retry: %+v %v", r, err)
	}
	r, err = f.run(CompleteOpts{})
	if err != nil || !r.BeadClosed {
		t.Fatalf("amend close: %+v %v", r, err)
	}
}
func TestComplete_ClosedNoMergeOwnershipConflictDefersToReentry(t *testing.T) {
	f := ownershipWindow(t)
	stubClosedWindowChild(t)
	ledger := f.registerLedger()
	f.closed = true
	_, err := f.run(CompleteOpts{})
	if err == nil || !gitutil.MergeInProgress(f.specWT) {
		t.Fatalf("closed no-merge path must preserve the conflict: %v", err)
	}
	if strings.Contains(err.Error(), "ownership manifests have merge conflicts") {
		t.Fatalf("closed path refused before merge: %v", err)
	}
	requireClosedConflictRecovery(t, err, windowManifest)
	if f.closeCalls != 0 {
		t.Fatalf("closed recovery called close again: %d", f.closeCalls)
	}
	writeFile(t, f.specWT, windowManifest, "paths:\n  - feature.txt\n  - internal/widget/**\n")
	gitRun(t, f.specWT, "add", windowManifest)
	r, err := f.run(CompleteOpts{ResolveMerge: true})
	if err != nil || !r.BeadClosed {
		t.Fatalf("closed ownership re-entry: %+v %v", r, err)
	}
	f.assertPublished(ledger)
	if !r.WorktreeRemoved || f.closeCalls != 0 {
		t.Fatalf("closed ownership cleanup or duplicate close: %+v closes=%d", r, f.closeCalls)
	}
}
