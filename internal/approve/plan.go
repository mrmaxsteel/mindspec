package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/contextpack"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// planRunBDCombinedFn is a package-level variable for testability.
var planRunBDCombinedFn = bead.RunBDCombined

// planRunBDFn is for JSON-returning bd commands (stdout only, no stderr mixing).
var planRunBDFn = bead.RunBD

// planListJSONFn wraps bead.ListJSON for testability.
var planListJSONFn = bead.ListJSON

// planMergeMetadataFn wraps bead.MergeMetadata for testability.
var planMergeMetadataFn = bead.MergeMetadata

// SetPlanMergeMetadataForTest swaps planMergeMetadataFn for testing and returns a restore function.
func SetPlanMergeMetadataForTest(fn func(issueID string, updates map[string]interface{}) error) func() {
	orig := planMergeMetadataFn
	planMergeMetadataFn = fn
	return func() { planMergeMetadataFn = orig }
}

// SetPlanRunBDForTest swaps planRunBDFn for testing and returns a restore function.
func SetPlanRunBDForTest(fn func(args ...string) ([]byte, error)) func() {
	orig := planRunBDFn
	planRunBDFn = fn
	return func() { planRunBDFn = orig }
}

// SetPlanRunBDCombinedForTest swaps planRunBDCombinedFn for testing and returns a restore function.
func SetPlanRunBDCombinedForTest(fn func(args ...string) ([]byte, error)) func() {
	orig := planRunBDCombinedFn
	planRunBDCombinedFn = fn
	return func() { planRunBDCombinedFn = orig }
}

// PlanResult holds the result of plan approval.
type PlanResult struct {
	SpecID   string
	GateID   string // empty if no gate found
	BeadIDs  []string
	Warnings []string
}

// ApprovePlan validates and approves a plan, creating beads and setting state.
func ApprovePlan(root, specID, approvedBy string, exec executor.Executor) (*PlanResult, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}
	// Spec 089 / ADR-0034: one-shot legacy-to-metadata migration on first
	// lifecycle command. No-op if the epic already has mindspec_phase, or
	// when no epic exists yet (pre-approve-spec). Migration errors fail
	// the command (spec 089 Requirement 9).
	if _, err := phase.EnsureMigrated(specID); err != nil {
		return nil, err
	}
	result := &PlanResult{SpecID: specID}

	// Step 1: Validate (SpecDir is worktree-aware per ADR-0022)
	vr := validate.ValidatePlan(root, specID)
	if vr.HasFailures() {
		// If plan.md doesn't exist, check whether the spec itself still needs
		// approval so we can guide agents that pick the wrong subcommand.
		// The authoritative phase signal is the epic's mindspec_phase metadata
		// (ADR-0023); falling back to YAML frontmatter only when no epic exists.
		specDir, sdErr := workspace.SpecDir(root, specID)
		if sdErr != nil {
			return nil, sdErr
		}
		planPath := filepath.Join(specDir, "plan.md")
		if _, statErr := os.Stat(planPath); os.IsNotExist(statErr) {
			if !specIsApproved(specDir, specID) {
				// Spec 092 Req 12 (integration finding INT-2): canonical
				// noun-verb form with a machine-greppable recovery line —
				// never the deprecated `mindspec approve spec` order.
				return nil, guard.NewFailure(
					fmt.Sprintf("spec %s has not been approved yet — no plan.md exists", specID),
					fmt.Sprintf("mindspec spec approve %s", specID),
				)
			}
		}
		return nil, planValidationFailure(specID, vr)
	}

	// Step 2: Find epic ID via beads metadata query (ADR-0023).
	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return nil, err
	}
	var parentID string
	epicID, epicErr := phase.FindEpicBySpecID(specID)
	if epicErr == nil {
		parentID = epicID
	}

	// Step 3: Update plan frontmatter
	planPath := filepath.Join(specDir, "plan.md")
	if err = updatePlanApproval(planPath, approvedBy); err != nil {
		return nil, fmt.Errorf("updating plan approval: %w", err)
	}

	// Step 3b: Auto-create implementation beads from plan sections
	if parentID != "" {
		beadIDs, err := createImplementationBeads(planPath, specID, parentID)
		if err != nil {
			// Spec 092 Req 12: context is PREPENDED so the cause's final
			// `recovery:` line stays the last line of the message
			// (`mindspec bead create-from-plan` can recreate beads
			// without re-approving when the plan itself is fine).
			return nil, fmt.Errorf("failed to create implementation beads — the plan frontmatter is already marked Approved but the bead set was NOT (fully) created:\n%w", err)
		} else if len(beadIDs) > 0 {
			result.BeadIDs = beadIDs
			if err := writeBeadIDsToFrontmatter(planPath, beadIDs); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not write bead IDs to plan frontmatter: %v", err))
			}
		} else {
			result.Warnings = append(result.Warnings, "plan has no '## Bead N:' sections; no implementation beads were created. Add bead sections to the plan or create beads manually.")
		}
	} else {
		result.Warnings = append(result.Warnings, "no epic found for spec via beads metadata; skipping bead auto-creation")
	}

	// Step 4: Auto-commit plan approval + bead_ids so implementation
	// worktrees that branch from spec/<id> contain the approved artifacts.
	// This is a hard error: leaving the spec worktree dirty here causes the
	// downstream `mindspec complete` merge to fail (`git merge` refuses to
	// run with uncommitted changes in the target worktree).
	//
	// Ordering invariant: this must happen BEFORE Step 4b flips the epic's
	// mindspec_phase to "implement". Once phase=implement is stored, the
	// pre-commit hook (internal/hook/dispatch.go) blocks further commits on
	// the spec/<id> branch — including this very commit, which would then
	// only land via the MINDSPEC_ALLOW_MAIN=1 escape hatch.
	cfg, cfgErr := config.Load(root)
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	specWtPath := workspace.SpecWorktreePath(root, cfg, specID)
	commitMsg := fmt.Sprintf("chore: approve plan for %s", specID)
	if err := exec.CommitAll(specWtPath, commitMsg); err != nil {
		return nil, fmt.Errorf("auto-commit plan approval failed: %w\n\nFix the issue in %s and re-run 'mindspec plan approve %s'", err, specWtPath, specID)
	}

	// Pre-commit hooks (beads, etc.) can modify tracked files as a side
	// effect of the commit above. A second CommitAll picks up those
	// residual changes so the spec worktree lands clean.
	if err := exec.CommitAll(specWtPath, fmt.Sprintf("chore: sync beads state after plan approval for %s", specID)); err != nil {
		return nil, fmt.Errorf("auto-commit residual state failed: %w\n\nInspect %s and re-run 'mindspec plan approve %s'", err, specWtPath, specID)
	}

	// Final guard: the spec worktree must be clean before beads can be
	// merged back into it during `mindspec complete`.
	if err := exec.IsTreeClean(specWtPath); err != nil {
		return nil, fmt.Errorf("spec worktree has uncommitted changes after plan approval: %w\n\ncd %s && git status", err, specWtPath)
	}

	// Step 4b (Spec 080): Write mindspec_phase: implement to epic metadata.
	// Must run AFTER Step 4 — see ordering invariant above.
	if parentID != "" {
		if err := planMergeMetadataFn(parentID, map[string]interface{}{"mindspec_phase": "implement"}); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not write phase metadata: %v", err))
		}
	}

	// Step 5: HandoffEpic — notify executor that beads are ready for dispatch.
	// For MindspecExecutor this is a no-op. Other executors may use this to schedule work.
	if parentID != "" && len(result.BeadIDs) > 0 {
		if err := exec.HandoffEpic(parentID, specID, result.BeadIDs); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("handoff epic failed: %v", err))
		}
	}

	// Step 6: Emit recording phase marker (best-effort)
	if err := recording.EmitPhaseMarker(root, specID, "plan", "plan-approved"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not emit recording marker: %v", err))
	}
	if err := recording.UpdatePhase(root, specID, "plan", "plan-approved"); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not update recording phase: %v", err))
	}

	return result, nil
}

// updatePlanApproval reads a plan file and updates YAML frontmatter with
// approval fields. The mutate-rewrite mechanics live in mutateFrontmatterFile
// (shared with writeBeadIDsToFrontmatter); this function only supplies the
// approval-field mutation.
func updatePlanApproval(planPath, approvedBy string) error {
	return updatePlanApprovalAt(planPath, approvedBy, time.Now().UTC())
}

// updatePlanApprovalAt is updatePlanApproval with an injected clock so the
// byte-identical golden test can pin approved_at deterministically.
func updatePlanApprovalAt(planPath, approvedBy string, now time.Time) error {
	return mutateFrontmatterFile(planPath, func(fmMap map[string]interface{}) {
		fmMap["status"] = "Approved"
		fmMap["approved_at"] = now.Format(time.RFC3339)
		fmMap["approved_by"] = approvedBy
	})
}

// createImplementationBeads parses plan.md for ## Bead sections, creates child
// beads under the lifecycle epic, and wires inter-bead dependencies.
// Each bead is populated with description, acceptance criteria, design, and metadata
// so agents can work from `bd show <id>` alone (Spec 074).
// Returns the ordered list of created bead IDs.
func createImplementationBeads(planPath, specID, parentID string) ([]string, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("reading plan: %w", err)
	}
	planContent := string(data)

	sections := validate.ParseBeadSections(planContent)
	if len(sections) == 0 {
		return nil, nil
	}

	// --- Re-approval safeguard: close-and-recreate existing beads (Spec 074) ---
	if err := handleExistingBeads(parentID, planContent); err != nil {
		return nil, err
	}

	// --- Assemble shared context from spec.md ---
	specDir := filepath.Dir(planPath)
	specContent := readFileOrEmpty(filepath.Join(specDir, "spec.md"))

	requirements := contextpack.ExtractSection(specContent, "Requirements")
	acceptanceCriteria := contextpack.ExtractSection(specContent, "Acceptance Criteria")

	// Parse the plan's structured frontmatter once: it is the validated
	// source of truth for both the bead `--design` ADR list (spec 097 R2)
	// and the inter-bead dependency wiring (spec 097 R3). A parse failure is
	// non-fatal for the ADR list (plan validation already gates the
	// frontmatter), so the design field simply omits ADR citations on a
	// malformed plan; the dep-wiring path below treats a parse failure as
	// "no structured deps" and wires nothing.
	var adrCitationIDs []string
	var workChunks []validate.WorkChunk
	if fm, err := validate.ParsePlanFrontmatter(planContent); err == nil {
		for _, c := range fm.ADRCitations {
			if c.ID != "" {
				adrCitationIDs = append(adrCitationIDs, c.ID)
			}
		}
		workChunks = fm.WorkChunks
	}

	// Alignment guard (spec 097 R3): the positional `bead_ids[N-1]` wiring
	// below requires every `work_chunks` id to map to exactly one `## Bead N`
	// section. Validate contiguity (1..K), the count match, and every
	// depends_on target BEFORE creating any beads, so a misaligned plan is
	// rejected up front rather than mis-wired or panicking mid-create.
	if err := validate.ValidateWorkChunkAlignment(workChunks, len(sections)); err != nil {
		return nil, fmt.Errorf("plan work_chunks misaligned with bead sections: %w", err)
	}

	// Build design field: spec requirements + ADR citations (by ID).
	design := buildDesignField(specDir, requirements, adrCitationIDs)

	// --- Extract raw bead section content from plan.md ---
	sectionContent := extractBeadSectionContents(planContent)

	// Beads are appended in `## Bead` section declaration order, so
	// beadIDs[N-1] is deterministically the Nth section — which the
	// alignment guard above ties to work_chunk id N for both the
	// dependency wiring and the per-bead key-file-paths source.
	var beadIDs []string

	// Index work chunks by their 1-based id so the Nth `## Bead` section can
	// source its declared `key_file_paths` (spec 097 R4) and dependencies
	// (spec 097 R3). The alignment guard above proved the ids are the
	// contiguous set 1..len(sections), so byID[n] is the Nth section's chunk.
	byID := make(map[int]validate.WorkChunk, len(workChunks))
	for _, c := range workChunks {
		byID[c.ID] = c
	}

	// Build a map from heading to parsed bead section for per-bead AC lookup.
	sectionByHeading := make(map[string]validate.BeadSection, len(sections))
	for _, sec := range sections {
		sectionByHeading[sec.Heading] = sec
	}

	for i, sec := range sections {
		title := fmt.Sprintf("[%s] %s", specID, sec.Heading)

		// Get the raw work chunk for this bead
		workChunk := sectionContent[sec.Heading]

		// Source this bead's key file paths from the declared, per-bead
		// `work_chunks[N-1].key_file_paths` (spec 097 R4) — the Nth section
		// (i is 0-based) maps to chunk id N=i+1. This replaces the retired
		// prose prefix-scan; when a chunk declares no paths the surface is
		// empty (acceptable — non-gating context enrichment).
		filePaths := byID[i+1].KeyFilePaths

		// Build metadata JSON
		metadataJSON := buildBeadMetadata(specID, filePaths)

		// Use per-bead acceptance criteria if available, fall back to spec-level AC (Spec 078)
		beadAC := acceptanceCriteria
		if parsed, ok := sectionByHeading[sec.Heading]; ok && parsed.AcceptanceCriteria != "" {
			beadAC = parsed.AcceptanceCriteria
		}

		args := []string{
			"create",
			"--title", title,
			"--type", "task",
			"--parent", parentID,
			"--description", workChunk,
			"--acceptance", beadAC,
			"--design", design,
			"--metadata", metadataJSON,
			"--json",
		}
		out, err := planRunBDFn(args...)
		if err != nil {
			return beadIDs, beadCreateFailure(specID, sec.Heading, beadIDs, args, err)
		}

		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &created); err != nil {
			return beadIDs, beadCreateFailure(specID, sec.Heading, beadIDs, args,
				fmt.Errorf("parsing create output: %w", err))
		}

		beadIDs = append(beadIDs, created.ID)
	}

	// Wire dependencies from the structured `work_chunks` frontmatter (spec
	// 097 R3 — the prose `bead\s+(\d+)` scrape is retired). Mapping: chunk
	// `id N` → bead_ids[N-1]; a `depends_on: [M]` entry makes bead_ids[N-1]
	// depend on bead_ids[M-1]. Iterate by ascending chunk id (the alignment
	// guard proved the ids are the contiguous set 1..len(beadIDs)) so the
	// `bd dep add` order is deterministic regardless of YAML ordering.
	// `byID` was built above (it also feeds the per-bead key-file-paths source).
	for n := 1; n <= len(beadIDs); n++ {
		chunk, ok := byID[n]
		if !ok {
			continue
		}
		for _, dep := range chunk.DependsOn {
			// Bounds were validated by ValidateWorkChunkAlignment above.
			if _, err := planRunBDFn("dep", "add", beadIDs[n-1], beadIDs[dep-1]); err != nil {
				// Best-effort: don't fail the whole operation for a dep wiring issue
				continue
			}
		}
	}

	return beadIDs, nil
}

// planValidationFailure aggregates EVERY error-severity validation
// issue into ONE guard failure: one bullet per issue, one final
// recovery line (spec 092 Req 15, mindspec-e6qq). Reporting all N
// violations in a single plan-approve invocation replaces the
// one-discovery-per-attempt loop the old first-failure formatting
// forced on agents.
func planValidationFailure(specID string, vr *validate.Result) error {
	var bullets []string
	for _, issue := range vr.Issues {
		if issue.Severity != validate.SevError {
			continue
		}
		bullets = append(bullets, fmt.Sprintf("  - [%s] %s", issue.Name, issue.Message))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "plan validation failed: %d issue(s) — fix ALL of them, then re-run plan approve:\n", len(bullets))
	b.WriteString(strings.Join(bullets, "\n"))
	return guard.NewFailure(b.String(), fmt.Sprintf("mindspec plan approve %s", specID))
}

// beadCreateFailure is the spec 092 Req 13b mid-batch containment
// failure (mindspec-lawq): ANY `bd create` failure inside
// createImplementationBeads — Dolt row-size ceiling (server Error
// 1105), daemon crash, lock contention, unparsable output — aborts
// with a structured error that names the failing bead heading, the
// likely offending field + byte size when the cause is 1105, LISTS the
// bead IDs already created (the partial set), and ends with recovery
// lines. Never a raw `Error 1105` with a silent partial bead set: exit
// codes never lie, and partial mutations always name their cleanup.
func beadCreateFailure(specID, heading string, created []string, createArgs []string, cause error) error {
	var b strings.Builder
	fmt.Fprintf(&b, "creating bead for %q failed: %v", heading, cause)
	if strings.Contains(cause.Error(), "1105") {
		if field, size := largestPayloadField(createArgs); field != "" {
			fmt.Fprintf(&b, "\nlikely oversized payload: %s is %d bytes (Dolt row-size ceiling — server Error 1105, not a mindspec limit)", field, size)
		}
	}
	if len(created) == 0 {
		b.WriteString("\nno beads were created before the failure — fix the cause, then re-run plan approve")
		return guard.NewFailure(b.String(), fmt.Sprintf("mindspec plan approve %s", specID))
	}
	fmt.Fprintf(&b, "\nbeads already created before the failure (PARTIAL set — the plan's remaining beads do not exist): %s", strings.Join(created, ", "))
	b.WriteString("\nremove the partial set first, then re-run plan approve (it recreates the full set)")
	// Integration finding INT-1: the recovery must CONVERGE. `bd close`
	// left closed children behind, which handleExistingBeads hard-rejects
	// on the re-run — a guaranteed dead end. `bd delete --force` actually
	// removes the partial set (it works on open AND closed beads, so even
	// a previously-pasted `bd close` still converges); `--force` is
	// mandatory because without it bd 1.0.4 only previews the deletion.
	// The IDs are by construction the partial set this failure created
	// (named state, HC-5-safe).
	return guard.NewFailure(b.String(),
		fmt.Sprintf("bd delete %s --force", strings.Join(created, " ")),
		fmt.Sprintf("mindspec plan approve %s", specID),
	)
}

// largestPayloadField returns the bd-create payload flag carrying the
// most bytes (the likely Error-1105 culprit) and its size. createArgs
// is the exact argv handed to `bd create`.
func largestPayloadField(createArgs []string) (string, int) {
	payloadFlags := map[string]bool{
		"--description": true,
		"--acceptance":  true,
		"--design":      true,
		"--metadata":    true,
	}
	field, size := "", -1
	for i := 0; i+1 < len(createArgs); i++ {
		if payloadFlags[createArgs[i]] && len(createArgs[i+1]) > size {
			field = createArgs[i]
			size = len(createArgs[i+1])
		}
	}
	return field, size
}

// handleExistingBeads checks if beads already exist under the epic (re-approval).
// If any are in_progress or closed, returns an error — the Spec 074
// supersede-safety rejection, with status-appropriate recovery lines per
// spec 092 Req 12 (integration finding INT-1). If all are open, closes them.
func handleExistingBeads(parentID, planContent string) error {
	out, err := planListJSONFn("--parent", parentID)
	if err != nil {
		return nil // Can't query — proceed with creation (first approval case)
	}

	var children []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if json.Unmarshal(out, &children) != nil || len(children) == 0 {
		return nil // No existing children — first approval
	}

	// Check for in-progress or closed beads. The rejection keeps its
	// teeth (Spec 074 supersede safety: closed children can be
	// legitimately completed work that a re-approve must not silently
	// supersede), but each dead end ends with a recovery line (spec 092
	// Req 12, integration finding INT-1).
	for _, c := range children {
		switch strings.ToLower(c.Status) {
		case "in_progress":
			return guard.NewFailure(
				fmt.Sprintf("cannot re-approve plan: bead %s is in_progress — complete the active work first, then re-run plan approve", c.ID),
				fmt.Sprintf("mindspec complete %s", c.ID),
			)
		case "closed":
			return guard.NewFailure(
				fmt.Sprintf("cannot re-approve plan: bead %s is closed — a closed child is either completed work under this epic (stop: re-approving would supersede a done record; reconsider the re-approve) or a leftover from a failed partial bead create. ONLY in the partial-create case, delete the leftover and re-run plan approve", c.ID),
				fmt.Sprintf("bd delete %s --force", c.ID),
			)
		}
	}

	// All open — close them with supersede reason
	version := extractPlanVersion(planContent)
	reason := fmt.Sprintf("superseded by plan v%s", version)
	var ids []string
	for _, c := range children {
		ids = append(ids, c.ID)
	}
	if len(ids) > 0 {
		// Close via bd close with reason
		args := append([]string{"close"}, ids...)
		args = append(args, "--reason", reason)
		if _, err := planRunBDCombinedFn(args...); err != nil {
			return fmt.Errorf("closing superseded beads: %w", err)
		}
	}

	return nil
}

// extractPlanVersion reads the version field from plan frontmatter.
func extractPlanVersion(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "version:") {
			v := strings.TrimPrefix(trimmed, "version:")
			v = strings.TrimSpace(v)
			v = strings.Trim(v, `"'`)
			return v
		}
	}
	return "unknown"
}

// buildDesignField assembles the design field content: spec requirements + ADR citations.
//
// Spec 092 Req 13a (mindspec-lawq): ADRs are cited by ID + title
// (`see ADR-NNNN — <title>`) instead of inlining each ADR's Decision
// snapshot. Inlining multiplied every cited ADR's Decision text into
// EVERY bead's --design payload, overflowing Dolt's row-size ceiling
// (server Error 1105) on plans citing many/large ADRs. By-ID citations
// bound the payload by construction — no size-limit constant is
// invented, because the ceiling is a Dolt server behavior, not a
// mindspec contract. The full text stays available under
// `.mindspec/docs/adr/`.
//
// Spec 097 R2 (mindspec-4axk): the ADR list is built from the plan's
// structured `adr_citations` frontmatter (each ADRCitation.ID — the
// validated source of truth) passed in as adrCitationIDs, NOT from a regex
// scrape of the spec's `## ADR Touchpoints` PROSE. This is forward-only:
// ADR IDs present only in prose but absent from declared `adr_citations`
// are no longer harvested. The frontmatter is the contract that the
// plan-validation gate already enforces.
func buildDesignField(specDir, requirements string, adrCitationIDs []string) string {
	var parts []string

	if requirements != "" {
		parts = append(parts, "## Requirements\n\n"+requirements)
	}

	if len(adrCitationIDs) > 0 {
		// specDir is e.g. .mindspec/docs/specs/074-slug; root is 3 levels up
		root := filepath.Join(specDir, "..", "..", "..")
		store := adr.NewFileStore(root)

		seen := make(map[string]bool)
		var citations []string
		for _, id := range adrCitationIDs {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			a, err := store.Get(id)
			if err != nil {
				continue
			}
			citations = append(citations, fmt.Sprintf("- see %s — %s", id, a.Title))
		}
		if len(citations) > 0 {
			parts = append(parts, "## ADR Decisions\n\nCited by ID — full text under `.mindspec/adr/`:\n\n"+strings.Join(citations, "\n"))
		}
	}

	return strings.Join(parts, "\n\n")
}

// extractBeadSectionContents extracts the raw markdown content for each ## Bead section.
// Returns a map from heading text (e.g., "Bead 1: Populate Fields") to section content.
func extractBeadSectionContents(content string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentHeading string
	var currentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "## Bead ") {
			// Save previous section
			if currentHeading != "" {
				result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			}
			currentHeading = strings.TrimPrefix(line, "## ")
			currentLines = nil
			continue
		}
		// A non-bead ## heading ends the current bead section
		if strings.HasPrefix(line, "## ") && currentHeading != "" {
			result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			currentHeading = ""
			currentLines = nil
			continue
		}
		if currentHeading != "" {
			currentLines = append(currentLines, line)
		}
	}
	if currentHeading != "" {
		result[currentHeading] = strings.TrimSpace(strings.Join(currentLines, "\n"))
	}

	return result
}

// buildBeadMetadata constructs the metadata JSON string for a bead.
func buildBeadMetadata(specID string, filePaths []string) string {
	meta := map[string]interface{}{
		"spec_id":    specID,
		"file_paths": filePaths,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Sprintf(`{"spec_id":"%s"}`, specID)
	}
	return string(data)
}

// readFileOrEmpty reads a file and returns its content, or empty string on error.
func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// writeBeadIDsToFrontmatter adds the bead_ids list to the plan's YAML
// frontmatter. Mechanics are shared with updatePlanApproval via
// mutateFrontmatterFile; only the bead_ids mutation is supplied here.
func writeBeadIDsToFrontmatter(planPath string, beadIDs []string) error {
	return mutateFrontmatterFile(planPath, func(fmMap map[string]interface{}) {
		// Convert []string to []interface{} for YAML (mirrors the historical
		// write so the marshaled bytes stay identical).
		ids := make([]interface{}, len(beadIDs))
		for i, id := range beadIDs {
			ids[i] = id
		}
		fmMap["bead_ids"] = ids
	})
}

// CreateBeadsFromPlan is a recovery function that creates implementation beads
// from an already-approved plan. Use this when plan-approve failed to create
// beads (e.g., bd was unreachable, CWD issue, etc.).
func CreateBeadsFromPlan(root, specID string) (*PlanResult, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}
	result := &PlanResult{SpecID: specID}

	epicID, epicErr := phase.FindEpicBySpecID(specID)
	if epicErr != nil || epicID == "" {
		return nil, fmt.Errorf("spec %s has no epic in beads; cannot create beads", specID)
	}

	specDir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return nil, err
	}
	planPath := filepath.Join(specDir, "plan.md")
	beadIDs, err := createImplementationBeads(planPath, specID, epicID)
	if err != nil {
		return nil, fmt.Errorf("creating beads: %w", err)
	}

	result.BeadIDs = beadIDs
	if len(beadIDs) > 0 {
		if err := writeBeadIDsToFrontmatter(planPath, beadIDs); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not write bead IDs to plan frontmatter: %v", err))
		}
	}

	return result, nil
}

// specIsApproved reports whether a spec has progressed past Spec Mode. The
// authoritative signal is the epic's mindspec_phase metadata in Beads
// (ADR-0023). If no epic is found we fall back to the spec.md YAML
// frontmatter via validate.SpecStatusAt. Substring matching on raw markdown
// is avoided — it silently misclassifies casing variations and frontmatter
// value changes (ZFC violation).
func specIsApproved(specDir, specID string) bool {
	if epicID, err := phase.FindEpicBySpecID(specID); err == nil && epicID != "" {
		if p, derr := phase.DerivePhase(epicID); derr == nil && p != "" {
			return p != state.ModeSpec
		}
	}
	return strings.EqualFold(validate.SpecStatusAt(specDir), "Approved")
}
