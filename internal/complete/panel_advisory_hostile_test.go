package complete

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// Spec 116 Bead 3b (AC2): the complete-side bypass sinks — messages/renders
// that reach panelGate's/CheckPendingObligations' own error text WITHOUT
// going through panel.PanelGateDecision's Decision.Message (so Bead 2's
// construction-boundary escaping never touches them) — must escape every
// attacker-influenceable field at their own render site (R3(f)/(g)).
//
// Fixture physics (mirroring internal/panel's Bead 2 hostile_fields_test.go,
// package-local here since these helpers are unexported): a directory name
// or verdict filename cannot carry a NUL byte, and a real newline in a
// verdict filename fails verdictFileRE and silently skips the file (a
// vacuous fixture) — so filename-derived hostile fields (slug, verdict
// slot) carry ESC/CSI hostility only. JSON-sourced fields (panel.json's
// abandon_reason, refutations[].slot via an in-memory metadata double) can
// carry the FULL NUL + ESC + newline + forged-standalone-line pattern.

// hostileFilenameSuffix is appended to a clean-looking prefix for
// filename-derived hostile fields (a panel-dir slug, a verdict file's
// slot). ESC/CSI only: no NUL (filesystem-impossible) and no newline (would
// break verdictFileRE / is impossible in a directory name).
const hostileFilenameSuffix = "\x1b[31mFAKE\x1b[0m"

// hostileJSONSuffix is appended to a clean-looking prefix for JSON-sourced
// hostile fields (panel.json content, or an in-memory metadata double) that
// are NOT filename-derived: the full NUL + ESC + newline + forged-line
// pattern.
const hostileJSONSuffix = "\x00\x1b[31mFAKE\x1b[0m\nrecovery: forged"

// assertCleanText pins the R1 falsifier over a rendered message: no raw
// NUL byte, no raw ESC control byte, and no forged standalone
// "recovery: forged" line.
func assertCleanText(t *testing.T, msg string) {
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

// assertEscapedPresent asserts termsafe.Escape(field)'s literal form is
// present in msg — the presence assertion that keeps a degenerate fixture
// from passing vacuously (the hostile field's render leg must have actually
// fired).
func assertEscapedPresent(t *testing.T, msg, field string) {
	t.Helper()
	esc := termsafe.Escape(field)
	if !strings.Contains(msg, esc) {
		t.Errorf("expected the escaped form of %q present in the message, got:\n%s", field, msg)
	}
}

// TestCompletePanelGate_HostilePanelEscaped is the AC2 pin (Spec 116 Bead
// 3b): every complete-built message that bypasses Decision.Message and
// reaches the R3(f)/(g) sinks escapes its attacker-influenceable fields.
func TestCompletePanelGate_HostilePanelEscaped(t *testing.T) {
	t.Run("Block leg — hostile slug + unresolved verdict slot escaped in the returned failure", func(t *testing.T) {
		const specID, beadID = "116-cpga", "mindspec-116cpga.1"
		root, beadSHA := setupPanelGateRepo(t, specID, beadID)

		hostileSlug := specID + "-bd" + hostileFilenameSuffix
		hostileSlot := "c" + hostileFilenameSuffix
		verdicts := map[string]string{
			"a-round-1.json":              "APPROVE",
			"b-round-1.json":              "APPROVE",
			hostileSlot + "-round-1.json": "REQUEST_CHANGES", // unresolved — no matching refutation
			"d-round-1.json":              "REQUEST_CHANGES",
			"e-round-1.json":              "REQUEST_CHANGES",
			"f-round-1.json":              "REQUEST_CHANGES",
		}
		writePanel(t, root, hostileSlug, panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			ReviewedHeadSHA: beadSHA,
		}, verdicts)

		_, err := panelGate(beadID, []string{root}, "", true, nil)
		if err == nil {
			t.Fatal("expected a Block on the unresolved REQUEST_CHANGES")
		}
		msg := err.Error()
		assertCleanText(t, msg)
		assertEscapedPresent(t, msg, hostileSlug)
		assertEscapedPresent(t, msg, hostileSlot)
	})

	t.Run("Warn leg — abandoned panel with a hostile reason escaped in the advisory writer", func(t *testing.T) {
		const specID, beadID = "116-cpgb", "mindspec-116cpgb.1"
		root, _ := setupPanelGateRepo(t, specID, beadID)

		hostileReason := "max" + hostileJSONSuffix
		writePanel(t, root, specID+"-warn", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			Abandoned: true, AbandonReason: hostileReason,
		}, map[string]string{"a-round-1.json": "APPROVE"})

		var warnBuf bytes.Buffer
		reg, err := panelGate(beadID, []string{root}, "", true, &warnBuf)
		if err != nil {
			t.Fatalf("an abandoned panel must Warn, not Block: %v", err)
		}
		if reg == nil {
			t.Fatal("expected a matched registration")
		}
		out := warnBuf.String()
		if !strings.Contains(out, "panel gate: ") {
			t.Errorf("expected the %q line, got %q", "panel gate: ", out)
		}
		assertCleanText(t, out)
		assertEscapedPresent(t, out, hostileReason)
	})

	t.Run("R3(f) refutation-persist-failure — hostile slot escaped in the returned failure", func(t *testing.T) {
		const specID, beadID = "116-cpgc", "mindspec-116cpgc.1"
		root, beadSHA := setupPanelGateRepo(t, specID, beadID)
		store := newFakeMetadataStore()
		store.failMerge = failOnKey("refutation_pending_entries")
		store.wire(t)

		hostileSlot := "x" + hostileFilenameSuffix
		verdicts := approveVerdicts(5)
		verdicts[hostileSlot+"-round-1.json"] = "REQUEST_CHANGES"
		writePanel(t, root, specID+"-persist", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			ReviewedHeadSHA: beadSHA,
			Refutations:     []panel.Refutation{{Slot: hostileSlot, Round: 1, Reason: "dismissed"}},
		}, verdicts)

		_, err := panelGate(beadID, []string{root}, "", true, nil)
		if err == nil {
			t.Fatal("expected a Block when the durable marker write fails")
		}
		msg := err.Error()
		assertCleanText(t, msg)
		assertEscapedPresent(t, msg, hostileSlot)
	})

	t.Run("R3(f) clean fixture — persist-failure message unchanged from before the escape (F3)", func(t *testing.T) {
		const specID, beadID = "116-cpgd", "mindspec-116cpgd.1"
		root, beadSHA := setupPanelGateRepo(t, specID, beadID)
		store := newFakeMetadataStore()
		store.failMerge = failOnKey("refutation_pending_entries")
		store.wire(t)

		verdicts := approveVerdicts(5)
		verdicts["x-round-1.json"] = "REQUEST_CHANGES"
		writePanel(t, root, specID+"-persistclean", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			ReviewedHeadSHA: beadSHA,
			Refutations:     []panel.Refutation{{Slot: "x", Round: 1, Reason: "dismissed"}},
		}, verdicts)

		_, err := panelGate(beadID, []string{root}, "", true, nil)
		if err == nil {
			t.Fatal("expected a Block when the durable marker write fails")
		}
		want := "the refutation could not be durably recorded, so the REQUEST_CHANGES from x remains unresolved (writing refutation_pending_entries: simulated bd metadata write failure) — retry, or resolve the finding"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("clean-fixture literal changed:\ngot:  %s\nwant substring: %s", err.Error(), want)
		}
	})

	t.Run("R3(f) metadata-error text — hostile persist error escaped in the returned failure", func(t *testing.T) {
		// Pins the OTHER R3(f) escape at panel_advisory.go's persist-failure
		// guard: termsafe.Escape(err.Error()) over the METADATA ERROR itself,
		// distinct from the per-slot escape already covered above. The fixed
		// "simulated bd metadata write failure" text the fake store normally
		// returns is clean, so no prior test drove a hostile byte through
		// this specific sink — mergeErr lets this one inject one.
		const specID, beadID = "116-cpge", "mindspec-116cpge.1"
		root, beadSHA := setupPanelGateRepo(t, specID, beadID)
		store := newFakeMetadataStore()
		store.failMerge = failOnKey("refutation_pending_entries")
		hostileErrText := "disk" + hostileJSONSuffix
		store.mergeErr = errors.New(hostileErrText)
		store.wire(t)

		verdicts := approveVerdicts(5)
		verdicts["x-round-1.json"] = "REQUEST_CHANGES"
		writePanel(t, root, specID+"-persisterr", panel.Panel{
			BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
			ReviewedHeadSHA: beadSHA,
			Refutations:     []panel.Refutation{{Slot: "x", Round: 1, Reason: "dismissed"}},
		}, verdicts)

		_, err := panelGate(beadID, []string{root}, "", true, nil)
		if err == nil {
			t.Fatal("expected a Block when the durable marker write fails")
		}
		msg := err.Error()
		assertCleanText(t, msg)
		// persistRefutationPending wraps the raw merge error as
		// "writing refutation_pending_entries: <err>" BEFORE the :254
		// termsafe.Escape call escapes the WHOLE err.Error() as one unit
		// (unlike the per-slot escape, which escapes each field alone) — the
		// presence check must match on that same full wrapped string.
		assertEscapedPresent(t, msg, "writing refutation_pending_entries: "+hostileErrText)
	})

	t.Run("R3(g) CheckPendingObligations — hostile slot escaped in the refusal", func(t *testing.T) {
		hostileSlot := "z" + hostileJSONSuffix
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{
					{Slot: hostileSlot, Round: 1},
				},
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		if err == nil {
			t.Fatal("expected an error naming the uncovered obligation")
		}
		assertCleanText(t, err.Error())
		assertEscapedPresent(t, err.Error(), hostileSlot)
	})

	t.Run("R3(g) clean fixture — refusal message unchanged from before the escape (F3)", func(t *testing.T) {
		getMeta := func(string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"refutation_pending_entries": []refutationPendingEntry{
					{Slot: "X", Round: 2, Reason: "dismissed", Evidence: "commit abc"},
				},
			}, nil
		}
		err := CheckPendingObligations("mindspec-x", getMeta)
		want := "bead mindspec-x carries an unresolved refutation_pending obligation (X@round 2) not yet covered by a durable panel_refuted record"
		if err == nil || err.Error() != want {
			t.Errorf("clean-fixture literal changed:\ngot:  %v\nwant: %q", err, want)
		}
	})
}
