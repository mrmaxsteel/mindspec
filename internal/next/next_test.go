package next

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// --- ParseBeadsJSON tests ---

func TestParseBeadsJSON_SingleItem(t *testing.T) {
	input := `[{
		"id": "mindspec-25p",
		"title": "Test bead for parsing",
		"status": "open",
		"priority": 4,
		"issue_type": "task",
		"owner": "max@enubiq.com",
		"created_at": "2026-02-12T08:50:30Z",
		"updated_at": "2026-02-12T08:50:30Z"
	}]`

	items, err := ParseBeadsJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "mindspec-25p" {
		t.Errorf("expected ID mindspec-25p, got %s", items[0].ID)
	}
	if items[0].IssueType != "task" {
		t.Errorf("expected issue_type task, got %s", items[0].IssueType)
	}
	if items[0].Priority != 4 {
		t.Errorf("expected priority 4, got %d", items[0].Priority)
	}
}

func TestParseBeadsJSON_MultipleItems(t *testing.T) {
	input := `[
		{"id": "a", "title": "First", "status": "open", "priority": 1, "issue_type": "task", "owner": "", "created_at": "", "updated_at": ""},
		{"id": "b", "title": "Second", "status": "open", "priority": 2, "issue_type": "feature", "owner": "", "created_at": "", "updated_at": ""}
	]`

	items, err := ParseBeadsJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[1].IssueType != "feature" {
		t.Errorf("expected second item type feature, got %s", items[1].IssueType)
	}
}

func TestParseBeadsJSON_EmptyArray(t *testing.T) {
	items, err := ParseBeadsJSON([]byte("[]"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestParseBeadsJSON_InvalidJSON(t *testing.T) {
	_, err := ParseBeadsJSON([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseBeadsJSON_MoleculeReadyPayload(t *testing.T) {
	input := `{
		"molecule_id": "mol-123",
		"steps": [
			{"issue": {"id":"mol-123","title":"Parent","status":"in_progress","issue_type":"epic"}},
			{"issue": {"id":"impl-1","title":"Implement","status":"open","issue_type":"task"}},
			{"issue": {"id":"closed-1","title":"Closed","status":"closed","issue_type":"task"}}
		]
	}`

	items, err := ParseBeadsJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "impl-1" {
		t.Errorf("expected impl-1, got %s", items[0].ID)
	}
}

// --- SelectWork tests ---

func TestSelectWork_SingleItem(t *testing.T) {
	items := []BeadInfo{{ID: "a", Title: "Only one"}}
	result, err := SelectWork(items, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "a" {
		t.Errorf("expected ID a, got %s", result.ID)
	}
}

func TestSelectWork_MultipleDefaultsToFirst(t *testing.T) {
	items := []BeadInfo{
		{ID: "a", Title: "First"},
		{ID: "b", Title: "Second"},
	}
	result, err := SelectWork(items, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "a" {
		t.Errorf("expected ID a, got %s", result.ID)
	}
}

func TestSelectWork_PickSpecific(t *testing.T) {
	items := []BeadInfo{
		{ID: "a", Title: "First"},
		{ID: "b", Title: "Second"},
		{ID: "c", Title: "Third"},
	}
	result, err := SelectWork(items, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "b" {
		t.Errorf("expected ID b, got %s", result.ID)
	}
}

func TestSelectWork_PickOutOfRange(t *testing.T) {
	items := []BeadInfo{
		{ID: "a", Title: "First"},
		{ID: "b", Title: "Second"},
	}
	_, err := SelectWork(items, 5)
	if err == nil {
		t.Fatal("expected error for out of range pick")
	}
}

func TestSelectWork_EmptyList(t *testing.T) {
	_, err := SelectWork([]BeadInfo{}, 0)
	if err == nil {
		t.Fatal("expected error for empty list")
	}
}

// --- SelectWorkByName tests (spec 101 R1 / mindspec-3cj0.1) ---

// A named bead that IS in the ready set selects exactly that bead, even when
// it is not items[0]. The pre-R1 code ignored the name and returned items[0].
func TestSelectWorkByName_NamedInSetNotFirst(t *testing.T) {
	items := []BeadInfo{
		{ID: "a", Title: "First"},
		{ID: "b", Title: "Second"},
		{ID: "c", Title: "Third"},
	}
	result, err := SelectWorkByName(items, "b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "b" {
		t.Errorf("expected named bead b, got %s", result.ID)
	}
}

// A named bead NOT in the ready set returns a clear error — never items[0].
func TestSelectWorkByName_NamedNotInSet(t *testing.T) {
	items := []BeadInfo{
		{ID: "a", Title: "First"},
		{ID: "b", Title: "Second"},
	}
	result, err := SelectWorkByName(items, "zzz")
	if err == nil {
		t.Fatalf("expected error for bead not in ready set, got %v", result)
	}
	if result.ID == "a" {
		t.Errorf("must not fall through to items[0]; got %s", result.ID)
	}
}

// A positional bead ID supplied in EITHER the short form ("xxxx") or the full
// prefixed form ("mindspec-xxxx") resolves to the same ready bead, even when it
// is not items[0]. RED before R3 (short form fails the exact `==` match).
func TestSelectWorkByName_ShortAndFullForm(t *testing.T) {
	items := []BeadInfo{
		{ID: "mindspec-aaaa", Title: "First"},
		{ID: "mindspec-xxxx", Title: "Target"},
		{ID: "mindspec-bbbb", Title: "Third"},
	}
	for _, name := range []string{"xxxx", "mindspec-xxxx"} {
		result, err := SelectWorkByName(items, name)
		if err != nil {
			t.Fatalf("SelectWorkByName(%q) unexpected error: %v", name, err)
		}
		if result.ID != "mindspec-xxxx" {
			t.Errorf("SelectWorkByName(%q): expected mindspec-xxxx, got %s", name, result.ID)
		}
	}
}

// A suffix that matches no bead in NEITHER short nor full form returns the
// not-in-ready-set error and never falls through to items[0] (spec 101 R1
// guarantee preserved under suffix-aware matching).
func TestSelectWorkByName_SuffixNotInSet(t *testing.T) {
	items := []BeadInfo{
		{ID: "mindspec-aaaa", Title: "First"},
		{ID: "mindspec-bbbb", Title: "Second"},
	}
	result, err := SelectWorkByName(items, "zzzz")
	if err == nil {
		t.Fatalf("expected error for suffix not in ready set, got %v", result)
	}
	if result.ID == "mindspec-aaaa" {
		t.Errorf("must not fall through to items[0]; got %s", result.ID)
	}
}

// An exact full-ID match must win even when an EARLIER item in the slice
// suffix-matches the same trailing segment. Exact intent is never shadowed by
// a looser suffix match (e.g. claiming "mindspec-qmq" must not resolve to an
// earlier "mindspec-mol-qmq" that shares the "-qmq" suffix).
func TestSelectWorkByName_ExactWinsOverEarlierSuffix(t *testing.T) {
	items := []BeadInfo{
		{ID: "mindspec-mol-qmq", Title: "Earlier suffix match"},
		{ID: "mindspec-qmq", Title: "Exact target"},
	}
	result, err := SelectWorkByName(items, "mindspec-qmq")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "mindspec-qmq" {
		t.Errorf("exact match must win; expected mindspec-qmq, got %s", result.ID)
	}
}

// A SHORT form that suffix-matches more than one ready item is genuinely
// ambiguous: return an error (telling the caller to qualify with the full ID)
// rather than silently claim an order-dependent arbitrary bead.
func TestSelectWorkByName_AmbiguousShortFormErrors(t *testing.T) {
	items := []BeadInfo{
		{ID: "mindspec-mol-qmq", Title: "First suffix match"},
		{ID: "mindspec-qmq", Title: "Second suffix match"},
	}
	result, err := SelectWorkByName(items, "qmq")
	if err == nil {
		t.Fatalf("expected ambiguity error for short form matching >1 item, got %v", result)
	}
	if result.ID != "" {
		t.Errorf("ambiguous match must not resolve to a bead; got %s", result.ID)
	}
}

// An empty name can never name a specific bead and must error, never matching
// an ID that merely ends in "-" (defense-in-depth; caller already guards).
func TestSelectWorkByName_EmptyNameErrors(t *testing.T) {
	items := []BeadInfo{
		{ID: "mindspec-mol-", Title: "Trailing hyphen"},
		{ID: "mindspec-aaaa", Title: "Real bead"},
	}
	result, err := SelectWorkByName(items, "")
	if err == nil {
		t.Fatalf("expected error for empty name, got %v", result)
	}
	if result.ID != "" {
		t.Errorf("empty name must not resolve to a bead; got %s", result.ID)
	}
}

func TestFormatWorkList(t *testing.T) {
	items := []BeadInfo{
		{ID: "abc", Title: "Do something", Priority: 2, IssueType: "task"},
		{ID: "def", Title: "Plan feature", Priority: 1, IssueType: "feature"},
	}
	result := FormatWorkList(items)
	if result == "" {
		t.Fatal("expected non-empty format output")
	}
	if !contains(result, "abc") || !contains(result, "def") {
		t.Errorf("format output missing item IDs: %s", result)
	}
	if !contains(result, "1.") || !contains(result, "2.") {
		t.Errorf("format output missing numbering: %s", result)
	}
}

// --- ResolveMode tests ---

func TestResolveMode_Task(t *testing.T) {
	bead := BeadInfo{ID: "x", Title: "005-next: Implement something", IssueType: "task"}
	result := ResolveMode("/nonexistent", bead)
	if result.Mode != "implement" {
		t.Errorf("expected implement, got %s", result.Mode)
	}
	if result.SpecID != "005-next" {
		t.Errorf("expected spec ID 005-next, got %s", result.SpecID)
	}
}

func TestResolveMode_Bug(t *testing.T) {
	bead := BeadInfo{ID: "x", Title: "003-context: Fix rendering", IssueType: "bug"}
	result := ResolveMode("/nonexistent", bead)
	if result.Mode != "implement" {
		t.Errorf("expected implement, got %s", result.Mode)
	}
	if result.SpecID != "003-context" {
		t.Errorf("expected spec ID 003-context, got %s", result.SpecID)
	}
}

func TestResolveMode_Feature_NoSpec(t *testing.T) {
	bead := BeadInfo{ID: "x", Title: "099-future: New feature", IssueType: "feature"}
	result := ResolveMode("/nonexistent", bead)
	if result.Mode != "spec" {
		t.Errorf("expected spec, got %s", result.Mode)
	}
}

func TestResolveMode_Feature_ApprovedSpec(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "010-test")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	// YAML frontmatter `status: Approved` is the contract — the resolver
	// no longer substring-matches "Status: APPROVED" in prose (ZFC: a
	// markdown body heuristic is not a reliable workflow signal).
	specContent := "---\nstatus: Approved\nspec_id: \"010-test\"\n---\n\n# Spec\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	bead := BeadInfo{ID: "x", Title: "010-test: Plan a feature", IssueType: "feature"}
	result := ResolveMode(tmp, bead)
	if result.Mode != "plan" {
		t.Errorf("expected plan, got %s", result.Mode)
	}
}

func TestResolveMode_Feature_DraftSpec(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "docs", "specs", "010-test")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := "---\nstatus: Draft\nspec_id: \"010-test\"\n---\n\n# Spec\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	bead := BeadInfo{ID: "x", Title: "010-test: Draft feature", IssueType: "feature"}
	result := ResolveMode(tmp, bead)
	if result.Mode != "spec" {
		t.Errorf("expected spec, got %s", result.Mode)
	}
}

// TestResolveMode_Feature_EpicMetadataPrimary exercises the authoritative
// path — when a beads epic exists, its mindspec_phase metadata is the signal,
// not the spec.md frontmatter. This is the ZFC-compliant replacement for the
// old raw-markdown "Status: APPROVED" substring check.
func TestResolveMode_Feature_EpicMetadataPrimary(t *testing.T) {
	cases := []struct {
		name     string
		phase    string
		wantMode string
	}{
		{"spec-phase → spec", "spec", "spec"},
		{"plan-phase → plan", "plan", "plan"},
		{"implement-phase → plan", "implement", "plan"},
		{"review-phase → plan", "review", "plan"},
		{"done-phase → idle", "done", "idle"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Stub phase-layer bd calls. FindEpicBySpecID enumerates epics
			// via bd list --type=epic; DerivePhase reads mindspec_phase via
			// bd show <id> --json. Our stubs answer just enough for both.
			restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
				for _, a := range args {
					if a == "--type=epic" {
						epics := []phase.EpicInfo{{
							ID:        "epic-999",
							Title:     "[SPEC 011-metatest] Metadata test",
							Status:    "open",
							IssueType: "epic",
							Metadata: map[string]interface{}{
								"spec_num":       float64(11),
								"spec_title":     "metatest",
								"mindspec_phase": tc.phase,
							},
						}}
						return json.Marshal(epics)
					}
				}
				return []byte("[]"), nil
			})
			defer restoreList()

			restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "show" {
					items := []phase.EpicInfo{{
						ID: "epic-999",
						Metadata: map[string]interface{}{
							"mindspec_phase": tc.phase,
						},
					}}
					return json.Marshal(items)
				}
				return []byte("[]"), nil
			})
			defer restoreRun()

			bead := BeadInfo{ID: "x", Title: "011-metatest: Metadata-driven route", IssueType: "feature"}
			result := ResolveMode(t.TempDir(), bead)
			if result.Mode != tc.wantMode {
				t.Errorf("phase=%q: got mode %q, want %q", tc.phase, result.Mode, tc.wantMode)
			}
		})
	}
}

// TestResolveModeHostileTitle is spec 120 AC-23's round-5 consumer-
// boundary test: a bd Title of the form "[IMPL 120-x;evil.1] …" — and the
// 116 hostile control-byte triple — yields ResolvedWork.SpecID == ""; the
// epic-lookup seam (findEpicForModeFn) is stubbed and asserts NO lookup is
// attempted with the malformed value; a clean "[IMPL 009-feature.1]"
// title parses byte-identically to today. The NO-recording-write leg
// (round-6 F2) is doubly covered by the explicit cross-reference to
// internal/recording's TestRecordingWriteGates — the class-5 write-gate —
// together with the existing cmd/mindspec/next.go:287-293 `SpecID != ""`
// guard this test's ResolvedWork.SpecID == "" assertion feeds.
func TestResolveModeHostileTitle(t *testing.T) {
	origFn := findEpicForModeFn
	defer func() { findEpicForModeFn = origFn }()

	hostileTitles := []string{
		"[IMPL 120-x;evil.1] pwn",
		"[IMPL x\x00\x1b[31m\nrecovery: forged.1] pwn",
	}
	for _, title := range hostileTitles {
		var lookupCalled bool
		findEpicForModeFn = func(specID string) (string, error) {
			lookupCalled = true
			return "", fmt.Errorf("epic-lookup should never be called for a malformed derived specID")
		}
		bead := BeadInfo{ID: "x", Title: title, IssueType: "feature"}
		result := ResolveMode(t.TempDir(), bead)
		if result.SpecID != "" {
			t.Errorf("title %q: SpecID = %q, want \"\" (malformed derivation)", title, result.SpecID)
		}
		if lookupCalled {
			t.Errorf("title %q: epic-lookup seam must NEVER be called with a malformed derived specID", title)
		}
		if strings.ContainsAny(result.SpecID, ";\x00\x1b") {
			t.Errorf("title %q: SpecID carries raw hostile bytes: %q", title, result.SpecID)
		}
	}

	// Clean title parses byte-identically to today.
	findEpicForModeFn = origFn
	cleanBead := BeadInfo{ID: "x", Title: "[IMPL 009-feature.1] Chunk title", IssueType: "task"}
	cleanResult := ResolveMode(t.TempDir(), cleanBead)
	if cleanResult.SpecID != "009-feature" {
		t.Errorf("clean title SpecID = %q, want 009-feature", cleanResult.SpecID)
	}
}

func TestResolveMode_NoColonInTitle(t *testing.T) {
	bead := BeadInfo{ID: "x", Title: "No colon here", IssueType: "task"}
	result := ResolveMode("/nonexistent", bead)
	if result.Mode != "implement" {
		t.Errorf("expected implement, got %s", result.Mode)
	}
	if result.SpecID != "" {
		t.Errorf("expected empty spec ID, got %s", result.SpecID)
	}
}

// --- parseSpecID tests ---

func TestParseSpecID(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"[IMPL 009-feature.1] Chunk title", "009-feature"},
		{"[IMPL 009-workflow-gaps.2] Approval enhancements", "009-workflow-gaps"},
		{"[SPEC 008b-gates] Human Gates Feature", "008b-gates"},
		{"[PLAN 009-feature] Plan decomposition", "009-feature"},
		// Spec 120 (ADR-0042 reverse-derivation consumer gate, AC-24
		// empty-sentinel discipline): a bare numeric slug with no
		// "-slug" tail is not a well-formed spec ID under the corrected
		// idvalidate.SpecID grammar (which requires <NNN>-<slug>) — the
		// derivation gate discards it and returns "" (the existing
		// no-spec sentinel), same as any other malformed derived value.
		{"[IMPL 001.3] Simple numeric", ""},
		// No-tag bracket format: [specID] Bead N: title
		{"[049-hook-command] Bead 1: Core hook infrastructure", "049-hook-command"},
		{"[010-spec-init] Bead 3: Worktree creation", "010-spec-init"},
		{"005-next: Implement work selection", "005-next"},
		{"003-context: Fix rendering bug", "003-context"},
		{"No colon here", ""},
		// Spec 120: "simple" has no leading digits, so it fails
		// idvalidate.SpecID and the gate returns "" (was never a
		// real spec-dir shape; every actual colon-convention title is
		// "NNN-slug: …").
		{"simple:", ""},
		{": leading colon", ""},
	}
	for _, tt := range tests {
		result := parseSpecID(tt.title)
		if result != tt.expected {
			t.Errorf("parseSpecID(%q) = %q, want %q", tt.title, result, tt.expected)
		}
	}
}

// --- QueryReady tests ---

func TestQueryReady_GlobalReady(t *testing.T) {
	origRunBD := runBDFn
	defer func() {
		runBDFn = origRunBD
	}()

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "ready" && args[1] == "--json" {
			items := []BeadInfo{
				{ID: "standalone-1", Title: "Standalone work"},
			}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("not available")
	}

	items, err := QueryReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "standalone-1" {
		t.Errorf("items[0].ID: got %q, want %q", items[0].ID, "standalone-1")
	}
}

// --- ClaimBead tests ---

func TestClaimBead_CallsRunBDCombined(t *testing.T) {
	origRunBDComb := runBDCombFn
	defer func() { runBDCombFn = origRunBDComb }()

	var capturedArgs []string
	runBDCombFn = func(args ...string) ([]byte, error) {
		capturedArgs = args
		return nil, nil
	}

	err := ClaimBead("bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedArgs) != 3 || capturedArgs[0] != "update" || capturedArgs[1] != "bead-abc" || capturedArgs[2] != "--claim" {
		t.Errorf("unexpected args: %v", capturedArgs)
	}
}

// TestNextClaimRejectsMalformedBeadID is spec 120 AC-6 (internal/next):
// ClaimBead's gate-all-ids validation refuses a malformed id before any
// bd spawn (never reaching runBDCombFn); and the ready-set claim seam
// (SelectWorkByName, the explicit-claim path `mindspec next <bead-id>`
// drives) is inert-by-construction — a malformed positional bead name can
// never resolve to a real ready-set entry, so the explicit claim of a
// malformed ID always refuses before ClaimBead is ever reached.
func TestNextClaimRejectsMalformedBeadID(t *testing.T) {
	origRunBDComb := runBDCombFn
	defer func() { runBDCombFn = origRunBDComb }()

	hostileIDs := []string{"--help", "x;evil", "x\x00\x1b[31m\nrecovery: forged"}
	for _, hostile := range hostileIDs {
		var spawned bool
		runBDCombFn = func(args ...string) ([]byte, error) {
			spawned = true
			return nil, nil
		}
		if err := ClaimBead(hostile); err == nil {
			t.Errorf("ClaimBead(%q) accepted a hostile id", hostile)
		}
		if spawned {
			t.Errorf("ClaimBead(%q) spawned bd before the gate refused", hostile)
		}
	}

	// The ready-set claim seam: an explicit claim of a malformed ID names
	// no real ready-set entry, so it refuses at SelectWorkByName — never
	// reaching ClaimBead.
	items := []BeadInfo{{ID: "mindspec-real", Title: "Real work", Status: "open"}}
	for _, hostile := range hostileIDs {
		if _, err := SelectWorkByName(items, hostile); err == nil {
			t.Errorf("SelectWorkByName(%q) resolved a hostile positional name against the ready set", hostile)
		}
	}
}

func TestClaimBead_PropagatesError(t *testing.T) {
	origRunBDComb := runBDCombFn
	defer func() { runBDCombFn = origRunBDComb }()

	runBDCombFn = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("bd update failed")
	}

	err := ClaimBead("bead-abc")
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestClaimBead_SurfacesRealStderr pins the R3 fix: a schema-drift failure
// from a stale bd binary must reach the caller verbatim, NOT flattened to the
// misleading "may already be claimed" prefix. The contains-stderr assertion is
// vacuous on the pre-fix code (it already embedded string(out)); the
// load-bearing RED assertion is the NOT-contains-"may already be claimed" one,
// which only passes once the misleading prefix is removed.
func TestClaimBead_SurfacesRealStderr(t *testing.T) {
	origRunBDComb := runBDCombFn
	defer func() { runBDCombFn = origRunBDComb }()

	const schemaErr = `column "depends_on_id" could not be found`
	runBDCombFn = func(args ...string) ([]byte, error) {
		return []byte(schemaErr), fmt.Errorf("exit status 1")
	}

	err := ClaimBead("bead-abc")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "depends_on_id") {
		t.Errorf("error should surface bd's real stderr; got %q", msg)
	}
	if strings.Contains(msg, "may already be claimed") {
		t.Errorf("error must NOT be flattened to the misleading prefix; got %q", msg)
	}
}

// --- EnsureWorktree tests ---

func TestEnsureWorktree_DelegatesToExecutor(t *testing.T) {
	root := t.TempDir()
	expectedPath := filepath.Join(root, ".worktrees", "worktree-bead-abc")

	mock := &executor.MockExecutor{
		DispatchBeadResult: executor.WorkspaceInfo{
			Path:   expectedPath,
			Branch: "bead/bead-abc",
		},
	}

	path, err := EnsureWorktree(root, "bead-abc", "046-test", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != expectedPath {
		t.Errorf("path: got %q, want %q", path, expectedPath)
	}

	// Verify executor was called with correct args.
	calls := mock.CallsTo("DispatchBead")
	if len(calls) != 1 {
		t.Fatalf("expected 1 DispatchBead call, got %d", len(calls))
	}
	if calls[0].Args[0] != "bead-abc" {
		t.Errorf("DispatchBead beadID = %v, want bead-abc", calls[0].Args[0])
	}
	if calls[0].Args[1] != "046-test" {
		t.Errorf("DispatchBead specID = %v, want 046-test", calls[0].Args[1])
	}
}

func TestEnsureWorktree_PropagatesError(t *testing.T) {
	mock := &executor.MockExecutor{
		DispatchBeadErr: fmt.Errorf("worktree creation failed"),
	}

	_, err := EnsureWorktree(t.TempDir(), "bead-xyz", "", mock)
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "worktree creation failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEnsureWorktree_EmptySpecID(t *testing.T) {
	root := t.TempDir()
	expectedPath := filepath.Join(root, ".worktrees", "worktree-bead-abc")

	mock := &executor.MockExecutor{
		DispatchBeadResult: executor.WorkspaceInfo{
			Path:   expectedPath,
			Branch: "bead/bead-abc",
		},
	}

	path, err := EnsureWorktree(root, "bead-abc", "", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != expectedPath {
		t.Errorf("path: got %q, want %q", path, expectedPath)
	}

	// Empty specID should be passed through to executor.
	calls := mock.CallsTo("DispatchBead")
	if calls[0].Args[1] != "" {
		t.Errorf("DispatchBead specID = %v, want empty string", calls[0].Args[1])
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- FetchBeadByID tests ---

func TestFetchBeadByID_ArrayResponse(t *testing.T) {
	orig := runBDFn
	defer func() { runBDFn = orig }()

	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "show" && args[1] == "bead-abc" {
			items := []BeadInfo{{
				ID:    "bead-abc",
				Title: "[IMPL 047.1] Test bead",
			}}
			return json.Marshal(items)
		}
		return nil, fmt.Errorf("unexpected: %v", args)
	}

	info, err := FetchBeadByID("bead-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "bead-abc" {
		t.Errorf("ID = %q, want %q", info.ID, "bead-abc")
	}
	if info.Title != "[IMPL 047.1] Test bead" {
		t.Errorf("Title = %q", info.Title)
	}
}

func TestFetchBeadByID_SingleObjectResponse(t *testing.T) {
	orig := runBDFn
	defer func() { runBDFn = orig }()

	runBDFn = func(args ...string) ([]byte, error) {
		item := BeadInfo{ID: "bead-xyz", Title: "Single object"}
		return json.Marshal(item)
	}

	info, err := FetchBeadByID("bead-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "bead-xyz" {
		t.Errorf("ID = %q, want %q", info.ID, "bead-xyz")
	}
}

func TestFetchBeadByID_NotFound(t *testing.T) {
	orig := runBDFn
	defer func() { runBDFn = orig }()

	runBDFn = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("bead not found")
	}

	_, err := FetchBeadByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent bead")
	}
}

func TestFetchBeadByID_EmptyArray(t *testing.T) {
	orig := runBDFn
	defer func() { runBDFn = orig }()

	runBDFn = func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}

	_, err := FetchBeadByID("bead-empty")
	if err == nil {
		t.Fatal("expected error for empty array response")
	}
}
