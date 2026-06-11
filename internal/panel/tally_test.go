package panel

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

// panelDir builds review/<slug>/ under a fresh root and returns the
// panel dir path. panelJSON == "" means no panel.json (legacy dir).
func panelDir(t *testing.T, panelJSON string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "review", "p")
	if panelJSON != "" {
		writeFile(t, root, "review/p/panel.json", panelJSON)
	} else {
		writeFile(t, root, "review/p/BRIEF.md", "# brief")
	}
	for name, content := range files {
		writeFile(t, root, "review/p/"+name, content)
	}
	return dir
}

func registered(round, expected int) string {
	return fmt.Sprintf(`{"bead_id":"mindspec-x.1","spec":"s","target":"bead/mindspec-x.1","round":%d,"expected_reviewers":%d,"reviewed_head_sha":"abc1234"}`, round, expected)
}

// sixSlots yields the default-panel slot names.
var sixSlots = []string{"claude-a", "claude-b", "claude-c", "codex-a", "codex-b", "codex-c"}

// roundFiles builds <slot>-round-<n>.json fixtures with the given
// verdicts (indexed against sixSlots).
func roundFiles(n int, verdicts ...string) map[string]string {
	files := make(map[string]string)
	for i, v := range verdicts {
		files[fmt.Sprintf("%s-round-%d.json", sixSlots[i], n)] = fmt.Sprintf(`{"verdict":%q,"confidence":0.9}`, v)
	}
	return files
}

func merge(ms ...map[string]string) map[string]string {
	out := make(map[string]string)
	for _, m := range ms {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// --- AC: APPROVE tally correct for 6/6, 5/6, 4/6 -------------------

func TestTally_ApproveCounts(t *testing.T) {
	cases := []struct {
		name     string
		verdicts []string
		approves int
		rejects  int
	}{
		{"6of6", []string{"APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE"}, 6, 0},
		{"5of6", []string{"APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE", "REQUEST_CHANGES"}, 5, 0},
		{"4of6", []string{"APPROVE", "APPROVE", "APPROVE", "APPROVE", "REQUEST_CHANGES", "REJECT"}, 4, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := panelDir(t, registered(1, 6), roundFiles(1, c.verdicts...))
			res, err := Tally(dir)
			if err != nil {
				t.Fatal(err)
			}
			if res.Panel == nil || res.PanelErr != nil {
				t.Fatalf("registration not parsed: %+v", res)
			}
			if res.LatestRound != 1 || res.RoundMismatch {
				t.Errorf("round state: latest=%d mismatch=%v", res.LatestRound, res.RoundMismatch)
			}
			if len(res.Verdicts) != 6 || res.Approves != c.approves || res.Rejects != c.rejects {
				t.Errorf("tally: verdicts=%d approves=%d rejects=%d, want 6/%d/%d",
					len(res.Verdicts), res.Approves, res.Rejects, c.approves, c.rejects)
			}
			if !res.Complete() || res.MissingCount() != 0 {
				t.Errorf("expected complete round: %+v", res)
			}
			if len(res.Malformed) != 0 || len(res.HardBlocks) != 0 {
				t.Errorf("unexpected malformed/hard blocks: %+v", res)
			}
		})
	}
}

// --- AC: filename-derived round wins over lagging panel.json.round --

func TestTally_FilenameRoundWinsOverLaggingPanelJSON(t *testing.T) {
	// panel.json still says round 1; reviewers have written round-2
	// files. The tally must reflect round 2 and report the mismatch.
	files := merge(
		roundFiles(1, "REQUEST_CHANGES", "REQUEST_CHANGES", "APPROVE", "APPROVE", "APPROVE", "APPROVE"),
		roundFiles(2, "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE"),
	)
	dir := panelDir(t, registered(1, 6), files)
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.LatestRound != 2 {
		t.Fatalf("LatestRound = %d, want filename-derived 2", res.LatestRound)
	}
	if !res.RoundMismatch {
		t.Error("RoundMismatch must report panel.json.round (1) != filename max (2)")
	}
	// Only round 2 tallied: 6 APPROVEs, not the round-1 mix.
	if res.Approves != 6 || len(res.Verdicts) != 6 {
		t.Errorf("tallied wrong round: approves=%d verdicts=%d", res.Approves, len(res.Verdicts))
	}
	for _, v := range res.Verdicts {
		if v.Round != 2 {
			t.Errorf("verdict from round %d leaked into latest-round tally: %+v", v.Round, v)
		}
	}
}

func TestTally_LeadingPanelJSONRoundAlsoMismatch(t *testing.T) {
	// Re-panel in flight: step 0 bumped panel.json to round 2 but no
	// round-2 verdicts exist yet. Filename-derived latest stays 1 and
	// the mismatch is reported — round-1 APPROVEs must not read as
	// the round-2 outcome.
	dir := panelDir(t, registered(2, 6), roundFiles(1, "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE"))
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.LatestRound != 1 || !res.RoundMismatch {
		t.Errorf("latest=%d mismatch=%v, want 1/true", res.LatestRound, res.RoundMismatch)
	}
}

func TestTally_NoVerdictFilesYet(t *testing.T) {
	// Step 0 just ran: panel.json exists, zero verdict files. Not a
	// round mismatch — an incomplete (0-verdict) round 0 state.
	dir := panelDir(t, registered(1, 6), nil)
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.LatestRound != 0 || res.RoundMismatch {
		t.Errorf("latest=%d mismatch=%v, want 0/false", res.LatestRound, res.RoundMismatch)
	}
	if len(res.Verdicts) != 0 || res.Complete() || res.MissingCount() != 6 {
		t.Errorf("expected empty incomplete tally: %+v", res)
	}
}

// --- AC: malformed verdict counted missing and named ----------------

func TestTally_MalformedVerdictIsMissingAndNamed(t *testing.T) {
	files := roundFiles(1, "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE")
	files["codex-c-round-1.json"] = `{this is not json`
	dir := panelDir(t, registered(1, 6), files)
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Verdicts) != 5 || res.Approves != 5 {
		t.Errorf("malformed verdict leaked into tally: verdicts=%d approves=%d", len(res.Verdicts), res.Approves)
	}
	if !reflect.DeepEqual(res.Malformed, []string{"codex-c-round-1.json"}) {
		t.Errorf("Malformed = %v, want the bad file named", res.Malformed)
	}
	if res.Complete() || res.MissingCount() != 1 {
		t.Errorf("malformed must count as missing: complete=%v missing=%d", res.Complete(), res.MissingCount())
	}
}

func TestTally_MissingVerdictFieldIsMalformed(t *testing.T) {
	files := roundFiles(1, "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE")
	files["codex-c-round-1.json"] = `{"confidence": 0.8, "concrete_changes_required": []}`
	dir := panelDir(t, registered(1, 6), files)
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Malformed, []string{"codex-c-round-1.json"}) {
		t.Errorf("verdict-less JSON must be malformed/missing, got %v", res.Malformed)
	}
	if len(res.Verdicts) != 5 {
		t.Errorf("verdicts = %d, want 5", len(res.Verdicts))
	}
}

// --- AC: hard_block parsed ------------------------------------------

func TestTally_HardBlockParsed(t *testing.T) {
	files := roundFiles(1, "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE")
	files["codex-c-round-1.json"] = `{"verdict":"REQUEST_CHANGES","hard_block":true}`
	dir := panelDir(t, registered(1, 6), files)
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.HardBlocks, []string{"codex-c"}) {
		t.Errorf("HardBlocks = %v, want [codex-c]", res.HardBlocks)
	}
	var hb *Verdict
	for i := range res.Verdicts {
		if res.Verdicts[i].Slot == "codex-c" {
			hb = &res.Verdicts[i]
		}
	}
	if hb == nil || !hb.HardBlock || hb.Verdict != VerdictRequestChanges {
		t.Errorf("hard_block verdict not parsed: %+v", hb)
	}
}

// --- AC: expected_reviewers parameterization (DQ5, 3-reviewer panel) -

func TestTally_ExpectedReviewersThree(t *testing.T) {
	files := map[string]string{
		"claude-a-round-1.json": `{"verdict":"APPROVE"}`,
		"claude-b-round-1.json": `{"verdict":"APPROVE"}`,
		"claude-c-round-1.json": `{"verdict":"REQUEST_CHANGES"}`,
	}
	dir := panelDir(t, registered(1, 3), files)
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExpectedReviewers() != 3 || !res.Complete() {
		t.Errorf("expected complete 3-reviewer round: %+v", res)
	}
	if res.Approves != 2 {
		t.Errorf("Approves = %d, want 2", res.Approves)
	}
	// Threshold rule: N−1 = 2 — this round meets it.
	if th := res.Panel.ApproveThreshold(); th != 2 || res.Approves < th {
		t.Errorf("threshold = %d approves = %d; 2-of-3 must meet N−1", th, res.Approves)
	}
}

// --- Supporting shapes ----------------------------------------------

func TestTally_UnregisteredLegacyDir(t *testing.T) {
	dir := panelDir(t, "", map[string]string{"claude-a-round-1.json": `{"verdict":"APPROVE"}`})
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Panel != nil || res.PanelErr != nil {
		t.Errorf("legacy dir must be unregistered: %+v", res)
	}
	if res.LatestRound != 1 || res.Approves != 1 {
		t.Errorf("verdicts still tallied for visibility: %+v", res)
	}
	if res.Complete() {
		t.Error("unregistered dir can never be Complete")
	}
}

func TestTally_MalformedPanelJSONSurfaced(t *testing.T) {
	dir := panelDir(t, `{broken`, roundFiles(1, "APPROVE"))
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Panel != nil || res.PanelErr == nil {
		t.Errorf("malformed panel.json must set PanelErr, not Panel: %+v", res)
	}
}

func TestTally_MissingDirErrors(t *testing.T) {
	if _, err := Tally(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestTally_ConsolidatedPresenceForLatestRoundOnly(t *testing.T) {
	files := merge(
		roundFiles(1, "APPROVE", "APPROVE", "APPROVE", "APPROVE", "REQUEST_CHANGES", "REQUEST_CHANGES"),
		roundFiles(2, "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE"),
	)
	files["consolidated-round-1.md"] = "# changes"
	dir := panelDir(t, registered(2, 6), files)
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasConsolidated {
		t.Error("consolidated-round-1.md must not count for latest round 2")
	}

	writeFile(t, filepath.Dir(filepath.Dir(dir)), "review/p/consolidated-round-2.md", "# changes")
	res, err = Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasConsolidated {
		t.Error("consolidated-round-2.md must be detected for latest round 2")
	}
}

func TestTally_VerdictNormalizationAndNonVerdictFilesIgnored(t *testing.T) {
	files := map[string]string{
		"claude-a-round-1.json": `{"verdict":" approve "}`, // trims + uppercases
		"claude-b-round-1.json": `{"verdict":"reject"}`,
		"notes.json":            `{"verdict":"APPROVE"}`, // no -round-<N> suffix: ignored
		"BRIEF.md":              "# brief",
	}
	dir := panelDir(t, registered(1, 6), files)
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Verdicts) != 2 || res.Approves != 1 || res.Rejects != 1 {
		t.Errorf("normalization/ignore rules: %+v", res)
	}
}

func TestTally_VerdictsSortedBySlot(t *testing.T) {
	dir := panelDir(t, registered(1, 6), roundFiles(1, "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE", "APPROVE"))
	res, err := Tally(dir)
	if err != nil {
		t.Fatal(err)
	}
	var slots []string
	for _, v := range res.Verdicts {
		slots = append(slots, v.Slot)
	}
	if !reflect.DeepEqual(slots, sixSlots) {
		t.Errorf("verdicts not sorted by slot: %v", slots)
	}
}
