package next

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// Acceptance fixtures fail closed when a required executable is missing.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is required for readiness acceptance tests")
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
	origRun := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRun })
	readyJSONFn = func(args ...string) ([]byte, error) {
		if len(args) == 0 {
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
	f := newNearMissFixture(t, false)
	f.rows = []BeadInfo{f.row("epic-129.1", "", "epic-129", "task"), f.row("nonplan-r", "", "epic-129", "task")}
	f.save(t)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Filtered) != 1 || rs.Filtered[0].ID != "epic-129.1" || rs.ExcludedCount() != 1 || !strings.Contains(rs.ExclusionNotice(), "nonplan-r") {
		t.Fatalf("wrong plan-bead accounting: %+v", rs)
	}
}

func TestBuildReadySet_EmptyAfterFilter_LoudError(t *testing.T) {
	f := newNearMissFixture(t, false)
	f.rows = []BeadInfo{f.row("nonplan-r", "", "epic-129", "task")}
	f.save(t)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err == nil || !strings.Contains(err.Error(), "nonplan-r") || !rs.HasBlockingNearMisses() {
		t.Fatalf("blocking near-miss must be loud: %+v %v", rs, err)
	}
	if len(rs.NearMisses) != 1 || rs.NearMisses[0].Class != "N3" || !strings.Contains(err.Error(), "but it is not a plan bead") {
		t.Fatalf("non-plan descendant must be excluded and named truthfully: %+v %v", rs, err)
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

	origRun := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRun })
	readyJSONFn = func(args ...string) ([]byte, error) {
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
	origRun := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRun })
	readyJSONFn = func(args ...string) ([]byte, error) {
		globalReadyCalled = true
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
	origRun := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRun })
	readyJSONFn = func(args ...string) ([]byte, error) {
		globalReadyCalled = true
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
	origRun := readyJSONFn
	t.Cleanup(func() { readyJSONFn = origRun })
	readyJSONFn = func(args ...string) ([]byte, error) {
		globalReadyCalled = true
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
	if !strings.Contains(err.Error(), "mindspec-bbbb") || !strings.Contains(err.Error(), "not linked") {
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

	_, err := SelectNamedFromReadySet(rs, "bbbb")
	excluded := err != nil
	if !excluded {
		t.Fatal("expected \"bbbb\" to match the excluded raw item mindspec-bbbb by suffix")
	}
	err = func() error { _, e := SelectNamedFromReadySet(rs, "bbbb"); return e }()
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
		if _, exclErr := SelectNamedFromReadySet(rs, hostile); exclErr != nil {
			if strings.Contains(exclErr.Error(), "mindspec next "+hostile) {
				t.Errorf("excluded selector %q survived into a command-shaped span: %q", hostile, exclErr.Error())
			}
		}
	}
}

func TestBuildReadySet_PlanBeadPredicate(t *testing.T) {
	f := newNearMissFixture(t, false)
	// Preserve priority order even when it differs from lexical ID order.
	f.rows[0], f.rows[2] = f.rows[2], f.rows[0]
	// Establish the fake's adverse behaviour before testing the production waist.
	original := f.rows
	for len(f.rows) < 101 {
		f.rows = append(f.rows, f.row(fmt.Sprintf("unrelated-%d", len(f.rows)), "neutral", "", "task"))
	}
	f.save(t)
	for _, args := range [][]string{{"ready"}, {"ready", "-n", "0"}, {"ready", "--parent", "epic-129", "-n", "0"}} {
		raw, err := exec.Command(filepath.Join(f.root, "bd"), args...).Output()
		if err != nil {
			t.Fatal(err)
		}
		var rows []BeadInfo
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		switch len(args) {
		case 1:
			if len(rows) != 100 {
				t.Fatalf("fake default cap: %d", len(rows))
			}
		case 3:
			if len(rows) != 101 {
				t.Fatalf("fake unlimited: %d", len(rows))
			}
		default:
			for _, row := range rows {
				if row.Parent != "epic-129" {
					t.Fatal("fake --parent must be edge-only")
				}
			}
			if len(rows) == 0 {
				t.Fatal("edge-only fake dropped all real edges")
			}
		}
	}
	f.rows = original
	f.save(t)
	if err := os.WriteFile(f.path+".argv", nil, 0600); err != nil {
		t.Fatal(err)
	}

	rs, err := BuildReadySet(f.root, "129-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids(rs.Filtered), []string{"epic-129.3", "epic-129.2", "epic-129.1", "epic-129.8"}) {
		t.Fatalf("members: %v", ids(rs.Filtered))
	}
	want := map[string]string{"epic-129.4": "N1", "epic-129.5": "N2a", "epic-129.7": "N1", "epic-129.12": "N2c", "nonplan-r": "N3", "epic-129.9": "N4", "epic-129.10": "N4", "epic-129.11": "N4", "nonplan-f": "N3", "nonplan-m": "N3", "nonplan-t": "N3"}
	if len(rs.NearMisses) != len(want) {
		t.Fatalf("near misses: %+v", rs.NearMisses)
	}
	for id, class := range want {
		m := missByID(t, rs, id)
		blocking := class != "N4" && id != "nonplan-f" && id != "nonplan-m" && id != "nonplan-t"
		if m.Class != class || m.Blocking != blocking {
			t.Errorf("%s: %+v", id, m)
		}
	}
	raw, err := os.ReadFile(f.path + ".argv")
	if err != nil {
		t.Fatal(err)
	}
	readyCalls, epicLists := 0, 0
	shows := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		if args[0] == "ready" {
			readyCalls++
			uncapped := false
			for i, arg := range args {
				if arg == "--parent" || strings.HasPrefix(arg, "--parent=") {
					t.Fatalf("scoped ready: %v", args)
				}
				if arg == "-n" && i+1 < len(args) && args[i+1] == "0" {
					uncapped = true
				}
			}
			if !uncapped {
				t.Fatalf("scoped/capped ready: %s", line)
			}
		}
		if args[0] == "show" {
			shows[args[1]]++
		}
		if args[0] == "list" && strings.Contains(line, "--type=epic") {
			epicLists++
		}
	}
	if readyCalls != 1 || epicLists != 1 {
		t.Fatalf("ready calls=%d epic lists=%d (one shared cache required)", readyCalls, epicLists)
	}
	for _, row := range f.rows {
		if _, form := bead.PlanBeadIndex("epic-129", row.ID); form {
			count := 1
			if shows[row.ID] != count {
				t.Errorf("bd show %s called %d times; want %d for class %s", row.ID, shows[row.ID], count, want[row.ID])
			}
		}
	}
	// Positional heading 8 is labelled 12; it still maps bead .8.
	f.deps = map[string][]map[string]string{}
	f.rows = []BeadInfo{f.row("epic-129.8", "neutral", "epic-129", "bug")}
	f.save(t)
	rs, err = BuildReadySet(f.root, "129-fixture")
	if err != nil || len(rs.Filtered) != 1 {
		t.Fatalf("positional mapping: %+v %v", rs, err)
	}
}

func TestBuildReadySet_LazyMapping(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		for _, mapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("foreign=%t/mapped=%t", foreign, mapped), func(t *testing.T) {
				f := newNearMissFixture(t, false)
				title := "neutral"
				if foreign {
					title = "[777] other"
				}
				f.rows = []BeadInfo{f.row("epic-129.1", title, "", "task")}
				f.save(t)
				plan := fixturePlan(1, 2)
				if mapped {
					plan = fixturePlan(1, 1)
				}
				if err := os.WriteFile(filepath.Join(f.root, ".mindspec/specs/129-fixture/plan.md"), []byte(plan), 0644); err != nil {
					t.Fatal(err)
				}
				rs, err := BuildReadySet(f.root, "129-fixture")
				want := "N4"
				if mapped {
					want = "N1"
					if foreign {
						want = "N1"
					}
				}
				if (err != nil) != mapped || missByID(t, rs, "epic-129.1").Class != want {
					t.Fatalf("lazy mapping: %+v %v", rs, err)
				}
			})
		}
	}
}

func TestBuildReadySet_PlanReadOnlyWhenNeeded(t *testing.T) {
	for _, id := range []string{"nonplan-r", "epic-129.5", "epic-129.12"} {
		t.Run(id, func(t *testing.T) {
			f := newNearMissFixture(t, false)
			for _, r := range f.rows {
				if r.ID == id {
					f.rows = []BeadInfo{r}
					break
				}
			}
			f.save(t)
			old := fileAtRefTypedFn
			t.Cleanup(func() { fileAtRefTypedFn = old })
			fileAtRefTypedFn = func(string, string, string) ([]byte, error) { t.Fatal("unnecessary plan probe"); return nil, nil }
			rs, err := BuildReadySet(f.root, "129-fixture")
			if err == nil || len(rs.NearMisses) != 1 {
				t.Fatalf("lazy bypass classification: %+v %v", rs, err)
			}
			want := map[string]string{"nonplan-r": "N3", "epic-129.5": "N2a", "epic-129.12": "N2c"}[id]
			if rs.NearMisses[0].Class != want {
				t.Fatalf("lazy bypass: got %s want %s", rs.NearMisses[0].Class, want)
			}
		})
	}
}

func TestBuildReadySet_LineageReadErrorRefuses(t *testing.T) {
	for _, fail := range []string{"epic-129.5", "other-parent"} {
		t.Run(fail, func(t *testing.T) {
			f := newNearMissFixture(t, false)
			f.rows = []BeadInfo{f.row("epic-129.5", "[129-fixture] P", "other-parent", "task")}
			f.failShow = fail
			f.save(t)
			rs, err := BuildReadySet(f.root, "129-fixture")
			if err == nil || !strings.Contains(err.Error(), "could not resolve the lineage of epic-129.5") || len(rs.NearMisses) != 0 {
				t.Fatalf("lineage error: %v; near-misses: %+v", err, rs.NearMisses)
			}
			if !strings.HasSuffix(err.Error(), "\nrecovery: bd show epic-129.5 --json") {
				t.Fatalf("lineage refusal recovery: %v", err)
			}
		})
	}
}

func TestBuildReadySet_FencedHeading(t *testing.T) {
	f := newNearMissFixture(t, false)
	f.rows = []BeadInfo{f.row("epic-129.2", "neutral", "epic-129", "task")}
	f.save(t)
	plan := "---\nwork_chunks:\n  - id: 1\n  - id: 2\n---\n## Bead 1: outer\n~~~markdown\n## Bead 99: fenced\n~~~\n"
	if err := os.WriteFile(filepath.Join(f.root, ".mindspec/specs/129-fixture/plan.md"), []byte(plan), 0644); err != nil {
		t.Fatal(err)
	}
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err != nil || len(rs.Filtered) != 1 || rs.Filtered[0].ID != "epic-129.2" {
		t.Fatalf("fenced headings must share MF-1's positional mapping: %+v %v", rs, err)
	}
}

func TestBuildReadySet_PlanSources(t *testing.T) {
	for _, source := range []string{"branch-only", "stale-checkout", "blob-absent", "identical"} {
		t.Run(source, func(t *testing.T) {
			f := newNearMissFixture(t, false)
			planPath := filepath.Join(f.root, ".mindspec/specs/129-fixture/plan.md")
			if source != "blob-absent" {
				runGitCmdForNext(t, f.root, "add", ".mindspec")
				runGitCmdForNext(t, f.root, "commit", "-m", "committed plan")
			}
			runGitCmdForNext(t, f.root, "branch", "spec/129-fixture")
			switch source {
			case "branch-only":
				if err := os.Rename(planPath, filepath.Join(f.root, "saved-plan")); err != nil {
					t.Fatal(err)
				}
			case "stale-checkout":
				if err := os.WriteFile(planPath, []byte(fixturePlan(7, 1, 2, 3, 4, 5, 6, 7, 10)), 0644); err != nil {
					t.Fatal(err)
				}
			}
			rs, err := BuildReadySet(f.root, "129-fixture")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ids(rs.Filtered), []string{"epic-129.1", "epic-129.2", "epic-129.3", "epic-129.8"}) {
				t.Fatalf("%s members: %v", source, ids(rs.Filtered))
			}
			for id, class := range map[string]string{"epic-129.4": "N1", "epic-129.5": "N2a", "epic-129.7": "N1", "epic-129.12": "N2c", "epic-129.9": "N4", "epic-129.10": "N4", "epic-129.11": "N4", "nonplan-r": "N3", "nonplan-f": "N3", "nonplan-m": "N3", "nonplan-t": "N3"} {
				if got := missByID(t, rs, id).Class; got != class {
					t.Fatalf("%s: %s class %s want %s", source, id, got, class)
				}
			}
			if (rs.PlanNotice != "") != (source == "stale-checkout") {
				t.Fatalf("%s notice: %q", source, rs.PlanNotice)
			}
			if source == "stale-checkout" {
				// The committed eighth section must still map the child.
				f.rows = []BeadInfo{f.row("epic-129.8", "neutral", "epic-129", "task")}
				f.deps = map[string][]map[string]string{}
				f.save(t)
				rs, err = BuildReadySet(f.root, "129-fixture")
				if err != nil || !reflect.DeepEqual(ids(rs.Filtered), []string{"epic-129.8"}) {
					t.Fatalf("one-section-stale checkout replaced committed mapping: members=%v err=%v", ids(rs.Filtered), err)
				}
			}
		})
	}
}
func TestBuildReadySet_InvalidPlanRefuses(t *testing.T) {
	f := newNearMissFixture(t, false)
	if err := os.WriteFile(filepath.Join(f.root, ".mindspec/specs/129-fixture/plan.md"), []byte("---\nwork_chunks: [broken\n---\n## Bead 1: bad\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := BuildReadySet(f.root, "129-fixture")
	if err == nil || !strings.Contains(err.Error(), "could not read plan.md for 129-fixture") {
		t.Fatalf("invalid frontmatter did not fail closed: %v", err)
	}
}

func TestBuildReadySet_PlanReadErrorRefuses(t *testing.T) {
	f := newNearMissFixture(t, false)
	old := readPlanFileFn
	t.Cleanup(func() { readPlanFileFn = old })
	readPlanFileFn = func(string) ([]byte, error) { return nil, os.ErrPermission }
	_, err := BuildReadySet(f.root, "129-fixture")
	if err == nil || !strings.Contains(err.Error(), "could not read plan.md for 129-fixture") || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("checkout read failure: %v", err)
	}
	if !strings.HasSuffix(err.Error(), "\nrecovery: mindspec validate plan 129-fixture") {
		t.Fatalf("plan refusal recovery: %v", err)
	}
}

func TestBuildReadySet_PlanSourcesWithSpecWorktree(t *testing.T) {
	for _, tier := range []string{".mindspec/specs", ".mindspec/docs/specs", "docs/specs"} {
		t.Run(tier, func(t *testing.T) {
			f := newNearMissFixture(t, false)
			f.rows = []BeadInfo{f.row("epic-129.3", "neutral", "epic-129", "task")}
			f.save(t)
			// Commit only this layout tier's plan on the spec branch.
			planPath := filepath.Join(f.root, tier, "129-fixture", "plan.md")
			if err := os.MkdirAll(filepath.Dir(planPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(planPath, []byte(fixturePlan(3, 1, 2, 3)), 0644); err != nil {
				t.Fatal(err)
			}
			runGitCmdForNext(t, f.root, "add", filepath.ToSlash(filepath.Join(tier, "129-fixture", "plan.md")))
			runGitCmdForNext(t, f.root, "commit", "-m", "spec plan")
			runGitCmdForNext(t, f.root, "branch", "spec/129-fixture")
			name, err := workspace.SpecWorktreeName("129-fixture")
			if err != nil {
				t.Fatal(err)
			}
			wtRoot := filepath.Join(f.root, ".worktrees", name)
			// A real worktree ensures SpecDir chooses its copy over the root.
			runGitCmdForNext(t, f.root, "worktree", "add", wtRoot, "spec/129-fixture")
			localDir, err := workspace.SpecDir(f.root, "129-fixture")
			if err != nil {
				t.Fatal(err)
			}
			// Legacy is not a worktree tier in SpecDir; its checkout is at root.
			if tier != "docs/specs" && !strings.HasPrefix(localDir, wtRoot+string(filepath.Separator)) {
				t.Fatalf("fixture did not select spec worktree: %s", localDir)
			}
			if err := os.WriteFile(filepath.Join(localDir, "plan.md"), []byte(fixturePlan(2, 1, 2)), 0644); err != nil {
				t.Fatal(err)
			}
			rs, err := BuildReadySet(f.root, "129-fixture")
			if err != nil || !reflect.DeepEqual(ids(rs.Filtered), []string{"epic-129.3"}) {
				t.Fatalf("committed mapping not used: members=%v error=%v", ids(rs.Filtered), err)
			}
			if rs.PlanNotice != "plan.md in this checkout differs from spec/129-fixture; mapping used: spec/129-fixture" {
				t.Fatalf("missing divergence notice: %q", rs.PlanNotice)
			}
		})
	}
}

func TestBuildReadySet_Status(t *testing.T) {
	for _, status := range []string{"", "open", "in_progress", "closed", "blocked"} {
		t.Run("status="+status, func(t *testing.T) {
			f := newNearMissFixture(t, false)
			row := f.row("epic-129.1", "neutral", "epic-129", "task")
			row.Status = status
			// Simulate even a misbehaving ready producer; membership must stay honest.
			old := readyJSONFn
			readyJSONFn = func(...string) ([]byte, error) { return json.Marshal([]BeadInfo{row}) }
			t.Cleanup(func() { readyJSONFn = old })
			f.rows = []BeadInfo{row}
			f.save(t)
			rs, err := BuildReadySet(f.root, "129-fixture")
			want := 0
			if status == "" || status == "open" {
				want = 1
			}
			if err != nil || len(rs.Filtered) != want || len(rs.NearMisses) != 0 {
				t.Fatalf("status %q: members=%v near-misses=%v err=%v", status, ids(rs.Filtered), rs.NearMisses, err)
			}
		})
	}
}

func TestClassifyNearMiss_NoFalseN3(t *testing.T) {
	for _, parent := range []string{"", "epic-129"} {
		row := BeadInfo{ID: "epic-129.1", Parent: parent, Status: "in_progress", IssueType: "task"}
		if miss, linked := classifyNearMiss(row, "epic-129", "129-fixture", 1, true, lineageInfo{epic: "epic-129", mapped: true}); linked {
			t.Fatalf("mapped lineage-E row misreported as non-plan: %s", miss.String())
		}
	}
	// A defensive N3 fallback must also refuse an empty link, even when
	// called independently with no recognized lineage result.
	row := BeadInfo{ID: "epic-129.1", Status: "in_progress"}
	if miss, linked := classifyNearMiss(row, "epic-129", "129-fixture", 1, true, lineageInfo{mapped: true}); linked {
		t.Fatalf("N3 rendered an empty link: %s", miss.String())
	}
}

func TestSelectNamedFromReadySet_Ambiguous(t *testing.T) {
	rs := ReadySet{SpecFlag: "129-fixture", EpicID: "epic-129", Raw: []BeadInfo{{ID: "one-shared"}, {ID: "two-shared"}}}
	_, want := SelectWorkByName(rs.Raw, "shared")
	_, got := SelectNamedFromReadySet(rs, "shared")
	if got == nil || got.Error() != want.Error() || !strings.Contains(got.Error(), "ambiguous") {
		t.Fatalf("ambiguity masked: got %v; want %v", got, want)
	}
}

func TestWorkflowSelectionDocs_Contracts(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"architecture.md", []string{"scopeResolutionRefusal", "R6.7/AC-15", "never widens to global unscoped", "on both ready-set paths", "SelectWork", "even with exactly one item", "negative pick errors", "bdRenderedError", "RenderBDError", "wrapBDError", "nextPhaseMergeMetadataFn", "R3.3 lock", "Struck without implementation", "#146 #2(a)", "#146 #3", "`an empty ready set is not evidence the spec is complete; the derived phase above is`."}},
		{"interfaces.md", []string{"ReadySet{Filtered, Raw, SpecFlag, EpicID, NearMisses, PlanNotice, Cache}", "for positional resolution", "NearMiss{Bead, Class, Message, Recoveries, Blocking, Lineage, NoLineage}", "EmptyAfterFilterError", "ReadySet.HasBlockingNearMisses()"}},
		{"../execution/interfaces.md", []string{"Low-level git operations used by `MindspecExecutor`; `internal/next` also consumes `FileAtRefTyped` for committed plan mapping:"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../../.mindspec/domains/workflow", tc.path))
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if tc.path == "interfaces.md" && strings.Contains(text, "FindEpicForBeadWithCacheProvenance") {
				t.Error("workflow interfaces must consume the unchanged phase resolver")
			}
			if tc.path == "architecture.md" {
				start := strings.Index(text, "### R6 — Deterministic, honest bead selection")
				end := strings.Index(text[start:], "### R7 —")
				text = text[start : start+end]
			}
			text = strings.Join(strings.Fields(text), " ")
			for _, contract := range tc.want {
				if !strings.Contains(text, contract) {
					t.Errorf("workflow %s omits contract %q", tc.path, contract)
				}
			}
		})
	}
}

func TestBuildReadySet_OneShowPerPlanRow(t *testing.T) {
	f := newNearMissFixture(t, false)
	f.rows = []BeadInfo{f.row("epic-129.12", "foreign", "epic-777", "task")}
	f.save(t)
	rs, err := BuildReadySet(f.root, "129-fixture")
	if err == nil || missByID(t, rs, "epic-129.12").Class != "N2c" {
		t.Fatalf("foreign row: %+v %v", rs, err)
	}
	raw, err := os.ReadFile(f.path + ".argv")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		if len(args) > 1 && args[0] == "show" && args[1] == "epic-129.12" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("plan row show count=%d, want 1", count)
	}
}
func TestBuildReadySet_ScopedLetteredSpecResolves(t *testing.T) {
	f := newNearMissFixture(t, false)
	f.epics = []phase.EpicInfo{{ID: "epic-letter", Status: "open", IssueType: "epic", Metadata: map[string]interface{}{"spec_id": "008b-human-gates"}}}
	f.rows = []BeadInfo{f.row("epic-letter.1", "work", "epic-letter", "task")}
	f.save(t)
	dir := filepath.Join(f.root, ".mindspec/specs/008b-human-gates")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte(fixturePlan(1, 1)), 0644); err != nil {
		t.Fatal(err)
	}
	rs, err := BuildReadySet(f.root, "008b-human-gates")
	if err != nil || rs.EpicID != "epic-letter" || len(rs.Filtered) != 1 {
		t.Fatalf("lettered scope: %+v %v", rs, err)
	}
}
func TestBuildReadySet_ScopedDuplicateEpicsRefuse(t *testing.T) {
	for _, status := range []string{"open", "closed"} {
		t.Run(status, func(t *testing.T) {
			f := newNearMissFixture(t, false)
			f.epics = append(f.epics, phase.EpicInfo{ID: "epic-duplicate", Status: status, IssueType: "epic", Metadata: map[string]interface{}{"spec_id": "129-fixture"}})
			f.save(t)
			_, err := BuildReadySet(f.root, "129-fixture")
			if err == nil || !strings.Contains(err.Error(), "epic-129, epic-duplicate") {
				t.Fatalf("duplicate scope: %v", err)
			}
		})
	}
}

func TestBuildReadySet_ForeignSpecResolutionDoesNotRefuseScope(t *testing.T) {
	for _, kind := range []string{"duplicate", "invalid", "legacy-conflict"} {
		t.Run(kind, func(t *testing.T) {
			f := newNearMissFixture(t, false)
			switch kind {
			case "duplicate":
				f.epics = append(f.epics, phase.EpicInfo{ID: "epic-777b", Status: "closed", IssueType: "epic", Metadata: map[string]interface{}{"spec_id": "777-other"}})
			case "invalid":
				f.epics[1].Metadata["spec_id"] = "Bad_ID"
			case "legacy-conflict":
				f.epics[1].Title = "[SPEC 777b-other] foreign"
			}
			f.rows = []BeadInfo{f.row("epic-129.1", "valid", "epic-129", "task"), f.row("epic-129.12", "foreign", "epic-777", "task")}
			f.save(t)
			rs, err := BuildReadySet(f.root, "129-fixture")
			if err != nil || len(rs.Filtered) != 1 || rs.Filtered[0].ID != "epic-129.1" || len(rs.NearMisses) != 1 {
				t.Fatalf("foreign spec blocked scope: %+v %v", rs, err)
			}
			m := missByID(t, rs, "epic-129.12")
			if m.Class != "N2c" || !m.Blocking || !strings.Contains(m.String(), "recovery: bd show epic-129.12 --json") || strings.Contains(m.String(), "bd update") {
				t.Fatalf("foreign near-miss: %+v", m)
			}
		})
	}
}
