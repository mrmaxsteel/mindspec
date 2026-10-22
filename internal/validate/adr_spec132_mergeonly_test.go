package validate

import (
	"os"
	"strings"
	"testing"
)

func TestDocs_Spec132Amendments_MergeOnly(t *testing.T) {
	files := map[string][]string{
		"../../.mindspec/adr/ADR-0041-gate-before-mutate.md": {
			"## Amendment (Spec 132)", "### Merge-only commit and closing call",
			"It prints the merge SHA and both parents and records the SHA.",
			"commit phase with no close", "M^1..M", "resolved index", "skips the presence check",
			"path-scoped export and ledger commit", "repeats any doc-skew or ADR override flag",
			"A closed child of the spec epic without a naming merge runs the merge recovery without a second close.",
		},
		"../../.mindspec/domains/workflow/architecture.md": {"complete --merge-only", "one naming merge", "skips the presence check"},
		"../../.mindspec/domains/workflow/interfaces.md":   {"mindspec_merge_only=pending", "mindspec_merge_only_sha", "A closed child of the spec epic without a naming merge runs the merge recovery without a second close."},
		"../../.mindspec/domains/workflow/runbook.md":      {"Do not revert a", "Restore and commit", "until it exits 0"},
		"../../project-docs/user/guides/merge-safety.md":   {"Do not revert a", "--merge-only", "Restore first, then close", "only one naming merge"},
		"../../.mindspec/domains/execution/interfaces.md":  {"Executor.MergeBead", "PublishBeadClose", "CleanupBead", "commits only the paths it is given", "The caller includes the export path"},
	}
	for name, phrases := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(data)), " ")
		for _, phrase := range phrases {
			if !strings.Contains(text, phrase) {
				t.Errorf("%s lacks %q", name, phrase)
			}
		}
	}
}
