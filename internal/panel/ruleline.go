package panel

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// RuleLine is shared by tally, verify and instruct. It describes the rule;
// live freshness and dirty-tree checks remain the gate caller's responsibility.
func RuleLine(r *Result) string {
	var unresolved []string
	for _, v := range r.UnresolvedVerdicts() {
		if v.Verdict == VerdictRequestChanges && !invalidVerdict(v) {
			unresolved = append(unresolved, termsafe.Escape(v.Slot)+" ("+strings.Join(r.unresolvedFindings(v), ", ")+")")
		}
	}

	return fmt.Sprintf("rule: PASS iff fresh · intact · all %d slots in · 0 REJECT · 0 hard_block · 0 INVALID · 0 unresolved REQUEST_CHANGES (fix/refute/defer) — now: %d/%d in, %d APPROVE, %d resolved, %d unresolved (%s), %d INVALID",
		r.ExpectedReviewers(), len(r.Verdicts), r.ExpectedReviewers(), r.Approves, r.ResolvedCount(), len(unresolved), strings.Join(unresolved, ", "), r.InvalidCount())
}

// LegacyThresholdNotice reports recorded compatibility data without using it.
func LegacyThresholdNotice(p *Panel) string {
	if p == nil || p.ApproveThresholdExpr == "" {
		return ""
	}
	return fmt.Sprintf("approve_threshold %s ignored (legacy; not decision-bearing)", termsafe.Escape(p.ApproveThresholdExpr))
}
