package panel

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestS133Digest_OwnWritesAndSemanticFrontmatter(t *testing.T) {
	const dir = ".mindspec/specs/900-example"
	const body = "# Spec\n\nBody.\n\n"
	draft := "## Approval\n\n- **Status**: Draft\n- **Approved By**: —\n- **Approval Date**: —\n- **Notes**: —\n"
	approved := "## Approval\n\n- **Status**: Approved\n- **Approved By**: user\n- **Approval Date**: today\n- **Notes**: good\n"
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		for _, policy := range []string{"warn", "enforce"} {
			t.Run(gate+"/"+policy, func(t *testing.T) {
				digest := func(spec, plan string) string {
					t.Helper()
					return s133Digest(t, dir, gate, map[string]string{dir + "/spec.md": spec, dir + "/plan.md": plan})
				}
				baseSpec := "---\nstatus: Draft\ntitle: 'Example'\npriority: 1\n---\n" + body
				basePlan := "---\nstatus: Draft\nwork_chunks: [{id: 1, depends_on: []}]\nadr_citations: [ADR-0037]\n---\n# Plan\n"
				base := digest(baseSpec, basePlan)
				for name, spec := range map[string]string{"draft": baseSpec + draft, "approved": baseSpec + approved, "absent": baseSpec, "reordered": "---\npriority: 1\napproved_by: user\ntitle: Example\nstatus: Approved\napproved_at: today\n---\n" + body} {
					if got := digest(spec, basePlan); got != base {
						t.Errorf("%s changed digest", name)
					}
				}
				if got := digest(baseSpec, "---\nadr_citations: [ADR-0037]\nbead_ids: [example]\nwork_chunks: [{depends_on: [], id: 1}]\napproved_at: now\napproved_by: user\nstatus: Approved\n---\n# Plan\n"); got != base {
					t.Error("plan approval rewrite changed digest")
				}
				for name, spec := range map[string]string{"extra approval line": baseSpec + approved + "extra line\n", "body": baseSpec + "edit\n", "new field": strings.Replace(baseSpec, "title:", "new_field: value\ntitle:", 1), "duplicate approval bullet": baseSpec + approved + "- **Notes**: duplicate\n"} {
					if digest(spec, basePlan) == base {
						t.Errorf("%s omitted", name)
					}
				}
				for name, plan := range map[string]string{"dependency": strings.Replace(basePlan, "depends_on: []", "depends_on: [2]", 1), "citation": strings.Replace(basePlan, "ADR-0037", "ADR-0041", 1)} {
					if digest(baseSpec, plan) == base {
						t.Errorf("%s omitted", name)
					}
				}
				without := s133Digest(t, dir, gate, map[string]string{dir + "/spec.md": baseSpec})
				if digest(baseSpec, PlanScaffold("900-example")) != without {
					t.Error("exact scaffold changed digest")
				}
				if digest(baseSpec, PlanScaffold("900-example")+"extra\n") == without {
					t.Error("edited scaffold omitted")
				}
			})
		}
	}
}

func TestS133Digest_PlanScaffoldValidID(t *testing.T) {
	const id = "900-example"
	scaffold := PlanScaffold(id)
	for _, want := range []string{"\nspec_id: " + id + "\n", "\n# Plan: " + id + "\n"} {
		if !strings.Contains(scaffold, want) {
			t.Errorf("scaffold missing unquoted ID: %q", want)
		}
	}
	if strings.Contains(scaffold, strconv.Quote(id)) {
		t.Error("scaffold quotes a valid ID")
	}
}

func s133Digest(t *testing.T, dir, gate string, files map[string]string) string {
	t.Helper()
	// Deliberately change blob IDs with content: a raw-tree digest fails the
	// equality assertions even when approval data alone changes.
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var listing strings.Builder
	for _, name := range names {
		checkDocumentSubjectLFCompatibility(t, name, []byte(files[name]), dir, gate)
		fmt.Fprintf(&listing, "100644 blob %x\t%s\n", sha1.Sum([]byte(files[name])), name)
	}
	got, err := DocumentSubjectDigest(listing.String(), Subject(gate, dir), dir, gate, func(name string) ([]byte, error) { return []byte(files[name]), nil })
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestS133Digest_FinalReview(t *testing.T) {
	const dir = ".mindspec/specs/demo"
	base := map[string]string{dir + "/spec.md": "---\ntitle: 'Example'\npriority: 1\n---\nBody\n", "source.go": "source\n"}
	before := s133Digest(t, dir, "final_review", base)
	base[dir+"/spec.md"] = "---\npriority: 1\ntitle: Example\n---\nBody\n"
	if s133Digest(t, dir, "final_review", base) != before {
		t.Fatal("frontmatter order changed final review")
	}
	placeholder := strings.ReplaceAll(placeholderTemplate, "NNNN", "0099")
	placeholder = strings.ReplaceAll(placeholder, "<Title>", "Placeholder for ADR-0099")
	placeholder = strings.ReplaceAll(placeholder, "<YYYY-MM-DD>", "2026-10-23")
	base[".mindspec/adr/ADR-0099.md"] = placeholder
	if s133Digest(t, dir, "final_review", base) != before {
		t.Fatal("placeholder changed digest")
	}
	base[".mindspec/adr/ADR-0099.md"] += "extra\n"
	if s133Digest(t, dir, "final_review", base) == before {
		t.Fatal("edited placeholder omitted")
	}
	base["source.go"] = "edited\n"
	if s133Digest(t, dir, "final_review", base) == before {
		t.Fatal("body change omitted")
	}
}

func TestS133Digest_Modes(t *testing.T) {
	const dir = ".mindspec/specs/demo"
	digest := func(mode, text string) string {
		t.Helper()
		listing := fmt.Sprintf("%s blob %x\t%s/spec.md\n", mode, sha1.Sum([]byte(text)), dir)
		got, err := DocumentSubjectDigest(listing, Subject("spec_approve", dir), dir, "spec_approve", func(string) ([]byte, error) { return []byte(text), nil })
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	draft := "---\nstatus: Draft\ntitle: Example\n---\nBody\n"
	approved := "---\ntitle: 'Example'\nstatus: Approved\n---\nBody\n"
	if digest("100755", draft) != digest("100755", approved) {
		t.Fatal("executable document lost semantic equality")
	}
	if digest("100644", draft) == digest("100755", draft) {
		t.Fatal("file mode change omitted")
	}
}
