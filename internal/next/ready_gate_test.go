package next

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
)

// The gate tests drive the real readiness engine through FakeBDStore or a
// report seam. Refusals audit fake tracker state and real temp git refs,
// worktrees and porcelain, with metadata writes forbidden. Command fixtures
// separately exercise the local fake tracker's marker/claim round-trip.

// TestGateReadiness_NegativeFixtureRefuses pins AC-4's refusal shape: a
// failing floor with allowNotReady=false returns a nil result and a
// guard-shaped error carrying the SAME per-signal rendering
// `bead ready-check` prints (ADR-0040 no-restate), each failing signal's
// recovery line, AND both R3 escape hatches (the standalone ready-check
// report, and re-running with --allow-not-ready).
func TestGateReadiness_NegativeFixtureRefuses(t *testing.T) {
	root := t.TempDir()
	fx, err := readiness.BuildNegativeFixture(root)
	if err != nil {
		t.Fatalf("BuildNegativeFixture: %v", err)
	}
	fx.Store.LandedDeps = map[string]bool{"unused": true}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	result, err := GateReadiness(root, fx.BeadID, GateOptions{})
	if err == nil {
		t.Fatal("expected a refusal error for the negative fixture, got nil")
	}
	if result != nil {
		t.Errorf("expected a nil GateResult on refusal, got %+v", result)
	}

	msg := err.Error()
	for _, want := range []string{"MF-1: FAIL", "MF-2: FAIL", "MF-3: FAIL", "MF-4: FAIL"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal message missing %q:\n%s", want, msg)
		}
	}
	if got := strings.Count(msg, "recovery: "); got < 6 {
		// 4 per-signal recovery lines + the 2 R3 escape hatches.
		t.Errorf("expected at least 6 recovery: lines (4 signals + 2 escape hatches), got %d:\n%s", got, msg)
	}
	if !strings.Contains(msg, "bead ready-check") {
		t.Errorf("refusal message missing the standalone ready-check escape hatch:\n%s", msg)
	}
	if !strings.Contains(msg, "--allow-not-ready") {
		t.Errorf("refusal message missing the --allow-not-ready escape hatch:\n%s", msg)
	}
}

// TestGateReadiness_NegativeFixtureAllowNotReady pins the override path:
// allowNotReady=true against a failing floor returns a nil error and a
// GateResult naming every failing signal (no refusal, no partial list).
func TestGateReadiness_NegativeFixtureAllowNotReady(t *testing.T) {
	root := t.TempDir()
	fx, err := readiness.BuildNegativeFixture(root)
	if err != nil {
		t.Fatalf("BuildNegativeFixture: %v", err)
	}
	fx.Store.LandedDeps = map[string]bool{"unused": true}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	result, err := GateReadiness(root, fx.BeadID, allOverrideOptions(t))
	if err != nil {
		t.Fatalf("expected no error with allowNotReady=true, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected a non-nil GateResult")
	}
	want := map[string]bool{"MF-1": true, "MF-2": true, "MF-3": true, "MF-4": true}
	if len(result.FailingSignals) != len(want) {
		t.Fatalf("expected %d failing signals, got %d: %v", len(want), len(result.FailingSignals), result.FailingSignals)
	}
	for _, id := range result.FailingSignals {
		if !want[id] {
			t.Errorf("unexpected failing signal %q", id)
		}
		delete(want, id)
	}
	if len(want) != 0 {
		t.Errorf("missing failing signal(s): %v", want)
	}
}

// TestGateReadiness_PositiveFixturePasses pins the "adds no interactive
// step" half of R3: a passing floor returns a nil error and an EMPTY
// GateResult, regardless of allowNotReady's value (a passing floor never
// treats allowNotReady as a signal to warn about).
func TestGateReadiness_PositiveFixturePasses(t *testing.T) {
	root := t.TempDir()
	fx, err := readiness.BuildPositiveFixture(root)
	if err != nil {
		t.Fatalf("BuildPositiveFixture: %v", err)
	}
	fx.Store.LandedDeps = map[string]bool{fx.DepBeadID: true}
	restore := fx.Store.Install()
	t.Cleanup(restore)

	for _, allowNotReady := range []bool{false, true} {
		result, err := GateReadiness(root, fx.BeadID, fixtureOverrideOptions(t, allowNotReady))
		if err != nil {
			t.Fatalf("allowNotReady=%v: expected no error for the positive fixture, got: %v", allowNotReady, err)
		}
		if result == nil {
			t.Fatalf("allowNotReady=%v: expected a non-nil GateResult", allowNotReady)
		}
		if len(result.FailingSignals) != 0 {
			t.Errorf("allowNotReady=%v: expected zero failing signals for the positive fixture, got %v", allowNotReady, result.FailingSignals)
		}
	}
}

// TestGateReadiness_EvaluationErrorPropagates pins the fail-closed
// behavior when the engine itself cannot evaluate (e.g. no discoverable
// bead lineage) — a bare evaluation error, not a NOT-READY guard failure,
// still returns (nil, err) so the caller never proceeds to claim.
func TestGateReadiness_EvaluationErrorPropagates(t *testing.T) {
	root := t.TempDir()
	store := readiness.NewFakeBDStore() // no lineage recorded for any bead
	restore := store.Install()
	t.Cleanup(restore)

	result, err := GateReadiness(root, "mindspec-nope.1", GateOptions{})
	if err == nil {
		t.Fatal("expected an evaluation error for an unrecorded bead, got nil")
	}
	if result != nil {
		t.Errorf("expected a nil GateResult on evaluation error, got %+v", result)
	}
}

// TestGateReadiness_RenderingLegControlBytesEscaped is round-2 panel's
// red-on-revert pin for GateReadiness's own render leg: a chain test
// alone (errors.As reaching through) would stay green even if the
// bdRenderedError wrapper were dropped from GateReadiness's evaluation-
// error wrap (reverting to a bare %w of err). This directly swaps
// evaluateReadinessFn to return a control-bearing error — as a real
// EvaluateReadiness failure would when it reads bd transitively via
// fetchBeadRecordReal -> bead.RunBD and bd's stderr carries hostile
// bytes — and asserts the schema-error text survives while no raw
// control byte reaches GateReadiness's rendered error.
func TestGateReadiness_RenderingLegControlBytesEscaped(t *testing.T) {
	orig := evaluateReadinessFn
	t.Cleanup(func() { evaluateReadinessFn = orig })

	evaluateReadinessFn = func(root, beadID string) (*readiness.Report, error) {
		return nil, errors.New("bd show " + beadID +
			" failed: exit status 1: Error 1105 (HY000): column \"depends_on_id\" could not be found\n" +
			"noise\x1b[31mmore-noise\r")
	}

	_, err := GateReadiness("root", "mindspec-abc", GateOptions{})
	if err == nil {
		t.Fatal("expected the evaluation error to propagate")
	}
	msg := err.Error()
	if !strings.Contains(msg, "depends_on_id") {
		t.Errorf("schema-error text must survive rendering, got: %q", msg)
	}
	if strings.ContainsAny(msg, "\x1b\r\x00") {
		t.Errorf("a raw control byte reached GateReadiness's rendered error: %q", msg)
	}
}

// TestRecordReadinessOverride_WritesMarkerViaMergeMetadata pins the
// durable-marker write shape: RecordReadinessOverride calls
// bead.MergeMetadata (via the mergeMetadataFn seam) with EXACTLY the
// bead.MetaKeyReadinessOverride key, carrying the signal list and a
// parseable UTC RFC3339 timestamp.
func TestRecordReadinessOverride_WritesMarkerViaMergeMetadata(t *testing.T) {
	orig := mergeMetadataFn
	t.Cleanup(func() { mergeMetadataFn = orig })

	var gotID string
	var gotUpdates map[string]interface{}
	mergeMetadataFn = func(issueID string, updates map[string]interface{}) error {
		gotID = issueID
		gotUpdates = updates
		return nil
	}

	signals := []string{"MF-2", "MF-4"}
	if err := RecordReadinessOverride("mindspec-abcd.1", recordResult(t, signals)); err != nil {
		t.Fatalf("RecordReadinessOverride: %v", err)
	}

	if gotID != "mindspec-abcd.1" {
		t.Errorf("expected issueID %q, got %q", "mindspec-abcd.1", gotID)
	}
	if len(gotUpdates) != 1 {
		t.Fatalf("expected exactly one metadata key written, got %d: %v", len(gotUpdates), gotUpdates)
	}
	raw, ok := gotUpdates[bead.MetaKeyReadinessOverride]
	if !ok {
		t.Fatalf("expected the %q key to be written, got keys %v", bead.MetaKeyReadinessOverride, gotUpdates)
	}
	marker, validity := bead.ParseOverrideMarker(raw)
	if validity != bead.MarkerValid {
		t.Fatalf("marker must be schema v2: %v %#v", validity, raw)
	}
	if !reflect.DeepEqual(marker.Signals, signals) {
		t.Fatalf("signals = %v", marker.Signals)
	}
	if _, err := time.Parse(time.RFC3339, marker.Overridden); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(marker.Fingerprints, recordResult(t, signals).Fingerprints) {
		t.Fatal("wrong complete failure fingerprints")
	}

}

// TestRecordReadinessOverride_PropagatesWriteFailure pins that a
// MergeMetadata failure is surfaced, never swallowed inside
// RecordReadinessOverride itself (the caller decides whether to warn-and-
// continue or fail hard — see cmd/mindspec/next.go's call site).
func TestRecordReadinessOverride_PropagatesWriteFailure(t *testing.T) {
	orig := mergeMetadataFn
	t.Cleanup(func() { mergeMetadataFn = orig })

	mergeMetadataFn = func(issueID string, updates map[string]interface{}) error {
		return errors.New("bd metadata merge-write failed")
	}

	if err := RecordReadinessOverride("mindspec-abcd.1", recordResult(t, []string{"MF-1"})); err == nil {
		t.Fatal("expected the MergeMetadata failure to propagate, got nil")
	}
}

// TestRecordReadinessOverride_HostileFailureRendersEscaped pins spec 128
// Bead 8's folded-in ta30.9 render-leg follow-up (verified live in this
// worktree rather than assumed): RecordReadinessOverride already wraps a
// MergeMetadata failure in bdRenderedError (ready_gate.go:130), so any
// caller formatting the returned error via %v/%s — e.g.
// cmd/mindspec/next.go's readinessOverrideFailure — gets the SAME
// per-line termsafe.Escape treatment bd-ready failures get (ADR-0042),
// with no second escaping path required at the call site. A hostile
// control byte (ESC) in the underlying failure text must never survive
// raw into err.Error().
func TestRecordReadinessOverride_HostileFailureRendersEscaped(t *testing.T) {
	orig := mergeMetadataFn
	t.Cleanup(func() { mergeMetadataFn = orig })

	hostile := "bd metadata merge-write failed\x1b[31mFORGED\x1b[0m"
	mergeMetadataFn = func(issueID string, updates map[string]interface{}) error {
		return errors.New(hostile)
	}

	err := RecordReadinessOverride("mindspec-abcd.1", recordResult(t, []string{"MF-1"}))
	if err == nil {
		t.Fatal("expected the MergeMetadata failure to propagate")
	}
	rendered := err.Error()
	if strings.ContainsRune(rendered, 0x1b) {
		t.Errorf("raw ESC byte survived into RecordReadinessOverride's rendered error: %q", rendered)
	}
	if !strings.Contains(rendered, "FORGED") {
		t.Errorf("expected the underlying hostile text to survive (quoted, not raw) into the rendered error, got: %q", rendered)
	}
}

// TestRollbackReadinessOverride_DeletesExactlyTheMarkerKey pins the
// G3-OVERRIDE-ORPHAN rollback shape (spec 124 final-review r1): a failed
// ClaimBead after the marker-before-claim write must delete EXACTLY the
// bead.MetaKeyReadinessOverride key, via the deleteMetadataKeysFn seam.
func TestRollbackReadinessOverride_DeletesExactlyTheMarkerKey(t *testing.T) {
	orig := deleteMetadataKeysFn
	t.Cleanup(func() { deleteMetadataKeysFn = orig })

	var gotID string
	var gotKeys []string
	deleteMetadataKeysFn = func(issueID string, keys ...string) error {
		gotID = issueID
		gotKeys = keys
		return nil
	}

	if err := RollbackReadinessOverride("mindspec-abcd.1"); err != nil {
		t.Fatalf("RollbackReadinessOverride: %v", err)
	}
	if gotID != "mindspec-abcd.1" {
		t.Errorf("expected issueID %q, got %q", "mindspec-abcd.1", gotID)
	}
	if len(gotKeys) != 1 || gotKeys[0] != bead.MetaKeyReadinessOverride {
		t.Errorf("expected exactly the %q key, got %v", bead.MetaKeyReadinessOverride, gotKeys)
	}
}

// TestRollbackReadinessOverride_PropagatesDeleteFailure pins that a
// delete failure surfaces to the caller — cmd/mindspec/next.go's
// claim-failure branch warns LOUDLY (naming the orphaned key) instead of
// silently swallowing a rollback that left the marker behind.
func TestRollbackReadinessOverride_PropagatesDeleteFailure(t *testing.T) {
	orig := deleteMetadataKeysFn
	t.Cleanup(func() { deleteMetadataKeysFn = orig })

	deleteMetadataKeysFn = func(issueID string, keys ...string) error {
		return errors.New("bd metadata delete-write failed")
	}

	if err := RollbackReadinessOverride("mindspec-abcd.1"); err == nil {
		t.Fatal("expected the delete failure to propagate, got nil")
	}
}

// These helpers never consult a live tracker. The audit includes the complete
// fake tracker record set and real temporary git refs/worktrees/porcelain.
func gateGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}
func gateAudit(t *testing.T, root string, store *readiness.FakeBDStore) string {
	t.Helper()
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + gateGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)") + gateGit(t, root, "worktree", "list", "--porcelain") + gateGit(t, root, "status", "--porcelain")
}
func fixtureOverrideOptions(t *testing.T, on bool) GateOptions {
	if on {
		return allOverrideOptions(t)
	}
	return GateOptions{}
}
func allOverrideOptions(t *testing.T) GateOptions {
	t.Helper()
	p := filepath.Join(t.TempDir(), "why.md")
	if err := os.WriteFile(p, []byte("operator justification"), 0600); err != nil {
		t.Fatal(err)
	}
	return GateOptions{FlagGiven: true, NamedSignals: []string{"MF-1", "MF-2", "MF-3", "MF-4"}, EvidencePath: p}
}
func recordResult(t *testing.T, ids []string) *GateResult {
	t.Helper()
	e, err := ReadOverrideEvidence(allOverrideOptions(t).EvidencePath)
	if err != nil {
		t.Fatal(err)
	}
	r := &GateResult{FailingSignals: ids, Fingerprints: map[string][]string{}, Evidence: e}
	for _, id := range ids {
		r.Fingerprints[id] = readiness.FailureRecordDigests(readiness.Signal{Failures: []string{id + ":fixture"}})
	}
	return r
}
func gateFixture(t *testing.T) (string, *readiness.Fixture) {
	t.Helper()
	root := t.TempDir()
	fx, err := readiness.BuildPositiveFixture(root)
	if err != nil {
		t.Fatal(err)
	}
	fx.Store.LandedDeps = map[string]bool{fx.DepBeadID: true}
	t.Cleanup(fx.Store.Install())
	return root, fx
}
func denyWrites(t *testing.T) *int {
	t.Helper()
	old := mergeMetadataFn
	n := new(int)
	mergeMetadataFn = func(string, map[string]interface{}) error { *n++; return errors.New("unexpected metadata write") }
	t.Cleanup(func() { mergeMetadataFn = old })
	return n
}
func assertGateRefusal(t *testing.T, root string, fx *readiness.Fixture, id string, opts GateOptions, wants ...string) {
	t.Helper()
	n := denyWrites(t)
	before := gateAudit(t, root, fx.Store)
	result, err := GateReadiness(root, id, opts)
	if err == nil || result != nil {
		t.Fatalf("expected refusal, got %+v %v", result, err)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q: %v", want, err)
		}
	}
	if *n != 0 || gateAudit(t, root, fx.Store) != before {
		t.Fatal("refusal mutated tracker, refs or worktrees")
	}
}
func TestGateReadiness_LineageSpecMismatchRefuses(t *testing.T) {
	root, fx := gateFixture(t)
	assertGateRefusal(t, root, fx, fx.BeadID, GateOptions{ScopeSpec: "997-other"}, "bead "+fx.BeadID+" resolves to spec "+fx.SpecID+" by lineage but is being claimed for spec 997-other; nothing was claimed", "recovery: bd show "+fx.BeadID+" --json")
}
func TestGateReadiness_NonPlanBeadRefuses(t *testing.T) {
	for _, id := range []string{"mindspec-bug"} {
		for _, scoped := range []bool{false, true} {
			t.Run(id+fmtBool(scoped), func(t *testing.T) {
				root, fx := gateFixture(t)
				fx.Store.Lineage[id] = readiness.FakeLineage{EpicID: "mindspec-posb", SpecID: fx.SpecID}
				fx.Store.Records[id] = readiness.FakeBeadRecord{}
				scope := ""
				if scoped {
					scope = fx.SpecID
				}
				assertGateRefusal(t, root, fx, id, GateOptions{ScopeSpec: scope}, "bead "+id+" is not a plan bead of spec "+fx.SpecID+" (no mindspec-posb.<N> ID); mindspec next claims only plan beads", "recovery: bd show "+id+" --json")
			})
		}
	}
}
func fmtBool(b bool) string {
	if b {
		return "-scoped"
	}
	return "-unscoped"
}
func TestGateReadiness_MissingSpecBranchRefuses(t *testing.T) {
	root, fx := gateFixture(t)
	// Move HEAD to main so the spec ref can be deleted in this local fixture.
	gateGit(t, root, "switch", "main")
	gateGit(t, root, "branch", "-D", "spec/"+fx.SpecID)
	assertGateRefusal(t, root, fx, fx.BeadID, GateOptions{}, "spec branch spec/"+fx.SpecID+" for bead "+fx.BeadID+" does not exist (finalized and deleted, or never created); nothing was claimed", "recovery: mindspec state show")
}
func TestGateReadiness_SpecBranchInfrastructureFailureRefuses(t *testing.T) {
	for _, failure := range []string{"exit-128", "exec-error-with-control-byte"} {
		t.Run(failure, func(t *testing.T) {
			root, fx := gateFixture(t)
			gateReport(t, reportWithFailures())
			writes := denyWrites(t)
			before := gateAudit(t, root, fx.Store)
			bin := filepath.Join(t.TempDir(), "bin\ncontrol")
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			script := "#!/bin/sh\nexit 128\n"
			if failure == "exec-error-with-control-byte" {
				script = "#!/absent-interpreter\n"
			}
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			oldPath := os.Getenv("PATH")
			t.Setenv("PATH", bin+string(os.PathListSeparator)+oldPath)
			result, err := GateReadiness(root, fx.BeadID, GateOptions{})
			t.Setenv("PATH", oldPath)
			if err == nil || result != nil {
				t.Fatalf("expected infrastructure refusal, got %+v %v", result, err)
			}
			for _, want := range []string{"could not resolve spec branch spec/" + fx.SpecID + " for bead " + fx.BeadID, "nothing was claimed", "recovery: git status   (repair the local git checkout, then retry)"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("missing %q: %v", want, err)
				}
			}
			if failure == "exit-128" && !strings.Contains(err.Error(), "exit status 128") {
				t.Fatalf("lost git failure diagnosis: %v", err)
			}
			if failure == "exec-error-with-control-byte" && (!strings.Contains(err.Error(), `bin\ncontrol/git`) || strings.Contains(err.Error(), "bin\ncontrol/git")) {
				t.Fatalf("git exec error was not escaped: %v", err)
			}
			if strings.Contains(err.Error(), "does not exist (finalized") {
				t.Fatalf("git failure misdiagnosed as missing branch: %v", err)
			}
			if *writes != 0 || gateAudit(t, root, fx.Store) != before {
				t.Fatal("infrastructure refusal mutated tracker, refs, worktrees or files")
			}
		})
	}
}

func reportWithFailures(ids ...string) *readiness.Report {
	r := &readiness.Report{BeadID: "mindspec-posb.1", EpicID: "mindspec-posb", SpecID: "998-fixture-positive"}
	for _, id := range []string{"MF-1", "MF-2", "MF-3", "MF-4"} {
		s := readiness.Signal{ID: id, Pass: true}
		for _, fail := range ids {
			if fail == id {
				s.Pass = false
				s.Detail = id + " fixture failure"
				s.Failures = []string{id + ":fixture"}
				s.Recovery = "bd show mindspec-posb.1 --json"
			}
		}
		r.Signals = append(r.Signals, s)
	}
	return r
}
func gateReport(t *testing.T, r *readiness.Report) {
	t.Helper()
	old := evaluateReadinessFn
	t.Cleanup(func() { evaluateReadinessFn = old })
	evaluateReadinessFn = func(string, string) (*readiness.Report, error) { return r, nil }
}
func TestGateReadiness_ScopedOverride(t *testing.T) {
	for _, tc := range []struct {
		name        string
		fail, named []string
		missing     bool
		want        string
	}{
		{"MF3", []string{"MF-3"}, []string{"MF-3"}, false, ""},
		{"subset-refusal", []string{"MF-2", "MF-3"}, []string{"MF-3"}, false, "--allow-not-ready=MF-2,MF-3 --evidence <path-to-justification>"},
		{"unknown", []string{"MF-3"}, []string{"MF-9"}, false, "unknown readiness signal MF-9"},
		{"all", []string{"MF-3"}, []string{"all"}, false, "unknown readiness signal all"},
		{"missing-evidence", []string{"MF-3"}, []string{"MF-3"}, true, "missing --evidence"},
		{"passing-note-on-refusal", []string{"MF-2"}, []string{"MF-3"}, false, "MF-3 named but passing — not recorded"},
		{"passing", nil, []string{"MF-3"}, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, fx := gateFixture(t)
			r := reportWithFailures(tc.fail...)
			gateReport(t, r)
			opts := allOverrideOptions(t)
			opts.NamedSignals = tc.named
			if tc.missing {
				opts.EvidencePath = ""
			}
			if tc.want != "" {
				assertGateRefusal(t, root, fx, fx.BeadID, opts, tc.want)
				_, err := GateReadiness(root, fx.BeadID, opts)
				if tc.name == "subset-refusal" {
					lines := strings.Split(err.Error(), "\n")
					last := lines[len(lines)-1]
					if last != "recovery: mindspec next "+fx.BeadID+" "+tc.want || strings.Contains(last, opts.EvidencePath) {
						t.Fatalf("final recovery leaked operator path or omitted full set: %s", last)
					}
				}
				return
			}
			got, err := GateReadiness(root, fx.BeadID, opts)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.FailingSignals, tc.fail) {
				t.Fatalf("recorded signals %v want %v", got.FailingSignals, tc.fail)
			}
			if len(tc.fail) == 0 && !reflect.DeepEqual(got.Notes, []string{"MF-3 named but passing — not recorded"}) {
				t.Fatal(got.Notes)
			}
			if len(tc.fail) > 0 {
				old := mergeMetadataFn
				defer func() { mergeMetadataFn = old }()
				mergeMetadataFn = func(id string, updates map[string]interface{}) error {
					m, v := bead.ParseOverrideMarker(updates[bead.MetaKeyReadinessOverride])
					if v != bead.MarkerValid || !reflect.DeepEqual(m.Signals, tc.fail) || !reflect.DeepEqual(m.Fingerprints["MF-3"], readiness.FailureRecordDigests(r.Signals[2])) {
						t.Fatalf("wrong marker: %+v %s", m, v)
					}
					return nil
				}
				if err := RecordReadinessOverride(fx.BeadID, got); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestGateReadiness_IndeterminateNotOverridable(t *testing.T) {
	for _, origin := range []string{"membership-read", "landed-merge-read"} {
		t.Run(origin, func(t *testing.T) {
			root := t.TempDir()
			fx, err := readiness.BuildPositiveFixture(root)
			if err != nil {
				t.Fatal(err)
			}
			dep := fx.DepBeadID
			if origin == "membership-read" {
				dep = "external-dep"
				rec := fx.Store.Records[fx.BeadID]
				rec.Dependencies = []readiness.FakeDependency{{ID: dep, Status: "closed"}}
				fx.Store.Records[fx.BeadID] = rec
			}
			// Membership uses the real dependency reader and local bd script;
			// landed-merge uses in-memory records and the real landed finder.
			bin := t.TempDir()
			script := "#!/bin/sh\nprintf '%s\\n' 'local metadata read failure' >&2\nexit 1\n"
			if origin == "membership-read" {
				t.Cleanup(phase.SetRunBDForTest(bead.RunBD))
				t.Cleanup(phase.SetListJSONForTest(bead.ListJSON))
				epic := map[string]interface{}{"id": "mindspec-posb", "title": "[SPEC 998] fixture-positive", "issue_type": "epic", "status": "open", "metadata": map[string]interface{}{"spec_num": 998, "spec_title": "fixture-positive"}}
				main := map[string]interface{}{"id": fx.BeadID, "title": "[998] fixture", "parent": "mindspec-posb", "description": fx.Store.Records[fx.BeadID].Description, "dependencies": []map[string]string{{"id": dep, "status": "closed", "dependency_type": "blocks"}, {"id": "mindspec-posb", "issue_type": "epic", "dependency_type": "parent-child"}}}
				epicJSON, err := json.Marshal([]interface{}{epic})
				if err != nil {
					t.Fatal(err)
				}
				mainJSON, err := json.Marshal([]interface{}{main})
				if err != nil {
					t.Fatal(err)
				}
				script = "#!/bin/sh\ncase \"$1:$2\" in\nshow:" + fx.BeadID + ") cat <<'MAIN'\n" + string(mainJSON) + "\nMAIN\nexit 0;;\nshow:mindspec-posb|list:*) cat <<'EPIC'\n" + string(epicJSON) + "\nEPIC\nexit 0;;\nesac\nprintf '%s\\n' 'local metadata read failure' >&2\nexit 1\n"
			} else {
				t.Cleanup(fx.Store.Install())
			}
			if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			if origin == "landed-merge-read" {
				gitPath, err := exec.LookPath("git")
				if err != nil {
					t.Fatal(err)
				}
				script := "#!/bin/sh\nfor arg in \"$@\"; do\n if [ \"$arg\" = log ]; then printf '%s\\n' 'local landed read failure' >&2; exit 1; fi\ndone\nexec \"" + gitPath + "\" \"$@\"\n"
				if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			report, err := readiness.EvaluateReadiness(root, fx.BeadID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(report.Signals[2].Failures, []string{dep + ":indeterminate"}) {
				t.Fatalf("fixture did not arm %s: %+v", origin, report.Signals[2])
			}
			if origin == "membership-read" && !strings.Contains(report.Signals[2].Detail, "local metadata read failure") {
				t.Fatalf("fake metadata script did not supply the indeterminate detail: %s", report.Signals[2].Detail)
			}
			assertGateRefusal(t, root, fx, fx.BeadID, allOverrideOptions(t), "MF-3 cannot be overridden: "+dep+":indeterminate could not be evaluated; fix the read first")
			_, err = GateReadiness(root, fx.BeadID, allOverrideOptions(t))
			if strings.Contains(err.Error(), "--allow-not-ready=") {
				t.Fatal("indeterminate refusal offered impossible override")
			}
		})
	}
}

func captureOverride(t *testing.T, id string, result *GateResult) bead.OverrideMarker {
	t.Helper()
	var marker bead.OverrideMarker
	old := mergeMetadataFn
	defer func() { mergeMetadataFn = old }()
	mergeMetadataFn = func(got string, updates map[string]interface{}) error {
		var v bead.MarkerValidity
		marker, v = bead.ParseOverrideMarker(updates[bead.MetaKeyReadinessOverride])
		if got != id || v != bead.MarkerValid {
			t.Fatalf("invalid marker: %s %s %+v", got, v, updates)
		}
		return nil
	}
	if err := RecordReadinessOverride(id, result); err != nil {
		t.Fatal(err)
	}
	return marker
}
func overrideFixtureReport(t *testing.T, fail ...string) (string, *readiness.Fixture, *readiness.Report, GateOptions) {
	root, fx := gateFixture(t)
	r := reportWithFailures(fail...)
	gateReport(t, r)
	return root, fx, r, allOverrideOptions(t)
}
func TestOverride_NoScopeWidening(t *testing.T) {
	root, fx, r, opts := overrideFixtureReport(t, "MF-3")
	opts.NamedSignals = []string{"MF-2", "MF-3"}
	got, err := GateReadiness(root, fx.BeadID, opts)
	if err != nil {
		t.Fatal(err)
	}
	m := captureOverride(t, fx.BeadID, got)
	if !reflect.DeepEqual(m.Signals, []string{"MF-3"}) || len(m.Fingerprints) != 1 {
		t.Fatalf("passing signal recorded: %+v", m)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "operator justification") || strings.Contains(string(raw), "\"text\"") {
		t.Fatalf("evidence text stored: %s", raw)
	}
	r.Signals[1] = readiness.Signal{ID: "MF-2", Failures: []string{"dangling:R999"}, Recovery: "bd show mindspec-posb.1 --json"}
	c := readiness.ApplyOverride(r, m, bead.MarkerValid)
	if !reflect.DeepEqual(c.Uncovered, []string{"MF-2"}) {
		t.Fatal(c)
	}
	assertGateRefusal(t, root, fx, fx.BeadID, GateOptions{}, "--allow-not-ready=MF-2,MF-3")
}
func TestOverride_GrowthUncovered(t *testing.T) {
	root, fx, r, opts := overrideFixtureReport(t, "MF-3")
	r.Signals[2].Failures = []string{"mindspec-a.1:no-candidate"}
	got, err := GateReadiness(root, fx.BeadID, opts)
	if err != nil {
		t.Fatal(err)
	}
	m := captureOverride(t, fx.BeadID, got)
	r.Signals[2].Failures = append(r.Signals[2].Failures, "mindspec-b.1:no-candidate")
	if c := readiness.ApplyOverride(r, m, bead.MarkerValid); !reflect.DeepEqual(c.Uncovered, []string{"MF-3"}) {
		t.Fatalf("growth stayed covered: %+v", c)
	}
	got, err = GateReadiness(root, fx.BeadID, opts)
	if err != nil {
		t.Fatal(err)
	}
	m = captureOverride(t, fx.BeadID, got)
	if c := readiness.ApplyOverride(r, m, bead.MarkerValid); len(c.Uncovered) != 0 {
		t.Fatalf("fresh override not honoured: %+v", c)
	}
}
func TestOverride_ShrinkageStaysCovered(t *testing.T) {
	root, fx, r, opts := overrideFixtureReport(t, "MF-3")
	r.Signals[2].Failures = []string{"mindspec-a.1:no-candidate", "mindspec-b.1:no-candidate"}
	got, err := GateReadiness(root, fx.BeadID, opts)
	if err != nil {
		t.Fatal(err)
	}
	m := captureOverride(t, fx.BeadID, got)
	r.Signals[2].Failures = r.Signals[2].Failures[:1]
	if c := readiness.ApplyOverride(r, m, bead.MarkerValid); len(c.Uncovered) != 0 {
		t.Fatalf("shrinkage uncovered: %+v", c)
	}
}
func TestOverride_MF4SecondBlockerUncovered(t *testing.T) {
	root, fx, r, opts := overrideFixtureReport(t, "MF-4")
	r.Signals[3].Failures = []string{"token:description:-:1:first"}
	got, err := GateReadiness(root, fx.BeadID, opts)
	if err != nil {
		t.Fatal(err)
	}
	m := captureOverride(t, fx.BeadID, got)
	r.Signals[3].Failures = append(r.Signals[3].Failures, "token:description:-:1:second")
	if c := readiness.ApplyOverride(r, m, bead.MarkerValid); !reflect.DeepEqual(c.Uncovered, []string{"MF-4"}) {
		t.Fatalf("MF4 growth covered: %+v", c)
	}
}
func TestOverride_MF4OccurrenceIdentity(t *testing.T) {
	for _, kind := range []string{"token", "checklist"} {
		t.Run(kind, func(t *testing.T) {
			root, fx := gateFixture(t)
			rec := fx.Store.Records[fx.BeadID]
			rec.Description = "TBD"
			if kind == "checklist" {
				path := filepath.Join(root, "docs/specs", fx.SpecID, "plan.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, []byte("\n**Blocking Questions**\n- [ ] Resolve retry count\n")...), 0600); err != nil {
					t.Fatal(err)
				}
				rec.Description = ""
			}
			fx.Store.Records[fx.BeadID] = rec
			opts := allOverrideOptions(t)
			got, err := GateReadiness(root, fx.BeadID, opts)
			if err != nil {
				t.Fatal(err)
			}
			m := captureOverride(t, fx.BeadID, got)
			if len(m.Fingerprints["MF-4"]) != 1 {
				t.Fatalf("expected one blocker: %+v", m)
			}
			if kind == "token" {
				path := filepath.Join(root, "docs/specs", fx.SpecID, "plan.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, []byte("\n### Design\nTBD\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				path := filepath.Join(root, "docs/specs", fx.SpecID, "plan.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, []byte("- [ ] Resolve retry count\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			r, err := readiness.EvaluateReadiness(root, fx.BeadID)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Signals[3].Failures) != 2 {
				t.Fatalf("multiplicity lost: %+v", r.Signals[3])
			}
			if c := readiness.ApplyOverride(r, m, bead.MarkerValid); !reflect.DeepEqual(c.Uncovered, []string{"MF-4"}) {
				t.Fatalf("second occurrence covered: %+v", c)
			}
		})
	}
}
func TestOverride_ExplicitNarrowerRequestRefuses(t *testing.T) {
	root, fx, r, opts := overrideFixtureReport(t, "MF-2", "MF-3")
	got, err := GateReadiness(root, fx.BeadID, opts)
	if err != nil {
		t.Fatal(err)
	}
	m := captureOverride(t, fx.BeadID, got)
	fx.Store.SeedReadinessAttempt(fx.BeadID, bead.MetaKeyReadinessOverride, m)
	opts.NamedSignals = []string{"MF-3"}
	assertGateRefusal(t, root, fx, fx.BeadID, opts, "--allow-not-ready=MF-2,MF-3")
	if !reflect.DeepEqual(m.Fingerprints["MF-2"], readiness.FailureRecordDigests(r.Signals[1])) {
		t.Fatal("wrong token digest")
	}
}
func TestOverride_WriterReaderAgreeRawMF4(t *testing.T) {
	root, fx := gateFixture(t)
	rec := fx.Store.Records[fx.BeadID]
	rec.Description = "TBD \x1b[31mretry"
	fx.Store.Records[fx.BeadID] = rec
	opts := allOverrideOptions(t)
	got, err := GateReadiness(root, fx.BeadID, opts)
	if err != nil {
		t.Fatal(err)
	}
	m := captureOverride(t, fx.BeadID, got)
	r, err := readiness.EvaluateReadiness(root, fx.BeadID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.Fingerprints["MF-4"], readiness.FailureRecordDigests(r.Signals[3])) {
		t.Fatal("writer/reader raw digests differ")
	}
	if c := readiness.ApplyOverride(r, m, bead.MarkerValid); len(c.Uncovered) != 0 {
		t.Fatal(c)
	}
}
