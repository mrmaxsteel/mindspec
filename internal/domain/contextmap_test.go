package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextMapSkeleton(t *testing.T) {
	skel := ContextMapSkeleton()

	boundedIdx := strings.Index(skel, "## Bounded Contexts")
	if boundedIdx < 0 {
		t.Fatalf("skeleton missing '## Bounded Contexts' heading:\n%s", skel)
	}
	sepIdx := strings.Index(skel, "---")
	if sepIdx < 0 {
		t.Fatalf("skeleton missing '---' separator:\n%s", skel)
	}
	if sepIdx < boundedIdx {
		t.Errorf("separator must come AFTER the Bounded Contexts heading, got skeleton:\n%s", skel)
	}
}

func TestEntryHeadingForDomain(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"alpha", "### Alpha"},
		{"my-cool-domain", "### My-Cool-Domain"},
	}
	for _, tt := range tests {
		if got := EntryHeadingForDomain(tt.name); got != tt.want {
			t.Errorf("EntryHeadingForDomain(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestHasEntry(t *testing.T) {
	content := ContextMapSkeleton()
	if HasEntry(content, "alpha") {
		t.Error("a fresh skeleton should have no entries")
	}

	// Insert the entry INSIDE the Bounded Contexts section (before the `---`
	// separator), exactly where appendContextMap writes it.
	mapped := strings.Replace(content, "---", "### Alpha\n\n**Owns**: _(fill in)_\n\n---", 1)
	if !HasEntry(mapped, "alpha") {
		t.Error("expected HasEntry to find the in-section ### Alpha heading")
	}
	if HasEntry(mapped, "beta") {
		t.Error("HasEntry must not match an unrelated domain")
	}

	// Must match the EXACT heading line, not a substring occurrence inside
	// prose (e.g. a sentence that happens to mention "Alpha").
	prose := "# Context Map\n\nAlpha is mentioned here but has no heading.\n"
	if HasEntry(prose, "alpha") {
		t.Error("HasEntry must not match a prose mention, only the '### <Title>' heading line")
	}
}

// TestHasEntry_SectionAware pins FX-1: a `### <Title>` heading is a mapping
// ONLY inside the `## Bounded Contexts` section. A same-named heading that
// sits AFTER the section's `---` separator (e.g. under `## Notes`) must NOT
// count as mapped — otherwise a fully-scaffolded domain whose only matching
// heading lives out-of-section gets wrongly refused and stranded in a
// terminal unmapped state a re-run can't repair.
func TestHasEntry_SectionAware(t *testing.T) {
	outOfSection := "# Context Map\n\n## Bounded Contexts\n\n---\n\n## Notes\n\n### Alpha\n"
	if HasEntry(outOfSection, "alpha") {
		t.Error("a ### Alpha heading OUTSIDE the Bounded Contexts section must NOT count as mapped")
	}

	// A heading under a different section that PRECEDES Bounded Contexts is
	// likewise not a mapping.
	beforeSection := "# Context Map\n\n## Overview\n\n### Alpha\n\n## Bounded Contexts\n\n---\n"
	if HasEntry(beforeSection, "alpha") {
		t.Error("a ### Alpha heading BEFORE the Bounded Contexts section must NOT count as mapped")
	}

	// In-section heading (before the separator) IS a mapping.
	inSection := "# Context Map\n\n## Bounded Contexts\n\n### Alpha\n\n**Owns**: _(fill in)_\n\n---\n"
	if !HasEntry(inSection, "alpha") {
		t.Error("a ### Alpha heading INSIDE the Bounded Contexts section must count as mapped")
	}
}

// TestAddConverges_OutOfSectionHeading is the end-to-end FX-1 repro: a
// fully-scaffolded alpha whose context-map's only `### Alpha` heading sits
// under `## Notes` (outside Bounded Contexts) must be BACKFILLED by
// `domain add alpha` (a real in-section entry added), not refused. A
// section-blind predicate would refuse here and strand the domain.
func TestAddConverges_OutOfSectionHeading(t *testing.T) {
	root := setupFlatRoot(t)

	domainDir := filepath.Join(root, ".mindspec", "domains", "alpha")
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"overview.md", "architecture.md", "interfaces.md", "runbook.md", "OWNERSHIP.yaml"} {
		if err := os.WriteFile(filepath.Join(domainDir, name), []byte("# existing\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmPath := filepath.Join(root, ".mindspec", "context-map.md")
	if err := os.WriteFile(cmPath, []byte("# Context Map\n\n## Bounded Contexts\n\n---\n\n## Notes\n\n### Alpha\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Add(root, "alpha"); err != nil {
		t.Fatalf("Add() must backfill (not refuse) an out-of-section heading, got: %v", err)
	}

	data, _ := os.ReadFile(cmPath)
	if !HasEntry(string(data), "alpha") {
		t.Errorf("expected an in-section ### Alpha entry after backfill; got:\n%s", data)
	}
}
