package approve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// goldenPlanInput is a representative well-formed plan.md: a leading YAML
// frontmatter block followed by a multi-line body. Both approve writes must
// leave the post-fence body byte-for-byte untouched.
const goldenPlanInput = `---
spec_id: "108-example"
status: Draft
version: "1"
work_chunks:
    - depends_on: []
      id: 1
      key_file_paths:
        - internal/approve/plan.go
---

# Plan: 108-example

## Bead 1: Do the thing

Body content here.
`

// goldenPlanBody is the exact byte sequence after goldenPlanInput's closing
// `---\n` fence — the part both writes must preserve verbatim.
const goldenPlanBody = "\n# Plan: 108-example\n\n## Bead 1: Do the thing\n\nBody content here.\n"

// TestApprovalWriteByteIdentical locks the exact bytes updatePlanApprovalAt
// emits for a well-formed plan (spec 108 R4). The golden captures today's
// behavior: alphabetically-sorted frontmatter keys, `"---\n" + TrimRight(marshaled,
// "\n") + "\n---\n"`, and the untouched post-fence body. The clock is injected so
// approved_at is deterministic.
func TestApprovalWriteByteIdentical(t *testing.T) {
	const golden = "---\napproved_at: \"2026-07-02T15:05:24Z\"\napproved_by: user\nspec_id: 108-example\nstatus: Approved\nversion: \"1\"\nwork_chunks:\n    - depends_on: []\n      id: 1\n      key_file_paths:\n        - internal/approve/plan.go\n---\n\n# Plan: 108-example\n\n## Bead 1: Do the thing\n\nBody content here.\n"

	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	if err := os.WriteFile(planPath, []byte(goldenPlanInput), 0644); err != nil {
		t.Fatal(err)
	}

	fixed := time.Date(2026, 7, 2, 15, 5, 24, 0, time.UTC)
	if err := updatePlanApprovalAt(planPath, "user", fixed); err != nil {
		t.Fatalf("updatePlanApprovalAt: %v", err)
	}

	got, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != golden {
		t.Errorf("approval write not byte-identical to golden.\n got: %q\nwant: %q", string(got), golden)
	}
	// The post-fence body must be preserved verbatim.
	if !strings.HasSuffix(string(got), goldenPlanBody) {
		t.Errorf("approval write did not preserve the post-fence body verbatim: got %q", string(got))
	}
}

// TestBeadIDsWriteByteIdentical locks the exact bytes writeBeadIDsToFrontmatter
// emits for a well-formed plan (spec 108 R4). Fully deterministic; the golden
// captures today's behavior through the shared mutateFrontmatterFile helper.
func TestBeadIDsWriteByteIdentical(t *testing.T) {
	const golden = "---\nbead_ids:\n    - mindspec-aaa.1\n    - mindspec-aaa.2\nspec_id: 108-example\nstatus: Draft\nversion: \"1\"\nwork_chunks:\n    - depends_on: []\n      id: 1\n      key_file_paths:\n        - internal/approve/plan.go\n---\n\n# Plan: 108-example\n\n## Bead 1: Do the thing\n\nBody content here.\n"

	tmp := t.TempDir()
	planPath := filepath.Join(tmp, "plan.md")
	if err := os.WriteFile(planPath, []byte(goldenPlanInput), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeBeadIDsToFrontmatter(planPath, []string{"mindspec-aaa.1", "mindspec-aaa.2"}); err != nil {
		t.Fatalf("writeBeadIDsToFrontmatter: %v", err)
	}

	got, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != golden {
		t.Errorf("bead_ids write not byte-identical to golden.\n got: %q\nwant: %q", string(got), golden)
	}
	if !strings.HasSuffix(string(got), goldenPlanBody) {
		t.Errorf("bead_ids write did not preserve the post-fence body verbatim: got %q", string(got))
	}
}
