package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// stubDryRunFinders wires the package-level phase bd seams so the dry-run
// reporter can resolve specs against in-memory fixtures.
//
// epicsJSON is the JSON array body of a `bd list --type=epic …` response.
// childrenByEpicID maps epicID → JSON array body for `bd list --parent …`.
// epicByID maps epicID → JSON object body for `bd show <epicID> --json`
// (each value is one EpicInfo serialised as a JSON object; the stub
// wraps it in `[…]` to match bd's array-of-one output shape).
func stubDryRunFinders(
	t *testing.T,
	epicsJSON string,
	childrenByEpicID map[string]string,
	epicByID map[string]string,
) func() {
	t.Helper()

	restoreList := phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--parent" && i+1 < len(args) {
				parent := args[i+1]
				if body, ok := childrenByEpicID[parent]; ok {
					return []byte(body), nil
				}
				return []byte("[]"), nil
			}
			if strings.HasPrefix(a, "--parent=") {
				parent := strings.TrimPrefix(a, "--parent=")
				if body, ok := childrenByEpicID[parent]; ok {
					return []byte(body), nil
				}
				return []byte("[]"), nil
			}
		}
		for _, a := range args {
			if a == "--type=epic" {
				return []byte(epicsJSON), nil
			}
		}
		return []byte("[]"), nil
	})

	restoreRun := phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			id := args[1]
			if body, ok := epicByID[id]; ok {
				return []byte("[" + body + "]"), nil
			}
			return []byte("[]"), nil
		}
		return []byte("[]"), nil
	})

	return func() {
		restoreList()
		restoreRun()
	}
}

// writeSpecDir creates an empty `.mindspec/docs/specs/<specID>/` directory
// under root. The reporter only inspects directory names, not contents.
func writeSpecDir(t *testing.T, root, specID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "specs", specID), 0o755); err != nil {
		t.Fatalf("mkdir spec %s: %v", specID, err)
	}
}

// TestDoctorDryRunMigrationReports — fixture with one legacy spec
// (epic missing mindspec_phase) and one already-migrated spec; the
// report must contain a `would-migrate` line for the legacy and NOT
// for the migrated. The legacy epic metadata must remain untouched
// after the run (no writes — the seam catches any MergeMetadata call).
func TestDoctorDryRunMigrationReports(t *testing.T) {
	root := t.TempDir()
	writeSpecDir(t, root, "007-legacy")
	writeSpecDir(t, root, "008-native")

	epicLegacy := `{"id":"epic-7","title":"[SPEC 007-legacy] Legacy","status":"open","issue_type":"epic","metadata":{"spec_num":7,"spec_title":"legacy"}}`
	epicNative := `{"id":"epic-8","title":"[SPEC 008-native] Native","status":"open","issue_type":"epic","metadata":{"spec_num":8,"spec_title":"native","mindspec_phase":"review"}}`

	epicsJSON := "[" + epicLegacy + "," + epicNative + "]"
	defer stubDryRunFinders(
		t,
		epicsJSON,
		map[string]string{
			"epic-7": `[{"id":"b1","title":"bead1","status":"closed","issue_type":"task"},{"id":"b2","title":"bead2","status":"in_progress","issue_type":"task"}]`,
			"epic-8": `[{"id":"b3","title":"bead3","status":"closed","issue_type":"task"}]`,
		},
		map[string]string{
			"epic-7": epicLegacy,
			"epic-8": epicNative,
		},
	)()

	// Catch any accidental write: MergeMetadata must NOT be called by
	// the dry-run reporter. If it ever is, the test fails loudly.
	mergeCalls := 0
	restoreMerge := phase.SetMergeMetadataForTest(func(issueID string, updates map[string]interface{}) error {
		mergeCalls++
		return nil
	})
	defer restoreMerge()

	report := RunWithOptions(root, Options{DryRunMigration: true})

	if mergeCalls != 0 {
		t.Fatalf("dry-run path must not write; MergeMetadata calls = %d", mergeCalls)
	}

	var sawLegacy, sawNative bool
	for _, c := range report.Checks {
		switch c.Name {
		case "would-migrate: spec=007-legacy":
			sawLegacy = true
			if c.Status != Warn {
				t.Errorf("legacy spec status = %d, want Warn (%d)", c.Status, Warn)
			}
			if !strings.Contains(c.Message, "epic=epic-7") {
				t.Errorf("legacy message missing epic id: %q", c.Message)
			}
			if !strings.Contains(c.Message, "phase=implement") {
				t.Errorf("legacy message wrong derived phase: %q (want phase=implement)", c.Message)
			}
		case "would-migrate: spec=008-native":
			sawNative = true
		}
	}
	if !sawLegacy {
		t.Errorf("expected would-migrate report for 007-legacy; checks=%+v", report.Checks)
	}
	if sawNative {
		t.Errorf("did not expect would-migrate report for already-migrated 008-native; checks=%+v", report.Checks)
	}
}

// TestDoctorDryRunMigrationExitsZeroWithLegacySpecs — the presence of
// legacy specs must NOT cause HasFailures() true. Per spec 089
// Requirement 11 the dry-run path emits Warn-status checks only.
func TestDoctorDryRunMigrationExitsZeroWithLegacySpecs(t *testing.T) {
	root := t.TempDir()
	writeSpecDir(t, root, "010-legacy-a")
	writeSpecDir(t, root, "011-legacy-b")

	epicA := `{"id":"epic-10","title":"[SPEC 010-legacy-a] A","status":"open","issue_type":"epic","metadata":{"spec_num":10,"spec_title":"legacy-a"}}`
	epicB := `{"id":"epic-11","title":"[SPEC 011-legacy-b] B","status":"open","issue_type":"epic","metadata":{"spec_num":11,"spec_title":"legacy-b"}}`

	epicsJSON := "[" + epicA + "," + epicB + "]"
	defer stubDryRunFinders(
		t,
		epicsJSON,
		map[string]string{
			"epic-10": `[{"id":"b1","status":"open","issue_type":"task"}]`,
			"epic-11": `[{"id":"b2","status":"closed","issue_type":"task"}]`,
		},
		map[string]string{
			"epic-10": epicA,
			"epic-11": epicB,
		},
	)()

	report := RunWithOptions(root, Options{DryRunMigration: true})

	if report.HasFailures() {
		t.Fatalf("HasFailures() = true with only Warn-status legacy reports; checks=%+v", report.Checks)
	}

	// Sanity: both legacies showed up. Otherwise the "no failures"
	// assertion is vacuous.
	count := 0
	for _, c := range report.Checks {
		if strings.HasPrefix(c.Name, "would-migrate: ") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 would-migrate checks, got %d; checks=%+v", count, report.Checks)
	}
}

// TestDoctorDryRunMigrationSkipsExcludedTrees — spec-directory entries
// whose names begin with a viz/agentmind/bench prefix (defensive HC-4
// guard) must not appear in the report even when the bd fixture would
// otherwise resolve them.
func TestDoctorDryRunMigrationSkipsExcludedTrees(t *testing.T) {
	root := t.TempDir()
	writeSpecDir(t, root, "020-real-legacy")
	writeSpecDir(t, root, "viz-something")
	writeSpecDir(t, root, "agentmind-extract")
	writeSpecDir(t, root, "bench-suite")

	epicReal := `{"id":"epic-20","title":"[SPEC 020-real-legacy] R","status":"open","issue_type":"epic","metadata":{"spec_num":20,"spec_title":"real-legacy"}}`
	// Excluded-prefix epics deliberately registered so we can prove
	// the guard fires at the reporter (not just at bd resolution).
	epicViz := `{"id":"epic-viz","title":"viz","status":"open","issue_type":"epic","metadata":{"spec_num":900,"spec_title":"something"}}`

	epicsJSON := "[" + epicReal + "," + epicViz + "]"
	defer stubDryRunFinders(
		t,
		epicsJSON,
		map[string]string{
			"epic-20":  `[{"id":"b1","status":"in_progress","issue_type":"task"}]`,
			"epic-viz": `[{"id":"b2","status":"open","issue_type":"task"}]`,
		},
		map[string]string{
			"epic-20":  epicReal,
			"epic-viz": epicViz,
		},
	)()

	report := RunWithOptions(root, Options{DryRunMigration: true})

	for _, c := range report.Checks {
		for _, banned := range []string{"viz-", "agentmind-", "bench-"} {
			if strings.Contains(c.Name, "spec="+banned) {
				t.Errorf("excluded-tree spec leaked into report: %+v", c)
			}
		}
	}

	sawReal := false
	for _, c := range report.Checks {
		if c.Name == "would-migrate: spec=020-real-legacy" {
			sawReal = true
		}
	}
	if !sawReal {
		t.Errorf("expected legitimate legacy 020-real-legacy in report; checks=%+v", report.Checks)
	}
}

// TestADR0034FinalizedStatus — ADR-0034's ## Status block must no
// longer contain the "Stub created" placeholder language after Bead 3.
func TestADR0034FinalizedStatus(t *testing.T) {
	// Walk up from this test file's directory to the repo root.
	// Using a relative path keeps the test independent of where the
	// repo is checked out.
	// Spec 106: the flat layout serves ADRs from .mindspec/adr/; the
	// canonical .mindspec/docs/adr/ candidates are kept for historical
	// checkouts.
	candidates := []string{
		filepath.Join("..", "..", ".mindspec", "adr", "ADR-0034-ceremony-collapse.md"),
		filepath.Join("..", "..", "..", ".mindspec", "adr", "ADR-0034-ceremony-collapse.md"),
		filepath.Join("..", "..", ".mindspec", "docs", "adr", "ADR-0034-ceremony-collapse.md"),
		filepath.Join("..", "..", "..", ".mindspec", "docs", "adr", "ADR-0034-ceremony-collapse.md"),
	}
	var data []byte
	var err error
	for _, p := range candidates {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("read ADR-0034: %v (tried: %v)", err, candidates)
	}
	body := string(data)
	if strings.Contains(body, "Stub created") {
		t.Fatalf("ADR-0034 still contains the 'Stub created' placeholder; Bead 3 step 5 not applied")
	}
	if !strings.Contains(body, "Finalized in spec 089 Bead 3") {
		t.Fatalf("ADR-0034 missing the finalized-status marker text")
	}
}
