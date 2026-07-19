package approve

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// gate-all-ids (ADR-0042): resolveTargetEpic returns epic.ID, which flows into
// `bd list --parent` / `bd create --parent` argv on the ApprovePlan path
// (queryExistingChildren / createBeadsFromParsed). epic.ID is bd-sourced and
// agent-steerable (`bd create --force --id=--help` succeeds), so resolveTargetEpic
// gates it through idvalidate.BeadID and treats a malformed id as no-match — it
// must NEVER return a hostile id that would reach an ID-authority-bearing argv.
func TestResolveTargetEpic_HostileEpicIDGatedFromArgv(t *testing.T) {
	const specID = "042-test"
	// An agent-steerable bd id that, if returned, would be an argv-injection
	// flag to `bd ... --parent <id>`.
	const hostileEpicID = "--help"

	restore := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		// resolveTargetEpic -> phase.Cache.AllEpics issues `bd list --type=epic`.
		// This epic's metadata (spec_num=42, spec_title="test") derives cleanly
		// to specID and WOULD match, but its ID fails idvalidate.BeadID.
		return epicJSONFor(hostileEpicID, specID), nil
	})
	defer restore()

	got, err := resolveTargetEpic(specID)
	if got == hostileEpicID {
		t.Fatalf("SECURITY: resolveTargetEpic returned hostile epic ID %q — it would flow into `bd list/create --parent` argv", hostileEpicID)
	}
	if got != "" {
		t.Fatalf("expected no-match (empty) when the only matching epic is gate-rejected, got %q", got)
	}
	if err == nil {
		t.Fatalf("expected the no-epic-found failure when the matching epic is gate-rejected")
	}
}

// Positive control: a valid epic ID whose metadata matches is returned
// unchanged (the gate is the identity for a genuine bd-minted id).
func TestResolveTargetEpic_CleanEpicIDReturned(t *testing.T) {
	const specID = "042-test"
	const cleanEpicID = "mindspec-ab12"

	restore := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		return epicJSONFor(cleanEpicID, specID), nil
	})
	defer restore()

	got, err := resolveTargetEpic(specID)
	if err != nil {
		t.Fatalf("unexpected error for a clean epic: %v", err)
	}
	if got != cleanEpicID {
		t.Fatalf("expected clean epic ID %q returned, got %q", cleanEpicID, got)
	}
}
