package validate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ownershipAmendmentGaps checks the governing text, not test names.
func ownershipAmendmentGaps(adr, spec, plan string) []string {
	var gaps []string
	for _, text := range []string{
		"## Amendment (Spec 132)",
		"This narrows the ADR-0041 snapshot-gate clause",
		"every OWNERSHIP/content read anchored at `T` too",
		"The ownership read uses the landing tree",
		"The diffed content stays at `T`",
		"One `ownerRef` feeds all six readers",
	} {
		if !strings.Contains(adr, text) {
			gaps = append(gaps, "ADR: "+text)
		}
	}
	for _, text := range []string{
		"Deferred to **spec 132-readiness-followon**",
		"Status note (2026-10-21): spec 132 owns G, H, #250, I, and phase-lineage unification.",
		"spec 132 uses one landing-tree ownership ref",
		`"bead <id> has the plan-bead form for epic <epic>, but its lineage resolves to <lineage> through its dependency on epic <lineage>"`,
		"advisories change printed lines only. They never change the exit code, the binding, or the audited tuple.",
	} {
		if !strings.Contains(spec, text) {
			gaps = append(gaps, "spec: "+text)
		}
	}
	found := false
	for _, line := range strings.Split(plan, "\n") {
		for _, command := range strings.Split(line, "`") {
			if strings.Contains(command, "go test ./... -short") {
				found = true
				if !strings.Contains(command, "-timeout=20m") {
					gaps = append(gaps, "unbounded suite: "+command)
				}
			}
		}
	}
	if !found {
		gaps = append(gaps, "plan has no suite command")
	}
	return gaps
}

func TestDocs_Spec132Amendments_Ownership(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source directory unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	adr := read(".mindspec/adr/ADR-0036-ownership-discovery.md")
	spec := read(".mindspec/specs/129-readiness-tracker-honesty/spec.md")
	plan := read(".mindspec/specs/129-readiness-tracker-honesty/plan.md")
	if gaps := ownershipAmendmentGaps(adr, spec, plan); len(gaps) != 0 {
		t.Fatalf("ownership amendment gaps: %v", gaps)
	}
	// Each wrong value must fire the pin. No deleted input counts as proof.
	for _, mutation := range []struct{ name, adr, spec, plan string }{
		{"wrong section", strings.Replace(adr, "## Amendment (Spec 132)", "## Amendment (Spec 999)", 1), spec, plan},
		{"wrong owner", strings.ReplaceAll(adr, "The ownership read uses the landing tree", "the ownership read uses the bead tip"), spec, plan},
		{"wrong snapshot clause", strings.ReplaceAll(adr, "read anchored at `T` too", "read anchored at `HEAD` too"), spec, plan},
		{"wrong follow-on", adr, strings.ReplaceAll(spec, "spec 132 owns G, H, #250, I, and phase-lineage unification.", "spec 999 owns G, H, #250, I, and phase-lineage unification."), plan},
		{"wrong advisory", adr, strings.ReplaceAll(spec, "They never change the exit code, the binding, or the audited tuple.", "They can change the exit code, the binding, or the audited tuple."), plan},
		{"wrong diagnostic", adr, strings.ReplaceAll(spec, "through its dependency on epic <lineage>", "through its title number"), plan},
		{"wrong timeout", adr, spec, strings.ReplaceAll(plan, "-timeout=20m", "-timeout=2m")},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			if gaps := ownershipAmendmentGaps(mutation.adr, mutation.spec, mutation.plan); len(gaps) == 0 {
				t.Fatal("wrong value did not fire the pin")
			}
		})
	}
}
