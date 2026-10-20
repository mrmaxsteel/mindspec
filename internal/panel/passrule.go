package panel

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// PassRule is the single vote rule. Freshness is supplied by the caller;
// approval counts and deferrals are deliberately not decision inputs.
func (r *Result) PassRule(fresh bool) bool {
	return fresh && r.intactError() == nil && !r.RoundMismatch && r.Complete() &&
		r.Rejects == 0 && len(r.HardBlocks) == 0 && r.InvalidCount() == 0 &&
		!r.unexpectedVeto() && len(r.UnresolvedVerdicts()) == 0
}

func (r *Result) intactError() error {
	if r.PanelErr != nil || r.Panel == nil {
		return fmt.Errorf("panel registration unreadable")
	}
	p := r.Panel
	scope := " --spec " + termsafe.Escape(p.Spec)
	if p.Gate == "adhoc" {
		scope = " --gate adhoc"
	}
	recovery := fmt.Sprintf("mindspec panel create %s%s --round %d", termsafe.Escape((&Registration{Dir: r.Dir}).Slug()), scope, p.Round+1)
	if p.ExpectedReviewers < 2 {
		return fmt.Errorf("panel registration requires at least 2 reviewers; recovery: %s --reviewers 2", recovery)
	}
	if r.IntegrityErr != nil {
		return fmt.Errorf("panel.json changed outside the panel verbs (%s); recovery: %s", termsafe.Escape(r.IntegrityErr.Error()), recovery)
	}
	return nil
}

func invalidVerdict(v Verdict) bool {
	return v.Invalid || (v.Verdict != VerdictApprove && v.Verdict != VerdictReject && v.Verdict != VerdictRequestChanges)
}

func (r *Result) InvalidCount() int {
	n := 0
	for _, v := range r.Verdicts {
		if invalidVerdict(v) {
			n++
		}
	}
	for _, v := range r.Unexpected {
		if invalidVerdict(v) {
			n++
		}
	}
	return n
}

func (r *Result) vetoSummary() string {
	rejectOrHard := r.Rejects > 0 || len(r.HardBlocks) > 0
	for _, v := range r.Unexpected {
		rejectOrHard = rejectOrHard || v.Verdict == VerdictReject || v.HardBlock
	}
	if rejectOrHard {
		if r.InvalidCount() > 0 {
			return "HARD block / REJECT / INVALID"
		}
		return "HARD block / REJECT"
	}
	return "INVALID"
}

func (r *Result) unexpectedVeto() bool {
	for _, v := range r.Unexpected {
		if v.Verdict == VerdictReject || v.HardBlock || invalidVerdict(v) {
			return true
		}
	}
	return false
}

func (r *Result) unexpectedVetoNames() string {
	var names []string
	for _, v := range r.Unexpected {
		if v.Verdict == VerdictReject || v.HardBlock || invalidVerdict(v) {
			names = append(names, termsafe.Escape(v.File))
		}
	}
	if len(names) == 0 {
		return ""
	}
	return ", unexpected veto: " + strings.Join(names, ", ")
}

func (r *Result) refutationApplies(v Verdict, ref Refutation) bool {
	if r.Panel == nil || v.HardBlock || invalidVerdict(v) || v.Verdict != VerdictRequestChanges || ref.Slot != v.Slot || ref.Round != r.LatestRound {
		return false
	}
	if ref.FindingSHA256 == "" {
		return r.Panel.Telemetry == ""
	}
	if ref.Finding == "slot" {
		return ref.FindingSHA256 == v.SlotSHA256
	}
	for _, f := range v.Findings {
		if ref.Finding == f.ID && ref.FindingSHA256 == f.SHA256 {
			return true
		}
	}
	return false
}

func (r *Result) unresolvedFindings(v Verdict) []string {
	if r.Panel == nil {
		return []string{"slot"}
	}
	// Legacy in-memory fixtures can omit finding payloads; their unpinned
	// refutations retain the legacy slot semantics.
	for _, ref := range r.Panel.Refutations {
		if r.refutationApplies(v, ref) && (ref.Finding == "slot" || ref.FindingSHA256 == "") {
			return nil
		}
	}
	var unresolved []string
	for _, f := range v.Findings {
		covered := false
		for _, ref := range r.Panel.Refutations {
			if ref.Finding == f.ID && r.refutationApplies(v, ref) {
				covered = true
				break
			}
		}
		if !covered {
			unresolved = append(unresolved, f.ID)
		}
	}
	if len(v.Findings) == 0 {
		return []string{"slot"}
	}
	return unresolved
}

// UnresolvedNames names each unresolved finding, including text-hash mismatches.
func (r *Result) UnresolvedNames() string {
	var names []string
	for _, v := range r.UnresolvedVerdicts() {
		label := termsafe.Escape(v.Slot)
		if r.Panel != nil && v.Verdict == VerdictRequestChanges {
			label += " (" + strings.Join(r.unresolvedFindings(v), ", ") + ")"
		}
		names = append(names, label)
	}
	return strings.Join(names, ", ")
}

func (r *Result) ResolvedCount() int {
	n := 0
	for _, v := range r.Verdicts {
		if v.Verdict == VerdictRequestChanges && r.isRefuted(v.Slot) {
			n++
		}
	}
	return n
}

func (r *Result) invalidNames() string {
	var names []string
	for _, v := range r.Verdicts {
		if invalidVerdict(v) {
			names = append(names, termsafe.Escape(v.Slot))
		}
	}
	if len(names) == 0 {
		return ""
	}
	return ", INVALID from " + strings.Join(names, ", ")
}
