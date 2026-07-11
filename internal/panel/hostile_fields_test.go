package panel

import (
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// hostileSuffix is the shared attacker-controlled payload appended to a
// clean-looking prefix in every hostile field fixture below: a printable
// non-ASCII rune (é), a NUL byte, an ESC/CSI ANSI sequence, a real newline,
// and a forged standalone "recovery: forged" line.
//
// The café rune is deliberate, not decorative (AC4(d), carried forward from
// the Bead 1 panel): on an ASCII-only control-byte string, termsafe.Escape
// is a FIXED POINT (Escape(Escape(x)) == Escape(x)) because strconv.Quote's
// output for ASCII input is itself all printable ASCII — so a bare
// "the nested double-escaped form appears nowhere" assertion over an
// ASCII-only fixture can never fail, i.e. it would be VACUOUS. Mixing in a
// printable non-ASCII rune breaks that: strconv.Quote leaves é raw (per
// unicode.IsPrint), so escaping the already-escaped output sees a non-ASCII
// rune and re-quotes the whole thing — Escape(Escape(x)) != Escape(x). That
// makes assertNoNestedEscape below a real, falsifiable check.
const hostileSuffix = "-café\x00\x1b[31m\nrecovery: forged"

const (
	cleanSHA1 = "abc1234def5678abc1234def5678abc1234def56"
	cleanSHA2 = "999000999000999000999000999000999000beef"
)

// assertCleanTriple pins the R1/AC4(a) falsifier over a rendered Message: no
// raw NUL byte, no raw ESC control byte, and no forged standalone line. Real
// newlines the templates themselves own (leg 7's per-path separators,
// RawMergeFence's leading "\n") are exempt — only a *forged* bare line
// mimicking gate/recovery prose is banned.
func assertCleanTriple(t *testing.T, msg string) {
	t.Helper()
	if strings.ContainsRune(msg, 0x00) {
		t.Errorf("message contains a raw NUL byte:\n%q", msg)
	}
	if strings.ContainsRune(msg, 0x1b) {
		t.Errorf("message contains a raw ESC control byte:\n%q", msg)
	}
	for _, line := range strings.Split(msg, "\n") {
		if line == "recovery: forged" {
			t.Errorf("a forged standalone `recovery: forged` line reached the message:\n%q", msg)
		}
	}
}

// assertEscapedFieldCount asserts termsafe.Escape(field)'s literal form
// appears in msg exactly wantCount times — the leg's interpolation-site
// count for that field (AC4(d), first half).
func assertEscapedFieldCount(t *testing.T, msg, field string, wantCount int) {
	t.Helper()
	esc := termsafe.Escape(field)
	if got := strings.Count(msg, esc); got != wantCount {
		t.Errorf("message contains the escaped field %d times, want %d\nescaped form: %q\nmessage: %s", got, wantCount, esc, msg)
	}
}

// assertNoNestedEscape asserts the NESTED double-escaped form of field
// (termsafe.Escape(termsafe.Escape(field))) appears nowhere in msg — the
// operationalized no-double-escape falsifier (AC4(d), second half). It
// fails loudly (Fatalf, not a silent pass) if field happens to be a fixed
// point under Escape, since that would make the check vacuous — see
// hostileSuffix's doc comment above for why every caller of this function
// uses a café-laced fixture.
func assertNoNestedEscape(t *testing.T, msg, field string) {
	t.Helper()
	esc := termsafe.Escape(field)
	doubled := termsafe.Escape(esc)
	if doubled == esc {
		t.Fatalf("test fixture bug: %q is a FIXED POINT under Escape (single- and double-escaped forms are identical) — this field cannot distinguish single vs double escaping; use a fixture mixing a printable non-ASCII rune with a control byte", field)
	}
	if strings.Contains(msg, doubled) {
		t.Errorf("message contains the DOUBLE-escaped (nested) form of %q:\n%s", field, msg)
	}
}

// TestPanelGateDecision_HostileFieldsEscaped (Spec 116 AC4) is the
// construction-boundary field-sweep matrix: one subtest per leg
// (0/2/3/4/5/5b/6/7/8/9/9.5/10), planting the hostile pattern in each
// R2-enumerated field that leg interpolates, asserting (a) the clean triple
// over the whole rendered Message, (b) Action parity against a
// structurally-identical clean-fixture baseline, (c) leg 7's intended
// multi-line layout survives (real newlines, not collapsed), and (d) each
// hostile field's single-escaped form is present at the correct
// interpolation count and its nested double-escaped form is absent.
//
// Panel round-1 note (O3, evidence-refuted, no code change): a printable-
// ASCII ", " embedded IN a list element (a verdict filename / hard-block
// slot / malformed name) survives Escape unchanged (comma+space are
// safe-set by AC6 design) and can render as an apparent extra list item on
// the SAME line as its neighbor. This creates no control byte and no forged
// standalone line, so it does not violate R1's clean triple — it is the
// same documented printable-ASCII within-line residual class as homoglyphs
// (termsafe.Escape's own doc comment), already declared out of scope by the
// spec's M3 Non-Goal ("Not full prompt-injection safety at the instruct
// sink … a printable-ASCII within-line … string … deliberately out of
// scope").
func TestPanelGateDecision_HostileFieldsEscaped(t *testing.T) {
	t.Run("leg 0: escape hatch — BeadID", func(t *testing.T) {
		hostileBeadID := "mindspec-x0" + hostileSuffix
		cleanFacts := GateFacts{SkipEnv: true, BeadID: "mindspec-x0"}
		hostileFacts := GateFacts{SkipEnv: true, BeadID: hostileBeadID}

		wantAction := PanelGateDecision(cleanFacts).Action
		got := PanelGateDecision(hostileFacts)
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v (clean-fixture baseline)", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileBeadID, 1)
		assertNoNestedEscape(t, got.Message, hostileBeadID)
	})

	t.Run("leg 2: malformed registration — slug", func(t *testing.T) {
		hostileSlug := "evil2" + hostileSuffix
		buildFacts := func(slug string) GateFacts {
			reg := regn("/wt/review/" + slug)
			return GateFacts{BeadID: "mindspec-bd01", Reg: reg, Res: &Result{Dir: reg.Dir, PanelErr: errFake}}
		}

		wantAction := PanelGateDecision(buildFacts("evil2")).Action
		got := PanelGateDecision(buildFacts(hostileSlug))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileSlug, 1)
		assertNoNestedEscape(t, got.Message, hostileSlug)
	})

	t.Run("leg 3: abandoned — slug + AbandonReason", func(t *testing.T) {
		hostileSlug := "evil3" + hostileSuffix
		hostileReason := "max" + hostileSuffix
		buildFacts := func(slug, reason string) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: cleanSHA1, Abandoned: true, AbandonReason: reason}
			return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/" + slug),
				Res: result(p, 0, 0, 1, nil, nil), HeadSHA: cleanSHA2}
		}

		wantAction := PanelGateDecision(buildFacts("evil3", "max ok")).Action
		got := PanelGateDecision(buildFacts(hostileSlug, hostileReason))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileSlug, 1)
		assertNoNestedEscape(t, got.Message, hostileSlug)
		assertEscapedFieldCount(t, got.Message, hostileReason, 1)
		assertNoNestedEscape(t, got.Message, hostileReason)
	})

	t.Run("leg 4: round mismatch — slug", func(t *testing.T) {
		hostileSlug := "evil4" + hostileSuffix
		buildFacts := func(slug string) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: cleanSHA1}
			return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/" + slug),
				Res: result(p, 6, 0, 2, nil, nil), HeadSHA: cleanSHA1}
		}

		wantAction := PanelGateDecision(buildFacts("evil4")).Action
		got := PanelGateDecision(buildFacts(hostileSlug))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileSlug, 1)
		assertNoNestedEscape(t, got.Message, hostileSlug)
	})

	t.Run("leg 5: missing ref — BeadID twice", func(t *testing.T) {
		hostileBeadID := "mindspec-x5" + hostileSuffix
		buildFacts := func(beadID string) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: cleanSHA1}
			return GateFacts{BeadID: beadID, Reg: regn("/wt/review/demo5"),
				Res: result(p, 6, 0, 1, nil, nil), MissingRef: true}
		}

		wantAction := PanelGateDecision(buildFacts("mindspec-x5")).Action
		got := PanelGateDecision(buildFacts(hostileBeadID))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileBeadID, 2)
		assertNoNestedEscape(t, got.Message, hostileBeadID)
	})

	t.Run("leg 5b: transient git error — BeadID twice + GitErr", func(t *testing.T) {
		hostileBeadID := "mindspec-x5b" + hostileSuffix
		hostileGitErr := errors.New("rev-parse bead/x: simulated" + hostileSuffix)
		cleanGitErr := errors.New("rev-parse bead/x: simulated (clean)")
		buildFacts := func(beadID string, gitErr error) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: cleanSHA1}
			return GateFacts{BeadID: beadID, Reg: regn("/wt/review/demo5b"),
				Res: result(p, 6, 0, 1, nil, nil), GitErr: gitErr}
		}

		wantAction := PanelGateDecision(buildFacts("mindspec-x5b", cleanGitErr)).Action
		got := PanelGateDecision(buildFacts(hostileBeadID, hostileGitErr))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileBeadID, 2)
		assertNoNestedEscape(t, got.Message, hostileBeadID)
		assertEscapedFieldCount(t, got.Message, hostileGitErr.Error(), 1)
		assertNoNestedEscape(t, got.Message, hostileGitErr.Error())
	})

	t.Run("leg 6: stale SHA — short(ReviewedHeadSHA) and short(HeadSHA), escaped AFTER truncation", func(t *testing.T) {
		// The hostile bytes must sit within short()'s leading 7-byte
		// truncation window to prove the escape-after-truncate ORDER (not
		// escape-before, which could split the CSI intro): a 5-byte
		// ESC[31m sequence plus a 2-byte printable non-ASCII rune (é / è)
		// exactly fills 7 bytes. The non-ASCII filler — in place of plain
		// ASCII — is deliberate: per hostileSuffix's fixed-point rationale
		// above, an ASCII-only control-byte string is a FIXED POINT under
		// Escape, which would make assertNoNestedEscape on short(sha)
		// vacuous; mixing in é/è breaks that fixed point.
		hostileReviewed := "\x1b[31mé" + strings.Repeat("a", 33)
		hostileHead := "\x1b[31mè" + strings.Repeat("b", 33)
		buildFacts := func(reviewed, head string) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: reviewed}
			return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/demo6"),
				Res: result(p, 6, 0, 1, nil, nil), HeadSHA: head}
		}

		wantAction := PanelGateDecision(buildFacts(cleanSHA1, cleanSHA2)).Action
		got := PanelGateDecision(buildFacts(hostileReviewed, hostileHead))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, short(hostileReviewed), 1)
		assertNoNestedEscape(t, got.Message, short(hostileReviewed))
		assertEscapedFieldCount(t, got.Message, short(hostileHead), 1)
		assertNoNestedEscape(t, got.Message, short(hostileHead))
		// Escaping AFTER truncation: the raw pre-truncation tail bytes
		// ("aaa...", "bbb...") must NOT appear at all — short() is what
		// keeps the message short, escaping never re-expands it.
		if strings.Contains(got.Message, strings.Repeat("a", 33)) {
			t.Errorf("full pre-truncation ReviewedHeadSHA leaked into the message — short() must run BEFORE Escape:\n%s", got.Message)
		}
	})

	t.Run("leg 7: dirty tree — WorktreePath + UserDirt entries, real newlines preserved", func(t *testing.T) {
		hostileWorktree := "/wt/evil7" + hostileSuffix
		hostileDirt := "src/evil7a" + hostileSuffix + ".go"
		cleanDirt := "clean/file.go"
		// hostileBeadID7 is a HOSTILE BeadID (every other RawMergeFence-
		// calling leg in this matrix uses a clean BeadID — S1's mutation
		// proof: deleting termsafe.Escape(beadID) inside RawMergeFence left
		// all 12 subtests green). This leg is the one chosen to close that
		// gap since it already asserts RawMergeFence's leading newline.
		hostileBeadID7 := "mindspec-x7" + hostileSuffix
		buildFacts := func(beadID, worktree string, dirt []string) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6} // ReviewedHeadSHA "" skips leg 6
			return GateFacts{BeadID: beadID, Reg: regn("/wt/review/demo7"),
				Res: result(p, 6, 0, 1, nil, nil), HeadSHA: cleanSHA1,
				WorktreePath: worktree, UserDirt: dirt}
		}

		wantAction := PanelGateDecision(buildFacts("mindspec-bd01", "/wt/evil7", []string{"clean1.go", cleanDirt})).Action
		got := PanelGateDecision(buildFacts(hostileBeadID7, hostileWorktree, []string{hostileDirt, cleanDirt}))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileWorktree, 1)
		assertNoNestedEscape(t, got.Message, hostileWorktree)
		assertEscapedFieldCount(t, got.Message, hostileDirt, 1)
		assertNoNestedEscape(t, got.Message, hostileDirt)

		// RawMergeFence(f.BeadID)'s own internal escape, pinned at the
		// construction boundary with a hostile BeadID (the blocking finding
		// above): this is the first — and only — leg in the matrix where
		// RawMergeFence sees a hostile BeadID.
		assertEscapedFieldCount(t, got.Message, hostileBeadID7, 1)
		assertNoNestedEscape(t, got.Message, hostileBeadID7)

		// (c) leg 7's intended multi-line layout survives: the template's
		// OWN "\n  " per-path separator is a REAL newline, never collapsed
		// or absorbed into the escaped literal.
		if !strings.Contains(got.Message, "\n  "+termsafe.Escape(hostileDirt)) {
			t.Errorf("leg 7's per-path real-newline separator not preserved before the escaped hostile entry:\n%s", got.Message)
		}
		if !strings.Contains(got.Message, "\n  "+cleanDirt) {
			t.Errorf("leg 7's per-path real-newline separator not preserved before the clean entry:\n%s", got.Message)
		}
		if !strings.Contains(got.Message, "\nDo NOT bypass") {
			t.Errorf("RawMergeFence's leading real newline missing:\n%s", got.Message)
		}
	})

	t.Run("leg 8: incomplete — presentVerdictFiles (v.File + res.Malformed) + slug", func(t *testing.T) {
		hostileSlug := "evil8" + hostileSuffix
		hostileVerdictFile := "hb8" + hostileSuffix + "-round-1.json"
		hostileMalformed := "mf8" + hostileSuffix + "-round-1.json"
		// A CLEAN sibling element alongside the hostile one in BOTH the
		// Verdicts and Malformed lists proves per-element-before-join
		// independently for these two joins, not merely inferred from a
		// single-element fixture.
		const cleanSiblingVerdictFile = "clean8-sibling-round-1.json"
		const cleanSiblingMalformed = "cleanmf8-sibling-round-1.json"
		buildFacts := func(slug, verdictFile, malformed string) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6}
			res := &Result{
				Dir: "/wt/review/" + slug, Panel: p, LatestRound: 1,
				Verdicts: []Verdict{
					{File: verdictFile, Slot: "hb8", Round: 1, Verdict: VerdictApprove},
					{File: cleanSiblingVerdictFile, Slot: "clean8b", Round: 1, Verdict: VerdictApprove},
				},
				Approves:  2,
				Malformed: []string{malformed, cleanSiblingMalformed},
			}
			return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/" + slug), Res: res, HeadSHA: cleanSHA1}
		}

		wantAction := PanelGateDecision(buildFacts("evil8", "clean-round-1.json", "cleanmf-round-1.json")).Action
		got := PanelGateDecision(buildFacts(hostileSlug, hostileVerdictFile, hostileMalformed))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileSlug, 1)
		assertNoNestedEscape(t, got.Message, hostileSlug)
		assertEscapedFieldCount(t, got.Message, hostileVerdictFile, 1)
		assertNoNestedEscape(t, got.Message, hostileVerdictFile)
		assertEscapedFieldCount(t, got.Message, hostileMalformed, 1)
		assertNoNestedEscape(t, got.Message, hostileMalformed)
		if !strings.Contains(got.Message, cleanSiblingVerdictFile) {
			t.Errorf("clean sibling verdict file missing from presentVerdictFiles' joined list:\n%s", got.Message)
		}
		if !strings.Contains(got.Message, cleanSiblingMalformed) {
			t.Errorf("clean sibling malformed file missing from presentVerdictFiles' joined list:\n%s", got.Message)
		}
	})

	t.Run("leg 9: REJECT/hard_block — HardBlocks + slug", func(t *testing.T) {
		hostileSlug := "evil9" + hostileSuffix
		hostileHardBlock := "hb9" + hostileSuffix
		// A CLEAN sibling element alongside the hostile one proves
		// per-element-before-join independently for this join too.
		const cleanSiblingHardBlock = "cleanhb-sibling"
		buildFacts := func(slug string, hardBlocks []string) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6}
			res := &Result{
				Dir: "/wt/review/" + slug, Panel: p, LatestRound: 1,
				Verdicts:   makeApproveVerdicts(6),
				Approves:   6,
				HardBlocks: hardBlocks,
			}
			return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/" + slug), Res: res, HeadSHA: cleanSHA1}
		}

		wantAction := PanelGateDecision(buildFacts("evil9", []string{"cleanhb", cleanSiblingHardBlock})).Action
		got := PanelGateDecision(buildFacts(hostileSlug, []string{hostileHardBlock, cleanSiblingHardBlock}))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileSlug, 1)
		assertNoNestedEscape(t, got.Message, hostileSlug)
		assertEscapedFieldCount(t, got.Message, hostileHardBlock, 1)
		assertNoNestedEscape(t, got.Message, hostileHardBlock)
		if !strings.Contains(got.Message, termsafe.Escape(hostileHardBlock)+", "+cleanSiblingHardBlock) {
			t.Errorf("HardBlocks join did not place the escaped hostile element and its clean sibling correctly:\n%s", got.Message)
		}
	})

	t.Run("leg 9.5: unresolved verdict — slot name + slug", func(t *testing.T) {
		hostileSlug := "evil95" + hostileSuffix
		hostileSlot := "rc95" + hostileSuffix
		// A CLEAN sibling unresolved slot alongside the hostile one proves
		// per-element-before-join independently for this join too.
		const cleanSiblingSlot = "cleanrc-sibling"
		buildFacts := func(slug, slot string) GateFacts {
			p := &Panel{Round: 1, ExpectedReviewers: 6}
			verdicts := makeApproveVerdicts(5)
			verdicts = append(verdicts,
				Verdict{File: "x-round-1.json", Slot: slot, Round: 1, Verdict: VerdictRequestChanges},
				Verdict{File: "x95-sibling-round-1.json", Slot: cleanSiblingSlot, Round: 1, Verdict: VerdictRequestChanges},
			)
			res := &Result{
				Dir: "/wt/review/" + slug, Panel: p, LatestRound: 1,
				Verdicts: verdicts, Approves: 5,
			}
			return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/" + slug), Res: res, HeadSHA: cleanSHA1}
		}

		wantAction := PanelGateDecision(buildFacts("evil95", "cleanrc")).Action
		got := PanelGateDecision(buildFacts(hostileSlug, hostileSlot))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileSlug, 1)
		assertNoNestedEscape(t, got.Message, hostileSlug)
		assertEscapedFieldCount(t, got.Message, hostileSlot, 1)
		assertNoNestedEscape(t, got.Message, hostileSlot)
		if !strings.Contains(got.Message, cleanSiblingSlot) {
			t.Errorf("clean sibling slot missing from the unresolved-slots joined list:\n%s", got.Message)
		}
	})

	t.Run("leg 10: threshold Block (sub-threshold via audited refutations) — slug", func(t *testing.T) {
		hostileSlug := "evil10" + hostileSuffix
		buildFacts := func(slug string) GateFacts {
			p := &Panel{
				Round: 1, ExpectedReviewers: 6,
				Refutations: []Refutation{{Slot: "rc1", Round: 1}, {Slot: "rc2", Round: 1}},
			}
			verdicts := makeApproveVerdicts(4)
			verdicts = append(verdicts,
				Verdict{File: "rc1-round-1.json", Slot: "rc1", Round: 1, Verdict: VerdictRequestChanges},
				Verdict{File: "rc2-round-1.json", Slot: "rc2", Round: 1, Verdict: VerdictRequestChanges},
			)
			res := &Result{
				Dir: "/wt/review/" + slug, Panel: p, LatestRound: 1,
				Verdicts: verdicts, Approves: 4,
			}
			return GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/" + slug), Res: res, HeadSHA: cleanSHA1}
		}

		wantAction := PanelGateDecision(buildFacts("evil10")).Action
		if wantAction != Block {
			t.Fatalf("test fixture bug: expected the clean-baseline leg-10 fixture to Block (sub-threshold, 4/6 with both dissents refuted), got %v", wantAction)
		}
		got := PanelGateDecision(buildFacts(hostileSlug))
		if got.Action != wantAction {
			t.Fatalf("Action = %v, want %v", got.Action, wantAction)
		}
		assertCleanTriple(t, got.Message)
		assertEscapedFieldCount(t, got.Message, hostileSlug, 1)
		assertNoNestedEscape(t, got.Message, hostileSlug)
	})
}

// TestRawMergeFence_HostileBeadIDEscaped (Spec 116 AC4, panel round-1
// blocking finding) pins RawMergeFence's own internal termsafe.Escape call
// directly, at its source, regardless of which of the 8 message legs calls
// it. Before this test existed, every RawMergeFence-calling leg in
// TestPanelGateDecision_HostileFieldsEscaped used a clean BeadID fixture, so
// deleting the Escape call inside RawMergeFence left all 12 subtests
// passing — see this test's mutation-verification note in the fix commit.
func TestRawMergeFence_HostileBeadIDEscaped(t *testing.T) {
	hostileBeadID := "mindspec-xrmf" + hostileSuffix
	msg := RawMergeFence(hostileBeadID)
	assertCleanTriple(t, msg)
	assertEscapedFieldCount(t, msg, hostileBeadID, 1)
	assertNoNestedEscape(t, msg, hostileBeadID)
}
