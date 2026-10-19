package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/harness"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
)

// writeReadinessTestWorkspace writes spec.md/plan.md for specID under root
// at the on-disk path workspace.SpecDir resolves for a plain,
// non-worktree, non-.mindspec root: docs/specs/<id>/ (the same convention
// internal/validate/readiness's own fixtures.go uses).
func writeReadinessTestWorkspace(t *testing.T, root, specID, specMD, planMD string) error {
	t.Helper()
	dir := filepath.Join(root, "docs", "specs", specID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(specMD), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "plan.md"), []byte(planMD), 0o644)
}

// runBeadReadyCheck invokes beadReadyCheckCmd's RunE in-process (the same
// object rootCmd wires up), capturing stdout the way a real invocation
// would print it.
func runBeadReadyCheck(t *testing.T, beadID string) (stdout string, err error) {
	t.Helper()
	return captureStdout(t, func() error {
		return beadReadyCheckCmd.RunE(beadReadyCheckCmd, []string{beadID})
	})
}

func installAbsentReadinessMarker(t *testing.T) {
	t.Helper()
	oldStatus := readyCheckBeadFn
	readyCheckBeadFn = func(id string) (next.BeadInfo, error) { return next.BeadInfo{ID: id, Status: "open"}, nil }
	t.Cleanup(func() { readyCheckBeadFn = oldStatus })
	old := readyCheckReadMarkerFn
	t.Cleanup(func() { readyCheckReadMarkerFn = old })
	readyCheckReadMarkerFn = func(string) (bead.OverrideMarker, bead.MarkerValidity, error) {
		return bead.OverrideMarker{}, bead.MarkerAbsent, nil
	}
}

func readyOverrideReport(records ...string) *readiness.Report {
	return &readiness.Report{BeadID: "mindspec-fixture.2", Signals: []readiness.Signal{
		{ID: "MF-1", Pass: true}, {ID: "MF-2", Pass: true},
		{ID: "MF-3", Pass: len(records) == 0, Failures: records, Detail: "dependency failed", Recovery: "bd show mindspec-fixture.1 --json"},
		{ID: "MF-4", Pass: true},
	}}
}

func readyOverrideMarker(t *testing.T, r *readiness.Report) bead.OverrideMarker {
	t.Helper()
	var ids []string
	fps := map[string][]string{}
	for _, s := range r.FailingSignals() {
		ids = append(ids, s.ID)
		fps[s.ID] = readiness.FailureRecordDigests(s)
	}
	m, err := bead.NewOverrideMarkerV2(ids, fps, bead.OverrideEvidence{Path: "/tmp/justification", SHA256: strings.Repeat("a", 64), Bytes: 8}, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Invokes the command's own RunE with pure read seams, never a live tracker.
func runReadyOverride(t *testing.T, r *readiness.Report, raw any, readErr error, status ...string) (string, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	chdir(t, root)
	oldStatus := readyCheckBeadFn
	readyCheckBeadFn = func(id string) (next.BeadInfo, error) {
		claimStatus := "in_progress"
		if len(status) > 0 {
			claimStatus = status[0]
		}
		return next.BeadInfo{ID: id, Status: claimStatus}, nil
	}
	t.Cleanup(func() { readyCheckBeadFn = oldStatus })
	oldEval, oldRead := readyCheckEvaluateFn, readyCheckReadMarkerFn
	t.Cleanup(func() { readyCheckEvaluateFn, readyCheckReadMarkerFn = oldEval, oldRead })
	readyCheckEvaluateFn = func(gotRoot, gotID string) (*readiness.Report, error) {
		if gotRoot != canonicalRoot || gotID != r.BeadID {
			t.Fatalf("evaluate(%q,%q)", gotRoot, gotID)
		}
		return r, nil
	}
	readyCheckReadMarkerFn = func(gotID string) (bead.OverrideMarker, bead.MarkerValidity, error) {
		if gotID != r.BeadID {
			t.Fatalf("read marker(%q)", gotID)
		}
		if readErr != nil {
			return bead.OverrideMarker{}, bead.MarkerAbsent, readErr
		}
		if raw == nil {
			return bead.OverrideMarker{}, bead.MarkerAbsent, nil
		}
		m, v := bead.ParseOverrideMarker(raw)
		return m, v, nil
	}
	out, err := runBeadReadyCheck(t, r.BeadID)
	if err != nil {
		out += err.Error()
	}
	return out, err
}

func TestReadyCheck_HonoursValidMarkerOnly(t *testing.T) {
	a := readiness.DependencyFailureRecord("mindspec-fixture.1", readiness.FailureNoCandidate, "")
	b := readiness.DependencyFailureRecord("mindspec-fixture.3", readiness.FailureNoCandidate, "")
	indeterminate := readiness.DependencyFailureRecord("mindspec-fixture.1", readiness.FailureIndeterminate, "")
	t.Run("valid-matching", func(t *testing.T) {
		r := readyOverrideReport(a)
		out, err := runReadyOverride(t, r, readyOverrideMarker(t, r), nil)
		want := "MF-3: OVERRIDDEN (recorded 2026-10-19T00:00:00Z; evidence aaaaaaaaaaaa /tmp/justification)"
		if err != nil || !strings.Contains(out, want) {
			t.Fatalf("matching marker must exit 0 with %q; err=%v\n%s", want, err, out)
		}
	})
	t.Run("MF-2-also-failing", func(t *testing.T) {
		r := readyOverrideReport(a)
		m := readyOverrideMarker(t, r)
		r.Signals[1] = readiness.Signal{ID: "MF-2", Failures: []string{"dangling:R999"}, Detail: "R999 unresolved", Recovery: "bd show mindspec-fixture.2 --json"}
		out, err := runReadyOverride(t, r, m, nil)
		if err == nil || !strings.Contains(out, "uncovered: MF-2\n") || !strings.Contains(out, "MF-3: OVERRIDDEN") {
			t.Fatalf("MF-2 must refuse:\n%s (%v)", out, err)
		}
	})
	t.Run("changed-dependency", func(t *testing.T) {
		out, err := runReadyOverride(t, readyOverrideReport(b), readyOverrideMarker(t, readyOverrideReport(a)), nil)
		if err == nil || !strings.Contains(out, "uncovered: MF-3") {
			t.Fatalf("changed dependency covered:\n%s (%v)", out, err)
		}
	})
	t.Run("now-passing", func(t *testing.T) {
		out, err := runReadyOverride(t, readyOverrideReport(), readyOverrideMarker(t, readyOverrideReport(a)), nil)
		if err != nil || !strings.Contains(out, "MF-3: PASS (override recorded but no longer needed)") {
			t.Fatalf("passing override note:\n%s (%v)", out, err)
		}
	})
	t.Run("indeterminate", func(t *testing.T) {
		r := readyOverrideReport(indeterminate)
		out, err := runReadyOverride(t, r, readyOverrideMarker(t, r), nil)
		if err == nil || !strings.Contains(out, "uncovered: MF-3") || strings.Contains(out, "OVERRIDDEN") {
			t.Fatalf("indeterminate covered:\n%s (%v)", out, err)
		}
	})
	t.Run("metadata-read-error", func(t *testing.T) {
		out, err := runReadyOverride(t, readyOverrideReport(), nil, errors.New("tracker read failed\n\x1b[31m"))
		if err == nil || !strings.Contains(out, "could not read readiness override metadata") || !strings.Contains(out, `tracker read failed\n\x1b[31m`) || strings.ContainsRune(out, '\x1b') {
			t.Fatalf("metadata read must refuse and escape:\n%s (%v)", out, err)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		cases := []struct {
			name string
			edit func(map[string]any)
		}{
			{"wrong-schema", func(m map[string]any) { m["schema"] = 1 }},
			{"empty-signals", func(m map[string]any) { m["signals"] = []string{} }},
			{"unknown-signal", func(m map[string]any) { m["signals"] = []string{"MF-5"} }},
			{"duplicate-signal", func(m map[string]any) { m["signals"] = []string{"MF-3", "MF-3"} }},
			{"missing-fingerprints", func(m map[string]any) { delete(m, "fingerprints") }},
			{"extra-fingerprint-signal", func(m map[string]any) { m["fingerprints"].(map[string]any)["MF-2"] = []string{strings.Repeat("a", 64)} }},
			{"empty-fingerprints", func(m map[string]any) { m["fingerprints"].(map[string]any)["MF-3"] = []string{} }},
			{"nonhex-fingerprint", func(m map[string]any) { m["fingerprints"].(map[string]any)["MF-3"] = []string{strings.Repeat("g", 64)} }},
			{"duplicate-fingerprints", func(m map[string]any) {
				m["fingerprints"].(map[string]any)["MF-3"] = []string{strings.Repeat("a", 64), strings.Repeat("a", 64)}
			}},
			{"unsorted-fingerprints", func(m map[string]any) {
				m["fingerprints"].(map[string]any)["MF-3"] = []string{strings.Repeat("b", 64), strings.Repeat("a", 64)}
			}},
			{"unknown-extra-key", func(m map[string]any) { m["extra"] = true }},
			{"invalid-time", func(m map[string]any) { m["overridden"] = "yesterday" }},
			{"empty-evidence-path", func(m map[string]any) { m["evidence"].(map[string]any)["path"] = "" }},
			{"invalid-evidence-sha", func(m map[string]any) { m["evidence"].(map[string]any)["sha256"] = "short" }},
			{"zero-evidence-bytes", func(m map[string]any) { m["evidence"].(map[string]any)["bytes"] = 0 }},
			{"too-many-evidence-bytes", func(m map[string]any) { m["evidence"].(map[string]any)["bytes"] = 65537 }},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				r := readyOverrideReport(a)
				data, err := json.Marshal(readyOverrideMarker(t, r))
				if err != nil {
					t.Fatal(err)
				}
				var raw map[string]any
				if err := json.Unmarshal(data, &raw); err != nil {
					t.Fatal(err)
				}
				tt.edit(raw)
				out, err := runReadyOverride(t, r, raw, nil)
				for _, want := range []string{"override marker is malformed — not honoured", "uncovered: MF-3", "recovery: mindspec release mindspec-fixture.2\nrecovery: mindspec next mindspec-fixture.2 --allow-not-ready=MF-3 --evidence <path-to-justification>"} {
					if err == nil || !strings.Contains(out, want) {
						t.Fatalf("malformed marker missing %q:\n%s (%v)", want, out, err)
					}
				}
			})
		}
	})
}

func TestReadyCheck_LegacyRecoveryReleasesThenReclaims(t *testing.T) {
	r := readyOverrideReport(readiness.DependencyFailureRecord("mindspec-fixture.1", readiness.FailureNoCandidate, ""))
	out, err := runReadyOverride(t, r, map[string]any{"signals": []string{"MF-3"}, "overridden": "2026-10-19T00:00:00Z"}, nil)
	for _, want := range []string{"override marker is legacy (no evidence) — not honoured", "uncovered: MF-3", "recovery: mindspec release mindspec-fixture.2\nrecovery: mindspec next mindspec-fixture.2 --allow-not-ready=MF-3 --evidence <path-to-justification>"} {
		if err == nil || !strings.Contains(out, want) {
			t.Fatalf("legacy marker missing %q:\n%s (%v)", want, out, err)
		}
	}
}

func TestReadyCheck_MixedCoverageRecoveryNamesAllFailing(t *testing.T) {
	r := readyOverrideReport(readiness.DependencyFailureRecord("mindspec-fixture.1", readiness.FailureNoCandidate, ""))
	r.Signals[1] = readiness.Signal{ID: "MF-2", Failures: []string{"dangling:R999"}, Detail: "R999 unresolved", Recovery: "bd show mindspec-fixture.2 --json"}
	m := readyOverrideMarker(t, r)
	r.Signals[2].Failures = []string{readiness.DependencyFailureRecord("mindspec-fixture.3", readiness.FailureNoCandidate, "")}
	out, err := runReadyOverride(t, r, m, nil)
	for _, want := range []string{"MF-2: OVERRIDDEN", "uncovered: MF-3\n", "recovery: mindspec release mindspec-fixture.2\nrecovery: mindspec next mindspec-fixture.2 --allow-not-ready=MF-2,MF-3 --evidence <path-to-justification>"} {
		if err == nil || !strings.Contains(out, want) {
			t.Fatalf("mixed coverage missing %q:\n%s (%v)", want, out, err)
		}
	}
}

func gitPorcelainCmd(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status --porcelain: %v: %s", err, out)
	}
	return string(out)
}

func gitBranchesCmd(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "branch", "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list: %v: %s", err, out)
	}
	return string(out)
}

// TestBeadReadyCheck_NegativeFixture pins AC-1: the negative fixture exits
// non-zero with a FAIL line (+ recovery) for each of MF-1..MF-4.
func TestBeadReadyCheck_NegativeFixture(t *testing.T) {
	installAbsentReadinessMarker(t)
	root := t.TempDir()
	fx, err := readiness.BuildNegativeFixture(root)
	if err != nil {
		t.Fatalf("BuildNegativeFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)
	chdir(t, root)

	_, runErr := runBeadReadyCheck(t, fx.BeadID)
	if runErr == nil {
		t.Fatal("expected a non-zero-exit error for the negative fixture, got nil")
	}
	msg := runErr.Error()
	for _, want := range []string{"MF-1: FAIL", "MF-2: FAIL", "MF-3: FAIL", "MF-4: FAIL"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q:\n%s", want, msg)
		}
	}
	if got := strings.Count(msg, "recovery: "); got < 4 {
		t.Errorf("expected at least 4 recovery: lines (one per FAILing signal), got %d:\n%s", got, msg)
	}
}

// TestBeadReadyCheck_PositiveFixture pins AC-2: the positive fixture exits
// 0, is idempotent, and performs no mutation (git status/branches
// byte-identical before/after two consecutive invocations).
func TestBeadReadyCheck_PositiveFixture(t *testing.T) {
	installAbsentReadinessMarker(t)
	root := t.TempDir()
	fx, err := readiness.BuildPositiveFixture(root)
	if err != nil {
		t.Fatalf("BuildPositiveFixture: %v", err)
	}
	restore := fx.Store.Install()
	t.Cleanup(restore)
	chdir(t, root)

	statusBefore := gitPorcelainCmd(t, root)
	branchesBefore := gitBranchesCmd(t, root)

	out1, err1 := runBeadReadyCheck(t, fx.BeadID)
	if err1 != nil {
		t.Fatalf("expected exit 0 for the positive fixture, got: %v\n%s", err1, out1)
	}
	for _, want := range []string{"MF-1: PASS", "MF-2: PASS", "MF-3: PASS", "MF-4: PASS"} {
		if !strings.Contains(out1, want) {
			t.Errorf("stdout missing %q:\n%s", want, out1)
		}
	}

	out2, err2 := runBeadReadyCheck(t, fx.BeadID)
	if err2 != nil {
		t.Fatalf("2nd invocation: expected exit 0, got: %v\n%s", err2, out2)
	}
	if out1 != out2 {
		t.Errorf("ready-check is not idempotent:\n1st: %s\n2nd: %s", out1, out2)
	}

	statusAfter := gitPorcelainCmd(t, root)
	branchesAfter := gitBranchesCmd(t, root)
	if statusBefore != statusAfter {
		t.Errorf("git status --porcelain changed across invocations: before=%q after=%q", statusBefore, statusAfter)
	}
	if branchesBefore != branchesAfter {
		t.Errorf("git branch --list changed across invocations: before=%q after=%q", branchesBefore, branchesAfter)
	}
}

// TestBeadReadyCheck_MalformedBeadID pins the ingress refusal: a hostile
// bead-ID argument is refused at idvalidate, forced-quoted, before any
// root resolution or evaluation is attempted (no fixture/root needed).
func TestBeadReadyCheck_MalformedBeadID(t *testing.T) {
	hostileIDs := []string{
		"mindspec-1\n--help",
		"mindspec-1;evil",
		"../../etc/passwd",
		"",
	}
	for _, id := range hostileIDs {
		_, err := runBeadReadyCheck(t, id)
		if err == nil {
			t.Errorf("runBeadReadyCheck(%q): expected a refusal error, got nil", id)
			continue
		}
		msg := err.Error()
		// termsafe.Escape's contract is printable-ASCII-is-a-no-op — a
		// semicolon or slash is printable ASCII and legitimately renders
		// raw (it cannot forge a terminal line on its own). The property
		// that actually matters is: a raw NEWLINE embedded in a hostile
		// id must never surface as a second real line (it must be folded
		// into the escaped two-byte `\n` sequence via the whole-string
		// quoting).
		if strings.Contains(id, "\n") {
			wantQuoted := strconv.Quote(id)
			if !strings.Contains(msg, wantQuoted) {
				t.Errorf("runBeadReadyCheck(%q): expected the forced-quoted id %q present in %q", id, wantQuoted, msg)
			}
		}
	}
}

// TestBeadReadyCheck_HostileDescriptionEscaped pins AC-8: a bd
// description/plan-section line embedding terminal-escape bytes reaches
// the rendered report only in its termsafe-escaped (quoted) form — never
// as raw control bytes that could forge extra terminal lines.
func TestBeadReadyCheck_HostileDescriptionEscaped(t *testing.T) {
	installAbsentReadinessMarker(t)
	root := t.TempDir()
	const specID = "994-fixture-hostile"
	const beadID = "mindspec-hostile.1"
	const epicID = "mindspec-hostile"

	hostileLine := "Note: TBD - \x1b[31mHACKED\x1b[0m the retry count."
	specMD := "# Spec 994-fixture-hostile\n\n## Acceptance Criteria\n\n- [ ] AC-1 — a criterion.\n"
	planMD := "---\nstatus: Approved\nspec_id: " + specID + "\nversion: \"1\"\nwork_chunks:\n" +
		"  - id: 1\n    depends_on: []\n    key_file_paths:\n      - internal/fixture/hostile.go\n---\n" +
		"# Plan: " + specID + "\n\n## ADR Fitness\n\nNo ADRs relevant.\n\n## Testing Strategy\n\nUnit tests.\n\n" +
		"## Bead 1: Hostile fixture\n\n**Steps**\n1. Step one\n\n**Acceptance Criteria**\n" +
		"- [ ] Provide a concrete, testable outcome for this fixture case.\n\n" + hostileLine + "\n"

	if err := writeReadinessTestWorkspace(t, root, specID, specMD, planMD); err != nil {
		t.Fatalf("writing workspace: %v", err)
	}

	store := readiness.NewFakeBDStore()
	store.Lineage[beadID] = readiness.FakeLineage{EpicID: epicID, SpecID: specID}
	store.Records[beadID] = readiness.FakeBeadRecord{Description: ""}
	restore := store.Install()
	t.Cleanup(restore)
	chdir(t, root)

	if err := exec.Command("git", "-C", root, "init", "-b", "main").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	_, runErr := runBeadReadyCheck(t, beadID)
	if runErr == nil {
		t.Fatal("expected a non-zero-exit error (the hostile line is also a genuine TBD marker)")
	}
	msg := runErr.Error()
	if strings.ContainsRune(msg, '\x1b') {
		t.Errorf("raw ESC control byte leaked into the rendered report: %q", msg)
	}
	// strconv.Quote (termsafe.Escape's whole-string quoting for any
	// non-printable byte) renders a raw ESC as the four-byte textual
	// escape `\x1b` — proving the hostile byte survived in its escaped
	// form rather than being silently dropped.
	if !strings.Contains(msg, `\x1b`) {
		t.Errorf("expected the escaped ESC sequence `\\x1b` present in the report; got:\n%s", msg)
	}
	if !strings.Contains(msg, "HACKED") {
		t.Errorf("expected the surrounding hostile-line text to survive (escaped) in the report; got:\n%s", msg)
	}
}

// TestBeadReadyCheck_TemporalFlow_RealPlanApproveAndComplete pins AC-10:
// readiness is a TEMPORAL fact re-derived per invocation, not a cached
// approval-time judgment. A real two-bead plan (bead 2 depends_on bead 1)
// is approved via the real `mindspec plan approve` path (real bd, real
// git): immediately after approval, `ready-check` PASSes bead 1 and FAILs
// bead 2 on MF-3 (dependency not yet landed-merged); after bead 1 is
// landed via a real `mindspec complete` (branch deleted), `ready-check`
// PASSes bead 2 — proving MF-3 tolerates the deleted branch on the real
// completion path, not just the fixture's synthetic corroboration.
//
// This drives the REAL mindspec binary end-to-end (a freshly-built copy,
// spec 084 Bead 3's buildMindspecBinary helper) against a real bd +
// git sandbox (internal/harness.NewSandbox) — no fake-bd shim substitutes
// for `plan approve`/`complete`'s own bd/git mutation chain, which cannot
// be faithfully faked without reimplementing bd. Per the repo's own
// no-skip-gating convention for real-bd/real-git flows (see
// internal/harness/scenario_finalize_pr_test.go's hideRealGH,
// internal/harness/sandbox.go's initBeads), a missing `bd`/`git` on PATH
// is a hard failure (t.Fatalf), never a t.Skip.
func TestBeadReadyCheck_TemporalFlow_RealPlanApproveAndComplete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-bd/real-git end-to-end flow under -short (matches internal/harness's own -short gating convention)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Fatalf("real bd required for this AC-10 temporal-flow test (no-skip-gating, spec 124 plan-gate F3-1): %v", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("real git required: %v", err)
	}

	// Build a real mindspec binary and put its directory FIRST on the
	// live process PATH (restored via t.Cleanup): harness.Sandbox.Env()
	// re-reads os.Getenv("PATH") on every call, and mustRunWithBin's
	// nil-Env subprocess calls (used when the conventional bin/mindspec
	// isn't present) inherit the process environment — so this makes
	// `mindspec` resolvable to every subprocess the sandbox spawns
	// (git hooks included) without requiring a prior `make build`.
	binPath := buildMindspecBinary(t)
	binDir := filepath.Dir(binPath)
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	sb := harness.NewSandbox(t)

	const specID = "997-readiness-temporal"
	epicID := sb.CreateSpecEpic(specID)

	specBranch := "spec/" + specID
	if out, err := sb.Run("git", "branch", specBranch); err != nil {
		t.Fatalf("git branch %s: %v\n%s", specBranch, err, out)
	}
	specWtDir := ".worktrees/worktree-spec-" + specID
	if out, err := sb.Run("git", "worktree", "add", specWtDir, specBranch); err != nil {
		t.Fatalf("git worktree add: %v\n%s", out, err)
	}

	specMD := `---
title: Readiness Temporal Flow Fixture
status: Approved
---
# Readiness Temporal Flow Fixture

## Acceptance Criteria
- [ ] AC-1 — bead 1 lands real work
- [ ] AC-2 — bead 2 depends on bead 1
`
	planMD := `---
status: Draft
spec_id: 997-readiness-temporal
version: "1"
adr_citations: []
work_chunks:
  - id: 1
    depends_on: []
    key_file_paths:
      - internal/fixture/temporal_bead1.go
  - id: 2
    depends_on: [1]
    key_file_paths:
      - internal/fixture/temporal_bead2.go
---
# Plan: 997-readiness-temporal

## ADR Fitness

No existing ADRs are impacted; this is a self-contained test fixture.

## Testing Strategy

Unit tests via ` + "`go test`" + `.

## Bead 1: Land first

**Steps**
1. Create internal/fixture/temporal_bead1.go
2. Implement the fixture function
3. Add a unit test

**Verification**
- [ ] ` + "`go test ./internal/fixture/`" + ` passes

**Acceptance Criteria**
- [ ] AC-1 — bead 1 lands real work

**Depends on**
None

## Bead 2: Depends on Bead 1

**Steps**
1. Create internal/fixture/temporal_bead2.go
2. Implement the fixture function
3. Add a unit test

**Verification**
- [ ] ` + "`go test ./internal/fixture/`" + ` passes

**Acceptance Criteria**
- [ ] AC-2 — bead 2 depends on bead 1

**Depends on**
Bead 1
`
	sb.WriteFile(specWtDir+"/.mindspec/specs/"+specID+"/spec.md", specMD)
	sb.WriteFile(specWtDir+"/.mindspec/specs/"+specID+"/plan.md", planMD)

	specWtAbs := filepath.Join(sb.Root, specWtDir)
	if out, err := exec.Command("git", "-C", specWtAbs, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add (spec worktree): %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", specWtAbs, "commit", "-m", "setup: draft plan").CombinedOutput(); err != nil {
		t.Fatalf("git commit (spec worktree): %v\n%s", err, out)
	}
	sb.Commit("setup: spec worktree state")

	if out, err := sb.Run("mindspec", "plan", "approve", specID); err != nil {
		t.Fatalf("mindspec plan approve %s: %v\n%s", specID, err, out)
	}

	bead1 := epicID + ".1"
	bead2 := epicID + ".2"

	// Immediately after approval: bead 1 PASSes, bead 2 FAILs MF-3 (its
	// dependency is not yet landed-merged).
	out1, err1 := sb.Run("mindspec", "bead", "ready-check", bead1)
	if err1 != nil {
		t.Fatalf("ready-check %s expected exit 0 right after approval, got error: %v\n%s", bead1, err1, out1)
	}
	out2, err2 := sb.Run("mindspec", "bead", "ready-check", bead2)
	if err2 == nil {
		t.Fatalf("ready-check %s expected non-zero exit right after approval (dependency not landed), got exit 0:\n%s", bead2, out2)
	}
	if !strings.Contains(out2, "MF-3: FAIL") {
		t.Errorf("ready-check %s expected an MF-3 FAIL right after approval, got:\n%s", bead2, out2)
	}

	// Claim + land bead 1 via the real next/complete lifecycle.
	if out, err := sb.Run("mindspec", "next", bead1); err != nil {
		t.Fatalf("mindspec next %s: %v\n%s", bead1, err, out)
	}
	bead1WtDir := ".worktrees/worktree-" + bead1
	if !sb.FileExists(bead1WtDir) {
		t.Fatalf("expected bead 1 worktree at %s after `mindspec next`", bead1WtDir)
	}
	sb.WriteFile(bead1WtDir+"/internal/fixture/temporal_bead1.go", "package fixture\n\n// TemporalBead1 is the AC-10 fixture's landed artifact.\nfunc TemporalBead1() string { return \"bead1\" }\n")
	bead1WtAbs := filepath.Join(sb.Root, bead1WtDir)
	if out, err := exec.Command("git", "-C", bead1WtAbs, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add (bead 1 worktree): %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", bead1WtAbs, "commit", "-m", "bead1: land fixture work").CombinedOutput(); err != nil {
		t.Fatalf("git commit (bead 1 worktree): %v\n%s", err, out)
	}

	if out, err := sb.Run("mindspec", "complete", bead1, "land bead 1",
		"--allow-doc-skew", "test fixture, no docs domain",
		"--override-adr", "test fixture, no OWNERSHIP.yaml domain"); err != nil {
		t.Fatalf("mindspec complete %s: %v\n%s", bead1, err, out)
	}

	// After bead 1 lands (branch deleted per the normal completion
	// path), bead 2's dependency is now closed AND landed-merged:
	// ready-check PASSes, re-derived fresh (not a cached judgment).
	out3, err3 := sb.Run("mindspec", "bead", "ready-check", bead2)
	if err3 != nil {
		t.Fatalf("ready-check %s expected exit 0 after bead 1 landed, got error: %v\n%s", bead2, err3, out3)
	}
	if !strings.Contains(out3, "MF-3: PASS") {
		t.Errorf("ready-check %s expected MF-3: PASS after bead 1 landed, got:\n%s", bead2, out3)
	}
}

func TestReadyCheck_IndeterminateRecoveryNeverNamesOverride(t *testing.T) {
	for _, origin := range []string{"membership", "landed-merge"} {
		for _, markerKind := range []string{"absent", "legacy", "valid", "malformed"} {
			t.Run(origin+"/"+markerKind, func(t *testing.T) {
				r := readyOverrideReport("mindspec-fixture.1:indeterminate")
				r.Signals[2].Detail = origin + " read failed"
				r.Signals[1] = readiness.Signal{ID: "MF-2", Failures: []string{"dangling:R999"}, Detail: "R999 unresolved", Recovery: "bd show mindspec-fixture.2 --json"}
				var raw any
				switch markerKind {
				case "legacy":
					raw = map[string]any{"signals": []string{"MF-3"}, "overridden": "2026-10-19T00:00:00Z"}
				case "valid":
					raw = readyOverrideMarker(t, r)
				case "malformed":
					raw = map[string]any{"schema": 99}
				}
				out, err := runReadyOverride(t, r, raw, nil)
				if err == nil {
					t.Fatal("indeterminate ready-check passed")
				}
				requireText(t, out, "MF-3 cannot be overridden: mindspec-fixture.1:indeterminate could not be evaluated; fix the read first", "uncovered: MF-")
				if strings.Contains(out, "--allow-not-ready=") || strings.Contains(out, "recovery: mindspec release") {
					t.Fatalf("impossible re-claim hinted: %s", out)
				}
			})
		}
	}
}
func TestReadyCheck_AbsentUnclaimedDoesNotRelease(t *testing.T) {
	r := readyOverrideReport("mindspec-fixture.1:no-candidate")
	for _, status := range []string{"open", "in_progress"} {
		t.Run(status, func(t *testing.T) {
			out, err := runReadyOverride(t, r, nil, nil, status)
			if err == nil {
				t.Fatal("uncovered failure passed")
			}
			requireText(t, out, "uncovered: MF-3", "recovery: mindspec next mindspec-fixture.2 --allow-not-ready=MF-3 --evidence <path-to-justification>")
			if strings.Contains(out, "recovery: mindspec release") != (status == "in_progress") {
				t.Fatalf("release hint does not match claim: %s", out)
			}
		})
	}
}
func TestReadyCheck_MF4FailureSetThroughCommand(t *testing.T) {
	for _, mode := range []string{"second-token", "plan-occurrence", "repeated-checklist"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			fx, err := readiness.BuildPositiveFixture(root)
			if err != nil {
				t.Fatal(err)
			}
			fx.Store.LandedDeps = map[string]bool{fx.DepBeadID: true}
			t.Cleanup(fx.Store.Install())
			rec := fx.Store.Records[fx.BeadID]
			rec.Description = "TBD"
			planPath := filepath.Join(root, "docs/specs", fx.SpecID, "plan.md")
			if mode == "repeated-checklist" {
				rec.Description = ""
				data, err := os.ReadFile(planPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(planPath, append(data, []byte("\n**Blocking Questions**\n- [ ] Decide retry count\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			fx.Store.Records[fx.BeadID] = rec
			initial, err := readiness.EvaluateReadiness(root, fx.BeadID)
			if err != nil {
				t.Fatal(err)
			}
			if len(initial.Signals[3].Failures) != 1 {
				t.Fatalf("fixture needs one MF4 record: %+v", initial.Signals[3])
			}
			m := readyOverrideMarker(t, initial)
			out, err := runReadyOverride(t, initial, m, nil)
			if err != nil {
				t.Fatalf("initial override not honoured: %v %s", err, out)
			}
			requireText(t, out, "MF-4: OVERRIDDEN")
			switch mode {
			case "second-token":
				rec.Description += "\nOPEN QUESTION: retry"
				fx.Store.Records[fx.BeadID] = rec
			case "plan-occurrence":
				data, err := os.ReadFile(planPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(planPath, append(data, []byte("\n### Design\nTBD\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			case "repeated-checklist":
				data, err := os.ReadFile(planPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(planPath, append(data, []byte("- [ ] Decide retry count\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			grown, err := readiness.EvaluateReadiness(root, fx.BeadID)
			if err != nil {
				t.Fatal(err)
			}
			if len(grown.Signals[3].Failures) != 2 {
				t.Fatalf("growth lost occurrence: %+v", grown.Signals[3])
			}
			out, err = runReadyOverride(t, grown, m, nil)
			if err == nil {
				t.Fatal("command covered grown MF4 failure set")
			}
			requireText(t, out, "uncovered: MF-4", "--allow-not-ready=MF-4 --evidence <path-to-justification>")
		})
	}
}

// Drive release's own orchestration and next's real gate/marker/claim path.
// Surviving bead commits must be reused, and every refusal is byte-identical.
func TestReadyCheck_ReleaseFreshReclaimReplacesMarker(t *testing.T) {
	for _, shape := range []string{"MF2-MF3", "MF3-MF4", "grown-MF3-with-MF2"} {
		t.Run(shape, func(t *testing.T) {
			f := newScopedNextFixture(t)
			id := "epic-129.1"
			f.rows = append(f.rows, nextHonestyRow("epic-129.2", "prerequisite A", "epic-129", "task", "closed"), nextHonestyRow("epic-129.3", "prerequisite B", "epic-129", "task", "closed"))
			firstSignals := "MF-2"
			if shape != "MF3-MF4" {
				f.rows[0]["description"] = "AC-999"
			}
			if shape != "MF2-MF3" {
				f.deps[id] = []map[string]string{{"id": "epic-129.2", "status": "closed", "issue_type": "task", "dependency_type": "blocks"}}
				firstSignals = "MF-3"
				if shape == "grown-MF3-with-MF2" {
					firstSignals = "MF-2,MF-3"
				}
			}
			f.write(t)
			wtOps := localNextWorktrees{t, f.root}
			nextEnsureWorktreeFn = func(root, id, spec string, _ executor.Executor) (string, error) {
				e := executor.NewMindspecExecutor(root)
				e.WorktreeOps = wtOps
				return next.EnsureWorktree(root, id, spec, e)
			}
			oldEvidence := nextEvidenceFile(t)
			out, err := runScopedNext(t, f, id, "--spec", "129-fixture", "--allow-not-ready="+firstSignals, "--evidence", oldEvidence)
			if err != nil {
				t.Fatalf("initial claim: %v %s", err, out)
			}
			// Give the surviving branch a unique commit before release.
			wtPath := filepath.Join(f.root, ".worktrees", "worktree-"+id)
			honestyGit(t, wtPath, "commit", "--allow-empty", "-m", "surviving bead work")
			surviving := strings.TrimSpace(honestyGit(t, f.root, "rev-parse", "bead/"+id))
			marker, validity, err := bead.ReadOverrideMarker(id)
			if err != nil || validity != bead.MarkerValid {
				t.Fatalf("initial marker %s %v", validity, err)
			}
			// Add new failures without overwriting metadata or claim status.
			data, err := os.ReadFile(f.path)
			if err != nil {
				t.Fatal(err)
			}
			var tracker map[string]any
			if err := json.Unmarshal(data, &tracker); err != nil {
				t.Fatal(err)
			}
			row := tracker["rows"].([]any)[0].(map[string]any)
			allSignals, uncovered, narrower := "MF-2,MF-3", "MF-3", "MF-3"
			switch shape {
			case "MF2-MF3":
				tracker["deps"].(map[string]any)[id] = []map[string]string{{"id": "epic-129.2", "status": "closed", "issue_type": "task", "dependency_type": "blocks"}}
			case "MF3-MF4":
				row["description"] = "TBD"
				allSignals, uncovered, narrower = "MF-3,MF-4", "MF-4", "MF-4"
			case "grown-MF3-with-MF2":
				tracker["deps"].(map[string]any)[id] = []map[string]string{
					{"id": "epic-129.2", "status": "closed", "issue_type": "task", "dependency_type": "blocks"},
					{"id": "epic-129.3", "status": "closed", "issue_type": "task", "dependency_type": "blocks"},
				}
			}
			data, err = json.Marshal(tracker)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f.path, data, 0600); err != nil {
				t.Fatal(err)
			}
			oldEval, oldRead, oldStatus := readyCheckEvaluateFn, readyCheckReadMarkerFn, readyCheckBeadFn
			readyCheckEvaluateFn, readyCheckReadMarkerFn, readyCheckBeadFn = readiness.EvaluateReadiness, bead.ReadOverrideMarker, next.FetchBeadByID
			defer func() { readyCheckEvaluateFn, readyCheckReadMarkerFn, readyCheckBeadFn = oldEval, oldRead, oldStatus }()
			before := f.audit(t)
			writesBefore := scopedTrackerUpdates(t, f)
			_, err = runBeadReadyCheck(t, id)
			if err == nil {
				t.Fatal("mixed coverage passed")
			}
			requireText(t, err.Error(), "uncovered: "+uncovered, "recovery: mindspec release "+id, "recovery: mindspec next "+id+" --allow-not-ready="+allSignals+" --evidence <path-to-justification>")
			if f.audit(t) != before || scopedTrackerUpdates(t, f) != writesBefore {
				t.Fatal("ready-check mutated")
			}
			// Invoke release's own path, with the existing git-worktree I/O seam.
			deps := releaseDeps{
				root: f.root, beadWorktreePath: wtPath,
				checkDirty:     func(string, string) ([]string, error) { return nil, nil },
				removeWorktree: wtOps.Remove, setOpen: defaultSetOpen,
				activeBead: func(string) (string, string, bool) { return id, "epic-129", true },
				syncPhase:  func(string) {}, stdout: io.Discard, stderr: io.Discard,
			}
			if err := runRelease(deps, id, false); err != nil {
				t.Fatal(err)
			}
			if f.status(t, id) != "open" {
				t.Fatal("release did not return bead to open")
			}
			afterRelease, _, err := bead.ReadOverrideMarker(id)
			if err != nil || !reflect.DeepEqual(marker, afterRelease) {
				t.Fatalf("release changed marker: %+v %v", afterRelease, err)
			}
			// A prior marker never grants claim authority, even if it covers part.
			before = f.audit(t)
			writesBefore = scopedTrackerUpdates(t, f)
			_, err = runScopedNext(t, f, id, "--spec", "129-fixture", "--allow-not-ready="+narrower, "--evidence", nextEvidenceFile(t))
			if err == nil {
				t.Fatal("narrow re-claim succeeded")
			}
			requireText(t, err.Error(), "--allow-not-ready="+allSignals+" --evidence <path-to-justification>")
			if f.audit(t) != before || scopedTrackerUpdates(t, f) != writesBefore {
				t.Fatal("narrow re-claim mutated")
			}
			_, err = runScopedNext(t, f, id, "--spec", "129-fixture")
			if err == nil || f.audit(t) != before || scopedTrackerUpdates(t, f) != writesBefore {
				t.Fatal("unflagged re-claim acquired authority from old marker")
			}
			newEvidence := nextEvidenceFile(t)
			out, err = runScopedNext(t, f, id, "--spec", "129-fixture", "--allow-not-ready="+allSignals, "--evidence", newEvidence)
			if err != nil {
				t.Fatalf("fresh re-claim: %v %s", err, out)
			}
			replacement, v, err := bead.ReadOverrideMarker(id)
			if err != nil || v != bead.MarkerValid || strings.Join(replacement.Signals, ",") != allSignals || replacement.Evidence.Path != newEvidence || replacement.Evidence.Path == oldEvidence {
				t.Fatalf("marker was not replaced: %+v %s %v", replacement, v, err)
			}
			report, err := readiness.EvaluateReadiness(f.root, id)
			if err != nil {
				t.Fatal(err)
			}
			for _, signal := range report.FailingSignals() {
				if !reflect.DeepEqual(replacement.Fingerprints[signal.ID], readiness.FailureRecordDigests(signal)) {
					t.Fatalf("fresh digests mismatch: %+v", replacement)
				}
			}
			if got := strings.TrimSpace(honestyGit(t, f.root, "rev-parse", "bead/"+id)); got != surviving {
				t.Fatalf("surviving branch reset: %s want %s", got, surviving)
			}
			out, err = runBeadReadyCheck(t, id)
			if err != nil {
				t.Fatalf("fresh marker not honoured: %v %s", err, out)
			}
			for _, signal := range replacement.Signals {
				requireText(t, out, signal+": OVERRIDDEN")
			}
		})
	}
}

func TestReadyCheck_CycleSkillCopiesMatch(t *testing.T) {
	source, err := os.ReadFile("../../plugins/mindspec/skills/ms-bead-cycle/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile("../../.claude/skills/ms-bead-cycle/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, installed) {
		t.Fatal("cycle skill installed copy differs from canonical source")
	}
	if strings.Contains(string(source), "--allow-not-ready"+string(rune(96))) {
		t.Fatal("cycle skill still offers bare override")
	}
	requireText(t, string(source), "--allow-not-ready=<ids> --evidence <path>")
}

func TestReadyCheck_ReadinessDocContracts(t *testing.T) {
	for _, tc := range []struct {
		path    string
		phrases []string
	}{
		{"../../.mindspec/core/USAGE.md", []string{"--allow-not-ready=<ids> --evidence <path>", "mindspec bead ready-check <id>", "schema-v2", "1 byte to 64 KiB"}},
		{"../../.mindspec/domains/execution/interfaces.md", []string{"MetaKeyReadinessOverride", "{schema: 2, signals, fingerprints, overridden, evidence: {path, sha256, bytes}}", "repository-relative evidence path"}},
		{"../../.mindspec/domains/workflow/interfaces.md", []string{"opts GateOptions", "result *GateResult", "--allow-not-ready=<ids> --evidence <path>"}},
	} {
		t.Run(filepath.Base(filepath.Dir(tc.path))+"/"+filepath.Base(tc.path), func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, phrase := range tc.phrases {
				requireText(t, string(data), phrase)
			}
		})
	}
}
