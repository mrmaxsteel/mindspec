package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// The real entry points share the full-ID resolver. The fake tracker and mock
// executor make all reads and writes local to this test.
func TestLifecycle_LetteredSpecResolves(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			f := newNextHonestyFixture(t)
			withTestChdir(t, f.root)
			specID := "129b-y"
			meta := map[string]interface{}{"spec_id": specID, "spec_num": 129, "spec_title": "y", "mindspec_phase": "plan", phase.PhaseSemanticsV2MarkerKey: "v2"}
			if legacy {
				specID = "008b-human-gates"
				meta["spec_num"] = 8
				meta["spec_title"] = "human-gates"
				delete(meta, "spec_id")
			}
			f.epics = []map[string]interface{}{
				{"id": "epic-base", "title": "[SPEC 129-x] base", "status": "open", "issue_type": "epic", "metadata": map[string]interface{}{"spec_id": "129-x", "mindspec_phase": "plan"}},
				{"id": "epic-letter", "title": "[SPEC " + specID + "] letter", "status": "open", "issue_type": "epic", "metadata": meta},
			}
			f.rows = []map[string]interface{}{nextHonestyRow("epic-letter.1", "AC-1", "epic-letter", "task", "open")}
			f.write(t)
			writeVerbFixtureFile(t, f.root, filepath.Join(".mindspec/specs", specID, "spec.md"), validVerbFixtureSpecMD(specID))
			plan := strings.Replace(validVerbFixturePlanMD(specID), "bead_ids: []", "bead_ids: []\nwork_chunks:\n  - id: 1\n    key_file_paths: [internal/fixture.go]", 1)
			writeVerbFixtureFile(t, f.root, filepath.Join(".mindspec/specs", specID, "plan.md"), plan)
			writeVerbFixtureFile(t, f.root, ".mindspec/adr/ADR-9999.md", "# ADR-9999: Fixture\n\n- **Status**: Accepted\n- **Domain(s)**: core\n\n## Decision\n\nUse this fixture.\n")

			assertRepair := func(t *testing.T, err error) {
				t.Helper()
				want := "recovery: bd update epic-letter --set-metadata spec_id=" + specID
				if err == nil {
					t.Fatal("expected spec resolution refusal")
				}
				lines := strings.Split(err.Error(), "\n")
				if !guard.HasFinalRecoveryLine(err.Error()) || lines[len(lines)-1] != want || strings.Contains(err.Error(), "repair spec-title") {
					t.Fatalf("expected final spec_id repair %q: %v", want, err)
				}
			}
			if legacy {
				t.Run("next-repair", func(t *testing.T) { _, err := next.BuildReadySet(f.root, specID); assertRepair(t, err) })
				t.Run("complete-repair", func(t *testing.T) {
					_, err := complete.Run(f.root, "epic-letter.1", "", "", &executor.MockExecutor{}, complete.CompleteOpts{})
					assertRepair(t, err)
				})
				t.Run("plan-repair", func(t *testing.T) {
					_, err := approve.ApprovePlan(f.root, specID, "test", &executor.MockExecutor{})
					assertRepair(t, err)
				})
				meta["spec_id"] = "Bad_ID"
				f.write(t)
				t.Run("complete-invalid-id", func(t *testing.T) {
					_, err := complete.Run(f.root, "epic-letter.1", "", "", &executor.MockExecutor{}, complete.CompleteOpts{})
					assertRepair(t, err)
				})
				meta["spec_id"] = specID
				f.write(t)
			}

			f.epics = append(f.epics, map[string]interface{}{"id": "epic-duplicate", "status": "closed", "issue_type": "epic", "metadata": map[string]interface{}{"spec_id": specID}})
			f.write(t)
			for _, verb := range []string{"next", "complete", "plan"} {
				t.Run(verb+"-duplicate", func(t *testing.T) {
					var err error
					switch verb {
					case "next":
						_, err = next.BuildReadySet(f.root, specID)
					case "complete":
						_, err = complete.Run(f.root, "epic-letter.1", "", "", &executor.MockExecutor{}, complete.CompleteOpts{})
					case "plan":
						_, err = approve.ApprovePlan(f.root, specID, "test", &executor.MockExecutor{})
					}
					if !errors.Is(err, phase.ErrSpecResolution) {
						t.Fatalf("expected a typed duplicate refusal: %v", err)
					}
					lines := strings.Split(err.Error(), "\n")
					if !guard.HasFinalRecoveryLine(err.Error()) || lines[len(lines)-1] != "recovery: bd show epic-letter --json" {
						t.Fatalf("expected final duplicate inspection: %v", err)
					}
				})
			}
			f.epics = f.epics[:len(f.epics)-1]
			f.write(t)
			rs, err := next.BuildReadySet(f.root, specID)
			if err != nil || rs.EpicID != "epic-letter" || len(rs.Filtered) != 1 || rs.Filtered[0].ID != "epic-letter.1" {
				t.Fatalf("next: %+v %v", rs, err)
			}
			// A matching hint reaches the real phase guard for the lettered spec.
			// It must not reach the numeric base epic or a later completion mutation.
			_, err = complete.Run(f.root, "epic-letter.1", specID, "", &executor.MockExecutor{}, complete.CompleteOpts{})
			if err == nil || !strings.Contains(err.Error(), "belongs to spec "+specID+" which is in 'plan' phase") {
				t.Fatalf("complete selected wrong spec: %v", err)
			}
			t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
			t.Cleanup(approve.SetPlanMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
			t.Cleanup(approve.SetPlanListJSONForTest(func(...string) ([]byte, error) { return []byte(`[]`), nil }))
			created := false
			fakeCreate := func(args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "create" {
					parent := ""
					for i, a := range args {
						if a == "--parent" && i+1 < len(args) {
							parent = args[i+1]
						}
					}
					if parent != "epic-letter" {
						t.Fatalf("plan used wrong epic: %v", args)
					}
					created = true
					return []byte(`{"id":"epic-letter.1"}`), nil
				}
				return []byte(`[]`), nil
			}
			t.Cleanup(approve.SetPlanRunBDForTest(fakeCreate))
			t.Cleanup(approve.SetPlanRunBDCombinedForTest(fakeCreate))
			mock := &executor.MockExecutor{}
			result, err := approve.ApprovePlan(f.root, specID, "test", mock)
			if err != nil || !created || result.SpecID != specID {
				t.Fatalf("plan approval: %+v %v", result, err)
			}
			calls := mock.CallsTo("HandoffEpic")
			if len(calls) != 1 || calls[0].Args[0] != "epic-letter" || calls[0].Args[1] != specID {
				raw, _ := json.Marshal(calls)
				t.Fatalf("wrong handoff: %s", raw)
			}
		})
	}
}
