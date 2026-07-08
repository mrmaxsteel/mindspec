package panel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestCreate_WritesRegistrationAtomically(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "reviews", "demo")
	beadID := "mindspec-x.1"

	in1 := CreateInput{
		BeadID:               &beadID,
		Spec:                 "110-panel-verbs-parser-parity",
		Target:               "bead/mindspec-x.1",
		Round:                1,
		ExpectedReviewers:    6,
		ApproveThresholdExpr: "n-1",
		ReviewedHeadSHA:      "abc1234abc1234abc1234abc1234abc1234abc1",
	}
	if err := Create(dir, in1); err != nil {
		t.Fatalf("Create (round 1): %v", err)
	}

	panelData, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read panel.json: %v", err)
	}
	if !strings.Contains(string(panelData), `"reviewed_head_sha"`) {
		t.Fatalf("panel.json omits the reviewed_head_sha key:\n%s", panelData)
	}
	var got Panel
	if err := json.Unmarshal(panelData, &got); err != nil {
		t.Fatalf("unmarshal panel.json: %v", err)
	}
	if got.ExpectedReviewers != in1.ExpectedReviewers ||
		got.ApproveThresholdExpr != in1.ApproveThresholdExpr ||
		got.ReviewedHeadSHA != in1.ReviewedHeadSHA ||
		got.Round != in1.Round ||
		got.BeadID == nil || *got.BeadID != *in1.BeadID ||
		got.Spec != in1.Spec ||
		got.Target != in1.Target {
		t.Fatalf("panel.json round-trip mismatch: got %+v, want fields of %+v", got, in1)
	}

	brief1, err := os.ReadFile(filepath.Join(dir, "BRIEF.md"))
	if err != nil {
		t.Fatalf("read BRIEF.md: %v", err)
	}
	header1 := extractHeader(t, string(brief1))
	if !strings.Contains(header1, "Round**: 1") || !strings.Contains(header1, in1.ReviewedHeadSHA) {
		t.Fatalf("BRIEF header missing round/SHA:\n%s", header1)
	}
	if !strings.Contains(header1, "## Your job") || !strings.Contains(header1, "hard_block") {
		t.Fatalf("BRIEF header missing the 'Your job' hard_block contract:\n%s", header1)
	}

	// Pre-seed a skill-authored body below the header and a round-1
	// verdict file, simulating a completed first round.
	skillBody := "## Summary\n\nThis panel reviews the leaf writer.\n"
	brief1Str := string(brief1)
	closeEnd1 := strings.Index(brief1Str, briefHeaderClose) + len(briefHeaderClose)
	seeded := brief1Str[:closeEnd1] + "\n\n" + skillBody
	if err := os.WriteFile(filepath.Join(dir, "BRIEF.md"), []byte(seeded), 0o644); err != nil {
		t.Fatalf("seed skill-authored body: %v", err)
	}
	verdictPath := filepath.Join(dir, "R1-round-1.json")
	verdictContent := `{"verdict":"APPROVE"}`
	if err := os.WriteFile(verdictPath, []byte(verdictContent), 0o644); err != nil {
		t.Fatalf("seed round-1 verdict file: %v", err)
	}
	wantAfterBody := afterHeader(seeded)

	in2 := in1
	in2.Round = 2
	in2.ReviewedHeadSHA = "def5678def5678def5678def5678def5678def5"
	if err := Create(dir, in2); err != nil {
		t.Fatalf("Create (round 2): %v", err)
	}

	panelData2, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read panel.json (round 2): %v", err)
	}
	var got2 Panel
	if err := json.Unmarshal(panelData2, &got2); err != nil {
		t.Fatalf("unmarshal panel.json (round 2): %v", err)
	}
	if got2.Round != 2 || got2.ReviewedHeadSHA != in2.ReviewedHeadSHA {
		t.Fatalf("panel.json not co-bumped: got round=%d sha=%s", got2.Round, got2.ReviewedHeadSHA)
	}

	brief2, err := os.ReadFile(filepath.Join(dir, "BRIEF.md"))
	if err != nil {
		t.Fatalf("read BRIEF.md (round 2): %v", err)
	}
	brief2Str := string(brief2)
	header2 := extractHeader(t, brief2Str)
	if !strings.Contains(header2, "Round**: 2") || !strings.Contains(header2, in2.ReviewedHeadSHA) {
		t.Fatalf("BRIEF header not co-bumped:\n%s", header2)
	}

	gotAfterBody := afterHeader(brief2Str)
	if gotAfterBody != wantAfterBody {
		t.Fatalf("skill-authored body changed by re-panel:\nbefore: %q\nafter:  %q", wantAfterBody, gotAfterBody)
	}

	verdictAfter, err := os.ReadFile(verdictPath)
	if err != nil {
		t.Fatalf("round-1 verdict file missing after re-panel: %v", err)
	}
	if string(verdictAfter) != verdictContent {
		t.Fatalf("round-1 verdict file modified by re-panel: got %q, want %q", verdictAfter, verdictContent)
	}
}

func TestCreate_BRIEFMarkerEdgeCases(t *testing.T) {
	in := CreateInput{
		Spec:              "110-panel-verbs-parser-parity",
		Target:            "bead/mindspec-x.1",
		Round:             1,
		ExpectedReviewers: 6,
		ReviewedHeadSHA:   "cafefeedcafefeedcafefeedcafefeedcafefeed",
	}

	t.Run("legacy_no_marker_body_preserved", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "reviews", "demo")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		original := "# Legacy Brief\n\nHand-written before the verb existed.\n"
		briefPath := filepath.Join(dir, "BRIEF.md")
		if err := os.WriteFile(briefPath, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := Create(dir, in); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := os.ReadFile(briefPath)
		if err != nil {
			t.Fatal(err)
		}
		want := renderBriefHeader(filepath.Base(dir), in.Round, in.Target, in.ReviewedHeadSHA) + "\n\n" + original
		if string(got) != want {
			t.Fatalf("legacy BRIEF.md not prepended byte-identically:\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("marker_only_open_rejected", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "reviews", "demo")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		corrupt := briefHeaderOpen + "\nno closing marker\n"
		assertCreateRejectedWithNeitherFileTouched(t, dir, in, corrupt)
	})

	t.Run("duplicated_markers_rejected", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "reviews", "demo")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		corrupt := briefHeaderOpen + "\nA\n" + briefHeaderClose + "\n" +
			briefHeaderOpen + "\nB\n" + briefHeaderClose + "\n"
		assertCreateRejectedWithNeitherFileTouched(t, dir, in, corrupt)
	})

	t.Run("crlf_body_preserved", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "reviews", "demo")
		if err := Create(dir, in); err != nil {
			t.Fatalf("initial Create: %v", err)
		}
		brief1, err := os.ReadFile(filepath.Join(dir, "BRIEF.md"))
		if err != nil {
			t.Fatal(err)
		}
		crlfBody := "\r\n## Summary\r\n\r\nWritten with CRLF endings.\r\n"
		closeEnd := strings.Index(string(brief1), briefHeaderClose) + len(briefHeaderClose)
		seeded := string(brief1)[:closeEnd] + crlfBody
		if err := os.WriteFile(filepath.Join(dir, "BRIEF.md"), []byte(seeded), 0o644); err != nil {
			t.Fatal(err)
		}

		in2 := in
		in2.Round = 2
		in2.ReviewedHeadSHA = "beefbeefbeefbeefbeefbeefbeefbeefbeefbeef"
		if err := Create(dir, in2); err != nil {
			t.Fatalf("re-panel Create: %v", err)
		}
		brief2, err := os.ReadFile(filepath.Join(dir, "BRIEF.md"))
		if err != nil {
			t.Fatal(err)
		}
		gotAfter := afterHeader(string(brief2))
		if gotAfter != crlfBody {
			t.Fatalf("CRLF body not preserved byte-for-byte:\ngot:  %q\nwant: %q", gotAfter, crlfBody)
		}
	})
}

// assertCreateRejectedWithNeitherFileTouched seeds dir/BRIEF.md with a
// corrupt-marker body, calls Create, and asserts it errors while
// leaving both panel.json (absent) and BRIEF.md (its exact pre-call
// content and mtime) untouched.
func assertCreateRejectedWithNeitherFileTouched(t *testing.T, dir string, in CreateInput, corruptBrief string) {
	t.Helper()
	briefPath := filepath.Join(dir, "BRIEF.md")
	if err := os.WriteFile(briefPath, []byte(corruptBrief), 0o644); err != nil {
		t.Fatal(err)
	}
	briefInfoBefore, err := os.Stat(briefPath)
	if err != nil {
		t.Fatal(err)
	}
	panelPath := filepath.Join(dir, FileName)
	if _, err := os.Stat(panelPath); !os.IsNotExist(err) {
		t.Fatalf("panel.json unexpectedly exists before Create: err=%v", err)
	}

	if err := Create(dir, in); err == nil {
		t.Fatal("Create: expected an error for a corrupt BRIEF marker state, got nil")
	}

	if _, err := os.Stat(panelPath); !os.IsNotExist(err) {
		t.Fatalf("panel.json was written despite the error: err=%v", err)
	}
	briefInfoAfter, err := os.Stat(briefPath)
	if err != nil {
		t.Fatal(err)
	}
	if !briefInfoBefore.ModTime().Equal(briefInfoAfter.ModTime()) || briefInfoBefore.Size() != briefInfoAfter.Size() {
		t.Fatalf("BRIEF.md metadata changed: before mtime=%v size=%d, after mtime=%v size=%d",
			briefInfoBefore.ModTime(), briefInfoBefore.Size(), briefInfoAfter.ModTime(), briefInfoAfter.Size())
	}
	gotBrief, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBrief) != corruptBrief {
		t.Fatalf("BRIEF.md content changed:\nbefore: %q\nafter:  %q", corruptBrief, gotBrief)
	}
}

// extractHeader returns the delimited machine-managed region
// (including both markers) from a BRIEF.md's contents, failing the
// test if the markers are absent.
func extractHeader(t *testing.T, brief string) string {
	t.Helper()
	openIdx := strings.Index(brief, briefHeaderOpen)
	closeIdx := strings.Index(brief, briefHeaderClose)
	if openIdx < 0 || closeIdx < 0 {
		t.Fatalf("BRIEF.md missing header markers:\n%s", brief)
	}
	return brief[openIdx : closeIdx+len(briefHeaderClose)]
}

// afterHeader returns everything after the closing header marker, or
// "" if the marker is absent.
func afterHeader(brief string) string {
	closeIdx := strings.Index(brief, briefHeaderClose)
	if closeIdx < 0 {
		return ""
	}
	return brief[closeIdx+len(briefHeaderClose):]
}

// roundFileTokenRE extracts a backtick-quoted `<slot>-round-<N>.json`
// example from the schema doc, plus an optional immediately-following
// "(nonconforming ...)" label that marks it as a deliberately invalid
// example rather than a conforming one.
var roundFileTokenRE = regexp.MustCompile("`([^`]+-round-[0-9]+\\.json)`(\\s*\\(nonconforming[^)]*\\))?")

// consolidatedTokenRE extracts a backtick-quoted
// `consolidated-round-<N>.md` example, capturing N.
var consolidatedTokenRE = regexp.MustCompile("`(consolidated-round-([0-9]+)\\.md)`")

// panelJSONTokenRE extracts the backtick-quoted `panel.json` literal.
var panelJSONTokenRE = regexp.MustCompile("`(panel\\.json)`")

// TestPanelSchemaDoc_MatchesConstants pins the R4 portability-contract
// doc (.mindspec/domains/workflow/interfaces.md § Panel Artifact
// Schema) against the internal/panel constants it documents, by
// extracting the doc's OWN backtick-quoted examples rather than
// testing a test-held mirror of them — a doc edit that widens or
// narrows the pattern is caught because the expectation is re-derived
// from the doc's own text.
func TestPanelSchemaDoc_MatchesConstants(t *testing.T) {
	docPath := filepath.Join("..", "..", ".mindspec", "domains", "workflow", "interfaces.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read schema doc %s: %v", docPath, err)
	}
	doc := string(data)

	// panel.json literal — exact equality, not merely "contains".
	pm := panelJSONTokenRE.FindStringSubmatch(doc)
	if pm == nil {
		t.Fatal("schema doc does not backtick-quote the panel.json registration filename")
	}
	if pm[1] != FileName {
		t.Errorf("schema doc's registration filename %q != panel.FileName %q", pm[1], FileName)
	}

	// Verdict-file examples: every non-labeled token must match
	// verdictFileRE; every "(nonconforming...)"-labeled token must NOT.
	roundMatches := roundFileTokenRE.FindAllStringSubmatch(doc, -1)
	if len(roundMatches) == 0 {
		t.Fatal("schema doc has no backtick-quoted <slot>-round-<N>.json examples")
	}
	var sawConforming, sawNonconforming bool
	for _, m := range roundMatches {
		token, labeledNonconforming := m[1], m[2] != ""
		matches := verdictFileRE.MatchString(token)
		if labeledNonconforming {
			sawNonconforming = true
			if matches {
				t.Errorf("schema doc's nonconforming example %q actually matches verdictFileRE", token)
			}
		} else {
			sawConforming = true
			if !matches {
				t.Errorf("schema doc's conforming example %q does not match verdictFileRE", token)
			}
		}
	}
	if !sawConforming {
		t.Error("schema doc has no conforming verdict-file example")
	}
	if !sawNonconforming {
		t.Error("schema doc has no doc-labeled nonconforming verdict-file example")
	}

	// Consolidated-file example — exact equality against ConsolidatedName(N).
	cm := consolidatedTokenRE.FindStringSubmatch(doc)
	if cm == nil {
		t.Fatal("schema doc has no backtick-quoted consolidated-round-<N>.md example")
	}
	n, err := strconv.Atoi(cm[2])
	if err != nil {
		t.Fatalf("schema doc's consolidated example has a non-numeric round: %v", err)
	}
	if want := ConsolidatedName(n); cm[1] != want {
		t.Errorf("schema doc's consolidated example %q != panel.ConsolidatedName(%d) = %q", cm[1], n, want)
	}

	// Verdict-enum literals and the top-level hard_block field.
	for _, lit := range []string{VerdictApprove, VerdictRequestChanges, VerdictReject} {
		if !strings.Contains(doc, lit) {
			t.Errorf("schema doc missing the verdict enum literal %q", lit)
		}
	}
	if !strings.Contains(doc, "hard_block") {
		t.Fatal("schema doc does not mention hard_block")
	}

	// Regression guard: hard_block must never read as a per-finding
	// field. A crude but effective proxy — no sentence mentioning
	// hard_block also mentions "finding" — catches the exact
	// finding-level phrasing this spec removes from the skills.
	for _, sentence := range regexp.MustCompile(`[.\n]`).Split(doc, -1) {
		lower := strings.ToLower(sentence)
		if strings.Contains(sentence, "hard_block") && strings.Contains(lower, "finding") {
			t.Errorf("schema doc's hard_block mention shares a sentence with 'finding' (per-finding phrasing is disallowed): %q", strings.TrimSpace(sentence))
		}
	}
}
