package doctor

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestDoctor_AncestorNamesPrerequisite(t *testing.T) {
	const prerequisite = "mindspec-ab12"
	const dependent = "mindspec-dep1"
	command := "mindspec next " + dependent + " --allow-not-ready=MF-3 --evidence <file>"
	stubScanIntegrity(t, func(string, *phase.Cache) lifecycle.IntegrityFindings {
		return lifecycle.IntegrityFindings{Ancestors: []lifecycle.ReattestRefusal{{BeadID: prerequisite, State: lifecycle.ReattestStateAncestor, Detail: prerequisite + "'s changes are present. A person must verify the work before using the override.", Recovery: []string{command}}}}
	})
	report := &Report{}
	checkLifecycleIntegrity(report, t.TempDir())
	report.Fix()
	if len(report.Checks) != 1 {
		t.Fatalf("checks: %+v", report.Checks)
	}
	check := report.Checks[0]
	if !strings.Contains(check.Name, prerequisite) || strings.Contains(check.Name, dependent) || check.Status != Warn || check.FixFunc != nil || !strings.Contains(check.Message, "recovery: "+command) || report.HasFailures() {
		t.Fatalf("wrong prerequisite finding: %+v", check)
	}
}
