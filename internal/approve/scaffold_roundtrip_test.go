package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/spec"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// TestScaffoldPlan_MinimallyFilled_RoundTripsThroughValidator proves the
// spec 119 Bead 4 scaffold additions (work_chunks frontmatter, per-bead
// **Acceptance Criteria**, labeled non-authoritative **Depends on**) are not
// merely decorative: filled in with the smallest possible edits (only the
// bracketed placeholders), the emitted plan.md passes EVERY structural check
// internal/validate/plan.go runs (AC-21). No bd/phase stubbing is needed —
// the scaffold's frontmatter carries no bead_ids and status: Draft, so
// checkBeadIDs and checkPlanApprovalGateConsistency both no-op.
//
// RED on revert: reverting the scaffoldPlan Step-2/3 additions drops
// `work_chunks` and the `**Acceptance Criteria**` section, and this test
// fails on `bead-acceptance-criteria` (a hard error, internal/validate/plan.go).
func TestScaffoldPlan_MinimallyFilled_RoundTripsThroughValidator(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	specID := "042-scaffold-roundtrip"

	scaffold := scaffoldPlan(specID)
	filled := strings.NewReplacer(
		"<Title>", "Do the thing",
		"<Specific, measurable criterion for this bead>", "the thing is done",
	).Replace(scaffold)

	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		t.Fatalf("SpecDir: %v", err)
	}
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir specDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte(filled), 0644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}

	vr := validate.ValidatePlan(root, specID)
	if vr.HasFailures() {
		t.Fatalf("scaffold, minimally filled, failed plan validation:\n%s\n\n--- scaffold content ---\n%s", vr.FormatText(), filled)
	}
}

// TestScaffoldSpec_MinimallyFilled_RoundTripsThroughValidator is the spec-half
// of AC-21: the spec.md template `mindspec spec create` emits
// (internal/spec/create.go's specTemplate, exercised here through the real
// spec.Run entry point — no export needed, no product change to internal/spec)
// passes internal/validate.ValidateSpec's structural checks once its bracketed
// placeholders are filled in. This is the existing, already-shipped spec
// scaffold; the test only proves it still round-trips, matching the plan
// scaffold's guarantee added by this bead.
func TestScaffoldSpec_MinimallyFilled_RoundTripsThroughValidator(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	specID := "042-scaffold-roundtrip"
	wtPath := filepath.Join(root, ".worktrees", "worktree-spec-"+specID)
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	mockExec := &executor.MockExecutor{
		InitSpecWorkspaceResult: executor.WorkspaceInfo{
			Path:   wtPath,
			Branch: "spec/" + specID,
		},
	}

	result, err := spec.Run(root, specID, "Scaffold Roundtrip", mockExec)
	if err != nil {
		t.Fatalf("spec.Run: %v", err)
	}

	specDir, err := workspace.SpecDir(result.WorktreePath, specID)
	if err != nil {
		t.Fatalf("SpecDir: %v", err)
	}
	specPath := filepath.Join(specDir, "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec.md: %v", err)
	}

	filled := fillSpecPlaceholders(string(data))
	if err := os.WriteFile(specPath, []byte(filled), 0644); err != nil {
		t.Fatalf("rewrite spec.md: %v", err)
	}

	vr := validate.ValidateSpec(result.WorktreePath, specID)
	if vr.HasFailures() {
		t.Fatalf("spec scaffold, minimally filled, failed spec validation:\n%s\n\n--- filled content ---\n%s", vr.FormatText(), filled)
	}
}

// fillSpecPlaceholders fills the exact bracketed placeholders
// internal/spec/create.go's specTemplate carries with minimal, valid content
// — matching the structural minimums internal/validate/spec.go enforces
// (>= 2 Requirements, >= 3 Acceptance Criteria checkboxes, no unresolved
// Open Question). A generic "replace every <...> span" fill is NOT enough
// here: the raw template only has 2 Acceptance Criteria placeholders
// (checkAcceptanceCriteria requires >= 3) and its Open Questions placeholder
// is an UNCHECKED `- [ ]` box (checkOpenQuestions flags any unchecked box),
// so both need targeted, not merely non-placeholder, filler.
func fillSpecPlaceholders(content string) string {
	replacer := strings.NewReplacer(
		"<Brief description of what this spec achieves and the target user outcome>",
		"filled goal description for the scaffold round-trip test",
		"<Context, motivation, and any relevant prior decisions>",
		"filled background for the scaffold round-trip test",
		"- <domain-1>: <how it is impacted>",
		"- core: touches internal/example for the scaffold round-trip test",
		"- [ADR-NNNN](../../adr/ADR-NNNN.md): <why this ADR is relevant>",
		"No ADRs are relevant to this scaffold round-trip test.",
		"1. <Requirement 1>",
		"1. filled requirement one",
		"2. <Requirement 2>",
		"2. filled requirement two",
		"- <File or component 1>",
		"- example/file.go",
		"- <Explicitly excluded items>",
		"- nothing excluded for this scaffold round-trip test",
		"- <What this spec intentionally does not address>",
		"- no non-goals for this scaffold round-trip test",
		"- [ ] <Specific, measurable criterion 1>",
		"- [ ] filled criterion one\n- [ ] filled criterion three",
		"- [ ] <Specific, measurable criterion 2>",
		"- [ ] filled criterion two",
		"- <command 1>: <Expected outcome>",
		"- go test ./...: passes",
		"- [ ] <Question that must be resolved before planning>",
		"None",
	)
	return replacer.Replace(content)
}

// TestScaffoldPlan_FilledWorkChunks_WireBDEdge is AC-18: extending the
// scaffold's single-bead work_chunks shape to a second chunk with
// `depends_on: [1]` — exactly the shape documented in the scaffold's
// frontmatter comment — produces a WIRED bd dependency edge through the
// shipped `work_chunks[].depends_on` path (internal/approve/plan.go), not a
// prose scrape. Verified by asserting the exact `bd dep add <bead2> <bead1>`
// call fires and NO dep call fires for bead 1 (which declares no
// dependencies) — the edge that makes Bead 1 bd-ready and Bead 2 blocked on
// it, and Bead 2 ready only once Bead 1 closes (bd's own dependency engine,
// exercised elsewhere; this test pins the wiring INPUT bd's engine consumes).
func TestScaffoldPlan_FilledWorkChunks_WireBDEdge(t *testing.T) {
	tmp := t.TempDir()

	scaffold := scaffoldPlan("042-scaffold-roundtrip")
	filled := strings.NewReplacer(
		"<Title>", "Do the thing",
		"<Specific, measurable criterion for this bead>", "the thing is done",
	).Replace(scaffold)

	// Extend the single-chunk/single-bead scaffold to two, mirroring the
	// scaffold's own documented shape (chunk id N maps to the Nth "## Bead N"
	// section; depends_on: [1] wires the second bead to the first).
	filled = strings.Replace(filled, "work_chunks:\n  - id: 1\n    depends_on: []\n    key_file_paths:\n      - path/to/file.go\n",
		"work_chunks:\n  - id: 1\n    depends_on: []\n    key_file_paths:\n      - path/to/file.go\n  - id: 2\n    depends_on: [1]\n    key_file_paths:\n      - path/to/other.go\n", 1)
	filled += `
## Bead 2: Do the next thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`make test`" + ` passes

**Acceptance Criteria**
- the next thing is done

**Depends on**
Bead 1 (human-readable documentation only — NOT parsed)
`

	planPath := filepath.Join(tmp, "plan.md")
	if err := os.WriteFile(planPath, []byte(filled), 0644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}

	var created atomic.Int32
	var depCalls []string
	origBD := planRunBDFn
	defer func() { planRunBDFn = origBD }()
	planRunBDFn = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			n := created.Add(1)
			return []byte(fmt.Sprintf(`{"id":"bead-%d"}`, n)), nil
		}
		if len(args) > 0 && args[0] == "dep" {
			depCalls = append(depCalls, strings.Join(args, " "))
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected bd call: %v", args)
	}
	origList := planListJSONFn
	defer func() { planListJSONFn = origList }()
	planListJSONFn = func(args ...string) ([]byte, error) { return []byte(`[]`), nil }

	beadIDs, warnings, err := createImplementationBeads(planPath, "042-scaffold-roundtrip", "epic-1")
	if err != nil {
		t.Fatalf("createImplementationBeads: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected zero warnings for a fully-wired scaffold, got: %v", warnings)
	}
	if len(beadIDs) != 2 {
		t.Fatalf("expected 2 beads, got %d: %v", len(beadIDs), beadIDs)
	}
	if len(depCalls) != 1 {
		t.Fatalf("expected exactly 1 dep-add call, got %d: %v", len(depCalls), depCalls)
	}
	want := fmt.Sprintf("dep add %s %s", beadIDs[1], beadIDs[0])
	if depCalls[0] != want {
		t.Errorf("expected %q, got %q", want, depCalls[0])
	}
}

// bdRunAt runs `bd <args...>` against a real bd repo rooted at dir and
// returns stdout, failing the test on error (with stderr, when available,
// included in the failure message).
func bdRunAt(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("bd", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("bd %v: %v\n%s", args, err, stderr)
	}
	return out
}

// bdJSONID extracts the "id" field from a `bd ... --json` single-object response.
func bdJSONID(t *testing.T, out []byte) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("parsing bd --json output %q: %v", out, err)
	}
	if v.ID == "" {
		t.Fatalf("bd --json output carried no id: %s", out)
	}
	return v.ID
}

// bdReadyIDs runs `bd ready --json` against a real bd repo and returns the
// ready set's bead IDs.
func bdReadyIDs(t *testing.T, dir string) []string {
	t.Helper()
	out := bdRunAt(t, dir, "ready", "--json")
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatalf("parsing bd ready --json output %q: %v", out, err)
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestScaffoldPlan_FilledWorkChunks_DrivesReadySet is the AC-18 ready-set
// half TestScaffoldPlan_FilledWorkChunks_WireBDEdge's own doc comment
// promised ("the edge that makes Bead 1 bd-ready and Bead 2 blocked ... bd's
// own dependency engine, exercised elsewhere") but that no test in this
// package actually pinned (panel finding, 5 reviewers on spec 119 bead 4's
// review). WireBDEdge only asserts the exact `bd dep add` call fires; it
// never asserts the call's CONSEQUENCE.
//
// This test drives a REAL bd repo (not a stubbed planRunBDFn/planListJSONFn)
// because the readiness consequence lives entirely inside bd's own
// dependency engine — no mindspec code computes "ready" — so a mocked bd
// call can only ever re-assert that the call was MADE (WireBDEdge's job),
// never that it actually WORKED. It is skipped when `bd` is not on PATH,
// matching the existing convention (e.g. internal/bootstrap/mergedriver_test.go,
// internal/executor/executor_test.go).
func TestScaffoldPlan_FilledWorkChunks_DrivesReadySet(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not available — this test drives a real bd repo to pin dependency-driven readiness")
	}

	bdRoot := t.TempDir()
	bdRunAt(t, bdRoot, "init", "--non-interactive", "--skip-agents", "--skip-hooks", "-q", "--prefix", "ac18")
	epicID := bdJSONID(t, bdRunAt(t, bdRoot, "create", "Epic", "--type", "epic", "--json"))

	// Same scaffold-extension shape as TestScaffoldPlan_FilledWorkChunks_WireBDEdge:
	// a second work_chunk with depends_on: [1], wired through the shipped
	// work_chunks[].depends_on path (not a prose scrape).
	scaffold := scaffoldPlan("042-scaffold-roundtrip")
	filled := strings.NewReplacer(
		"<Title>", "Do the thing",
		"<Specific, measurable criterion for this bead>", "the thing is done",
	).Replace(scaffold)
	filled = strings.Replace(filled, "work_chunks:\n  - id: 1\n    depends_on: []\n    key_file_paths:\n      - path/to/file.go\n",
		"work_chunks:\n  - id: 1\n    depends_on: []\n    key_file_paths:\n      - path/to/file.go\n  - id: 2\n    depends_on: [1]\n    key_file_paths:\n      - path/to/other.go\n", 1)
	filled += `
## Bead 2: Do the next thing

**Steps**
1. Step one
2. Step two
3. Step three

**Verification**
- [ ] ` + "`make test`" + ` passes

**Acceptance Criteria**
- the next thing is done

**Depends on**
Bead 1 (human-readable documentation only — NOT parsed)
`

	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	if err := os.WriteFile(planPath, []byte(filled), 0644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}

	realBD := func(args ...string) ([]byte, error) {
		cmd := exec.Command("bd", args...)
		cmd.Dir = bdRoot
		return cmd.Output()
	}
	origRunBD := planRunBDFn
	defer func() { planRunBDFn = origRunBD }()
	planRunBDFn = realBD

	// planListJSONFn mirrors bead.ListJSON's own contract (internal/bead/bdcli.go):
	// callers pass args WITHOUT the "list" verb or "--json" flag — ListJSON
	// prepends/appends both itself. queryExistingChildren relies on exactly
	// that shape, so the real-bd stub must reproduce it, not just shell the
	// args straight through (a bare `bd --parent <id> --all -n 0` is not a
	// valid bd invocation — it's missing the "list" subcommand).
	origList := planListJSONFn
	defer func() { planListJSONFn = origList }()
	planListJSONFn = func(args ...string) ([]byte, error) {
		full := append([]string{"list"}, args...)
		full = append(full, "--json")
		return realBD(full...)
	}

	origCombined := planRunBDCombinedFn
	defer func() { planRunBDCombinedFn = origCombined }()
	planRunBDCombinedFn = func(args ...string) ([]byte, error) {
		cmd := exec.Command("bd", args...)
		cmd.Dir = bdRoot
		return cmd.CombinedOutput()
	}

	beadIDs, warnings, err := createImplementationBeads(planPath, "042-scaffold-roundtrip", epicID)
	if err != nil {
		t.Fatalf("createImplementationBeads: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected zero warnings for a fully-wired scaffold, got: %v", warnings)
	}
	if len(beadIDs) != 2 {
		t.Fatalf("expected 2 beads, got %d: %v", len(beadIDs), beadIDs)
	}

	// The actual AC-18 consequence: bd's OWN readiness engine, not a mocked
	// call assertion.
	ready := bdReadyIDs(t, bdRoot)
	if !containsStr(ready, beadIDs[0]) {
		t.Errorf("bead 1 (%s, no deps) should be bd-ready; ready set: %v", beadIDs[0], ready)
	}
	if containsStr(ready, beadIDs[1]) {
		t.Errorf("bead 2 (%s, depends_on: [1]) should be BLOCKED, not ready; ready set: %v", beadIDs[1], ready)
	}

	// Close bead 1; bead 2 must become ready — the other half of the
	// consequence (a blocked bead's unblocking, not just the initial block).
	bdRunAt(t, bdRoot, "close", beadIDs[0], "--reason", "test")
	ready = bdReadyIDs(t, bdRoot)
	if !containsStr(ready, beadIDs[1]) {
		t.Errorf("bead 2 (%s) should become ready once bead 1 closes; ready set: %v", beadIDs[1], ready)
	}
}
