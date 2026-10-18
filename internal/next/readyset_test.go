package next

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// runGitCmdForNext runs git with args in dir, with a pinned test identity
// (matching internal/resolve/draft_test.go's runGitCmd convention).
func runGitCmdForNext(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=test@example.invalid",
		"-c", "user.name=mindspec-test",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newBareGitRoot builds a hermetic (t.TempDir()-rooted) git repo with a
// single baseline commit and nothing else — no spec directory, no
// spec/<id> branch for ANY spec — so a draft-status probe against any
// specID here correctly answers "not a draft" via a real, confirmed
// non-existent branch (gitutil.BranchExistsIn refuses to answer at all
// outside a real repository).
func newBareGitRoot(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmdForNext(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitCmdForNext(t, root, "add", "-A")
	runGitCmdForNext(t, root, "commit", "-m", "baseline")
	return root
}

// newDraftFixtureRootForNext builds a hermetic on-disk pre-epic draft
// (mirroring internal/resolve/draft_test.go's newDraftFixtureRoot,
// duplicated here since that helper is unexported in another package): a
// .mindspec/specs/<specID>/spec.md directory plus a committed
// spec/<specID> branch.
func newDraftFixtureRootForNext(t *testing.T, specID string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmdForNext(t, root, "init", "-b", "main")
	specDir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	runGitCmdForNext(t, root, "add", "-A")
	runGitCmdForNext(t, root, "commit", "-m", "draft spec")
	runGitCmdForNext(t, root, "branch", "spec/"+specID)
	return root
}

// readyset_test.go pins spec 128 Bead 8's shared ready-set construction
// (R6.1), filter loudness (R6.3), and explicit --spec scope honesty
// (R6.7) at the internal/next package level — the AC-9/AC-10/AC-10b/AC-15
// unit legs that don't need a real bd/git process.

// --- SelectWork total pick semantics (R6.1, round-1 G10) ---

// TestSelectWork_SingleItemOutOfRangePickErrors is AC-9's pinned
// red-on-revert: the old single-item early return (select.go:20-22)
// bypassed pick validation, silently returning the sole item for ANY
// pick value. An explicit out-of-range pick against exactly one ready
// item must now error.
func TestSelectWork_SingleItemOutOfRangePickErrors(t *testing.T) {
	items := []BeadInfo{{ID: "a", Title: "Only one"}}
	_, err := SelectWork(items, 2)
	if err == nil {
		t.Fatal("expected an out-of-range error for --pick 2 against a single-item ready set")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected an 'out of range' error, got: %v", err)
	}
}

// TestSelectWork_NegativePickRejected pins "a negative --pick is rejected
// with an error rather than silently ignored" (AC-9).
func TestSelectWork_NegativePickRejected(t *testing.T) {
	items := []BeadInfo{{ID: "a"}, {ID: "b"}}
	_, err := SelectWork(items, -1)
	if err == nil {
		t.Fatal("expected an error for a negative --pick")
	}
}

// TestSelectWork_PickZeroMeansUnset pins "--pick 0 means unset (the
// default path)" — 0 selects the OQ-5 default (displayed row 1), never
// an error.
func TestSelectWork_PickZeroMeansUnset(t *testing.T) {
	items := []BeadInfo{{ID: "a"}, {ID: "b"}}
	got, err := SelectWork(items, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "a" {
		t.Errorf("pick=0 should default to displayed row 1, got %q", got.ID)
	}
}

// --- BuildReadySet: unscoped construction (specFlag == "") ---

func TestBuildReadySet_Unscoped(t *testing.T) {
	origRun := runBDFn
	t.Cleanup(func() { runBDFn = origRun })
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []BeadInfo{{ID: "a", Title: "First", Status: "open"}, {ID: "b", Title: "Second", Status: "open"}}
			return json.Marshal(items)
		}
		return []byte("[]"), nil
	}

	rs, err := BuildReadySet(t.TempDir(), "")
	if err != nil {
		t.Fatalf("BuildReadySet: %v", err)
	}
	if len(rs.Filtered) != 2 || len(rs.Raw) != 2 {
		t.Fatalf("expected 2 raw and 2 filtered items unscoped, got raw=%d filtered=%d", len(rs.Raw), len(rs.Filtered))
	}
	if rs.ExcludedCount() != 0 || rs.ExclusionNotice() != "" {
		t.Errorf("unscoped construction must never report exclusions, got count=%d notice=%q", rs.ExcludedCount(), rs.ExclusionNotice())
	}
}

// stubEpicForSpec stubs phase's list/show seams so FindEpicBySpecID(specID)
// resolves to epicID.
func stubEpicForSpec(t *testing.T, specNum int, specTitle, epicID string) {
	t.Helper()
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				epics := []phase.EpicInfo{{
					ID: epicID, Title: "[SPEC fixture] " + specTitle, Status: "open", IssueType: "epic",
					Metadata: map[string]interface{}{"spec_num": float64(specNum), "spec_title": specTitle},
				}}
				return json.Marshal(epics)
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
}

// TestBuildReadySet_ScopedFiltersByTitle_ExclusionAccounted pins R6.1's
// shared construction plus R6.3(a)'s loud partial-exclusion accounting.
func TestBuildReadySet_ScopedFiltersByTitle_ExclusionAccounted(t *testing.T) {
	specID := "042-fixture"
	stubEpicForSpec(t, 42, "fixture", "epic-1")

	origRun := runBDFn
	t.Cleanup(func() { runBDFn = origRun })
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []BeadInfo{
				{ID: "epic-1.1", Title: "[042-fixture] A", Status: "open"},
				{ID: "epic-1.2", Title: "[042-fixture] B", Status: "open"},
				{ID: "epic-1.3", Title: "unrelated title", Status: "open"},
			}
			return json.Marshal(items)
		}
		return []byte("[]"), nil
	}

	rs, err := BuildReadySet(t.TempDir(), specID)
	if err != nil {
		t.Fatalf("BuildReadySet: %v", err)
	}
	if len(rs.Raw) != 3 {
		t.Fatalf("expected 3 raw items, got %d", len(rs.Raw))
	}
	if len(rs.Filtered) != 2 {
		t.Fatalf("expected 2 filtered items, got %d: %v", len(rs.Filtered), rs.Filtered)
	}
	if rs.ExcludedCount() != 1 {
		t.Errorf("expected ExcludedCount()==1, got %d", rs.ExcludedCount())
	}
	notice := rs.ExclusionNotice()
	if !strings.Contains(notice, "1") || !strings.Contains(notice, specID) {
		t.Errorf("expected the exclusion notice to name the count and spec, got %q", notice)
	}
}

// TestBuildReadySet_EmptyAfterFilter_LoudError pins R6.3(c)/AC-10b: a
// non-empty raw ready set that filters to EMPTY is a loud error naming
// the exclusions — never a silent empty success the caller could render
// as "No ready work found".
func TestBuildReadySet_EmptyAfterFilter_LoudError(t *testing.T) {
	specID := "042-fixture"
	stubEpicForSpec(t, 42, "fixture", "epic-1")

	origRun := runBDFn
	t.Cleanup(func() { runBDFn = origRun })
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			items := []BeadInfo{
				{ID: "epic-1.1", Title: "unrelated title one", Status: "open"},
				{ID: "epic-1.2", Title: "unrelated title two", Status: "open"},
			}
			return json.Marshal(items)
		}
		return []byte("[]"), nil
	}

	rs, err := BuildReadySet(t.TempDir(), specID)
	if err == nil {
		t.Fatalf("expected a loud error for an all-excluded raw ready set, got a successful ReadySet: %+v", rs)
	}
	if strings.Contains(err.Error(), "No ready work found") {
		t.Errorf("empty-after-filter must never read as the ordinary 'No ready work found' success line, got: %v", err)
	}
	for _, id := range []string{"epic-1.1", "epic-1.2"} {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("expected the error to name excluded bead %s, got: %v", id, err)
		}
	}
}

// TestBuildReadySet_GenuinelyEmptyRawSet_NoErrorNoExclusion pins the
// non-error counterpart: bd ready itself returning nothing is NOT an
// "all excluded" case — the caller's existing "No ready work found"
// handling applies, so BuildReadySet must return a plain empty set, not
// an error.
func TestBuildReadySet_GenuinelyEmptyRawSet_NoErrorNoExclusion(t *testing.T) {
	specID := "042-fixture"
	stubEpicForSpec(t, 42, "fixture", "epic-1")

	origRun := runBDFn
	t.Cleanup(func() { runBDFn = origRun })
	runBDFn = func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}

	rs, err := BuildReadySet(t.TempDir(), specID)
	if err != nil {
		t.Fatalf("a genuinely empty raw ready set must not be an error, got: %v", err)
	}
	if len(rs.Filtered) != 0 || len(rs.Raw) != 0 {
		t.Errorf("expected an empty ReadySet, got raw=%d filtered=%d", len(rs.Raw), len(rs.Filtered))
	}
}

// --- R6.7 scope honesty (AC-15): explicit --spec never falls back to a
// global bd ready query. ---

// TestBuildReadySet_ScopeRefusal_GenuineQueryError pins the
// genuine-bd-failure leg of scopeResolutionRefusal, and — the AC-15 core
// assertion — that the global `bd ready` query is NEVER issued when the
// scope can't be resolved.
func TestBuildReadySet_ScopeRefusal_GenuineQueryError(t *testing.T) {
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return nil, errors.New("bd: dolt unreachable")
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return nil, errors.New("bd: dolt unreachable")
	}))

	globalReadyCalled := false
	origRun := runBDFn
	t.Cleanup(func() { runBDFn = origRun })
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			globalReadyCalled = true
		}
		return []byte("[]"), nil
	}

	_, err := BuildReadySet(t.TempDir(), "042-fixture")
	if err == nil {
		t.Fatal("expected a scope-resolution refusal when the epic query genuinely fails")
	}
	if globalReadyCalled {
		t.Error("BuildReadySet must NEVER fall back to the global `bd ready` query when an explicit --spec can't be resolved (AC-15)")
	}
}

// TestBuildReadySet_ScopeRefusal_NonexistentSpec pins AC-15(i): a
// specFlag naming no epic, no draft, nothing at all — refuses naming the
// spec, never falls back to the global bd ready query.
func TestBuildReadySet_ScopeRefusal_NonexistentSpec(t *testing.T) {
	root := newBareGitRoot(t)
	// bd healthy, zero epics.
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))

	globalReadyCalled := false
	origRun := runBDFn
	t.Cleanup(func() { runBDFn = origRun })
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			globalReadyCalled = true
		}
		return []byte("[]"), nil
	}

	_, err := BuildReadySet(root, "999-does-not-exist")
	if err == nil {
		t.Fatal("expected a refusal for a spec that names no epic and no draft")
	}
	if !strings.Contains(err.Error(), "999-does-not-exist") {
		t.Errorf("expected the refusal to name the spec, got: %v", err)
	}
	if globalReadyCalled {
		t.Error("BuildReadySet must NEVER fall back to the global `bd ready` query for a nonexistent spec (AC-15(i))")
	}
}

// TestBuildReadySet_ScopeRefusal_ExistingDraft pins AC-15(ii): an
// existing on-disk draft (no epic yet) refuses with the honest
// no-epic/not-approved message, never a false "doesn't exist" claim, and
// never falls back to the global ready query.
func TestBuildReadySet_ScopeRefusal_ExistingDraft(t *testing.T) {
	specID := "042-draft-fixture"
	root := newDraftFixtureRootForNext(t, specID)
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))

	globalReadyCalled := false
	origRun := runBDFn
	t.Cleanup(func() { runBDFn = origRun })
	runBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ready" {
			globalReadyCalled = true
		}
		return []byte("[]"), nil
	}

	_, err := BuildReadySet(root, specID)
	if err == nil {
		t.Fatal("expected a no-epic refusal for an existing draft spec")
	}
	if !strings.Contains(err.Error(), "no epic") || !strings.Contains(err.Error(), "spec approve") {
		t.Errorf("expected the honest no-epic/not-approved message, got: %v", err)
	}
	if strings.Contains(err.Error(), "no spec found matching") {
		t.Errorf("an existing draft must never be reported as a nonexistent spec, got: %v", err)
	}
	if globalReadyCalled {
		t.Error("BuildReadySet must NEVER fall back to the global `bd ready` query for a draft-scoped spec (AC-15(ii))")
	}
}

// --- SelectNamedFromReadySet (R6.3(b)/AC-10) ---

func TestSelectNamedFromReadySet_ExcludedPositionalNamesReason(t *testing.T) {
	rs := ReadySet{
		Raw: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
			{ID: "mindspec-bbbb", Title: "unrelated"},
		},
		Filtered: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
		},
		SpecFlag: "042-fixture",
	}

	_, err := SelectNamedFromReadySet(rs, "mindspec-bbbb")
	if err == nil {
		t.Fatal("expected an exclusion-reason error for a positional ID the filter dropped")
	}
	if strings.Contains(err.Error(), "not found or not ready") {
		t.Errorf("AC-10: the false 'not found or not ready' message must not appear for a filter-excluded bead, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mindspec-bbbb") || !strings.Contains(err.Error(), "excluded") {
		t.Errorf("expected the error to name the excluded bead and the reason, got: %v", err)
	}
}

func TestSelectNamedFromReadySet_KeptPositionalSelectsNormally(t *testing.T) {
	rs := ReadySet{
		Raw: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
			{ID: "mindspec-bbbb", Title: "unrelated"},
		},
		Filtered: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
		},
		SpecFlag: "042-fixture",
	}

	got, err := SelectNamedFromReadySet(rs, "mindspec-aaaa")
	if err != nil {
		t.Fatalf("unexpected error selecting a kept positional ID: %v", err)
	}
	if got.ID != "mindspec-aaaa" {
		t.Errorf("expected mindspec-aaaa, got %s", got.ID)
	}
}

func TestSelectNamedFromReadySet_UnknownPositionalUsesGenericMessage(t *testing.T) {
	rs := ReadySet{
		Raw: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
		},
		Filtered: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
		},
		SpecFlag: "042-fixture",
	}

	_, err := SelectNamedFromReadySet(rs, "mindspec-zzzz")
	if err == nil {
		t.Fatal("expected an error for a positional ID absent from both raw and filtered")
	}
	if !strings.Contains(err.Error(), "not found or not ready") {
		t.Errorf("a genuinely-unknown positional ID should still get SelectWorkByName's generic message, got: %v", err)
	}
}

// --- Round-1 panel G3: selector rendering safety ---
//
// name (the caller-supplied positional selector) is agent-writable free
// text reaching SelectNamedFromReadySet/SelectWorkByName's error
// messages. These pin that a hostile name never leaks a raw control byte
// or forges a standalone recovery-shaped line, while a CLEAN selector
// (full ID or short suffix form) renders byte-identical.

func TestSelectNamedFromReadySet_HostileNameRendersEscaped(t *testing.T) {
	rs := ReadySet{
		Raw: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
			{ID: "mindspec-bbbb", Title: "unrelated"},
		},
		Filtered: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
		},
		SpecFlag: "042-fixture",
	}
	hostile := "mindspec-bbbb\x1b[31mFORGED\x1b[0m\nrecovery: forged"

	_, err := SelectNamedFromReadySet(rs, hostile)
	if err == nil {
		t.Fatal("expected an error for a hostile, genuinely-unresolvable name")
	}
	rendered := err.Error()
	if strings.ContainsRune(rendered, 0x1b) {
		t.Errorf("raw ESC byte survived into the rendered error: %q", rendered)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone recovery line leaked via the hostile name's raw newline: %q", rendered)
		}
	}
}

func TestSelectNamedFromReadySet_HostileExcludedNameRendersEscaped(t *testing.T) {
	// A hostile name can never itself exact/suffix-match a real (clean)
	// bd-returned raw item ID, so this exercises exclusionReasonForName's
	// OWN termsafe.Escape call directly (defense in depth) by hostile-
	// suffixing a real excluded ID's suffix match.
	rs := ReadySet{
		Raw: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
			{ID: "mindspec-bbbb", Title: "unrelated"},
		},
		Filtered: []BeadInfo{
			{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
		},
		SpecFlag: "042-fixture",
	}

	_, excluded := exclusionReasonForName(rs, "bbbb")
	if !excluded {
		t.Fatal("expected \"bbbb\" to match the excluded raw item mindspec-bbbb by suffix")
	}
	err, _ := exclusionReasonForName(rs, "bbbb")
	if !strings.Contains(err.Error(), "bbbb") {
		t.Errorf("expected a CLEAN short-form name to render byte-identical (not quoted), got: %v", err)
	}
	if strings.Contains(err.Error(), `"bbbb"`) {
		t.Errorf("a clean short form must not be force-quoted, got: %v", err)
	}
}

func TestSelectWorkByName_HostileNameRendersEscaped(t *testing.T) {
	items := []BeadInfo{{ID: "mindspec-aaaa", Title: "Real bead"}}
	hostile := "zzzz\x1b[31mFORGED\x1b[0m\nrecovery: forged"

	_, err := SelectWorkByName(items, hostile)
	if err == nil {
		t.Fatal("expected an error for an unresolvable hostile name")
	}
	rendered := err.Error()
	if strings.ContainsRune(rendered, 0x1b) {
		t.Errorf("raw ESC byte survived into SelectWorkByName's rendered error: %q", rendered)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone recovery line leaked via the hostile name's raw newline: %q", rendered)
		}
	}
}

// TestSelectWorkByName_AmbiguousCleanShortFormByteIdentical pins the
// "clean IDs byte-identical" half for the AMBIGUOUS message. The
// candidate list renders through idrender.Bead's validate-or-quote gate
// — NOT on an assumption that bd-returned IDs are clean (they are
// agent-writable, ADR-0042's own empirical finding; `bd create --force
// --id=<arbitrary>` is proven unsafe): a grammar-valid ID renders
// byte-identical, anything else force-quotes.
func TestSelectWorkByName_AmbiguousCleanShortFormByteIdentical(t *testing.T) {
	items := []BeadInfo{
		{ID: "mindspec-mol-qmq", Title: "First suffix match"},
		{ID: "mindspec-qmq", Title: "Second suffix match"},
	}

	_, err := SelectWorkByName(items, "qmq")
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}
	if !strings.Contains(err.Error(), "mindspec-qmq") {
		t.Errorf("expected the clean candidate IDs to render byte-identical, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mindspec-mol-qmq") {
		t.Errorf("expected BOTH real candidates to be named, got: %v", err)
	}
}

// TestSelectWorkByName_AmbiguousHostileCandidateIDsForceQuoted: two
// agent-authored hostile IDs CAN genuinely suffix-match one selector —
// the ambiguity candidate list must route them through
// idrender.Bead's force-quote, never render them raw.
func TestSelectWorkByName_AmbiguousHostileCandidateIDsForceQuoted(t *testing.T) {
	items := []BeadInfo{
		{ID: "evil\x1b[31m-qmq", Title: "First hostile match"},
		{ID: "also;evil-qmq", Title: "Second hostile match"},
	}

	_, err := SelectWorkByName(items, "qmq")
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}
	if strings.ContainsRune(err.Error(), 0x1b) {
		t.Errorf("raw ESC byte from a hostile candidate ID survived: %q", err.Error())
	}
}

// --- Final-review round 1 (R5/G4): printable-hostile selectors --------
//
// ADR-0042's motivating rule: termsafe.Escape is IDENTITY on printable
// ASCII, so control-byte pins alone cannot catch a `x;evil` or
// `$(evil)` selector surviving into a copy-pasteable command example.
// The fix removed every dynamic operand from executable-shaped
// (backtick-fenced `mindspec next ...`) spans; these pin that no
// rendered error carries a command shape with the hostile selector as
// its operand, for the exact printable classes ADR-0042 distinguishes.

func TestSelectorErrors_PrintableHostileNeverInCommandShape(t *testing.T) {
	hostiles := []string{"x;evil", "$(evil)", "a b --help", "--help", "`evil`"}

	for _, hostile := range hostiles {
		// Not-found leg (SelectWorkByName).
		_, err := SelectWorkByName([]BeadInfo{{ID: "mindspec-aaaa", Title: "real"}}, hostile)
		if err == nil {
			t.Fatalf("expected a not-found error for %q", hostile)
		}
		if strings.Contains(err.Error(), "mindspec next "+hostile) {
			t.Errorf("selector %q survived into a command-shaped span: %q", hostile, err.Error())
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("%q", hostile)) {
			t.Errorf("selector %q should still be named (quoted) for diagnosis, got: %q", hostile, err.Error())
		}

		// Excluded leg (exclusionReasonForName): force a suffix match on
		// an excluded raw item whose suffix IS the hostile text — the
		// rendered error must not embed the selector in a command shape.
		rs := ReadySet{
			Raw: []BeadInfo{
				{ID: "mindspec-aaaa", Title: "[042-fixture] kept"},
				{ID: "mindspec-" + hostile, Title: "unrelated"},
			},
			Filtered: []BeadInfo{{ID: "mindspec-aaaa", Title: "[042-fixture] kept"}},
			SpecFlag: "042-fixture",
		}
		if exclErr, excluded := exclusionReasonForName(rs, hostile); excluded {
			if strings.Contains(exclErr.Error(), "mindspec next "+hostile) {
				t.Errorf("excluded selector %q survived into a command-shaped span: %q", hostile, exclErr.Error())
			}
		}
	}
}
