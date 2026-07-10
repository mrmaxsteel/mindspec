package panel

import (
	"reflect"
	"strings"
	"testing"
)

// TestPanelGateDecision_Refutations is the Spec 114 R2 (Bead 2) audited-
// refutation-escape decision matrix, mirroring TestPanelGateDecision's
// table-driven style: every fixture is a synthetic *Result (no real
// files), so the AC12 duplicate-casing rows cannot be flaky on a
// case-insensitive filesystem.
func TestPanelGateDecision_Refutations(t *testing.T) {
	t.Parallel()
	sha := "abc1234def5678abc1234def5678abc1234def56"

	defaultPanel := func() *Panel {
		return &Panel{
			BeadID: ptr("mindspec-bd01"), Spec: "114", Target: "bead",
			Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha,
		}
	}

	// (i) AC2 gate-half: 5A+1RC plus a matching refutation at the latest
	// round → Allow, AppliedRefutations == exactly that entry.
	t.Run("AC2_MatchingRefutationClearsSoleRC_Allows", func(t *testing.T) {
		p := defaultPanel()
		p.Refutations = []Refutation{{Slot: "z", Round: 1, Reason: "max: reviewed, dismissed", Evidence: "commit abc123"}}
		r := result(p, 5, 0, 1, nil, nil)
		r.Verdicts = append(r.Verdicts, Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: VerdictRequestChanges})
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Allow {
			t.Fatalf("action = %v, want Allow (RC refuted): %+v", got.Action, got)
		}
		want := []Refutation{{Slot: "z", Round: 1, Reason: "max: reviewed, dismissed", Evidence: "commit abc123"}}
		if !reflect.DeepEqual(got.AppliedRefutations, want) {
			t.Errorf("AppliedRefutations = %+v, want %+v", got.AppliedRefutations, want)
		}
	})

	// (ii) AC3: a refutation naming a REJECT slot never clears leg 9 (halt
	// path fires regardless, applied set empty).
	t.Run("AC3_RefutationNamingRejectSlot_Leg9StillBlocks", func(t *testing.T) {
		p := defaultPanel()
		p.Refutations = []Refutation{{Slot: "z", Round: 1, Reason: "attempted"}}
		r := result(p, 5, 1, 1, nil, nil) // 5 approve + 1 REJECT (slot "revf" per result()'s naming)
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Block {
			t.Fatalf("action = %v, want Block (REJECT never refutable): %+v", got.Action, got)
		}
		if !strings.Contains(got.Message, "HARD block / REJECT") {
			t.Errorf("expected the halt-path message, got: %s", got.Message)
		}
		if len(got.AppliedRefutations) != 0 {
			t.Errorf("AppliedRefutations must be empty on a Block, got %+v", got.AppliedRefutations)
		}
	})

	// (ii cont'd) AC3: refuting slot A while slot B still holds an
	// unresolved RC → Block naming B (each RC must be refuted individually).
	t.Run("AC3_RefutingOneSlotLeavesOtherUnresolved_BlocksNamingIt", func(t *testing.T) {
		p := defaultPanel()
		p.Refutations = []Refutation{{Slot: "revA", Round: 1, Reason: "dismissed A"}}
		r := result(p, 4, 0, 1, nil, nil)
		r.Verdicts = append(r.Verdicts,
			Verdict{File: "revA-round-1.json", Slot: "revA", Round: 1, Verdict: VerdictRequestChanges},
			Verdict{File: "revB-round-1.json", Slot: "revB", Round: 1, Verdict: VerdictRequestChanges})
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Block {
			t.Fatalf("action = %v, want Block (revB unresolved): %+v", got.Action, got)
		}
		if !strings.Contains(got.Message, "revB") {
			t.Errorf("message must name the still-unresolved slot revB: %s", got.Message)
		}
		if strings.Contains(got.Message, "revA") {
			t.Errorf("message must NOT re-name the refuted slot revA: %s", got.Message)
		}
	})

	// (iii-a) AC4: a round-2 all-APPROVE re-panel with a stale round-1
	// refutation entry on file → Allow with ZERO applied refutations (the
	// tally only ever reads the filename-derived latest round — there is no
	// round-1 verdict to clear in the first place; zero-ceremony R3b).
	t.Run("AC4a_StaleRound1RefutationOnRound2AllApprove_ZeroApplied", func(t *testing.T) {
		p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 2, ExpectedReviewers: 6, ReviewedHeadSHA: sha}
		p.Refutations = []Refutation{{Slot: "z", Round: 1, Reason: "stale, from round 1"}}
		r := result(p, 6, 0, 2, nil, nil)
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Allow {
			t.Fatalf("action = %v, want Allow (round-2 all-APPROVE): %+v", got.Action, got)
		}
		if len(got.AppliedRefutations) != 0 {
			t.Errorf("AppliedRefutations must be empty (nothing at round 1 to clear), got %+v", got.AppliedRefutations)
		}
	})

	// (iii-b) AC4: a round-N refutation entry does not clear the SAME
	// slot's round-(N+1) re-RC — round-binding (R3c).
	t.Run("AC4b_RoundNRefutationDoesNotClearRoundNPlus1_Blocks", func(t *testing.T) {
		p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 2, ExpectedReviewers: 6, ReviewedHeadSHA: sha}
		p.Refutations = []Refutation{{Slot: "z", Round: 1, Reason: "cleared round 1's dissent"}}
		r := result(p, 5, 0, 2, nil, nil)
		r.Verdicts = append(r.Verdicts, Verdict{File: "z-round-2.json", Slot: "z", Round: 2, Verdict: VerdictRequestChanges})
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Block {
			t.Fatalf("action = %v, want Block (round-1 refutation must not clear round-2 re-RC): %+v", got.Action, got)
		}
		if !strings.Contains(got.Message, "z") {
			t.Errorf("message must name the still-unresolved round-2 slot z: %s", got.Message)
		}
	})

	// (iv) AC7: 6A+2RC BOTH refuted, but expected=8/threshold=7 (default
	// N-1) → Block via leg (10) naming the THRESHOLD tally, not the
	// unresolved-slot message — a refutation cannot buy past the floor.
	t.Run("AC7_BothRCsRefutedButSubThreshold_BlocksOnFloorNotUnresolved", func(t *testing.T) {
		p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 8, ReviewedHeadSHA: sha}
		p.Refutations = []Refutation{
			{Slot: "revA", Round: 1, Reason: "dismissed A"},
			{Slot: "revB", Round: 1, Reason: "dismissed B"},
		}
		r := result(p, 6, 0, 1, nil, nil)
		r.Verdicts = append(r.Verdicts,
			Verdict{File: "revA-round-1.json", Slot: "revA", Round: 1, Verdict: VerdictRequestChanges},
			Verdict{File: "revB-round-1.json", Slot: "revB", Round: 1, Verdict: VerdictRequestChanges})
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		if th := p.ApproveThreshold(); th != 7 {
			t.Fatalf("precondition: threshold = %d, want 7 (N-1 of 8)", th)
		}
		got := PanelGateDecision(facts)
		if got.Action != Block {
			t.Fatalf("action = %v, want Block (sub-threshold floor): %+v", got.Action, got)
		}
		for _, want := range []string{"6/8 APPROVE", "threshold is 7/8"} {
			if !strings.Contains(got.Message, want) {
				t.Errorf("message missing %q: %s", want, got.Message)
			}
		}
		if strings.Contains(got.Message, "unresolved REQUEST_CHANGES") {
			t.Errorf("message must be the THRESHOLD block, not the unresolved-slot message: %s", got.Message)
		}
		// Approves is the genuine tally: never incremented by a refutation
		// (AC9's per-row assertion).
		if r.Approves != 6 {
			t.Errorf("Approves = %d, want 6 (refuting an RC never increments Approves)", r.Approves)
		}
	})

	// (v) AC8 non-refutable-half: an unrecognized/non-standard verdict
	// string is never VerdictRequestChanges, so a refutation naming it can
	// never match — still Block.
	t.Run("AC8_RefutationCannotClearUnrecognizedVerdict_StillBlocks", func(t *testing.T) {
		p := defaultPanel()
		p.Refutations = []Refutation{{Slot: "z", Round: 1, Reason: "attempted"}}
		r := result(p, 5, 0, 1, nil, nil)
		r.Verdicts = append(r.Verdicts, Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: "MAYBE"})
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Block {
			t.Fatalf("action = %v, want Block (unrecognized verdict is not refutable): %+v", got.Action, got)
		}
		if len(got.AppliedRefutations) != 0 {
			t.Errorf("AppliedRefutations must be empty, got %+v", got.AppliedRefutations)
		}
	})

	// (viii) AC10 re-asserted: a panel that HAS a refutations array but
	// still carries an unresolved (unmatched) RC never advertises the
	// escape in its Block message.
	t.Run("AC10_BlockMessageWithRefutationsArrayPresent_NeverAdvertises", func(t *testing.T) {
		p := defaultPanel()
		p.Refutations = []Refutation{{Slot: "other-slot", Round: 1, Reason: "unrelated"}}
		r := result(p, 5, 0, 1, nil, nil)
		r.Verdicts = append(r.Verdicts, Verdict{File: "z-round-1.json", Slot: "z", Round: 1, Verdict: VerdictRequestChanges})
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Block {
			t.Fatalf("action = %v, want Block: %+v", got.Action, got)
		}
		for _, s := range []string{"refut", "Refut", "REFUT"} {
			if strings.Contains(got.Message, s) {
				t.Errorf("Block message must never advertise the refutation escape (AC10), found %q in: %s", s, got.Message)
			}
		}
	})
}

// TestPanelGateDecision_Refutations_DuplicateSlotCasing is named so
// `-run 'Dup'` selects it (Spec 114 R2 plan step 7A(vii), AC12): pins that
// refutation slot matching is byte-EXACT — "x" and "X" are distinct slots —
// and that two refutation entries naming the identical (slot, round)
// collapse to exactly ONE AppliedRefutations record.
func TestPanelGateDecision_Refutations_DuplicateSlotCasing(t *testing.T) {
	t.Parallel()
	sha := "abc1234def5678abc1234def5678abc1234def56"

	// DuplicateSlotRefutation_CollapsesToOne: two refutations entries name
	// the SAME (slot, round) — AppliedRefutations must return exactly one
	// (first-wins) record, and the gate Allows.
	t.Run("DuplicateSlotRefutation_CollapsesToOne", func(t *testing.T) {
		p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}
		p.Refutations = []Refutation{
			{Slot: "x", Round: 1, Reason: "first (wins)"},
			{Slot: "x", Round: 1, Reason: "second (dropped)"},
		}
		r := result(p, 5, 0, 1, nil, nil)
		r.Verdicts = append(r.Verdicts, Verdict{File: "x-round-1.json", Slot: "x", Round: 1, Verdict: VerdictRequestChanges})
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Allow {
			t.Fatalf("action = %v, want Allow: %+v", got.Action, got)
		}
		if len(got.AppliedRefutations) != 1 {
			t.Fatalf("AppliedRefutations = %+v, want exactly ONE deduplicated record", got.AppliedRefutations)
		}
		if got.AppliedRefutations[0].Reason != "first (wins)" {
			t.Errorf("dedup must be first-wins (stable array order), got reason %q", got.AppliedRefutations[0].Reason)
		}
	})

	// DuplicateSlotCaseSensitive_RejectNotClearedByRefutation: slot "X" is a
	// REJECT (not RC); slot "x" is an RC and IS refuted. Leg 9 (REJECT halt
	// path) still fires regardless of the refutation — a refutation only
	// ever targets a REQUEST_CHANGES verdict, never a REJECT under any name.
	t.Run("DuplicateSlotCaseSensitive_RejectNotClearedByRefutation", func(t *testing.T) {
		p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 6, ReviewedHeadSHA: sha}
		p.Refutations = []Refutation{{Slot: "x", Round: 1, Reason: "dismissed lowercase x"}}
		r := &Result{
			Dir: "/wt/review/slug", Panel: p, LatestRound: 1,
			Verdicts: []Verdict{
				{File: "a-round-1.json", Slot: "a", Round: 1, Verdict: VerdictApprove},
				{File: "b-round-1.json", Slot: "b", Round: 1, Verdict: VerdictApprove},
				{File: "c-round-1.json", Slot: "c", Round: 1, Verdict: VerdictApprove},
				{File: "d-round-1.json", Slot: "d", Round: 1, Verdict: VerdictApprove},
				{File: "x-round-1.json", Slot: "x", Round: 1, Verdict: VerdictRequestChanges},
				{File: "X-round-1.json", Slot: "X", Round: 1, Verdict: VerdictReject},
			},
			Approves: 4, Rejects: 1,
		}
		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}

		got := PanelGateDecision(facts)
		if got.Action != Block {
			t.Fatalf("action = %v, want Block (REJECT at slot X halts regardless): %+v", got.Action, got)
		}
		if !strings.Contains(got.Message, "HARD block / REJECT") {
			t.Errorf("expected the halt-path message (leg 9 fires before leg 9.5): %s", got.Message)
		}
	})

	// DuplicateSlotCaseSensitive_BothRC_RefutingLowerLeavesUpperBlocked:
	// BOTH "x" and "X" carry a latest-round REQUEST_CHANGES; refuting only
	// "x" leaves "X" genuinely unresolved (case-sensitive slot identity) —
	// asserted directly against Result.UnresolvedVerdicts() (message-text
	// scanning for a bare "x" is unreliable: fixed prose like
	// "/ms-bead-fix" always contains the letter x).
	t.Run("DuplicateSlotCaseSensitive_BothRC_RefutingLowerLeavesUpperBlocked", func(t *testing.T) {
		p := &Panel{BeadID: ptr("mindspec-bd01"), Round: 1, ExpectedReviewers: 7, ReviewedHeadSHA: sha}
		p.Refutations = []Refutation{{Slot: "x", Round: 1, Reason: "dismissed lowercase x"}}
		r := &Result{
			Dir: "/wt/review/slug", Panel: p, LatestRound: 1,
			Verdicts: []Verdict{
				{File: "a-round-1.json", Slot: "a", Round: 1, Verdict: VerdictApprove},
				{File: "b-round-1.json", Slot: "b", Round: 1, Verdict: VerdictApprove},
				{File: "c-round-1.json", Slot: "c", Round: 1, Verdict: VerdictApprove},
				{File: "d-round-1.json", Slot: "d", Round: 1, Verdict: VerdictApprove},
				{File: "e-round-1.json", Slot: "e", Round: 1, Verdict: VerdictApprove},
				{File: "x-round-1.json", Slot: "x", Round: 1, Verdict: VerdictRequestChanges},
				{File: "X-round-1.json", Slot: "X", Round: 1, Verdict: VerdictRequestChanges},
			},
			Approves: 5, Rejects: 0,
		}

		unresolved := r.UnresolvedVerdicts()
		if len(unresolved) != 1 || unresolved[0].Slot != "X" {
			t.Fatalf("UnresolvedVerdicts() = %+v, want exactly the unrefuted slot X", unresolved)
		}

		facts := GateFacts{BeadID: "mindspec-bd01", Reg: regn("/wt/review/slug"), Res: r, HeadSHA: sha}
		got := PanelGateDecision(facts)
		if got.Action != Block {
			t.Fatalf("action = %v, want Block (X still unresolved): %+v", got.Action, got)
		}
		if !strings.Contains(got.Message, "X") {
			t.Errorf("message must name the still-unresolved slot X: %s", got.Message)
		}
	})
}

// TestVoteDecision_Refutations pins the VoteDecision lockstep twin (Spec 114
// R2): since VoteDecision calls the SAME Result.UnresolvedVerdicts() the
// gate does, a refutation clears the vote-only outcome in lockstep with
// PanelGateDecision — no separate refutation logic in VoteDecision itself.
func TestVoteDecision_Refutations(t *testing.T) {
	t.Parallel()

	t.Run("matching refutation at latest round → Pass", func(t *testing.T) {
		p := &Panel{ExpectedReviewers: 6, Round: 1}
		p.Refutations = []Refutation{{Slot: "z", Round: 1, Reason: "dismissed"}}
		res := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: approveAndRCVerdicts(5), // 5 approve + 1 RC on the trailing slot
			Approves: 5,
		}
		// approveAndRCVerdicts names its trailing RC slot sequentially after
		// the approves (see votedecision_test.go); rename it to "z" so it
		// matches the refutation entry.
		res.Verdicts[len(res.Verdicts)-1].Slot = "z"

		v, s := res.VoteDecision()
		if v != VotePass {
			t.Fatalf("verdict = %v, want VotePass (RC refuted): %s", v, s)
		}
	})

	t.Run("refutation names the wrong round → still Block", func(t *testing.T) {
		p := &Panel{ExpectedReviewers: 6, Round: 1}
		p.Refutations = []Refutation{{Slot: "z", Round: 2, Reason: "wrong round"}}
		res := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: approveAndRCVerdicts(5),
			Approves: 5,
		}
		res.Verdicts[len(res.Verdicts)-1].Slot = "z"

		v, s := res.VoteDecision()
		if v != VoteBlock {
			t.Fatalf("verdict = %v, want VoteBlock (round mismatch — never cleared): %s", v, s)
		}
		if !strings.Contains(s, "unresolved") {
			t.Errorf("summary should flag the unresolved verdict: %s", s)
		}
	})

	t.Run("refutation names the wrong slot → still Block", func(t *testing.T) {
		p := &Panel{ExpectedReviewers: 6, Round: 1}
		p.Refutations = []Refutation{{Slot: "not-z", Round: 1, Reason: "wrong slot"}}
		res := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: approveAndRCVerdicts(5),
			Approves: 5,
		}
		res.Verdicts[len(res.Verdicts)-1].Slot = "z"

		v, _ := res.VoteDecision()
		if v != VoteBlock {
			t.Fatalf("verdict = %v, want VoteBlock (wrong slot named)", v)
		}
	})
}

// TestResult_AppliedRefutations is a focused unit suite over
// Result.AppliedRefutations() itself (Spec 114 R2), independent of the
// gate/vote-decision layers built on top of it.
func TestResult_AppliedRefutations(t *testing.T) {
	t.Parallel()

	t.Run("nil Panel → nil", func(t *testing.T) {
		r := &Result{LatestRound: 1}
		if got := r.AppliedRefutations(); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("Panel with no Refutations → nil", func(t *testing.T) {
		r := &Result{Panel: &Panel{}, LatestRound: 1}
		if got := r.AppliedRefutations(); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("matching entry returned", func(t *testing.T) {
		p := &Panel{Refutations: []Refutation{{Slot: "z", Round: 1, Reason: "r", Evidence: "e"}}}
		r := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: []Verdict{{Slot: "z", Round: 1, Verdict: VerdictRequestChanges}},
		}
		want := []Refutation{{Slot: "z", Round: 1, Reason: "r", Evidence: "e"}}
		if got := r.AppliedRefutations(); !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("stale round excluded", func(t *testing.T) {
		p := &Panel{Refutations: []Refutation{{Slot: "z", Round: 1}}}
		r := &Result{
			Panel: p, LatestRound: 2,
			Verdicts: []Verdict{{Slot: "z", Round: 2, Verdict: VerdictRequestChanges}},
		}
		if got := r.AppliedRefutations(); len(got) != 0 {
			t.Errorf("got %+v, want empty (round mismatch)", got)
		}
	})

	t.Run("unknown slot excluded", func(t *testing.T) {
		p := &Panel{Refutations: []Refutation{{Slot: "nope", Round: 1}}}
		r := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: []Verdict{{Slot: "z", Round: 1, Verdict: VerdictRequestChanges}},
		}
		if got := r.AppliedRefutations(); len(got) != 0 {
			t.Errorf("got %+v, want empty (unknown slot)", got)
		}
	})

	t.Run("REJECT slot excluded even when named", func(t *testing.T) {
		p := &Panel{Refutations: []Refutation{{Slot: "z", Round: 1}}}
		r := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: []Verdict{{Slot: "z", Round: 1, Verdict: VerdictReject}},
		}
		if got := r.AppliedRefutations(); len(got) != 0 {
			t.Errorf("got %+v, want empty (REJECT is never refutable)", got)
		}
	})

	t.Run("APPROVE slot excluded even when named", func(t *testing.T) {
		p := &Panel{Refutations: []Refutation{{Slot: "z", Round: 1}}}
		r := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: []Verdict{{Slot: "z", Round: 1, Verdict: VerdictApprove}},
		}
		if got := r.AppliedRefutations(); len(got) != 0 {
			t.Errorf("got %+v, want empty (APPROVE is never a target)", got)
		}
	})

	t.Run("duplicate (slot,round) entries collapse to one, first-wins", func(t *testing.T) {
		p := &Panel{Refutations: []Refutation{
			{Slot: "z", Round: 1, Reason: "first"},
			{Slot: "z", Round: 1, Reason: "second"},
		}}
		r := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: []Verdict{{Slot: "z", Round: 1, Verdict: VerdictRequestChanges}},
		}
		got := r.AppliedRefutations()
		if len(got) != 1 || got[0].Reason != "first" {
			t.Errorf("got %+v, want exactly one first-wins record", got)
		}
	})

	t.Run("multiple distinct slots returned slot-sorted", func(t *testing.T) {
		p := &Panel{Refutations: []Refutation{
			{Slot: "z", Round: 1, Reason: "z"},
			{Slot: "a", Round: 1, Reason: "a"},
		}}
		r := &Result{
			Panel: p, LatestRound: 1,
			Verdicts: []Verdict{
				{Slot: "z", Round: 1, Verdict: VerdictRequestChanges},
				{Slot: "a", Round: 1, Verdict: VerdictRequestChanges},
			},
		}
		got := r.AppliedRefutations()
		if len(got) != 2 || got[0].Slot != "a" || got[1].Slot != "z" {
			t.Errorf("got %+v, want slot-sorted [a, z]", got)
		}
	})
}
