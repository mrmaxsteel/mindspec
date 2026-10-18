package resolve

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestResolveTarget_ExplicitFlag(t *testing.T) {
	// Explicit --spec always wins, regardless of active specs
	got, err := ResolveTarget("/nonexistent", "042-my-spec")
	if err != nil {
		t.Fatalf("ResolveTarget() error: %v", err)
	}
	if got != "042-my-spec" {
		t.Errorf("ResolveTarget() = %q, want %q", got, "042-my-spec")
	}
}

func TestErrAmbiguousTarget_Message(t *testing.T) {
	err := &ErrAmbiguousTarget{
		Active: []SpecStatus{
			{SpecID: "001-alpha", Mode: "spec"},
			{SpecID: "002-beta", Mode: "plan"},
		},
	}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
	for _, id := range []string{"001-alpha", "002-beta"} {
		if !strings.Contains(msg, id) {
			t.Errorf("error message should contain %q: %s", id, msg)
		}
	}
	if !strings.Contains(msg, "--spec") {
		t.Errorf("error message should mention --spec flag: %s", msg)
	}
}

func TestResolveTarget_NoActiveSpecs(t *testing.T) {
	// Stub bd to return no epics
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	_, err := ResolveTarget(t.TempDir(), "")
	if err == nil {
		t.Fatal("expected error when no active specs found")
	}
	if !strings.Contains(err.Error(), "no active specs") {
		t.Errorf("error should mention 'no active specs': %v", err)
	}
}

func TestResolveTarget_NoActiveSpecs_SuggestsFlag(t *testing.T) {
	// Stub bd to return no epics
	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restoreList)
	restore := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})
	t.Cleanup(restore)

	_, err := ResolveTarget(t.TempDir(), "")
	if err == nil {
		t.Fatal("expected error when no active specs found")
	}
	if !strings.Contains(err.Error(), "--spec") {
		t.Errorf("error should suggest --spec flag: %v", err)
	}
}

// TestErrAmbiguousTarget_HostileSpecIDForcedQuoted is Spec 120 R4's
// (converging pass) Class B pin: SpecStatus.SpecID is derived from
// SpecIDFromMetadata(specNum, slugify(specTitle)) — specTitle is bd-epic
// metadata (agent-writable) and slugify does NOT strip control bytes, so a
// hostile epic title can produce a SpecID shaped to forge terminal output
// when listed here. idrender.Spec forces anything that fails
// idvalidate.SpecID through strconv.Quote so it can never masquerade as a
// genuine spec ID or inject a forged extra line.
func TestErrAmbiguousTarget_HostileSpecIDForcedQuoted(t *testing.T) {
	hostileID := "042-my-spec\nrecovery: forged"
	err := &ErrAmbiguousTarget{
		Active: []SpecStatus{{SpecID: hostileID, Mode: "spec"}},
	}
	msg := err.Error()
	wantQuoted := strconv.Quote(hostileID)
	if !strings.Contains(msg, wantQuoted) {
		t.Errorf("ErrAmbiguousTarget.Error() missing forced-quoted hostile SpecID %q:\n%s", wantQuoted, msg)
	}
	for _, line := range strings.Split(msg, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone line reached the message via the hostile SpecID's raw newline: %q", msg)
		}
	}
}

// TestErrAmbiguousTarget_CleanSpecIDByteIdentical is the clean-fixture
// counterpart (F3 discipline): TestErrAmbiguousTarget_Message above already
// pins this, but this test names the byte-identical guarantee explicitly
// per idrender.Spec's contract.
func TestErrAmbiguousTarget_CleanSpecIDByteIdentical(t *testing.T) {
	const clean = "042-my-spec"
	err := &ErrAmbiguousTarget{Active: []SpecStatus{{SpecID: clean, Mode: "spec"}}}
	msg := err.Error()
	if !strings.Contains(msg, "  "+clean+"  (mode: spec)\n") {
		t.Errorf("clean SpecID must render byte-identically:\n%s", msg)
	}
}

func TestErrAmbiguousTarget_IsDetectable(t *testing.T) {
	err := &ErrAmbiguousTarget{Active: []SpecStatus{{SpecID: "a"}}}
	var ambErr *ErrAmbiguousTarget
	if !isAmbiguousError(err, &ambErr) {
		t.Error("expected ErrAmbiguousTarget to be detectable via type assertion")
	}
}

// isAmbiguousError is a helper for type assertion tests.
func isAmbiguousError(err error, target **ErrAmbiguousTarget) bool {
	e, ok := err.(*ErrAmbiguousTarget)
	if ok {
		*target = e
	}
	return ok
}

// stubActiveSpecs sets up the list+runBD stubs to return the given epics (with metadata)
// and no children (so phase derives to "plan"). Returns a cleanup function.
func stubActiveSpecs(t *testing.T, epicsJSON string) {
	t.Helper()
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(epicsJSON), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		return []byte("[]"), nil
	}))
}

func TestResolveSpecPrefix_NumericMatch(t *testing.T) {
	stubActiveSpecs(t, `[{"id":"epic-1","title":"[SPEC 077-execution-layer-interface]","status":"open","issue_type":"epic","metadata":{"spec_num":77,"spec_title":"execution-layer-interface"}}]`)

	got, err := ResolveSpecPrefix("077")
	if err != nil {
		t.Fatalf("ResolveSpecPrefix(\"077\") error: %v", err)
	}
	if got != "077-execution-layer-interface" {
		t.Errorf("ResolveSpecPrefix(\"077\") = %q, want %q", got, "077-execution-layer-interface")
	}
}

// --- ResolveSpecPrefixProven (spec 128 R6.7/AC-15) ---

// TestResolveSpecPrefixProven_ExistingEpic_PassesThrough pins that a
// full-ID passthrough naming an epic-bearing spec is unaffected.
func TestResolveSpecPrefixProven_ExistingEpic_PassesThrough(t *testing.T) {
	stubActiveSpecs(t, `[{"id":"epic-1","title":"[SPEC 077-execution-layer-interface]","status":"open","issue_type":"epic","metadata":{"spec_num":77,"spec_title":"execution-layer-interface"}}]`)

	got, err := ResolveSpecPrefixProven(t.TempDir(), "077-execution-layer-interface")
	if err != nil {
		t.Fatalf("ResolveSpecPrefixProven() error: %v", err)
	}
	if got != "077-execution-layer-interface" {
		t.Errorf("ResolveSpecPrefixProven() = %q, want %q", got, "077-execution-layer-interface")
	}
}

// TestResolveSpecPrefixProven_ExistingDraft_PassesThrough pins AC-15(ii)'s
// non-refusal half at the ResolveSpecPrefix layer: an existing on-disk
// draft (no epic yet) is NOT reported as nonexistent here — its own
// honest "no epic; not approved" refusal is a downstream epic-lookup
// responsibility (internal/next's scopeResolutionRefusal).
func TestResolveSpecPrefixProven_ExistingDraft_PassesThrough(t *testing.T) {
	specID := "042-draft-fixture"
	root := newDraftFixtureRoot(t, specID)
	stubActiveEpics(t, nil, nil) // bd healthy, zero epics

	got, err := ResolveSpecPrefixProven(root, specID)
	if err != nil {
		t.Fatalf("ResolveSpecPrefixProven() error for an existing draft: %v", err)
	}
	if got != specID {
		t.Errorf("ResolveSpecPrefixProven() = %q, want %q", got, specID)
	}
}

// TestResolveSpecPrefixProven_NonexistentFullID_Refuses pins AC-15(i)'s
// core: a full-ID passthrough naming NO epic and NO on-disk draft refuses
// here, before mindspec next would ever issue a ready query.
func TestResolveSpecPrefixProven_NonexistentFullID_Refuses(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitCmd(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("baseline\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "-m", "baseline")
	stubActiveEpics(t, nil, nil) // bd healthy, zero epics

	_, err := ResolveSpecPrefixProven(root, "999-does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a full-ID spec with no epic and no on-disk draft")
	}
	if !strings.Contains(err.Error(), "999-does-not-exist") {
		t.Errorf("expected the error to name the spec, got: %v", err)
	}
}

// TestResolveSpecPrefixProven_NumericPrefixUnaffected pins that the
// numeric-prefix branch's existing DiscoverActiveSpecsWithCache-backed
// existence proof is untouched by the full-ID passthrough's new check.
func TestResolveSpecPrefixProven_NumericPrefixUnaffected(t *testing.T) {
	stubActiveSpecs(t, `[{"id":"epic-1","title":"[SPEC 077-execution-layer-interface]","status":"open","issue_type":"epic","metadata":{"spec_num":77,"spec_title":"execution-layer-interface"}}]`)

	got, err := ResolveSpecPrefixProven(t.TempDir(), "077")
	if err != nil {
		t.Fatalf("ResolveSpecPrefixProven(\"077\") error: %v", err)
	}
	if got != "077-execution-layer-interface" {
		t.Errorf("ResolveSpecPrefixProven(\"077\") = %q, want %q", got, "077-execution-layer-interface")
	}
}

func TestResolveSpecPrefix_FullIDPassthrough(t *testing.T) {
	// Full spec ID with hyphen should pass through without querying beads.
	got, err := ResolveSpecPrefix("077-execution-layer-interface")
	if err != nil {
		t.Fatalf("ResolveSpecPrefix() error: %v", err)
	}
	if got != "077-execution-layer-interface" {
		t.Errorf("ResolveSpecPrefix() = %q, want %q", got, "077-execution-layer-interface")
	}
}

func TestResolveSpecPrefix_NoMatch(t *testing.T) {
	stubActiveSpecs(t, `[{"id":"epic-1","title":"[SPEC 077-execution-layer-interface]","status":"open","issue_type":"epic","metadata":{"spec_num":77,"spec_title":"execution-layer-interface"}}]`)

	_, err := ResolveSpecPrefix("999")
	if err == nil {
		t.Fatal("expected error for non-matching prefix")
	}
	if !strings.Contains(err.Error(), "999") {
		t.Errorf("error should mention the prefix: %v", err)
	}
}

// TestResolveSpecPrefixValidatesResult is spec 120 AC-7 (R3 specID
// ingress): a hyphen-bearing hostile value REFUSES with the
// `mindspec spec list` lever instead of passing through raw; numeric-
// prefix resolution and every live spec ID incl. "008b-human-gates" pass
// byte-identically.
func TestResolveSpecPrefixValidatesResult(t *testing.T) {
	hostileIDs := []string{
		"x;evil",
		"../../outside",
		"x\x00\x1b[31m\nrecovery: forged",
	}
	for _, hostile := range hostileIDs {
		got, err := ResolveSpecPrefix(hostile)
		if err == nil {
			t.Errorf("ResolveSpecPrefix(%q) = (%q, nil), want a refusal", hostile, got)
			continue
		}
		if !strings.Contains(err.Error(), "mindspec spec list") {
			t.Errorf("ResolveSpecPrefix(%q) error must name the `mindspec spec list` lever, got: %v", hostile, err)
		}
	}

	// A non-hyphenated, non-numeric hostile value (e.g. "--help") also
	// refuses via the same result-validation gate.
	if got, err := ResolveSpecPrefix("--help"); err == nil {
		t.Errorf("ResolveSpecPrefix(--help) = (%q, nil), want a refusal", got)
	}

	// Clean values pass byte-identically.
	if got, err := ResolveSpecPrefix("008b-human-gates"); err != nil || got != "008b-human-gates" {
		t.Errorf("ResolveSpecPrefix(008b-human-gates) = (%q, %v), want (008b-human-gates, nil)", got, err)
	}
	stubActiveSpecs(t, `[{"id":"epic-1","title":"[SPEC 077-execution-layer-interface]","status":"open","issue_type":"epic","metadata":{"spec_num":77,"spec_title":"execution-layer-interface"}}]`)
	if got, err := ResolveSpecPrefix("077"); err != nil || got != "077-execution-layer-interface" {
		t.Errorf("ResolveSpecPrefix(077) = (%q, %v), want (077-execution-layer-interface, nil)", got, err)
	}
}

func TestResolveTarget_PrefixResolution(t *testing.T) {
	stubActiveSpecs(t, `[{"id":"epic-1","title":"[SPEC 077-execution-layer-interface]","status":"open","issue_type":"epic","metadata":{"spec_num":77,"spec_title":"execution-layer-interface"}}]`)

	got, err := ResolveTarget(t.TempDir(), "077")
	if err != nil {
		t.Fatalf("ResolveTarget(root, \"077\") error: %v", err)
	}
	if got != "077-execution-layer-interface" {
		t.Errorf("ResolveTarget(root, \"077\") = %q, want %q", got, "077-execution-layer-interface")
	}
}
