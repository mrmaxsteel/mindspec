package next

// ready_gate.go wires spec 124 (impl-readiness-gate) Bead 1's mechanical
// readiness floor into `mindspec next`'s claim path — the gate-before-
// mutate preflight leg ADR-0041's fourth-verb clause names (spec 124 R3 /
// R9 / AC-4 / AC-16): GateReadiness runs after bead selection and BEFORE
// any claim/branch/worktree mutation; RecordReadinessOverride writes the
// durable `--allow-not-ready` marker BEFORE ClaimBead, FAIL-CLOSED (the
// marker-before-claim ordering the spec 124 plan preamble records: AC-4
// makes `--allow-not-ready` success a GUARANTEE that the marker exists,
// and if ClaimBead then fails, the caller rolls the marker back via
// RollbackReadinessOverride so a lost claim leaves no orphaned override).

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Package-level function variables for testability (the same
// func-var-seam convention beads.go/guard.go already use in this
// package) — tests swap these to exercise GateReadiness/
// RecordReadinessOverride without a real bd process or a real
// internal/validate/readiness engine evaluation.
var (
	evaluateReadinessFn  = readiness.EvaluateReadiness
	mergeMetadataFn      = bead.MergeMetadata
	deleteMetadataKeysFn = bead.DeleteMetadataKeys
)

// BareOverride is pflag's sentinel for a flag without an equals-bound value.
const BareOverride = "__bare_allow_not_ready__"

// IsReadinessSignalToken separates space-separated signal tokens from bead IDs.
func IsReadinessSignalToken(value string) bool {
	return strings.HasPrefix(strings.ToUpper(value), "MF-")
}

// GateOptions describes fresh claim authority. Stored markers grant none.
type GateOptions struct {
	// BareFlag is a syntax refusal; readiness is read only to render recovery.
	BareFlag     bool
	ScopeSpec    string
	NamedSignals []string
	FlagGiven    bool
	EvidencePath string
}

// ParseReadinessSignals accepts only the closed MF-1..MF-4 vocabulary.
// Bare and space-separated forms refuse before any positional resolution.
func ParseReadinessSignals(value string, given bool) ([]string, error) {
	if !given {
		return nil, nil
	}
	if value == BareOverride || value == "" {
		return nil, guard.NewFailure("--allow-not-ready requires =<ids>; the bare and space-separated forms are not supported",
			"mindspec bead ready-check <bead-id>", "mindspec next <bead-id> --allow-not-ready=<ids> --evidence <path-to-justification>")
	}
	seen := map[string]bool{}
	for _, id := range strings.Split(value, ",") {
		if id != "MF-1" && id != "MF-2" && id != "MF-3" && id != "MF-4" {
			return nil, guard.NewFailure(fmt.Sprintf("unknown readiness signal %s; expected MF-1,MF-2,MF-3,MF-4", termsafe.Escape(id)), "mindspec bead ready-check <bead-id>")
		}
		seen[id] = true
	}
	var ids []string
	for _, id := range []string{"MF-1", "MF-2", "MF-3", "MF-4"} {
		if seen[id] {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// GateResult carries lineage-derived dispatch and the exact vouched failure set.
type GateResult struct {
	SpecID         string
	FailingSignals []string
	Fingerprints   map[string][]string
	Evidence       bead.OverrideEvidence
	Notes          []string
}

// IndeterminateReadiness names read failures no scoped override can cover.
// Readers use the same refusal instead of suggesting an unusable re-claim.
func IndeterminateReadiness(report *readiness.Report) []string {
	var lines []string
	for _, s := range report.FailingSignals() {
		for _, record := range s.Failures {
			if readiness.IsIndeterminateRecord(record) {
				lines = append(lines, fmt.Sprintf("%s cannot be overridden: %s could not be evaluated; fix the read first", s.ID, termsafe.Escape(record)))
			}
		}
	}
	return lines
}

func nonPlanDiagnostic(beadID, specID, epicID string) string {
	return fmt.Sprintf("bead %s is not a plan bead of spec %s (no %s.<N> ID); mindspec next claims only plan beads", idrender.Bead(beadID), idrender.Spec(specID), idrender.Bead(epicID))
}

func missingSpecBranchFailure(branch, beadID string) error {
	return guard.NewFailure(fmt.Sprintf("spec branch %s for bead %s does not exist (finalized and deleted, or never created); nothing was claimed", termsafe.Escape(branch), idrender.Bead(beadID)), "mindspec state show")
}

// GateReadiness is the read-only preflight before marker, claim, branch or
// worktree mutation (ADR-0041; spec 129 R1.8/R6). Evidence is read here too.
func GateReadiness(root, beadID string, opts GateOptions) (*GateResult, error) {
	recovery := []string{fmt.Sprintf("bd show %s --json", idrender.Bead(beadID))}
	if opts.BareFlag {
		message := "--allow-not-ready requires =<ids>; the bare and space-separated forms are not supported"
		if beadID == "" || IsReadinessSignalToken(beadID) {
			signals := "<ids>"
			if ids, err := ParseReadinessSignals(strings.ToUpper(beadID), true); err == nil {
				signals = strings.Join(ids, ",")
			}
			recovery = []string{"mindspec next <bead-id> --allow-not-ready=" + signals + " --evidence <path-to-justification>"}
		} else {
			// The syntax refusal is settled. Read readiness solely for
			// recovery; claim prerequisites cannot change this refusal.
			report, err := evaluateReadinessFn(root, beadID)
			if err != nil {
				message += "\n" + termsafe.Escape(err.Error())
				recovery = []string{fmt.Sprintf("mindspec bead ready-check %s", idrender.Bead(beadID))}
			} else if report.AllPass() {
				recovery = []string{fmt.Sprintf("mindspec next %s", idrender.Bead(beadID))}
				if _, ok := bead.PlanBeadIndex(report.EpicID, beadID); !ok {
					recovery = []string{fmt.Sprintf("mindspec bead ready-check %s", idrender.Bead(beadID))}
				}
			} else {
				message += "\n" + readiness.Render(report)
				recovery = append([]string{}, report.RecoveryCommands()...)
				recovery = append(recovery, fmt.Sprintf("mindspec bead ready-check %s   (the standalone per-signal report)", idrender.Bead(beadID)))
				if indeterminate := IndeterminateReadiness(report); len(indeterminate) > 0 {
					message += "\n" + strings.Join(indeterminate, "\n")
				} else {
					var ids []string
					for _, signal := range report.FailingSignals() {
						ids = append(ids, signal.ID)
					}
					recovery = append(recovery, fmt.Sprintf("mindspec next %s --allow-not-ready=%s --evidence <path-to-justification>", idrender.Bead(beadID), strings.Join(ids, ",")))
				}
			}
		}
		return nil, guard.NewFailure(message, recovery...)
	}
	namedIDs, err := ParseReadinessSignals(strings.Join(opts.NamedSignals, ","), opts.FlagGiven)
	if err != nil {
		if beadID == "" || IsReadinessSignalToken(beadID) {
			return nil, err
		}
		message := strings.SplitN(err.Error(), "\n"+guard.RecoveryPrefix, 2)[0]
		recovery = []string{fmt.Sprintf("mindspec bead ready-check %s", idrender.Bead(beadID))}
		if strings.Join(opts.NamedSignals, ",") == "" {
			recovery = append(recovery, fmt.Sprintf("mindspec next %s --allow-not-ready=<ids> --evidence <path-to-justification>", idrender.Bead(beadID)))
		}
		return nil, guard.NewFailure(message, recovery...)
	}
	report, err := evaluateReadinessFn(root, beadID)
	if err != nil {
		var nonPlan *readiness.NonPlanFormError
		if errors.As(err, &nonPlan) {
			return nil, guard.NewFailure(nonPlanDiagnostic(beadID, nonPlan.SpecID, nonPlan.EpicID), recovery...)
		}
		return nil, fmt.Errorf("evaluating readiness for %s: %w", idrender.Bead(beadID), &bdRenderedError{err: err})
	}
	if _, ok := bead.PlanBeadIndex(report.EpicID, beadID); !ok {
		return nil, guard.NewFailure(nonPlanDiagnostic(beadID, report.SpecID, report.EpicID), recovery...)
	}
	if opts.ScopeSpec != "" && opts.ScopeSpec != report.SpecID {
		return nil, guard.NewFailure(fmt.Sprintf("bead %s resolves to spec %s by lineage but is being claimed for spec %s; nothing was claimed", idrender.Bead(beadID), idrender.Spec(report.SpecID), idrender.Spec(opts.ScopeSpec)),
			recovery...)
	}
	branch, err := workspace.SpecBranch(report.SpecID)
	if err != nil {
		return nil, err
	}
	if _, err := gitutil.RevParseRef(root, "refs/heads/"+branch); err != nil {
		if errors.Is(err, gitutil.ErrRefNotFound) {
			return nil, missingSpecBranchFailure(branch, beadID)
		}
		return nil, guard.NewFailure(fmt.Sprintf("could not resolve spec branch %s for bead %s: %s; nothing was claimed", branch, idrender.Bead(beadID), termsafe.Escape(err.Error())), "git status   (repair the local git checkout, then retry)")
	}
	result := &GateResult{SpecID: report.SpecID, Fingerprints: map[string][]string{}}
	named := map[string]bool{}
	for _, id := range namedIDs {
		named[id] = true
	}
	for _, s := range report.Signals {
		if s.Pass && named[s.ID] {
			result.Notes = append(result.Notes, s.ID+" named but passing — not recorded")
		}
	}
	if report.AllPass() {
		return result, nil
	}
	var ids []string
	allNamed := true
	for _, s := range report.FailingSignals() {
		ids = append(ids, s.ID)
		if !named[s.ID] {
			allNamed = false
		}
		result.Fingerprints[s.ID] = readiness.FailureRecordDigests(s)
	}
	message := readiness.Render(report)
	if len(result.Notes) > 0 {
		message += "\n" + strings.Join(result.Notes, "\n")
	}
	recovery = append([]string{}, report.RecoveryCommands()...)
	recovery = append(recovery, fmt.Sprintf("mindspec bead ready-check %s   (the standalone per-signal report)", idrender.Bead(beadID)))
	if indeterminate := IndeterminateReadiness(report); len(indeterminate) > 0 {
		return nil, guard.NewFailure(message+"\n"+strings.Join(indeterminate, "\n"), recovery...)
	}
	recovery = append(recovery, fmt.Sprintf("mindspec next %s --allow-not-ready=%s --evidence <path-to-justification>", idrender.Bead(beadID), strings.Join(ids, ",")))
	if !allNamed {
		return nil, guard.NewFailure(message, recovery...)
	}
	evidence, err := ReadOverrideEvidence(opts.EvidencePath)
	if err != nil {
		recovery = recovery[len(recovery)-1:]
		return nil, guard.NewFailure(err.Error(), recovery...)
	}
	result.FailingSignals, result.Evidence = ids, evidence
	return result, nil
}

// RecordReadinessOverride replaces the marker before ClaimBead, fail-closed.
// Writer and reader share FailureRecordDigests over every raw failure record.
func RecordReadinessOverride(beadID string, result *GateResult) error {
	marker, err := bead.NewOverrideMarkerV2(result.FailingSignals, result.Fingerprints, result.Evidence, time.Now())
	if err != nil {
		return err
	}
	if err := mergeMetadataFn(beadID, map[string]interface{}{bead.MetaKeyReadinessOverride: marker}); err != nil {
		return &bdRenderedError{err: err}
	}
	return nil
}

// RollbackReadinessOverride deletes the durable `--allow-not-ready`
// override marker RecordReadinessOverride wrote (final-review r1
// G3-OVERRIDE-ORPHAN). The marker is written BEFORE ClaimBead (the AC-4
// fail-closed guarantee), so if ClaimBead then FAILS — a claim lost to
// contention, a bd error — the caller invokes this rollback so the
// failed invocation leaves no authorizing marker on a bead this run
// never claimed (an orphaned marker would otherwise let the dispatch
// ingress treat a LATER, independent claim as a deliberate readiness
// bypass). Best-effort at the call site: the caller surfaces a rollback
// failure LOUDLY (naming the orphaned key) but still reports the claim
// failure as the primary error. Deleting a key that is already absent
// is a no-op (nil).
func RollbackReadinessOverride(beadID string) error {
	if err := deleteMetadataKeysFn(beadID, bead.MetaKeyReadinessOverride); err != nil {
		// Same disposition as RecordReadinessOverride above:
		// bead.DeleteMetadataKeys also reads via GetMetadata ->
		// tracedOutput first, so a delete-READ failure can carry folded
		// bd stderr text — wrap for the render boundary, chain intact.
		return &bdRenderedError{err: err}
	}
	return nil
}
