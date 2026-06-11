package approve

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

var (
	implRunBDCombinedFn = bead.RunBDCombined
	implRunBDFn         = bead.RunBD
	implMergeMetadataFn = bead.MergeMetadata
	implGitUserEmailFn  = bead.GitUserEmail
	// implCreateWithIDFn is the Spec 087 Bead 3 seam for the
	// placeholder-ADR creation step in the supersede flow on the
	// backstop (`approve impl`) path.
	implCreateWithIDFn = adr.CreateWithID
)

// ImplOpts holds options for implementation approval.
//
// Spec 086 Bead 3: `AllowDocSkew` activates the doc-sync override
// gate. Empty string means "no override". A non-empty string is
// recorded as `mindspec_impl_skew_reason` (alongside `_at` and
// `_by`) on the spec EPIC's metadata AFTER `exec.FinalizeEpic`
// returns nil. If FinalizeEpic fails, no override metadata is
// written — the failure itself is the audit trail.
type ImplOpts struct {
	AllowDocSkew string

	// OverrideADR is the human-readable reason for bypassing the
	// ADR-divergence backstop gate. Empty string means "no override".
	// A non-empty string causes the gate to be SKIPPED. After
	// `exec.FinalizeEpic` returns nil the reason is recorded on the
	// spec EPIC's metadata under the `mindspec_adr_override_*`
	// namespace.
	// Spec 087 Bead 3.
	OverrideADR string

	// SupersedeADR is the user-supplied ADR ID (e.g. "ADR-0099") for
	// the supersede backstop flow. Empty string means "no supersede".
	// When set, a placeholder ADR is pre-created on disk (Status:
	// Proposed, Domain(s) seeded from the first uncovered
	// DivergenceFinding) BEFORE the gate-skip decision; the gate is
	// then SKIPPED, and the four `mindspec_adr_supersede_*` keys are
	// written to the EPIC's metadata AFTER FinalizeEpic returns nil.
	// Mutually exclusive with OverrideADR at the CLI layer.
	// Spec 087 Bead 3.
	SupersedeADR string
}

// ImplResult holds the result of implementation approval.
type ImplResult struct {
	SpecID      string
	Warnings    []string
	SpecBranch  string
	CommitCount int
	DiffStat    string
	Pushed      bool // true if branch was pushed to remote
}

// ApproveImpl transitions from review mode to idle, completing the spec lifecycle.
// Enforcement logic (phase validation, epic closure, bead status checks) stays here.
// Git operations (merge, push, cleanup) are delegated to the Executor.
//
// Spec 086 Bead 3 ordering contract — all gates run BEFORE every
// mutating/terminal operation; the override metadata write runs AFTER
// `exec.FinalizeEpic` returns nil:
//
//  1. readBeadStatus loop (bead-status verification, non-mutating)
//  2. validate.ValidateDocs (doc-sync gate; honors AllowDocSkew override)
//  3. validate.CheckADRDivergence (ADR-divergence gate; NOT covered by override)
//  4. implRunBDCombinedFn("close", epicID) (EPIC CLOSE — first mutation)
//  5. bead.MergeMetadata(epicID, mindspec_phase=done) (PHASE METADATA)
//  6. exec.CommitCount (pre-flight)
//  7. exec.FinalizeEpic (TERMINAL MUTATION)
//  8. implMergeMetadataFn(epicID, mindspec_impl_skew_*) — only if
//     AllowDocSkew set AND FinalizeEpic returned nil
func ApproveImpl(root, specID string, exec executor.Executor, opts ...ImplOpts) (*ImplResult, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}
	// Spec 089 / ADR-0034: one-shot legacy-to-metadata migration on first
	// lifecycle command. No-op if the epic already has mindspec_phase, or
	// when no epic exists yet. Migration errors fail the command
	// (spec 089 Requirement 9).
	if _, err := phase.EnsureMigrated(specID); err != nil {
		return nil, err
	}
	var o ImplOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	result := &ImplResult{SpecID: specID}

	// Find the epic for this spec directly (ADR-0023).
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil {
		return nil, fmt.Errorf("no epic found for spec %s: %w", specID, err)
	}

	// Verify state is review mode: all children closed, pending final merge.
	epicPhase, err := phase.DerivePhase(epicID)
	if err != nil {
		return nil, fmt.Errorf("deriving phase for spec %s: %w", specID, err)
	}
	if epicPhase != state.ModeReview && epicPhase != state.ModeDone {
		return nil, fmt.Errorf("expected review mode, got %q", epicPhase)
	}

	// Derive spec branch from convention.
	specBranch := workspace.SpecBranch(specID)
	result.SpecBranch = specBranch

	// Enforcement gate (1/3): verify all plan beads are closed.
	specDir, sdErr := workspace.SpecDir(root, specID)
	if sdErr != nil {
		return nil, sdErr
	}
	planPath := filepath.Join(specDir, "plan.md")
	beadIDs, planErr := readPlanBeadIDs(planPath)
	if planErr == nil {
		for _, bid := range beadIDs {
			status, err := readBeadStatus(bid)
			if err != nil {
				return nil, fmt.Errorf("checking bead %s status: %w", bid, err)
			}
			if status != "closed" {
				return nil, fmt.Errorf("bead %s is still %q — close all beads before approving implementation", bid, status)
			}
		}
	}

	// Enforcement gate (2/3): Spec 086 (F2) doc-sync. Compute the
	// merge-base against `main` so the gate sees the whole spec
	// branch's diff. The `--allow-doc-skew "<reason>"` override
	// suppresses the failure; the reason is recorded on the spec
	// epic's metadata only AFTER FinalizeEpic returns nil.
	base, mbErr := exec.MergeBase("main", specBranch)
	if mbErr != nil {
		return nil, fmt.Errorf("computing merge-base for doc-sync: %w", mbErr)
	}
	docResult := validate.ValidateDocs(root, base, exec)
	// Spec 091 Req 22(a): surface warning-severity issues BEFORE the
	// failure decision so they print on every run — including when
	// HasFailures() is false and the flow proceeds normally, and on
	// the override/error paths.
	printResultWarnings(warnWriter, docResult)
	if docResult.HasFailures() {
		if o.AllowDocSkew == "" {
			return nil, fmt.Errorf("doc-sync: %s\nhint: re-run with --allow-doc-skew \"<reason>\" to override (records the reason on the spec epic's metadata)", joinResultErrorMessages(docResult))
		}
		// Override path: fall through. Metadata write happens AFTER
		// FinalizeEpic per panel CONSENSUS revision 4.
	}

	// Enforcement gate (3/3): Spec 087 Bead 2 fills the body; this
	// backstop runs across the full spec branch. The
	// `--override-adr` and `--supersede-adr` flags (Spec 087 Bead 3)
	// bypass the gate; `--allow-doc-skew` does NOT (panel CONSENSUS
	// rev 6). The findings slice seeds the supersede placeholder's
	// Domains field structurally (revision 2 — no string parsing).
	adrResult, adrFindings := validate.CheckADRDivergence(root, base, exec, specDir, "")
	// Same severity-generic pipe for the ADR-divergence backstop: any
	// SevWarning the gate emits (e.g. adr-divergence-proposed) renders
	// without further wiring. No-op while the gate emits none.
	printResultWarnings(warnWriter, adrResult)

	// Pre-create the placeholder ADR FIRST when --supersede-adr is
	// requested so the new file exists on disk even if a downstream
	// step fails.
	var supersedeNewID string
	if o.SupersedeADR != "" {
		var seedDomains []string
		for _, f := range adrFindings {
			if f.Kind == "uncovered" && f.Domain != "" {
				seedDomains = []string{f.Domain}
				break
			}
		}
		title := "Placeholder for " + o.SupersedeADR
		if _, err := implCreateWithIDFn(root, o.SupersedeADR, title, adr.CreateOpts{Domains: seedDomains}); err != nil {
			return nil, fmt.Errorf("--supersede-adr: %w", err)
		}
		supersedeNewID = o.SupersedeADR
	}

	if o.OverrideADR == "" && o.SupersedeADR == "" && adrResult.HasFailures() {
		return nil, fmt.Errorf("adr-divergence: %s\nhint: re-run with --override-adr \"<reason>\" or --supersede-adr ADR-NNNN to bypass",
			joinResultErrorMessages(adrResult))
	}

	// MUTATION (1/3): close epic and mark as explicitly done.
	if epicID != "" {
		if _, err := implRunBDCombinedFn("close", epicID); err != nil {
			if !isAlreadyClosedErr(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not close lifecycle epic %s: %v", epicID, err))
			}
		}
		// MUTATION (2/3): Spec 080 phase metadata write.
		if err := bead.MergeMetadata(epicID, map[string]interface{}{
			"mindspec_phase": "done",
			"mindspec_done":  true,
		}); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not set done marker on epic %s: %v", epicID, err))
		}
	}

	// Pre-flight: check spec branch has commits (via executor).
	// Pinned between phase-metadata write and FinalizeEpic per panel
	// CONSENSUS revision 9 so a future regression that re-shuffles
	// this line is caught by TestApproveImplCallOrder.
	count, countErr := exec.CommitCount("main", specBranch)
	if countErr == nil {
		if count == 0 && (planErr != nil || len(beadIDs) == 0) {
			return nil, fmt.Errorf("preflight check failed: spec branch %s has no commits beyond main — nothing to merge", specBranch)
		}
	}

	// MUTATION (3/3, terminal): delegate to executor for merge/push.
	fr, err := exec.FinalizeEpic(epicID, specID, specBranch)
	if err != nil {
		return nil, fmt.Errorf("finalizing epic: %w", err)
	}

	// Spec 086 Bead 3: record doc-sync skew override ONLY after the
	// terminal mutation returned nil. On failure we'd have already
	// returned above — the absence of metadata is then the audit
	// trail. Best-effort: a metadata-write failure becomes a warning.
	if o.AllowDocSkew != "" && epicID != "" {
		meta := buildImplSkewMetadata(o.AllowDocSkew)
		if err := implMergeMetadataFn(epicID, meta); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not record impl-skew override metadata on %s: %v", epicID, err))
		}
	}

	// Spec 087 Bead 3: ADR-divergence override / supersede metadata
	// writes on the EPIC, mirroring the doc-skew discipline above.
	// Distinct namespace per spec.md Requirement 13.
	if o.OverrideADR != "" && epicID != "" {
		meta := map[string]interface{}{
			"mindspec_adr_override_reason": o.OverrideADR,
			"mindspec_adr_override_at":     time.Now().UTC().Format(time.RFC3339),
			"mindspec_adr_override_by":     implGitUserEmailFn(),
		}
		if err := implMergeMetadataFn(epicID, meta); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not record adr-override metadata on %s: %v", epicID, err))
		}
	}
	if o.SupersedeADR != "" && epicID != "" {
		reason := o.OverrideADR
		if reason == "" {
			reason = "superseded by " + supersedeNewID
		}
		meta := map[string]interface{}{
			"mindspec_adr_supersede_id":     supersedeNewID,
			"mindspec_adr_supersede_reason": reason,
			"mindspec_adr_supersede_at":     time.Now().UTC().Format(time.RFC3339),
			"mindspec_adr_supersede_by":     implGitUserEmailFn(),
		}
		if err := implMergeMetadataFn(epicID, meta); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not record adr-supersede metadata on %s: %v", epicID, err))
		}
	}

	result.CommitCount = fr.CommitCount
	result.DiffStat = fr.DiffStat
	result.Pushed = (fr.MergeStrategy == "pr")

	// Stop recording (best-effort — before transitioning to idle)
	if err := recording.StopRecording(root, specID); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not stop recording: %v", err))
	}

	return result, nil
}

func readBeadStatus(id string) (string, error) {
	out, err := implRunBDFn("show", id, "--json")
	if err != nil {
		return "", err
	}

	var payload []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parsing bd show output for %s: %w", id, err)
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("no bead returned for %s", id)
	}
	return strings.ToLower(strings.TrimSpace(payload[0].Status)), nil
}

// readPlanBeadIDs reads bead_ids from the plan.md YAML frontmatter.
func readPlanBeadIDs(planPath string) ([]string, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("no frontmatter found")
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return nil, fmt.Errorf("no frontmatter end marker")
	}
	fmContent := content[4 : 4+end]

	var fm struct {
		BeadIDs []string `yaml:"bead_ids"`
	}
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return nil, fmt.Errorf("parsing plan frontmatter: %w", err)
	}
	if len(fm.BeadIDs) == 0 {
		return nil, fmt.Errorf("no bead_ids in plan frontmatter")
	}
	return fm.BeadIDs, nil
}

func isAlreadyClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already closed")
}

// buildImplSkewMetadata returns the override metadata for the spec
// epic, mirroring `complete.buildSkewMetadata` but with the
// impl-prefixed keys baked in. Spec 086 Bead 3 panel CONSENSUS
// revision 4: write-order rule means this is only called AFTER
// `exec.FinalizeEpic` returns nil.
func buildImplSkewMetadata(reason string) map[string]interface{} {
	return map[string]interface{}{
		"mindspec_impl_skew_reason": reason,
		"mindspec_impl_skew_at":     time.Now().UTC().Format(time.RFC3339),
		"mindspec_impl_skew_by":     implGitUserEmailFn(),
	}
}

// warnWriter is the destination for WARN lines rendered from
// validation results (Spec 091 Bead 5, Req 22). Production writes to
// stderr; package-level seam so tests can capture the output.
var warnWriter io.Writer = os.Stderr

// printResultWarnings renders every warning-severity issue carried by
// a *validate.Result as `WARN <name>: <message>` — one line per
// issue. Severity-generic: it prints ANY SevWarning regardless of
// which validator lane produced it (cmd-docs, missing-source-globs,
// adr-divergence-proposed, ...). Stateless by construction (HC-2):
// no marker files, no seen-tracking, no dedup — the same warning
// prints on every invocation for as long as the Result carries it.
// Warnings never affect the pass/fail decision.
func printResultWarnings(w io.Writer, r *validate.Result) {
	for _, i := range r.Issues {
		if i.Severity != validate.SevWarning {
			continue
		}
		fmt.Fprintf(w, "WARN %s: %s\n", i.Name, i.Message)
	}
}

// joinResultErrorMessages flattens SevError-severity issues from a
// *validate.Result into a single line suitable for fmt.Errorf wrapping.
func joinResultErrorMessages(r *validate.Result) string {
	msgs := make([]string, 0, len(r.Issues))
	for _, i := range r.Issues {
		if i.Severity != validate.SevError {
			continue
		}
		msgs = append(msgs, fmt.Sprintf("[%s] %s", i.Name, i.Message))
	}
	return strings.Join(msgs, "; ")
}
