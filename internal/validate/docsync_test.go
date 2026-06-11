package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
)

func TestClassifyChanges(t *testing.T) {
	files := []string{
		"internal/validate/spec.go",
		"internal/validate/spec_test.go",
		"cmd/mindspec/validate.go",
		"docs/domains/workflow/interfaces.md",
		"CLAUDE.md",
		"go.mod",
	}

	source, docs := classifyChanges(files)

	// spec_test.go should NOT be in source (test files excluded)
	// go.mod should NOT be in either
	if len(source) != 2 {
		t.Errorf("expected 2 source files, got %d: %v", len(source), source)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 doc files, got %d: %v", len(docs), docs)
	}
}

func TestIsDocFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"docs/core/MODES.md", true},
		{"docs/domains/workflow/overview.md", true},
		{"CLAUDE.md", true},
		{"AGENTS.md", true},
		{".mindspec/docs/core/ARCHITECTURE.md", true},
		{"internal/validate/spec.go", false},
		{"cmd/mindspec/validate.go", false},
		{"go.mod", false},
	}
	for _, tt := range tests {
		if isDocFile(tt.path) != tt.expected {
			t.Errorf("isDocFile(%q) = %v, want %v", tt.path, !tt.expected, tt.expected)
		}
	}
}

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"internal/validate/spec.go", true},
		{"cmd/mindspec/validate.go", true},
		{"internal/validate/spec_test.go", false},
		{"docs/core/MODES.md", false},
		{"go.mod", false},
	}
	for _, tt := range tests {
		if isSourceFile(tt.path) != tt.expected {
			t.Errorf("isSourceFile(%q) = %v, want %v", tt.path, !tt.expected, tt.expected)
		}
	}
}

func TestParseChangedFiles(t *testing.T) {
	output := "internal/validate/spec.go\ncmd/mindspec/validate.go\ndocs/core/MODES.md\n"
	files := ParseChangedFiles(output)
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

func TestParseChangedFiles_Empty(t *testing.T) {
	files := ParseChangedFiles("")
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

// writeOwnershipFixture writes a minimal domain layout under root for
// internal-docs lane tests:
//
//	.mindspec/docs/domains/<domain>/OWNERSHIP.yaml  (paths: <paths>)
//
// Returns the absolute manifest path so tests can assert it appears in
// the lane's error message.
func writeOwnershipFixture(t *testing.T, root, domain string, paths []string) string {
	t.Helper()
	dir := filepath.Join(root, ".mindspec", "docs", "domains", domain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	manifest := filepath.Join(dir, "OWNERSHIP.yaml")
	body := "paths:\n"
	for _, p := range paths {
		body += "  - " + p + "\n"
	}
	if err := os.WriteFile(manifest, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", manifest, err)
	}
	return manifest
}

func TestCheckInternalPackages_WithDomainDocs(t *testing.T) {
	root := t.TempDir()
	writeOwnershipFixture(t, root, "workflow", []string{"internal/validate/**"})

	r := &Result{SubCommand: "docs"}
	source := []string{"internal/validate/spec.go"}
	docs := []string{".mindspec/docs/domains/workflow/interfaces.md"}

	checkInternalPackages(r, root, source, docs)
	if r.HasFailures() {
		t.Errorf("expected no failures when domain docs updated, got %+v", r.Issues)
	}
	if len(r.Issues) > 0 {
		t.Errorf("expected no issues, got %d: %+v", len(r.Issues), r.Issues)
	}
}

func TestCheckInternalPackages_WithoutDomainDocs(t *testing.T) {
	root := t.TempDir()
	writeOwnershipFixture(t, root, "workflow", []string{"internal/validate/**"})

	r := &Result{SubCommand: "docs"}
	source := []string{"internal/validate/spec.go"}
	docs := []string{"CLAUDE.md"}

	checkInternalPackages(r, root, source, docs)
	found := false
	for _, issue := range r.Issues {
		if issue.Name == "internal-docs" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected internal-docs error, got %+v", r.Issues)
	}
}

// TestValidateDocsErrorsOnInternalDocSkew is the spec-086 plan-mandated
// test (plan.md:675 grep target). It exercises the internal-docs lane
// end-to-end through checkInternalPackages with a real OWNERSHIP.yaml
// fixture: source under an owned path with no matching domain-docs
// update must produce an ERROR (not a warning) whose message names the
// manifest file that decided ownership.
func TestValidateDocsErrorsOnInternalDocSkew(t *testing.T) {
	root := t.TempDir()
	manifest := writeOwnershipFixture(t, root, "workflow", []string{"internal/validate/**"})

	r := &Result{SubCommand: "docs"}
	source := []string{"internal/validate/spec.go"}
	docs := []string{"CLAUDE.md"} // not a domain doc

	checkInternalPackages(r, root, source, docs)

	var issue *Issue
	for i := range r.Issues {
		if r.Issues[i].Name == "internal-docs" {
			issue = &r.Issues[i]
			break
		}
	}
	if issue == nil {
		t.Fatalf("expected internal-docs issue, got %+v", r.Issues)
	}
	if issue.Severity != SevError {
		t.Errorf("internal-docs severity = %v, want SevError (Req 7)", issue.Severity)
	}
	if !strings.Contains(issue.Message, manifest) {
		t.Errorf("internal-docs message must name manifest %q; got %q", manifest, issue.Message)
	}
	if !r.HasFailures() {
		t.Error("result should report failures")
	}
}

// TestValidateDocsErrorsOnInternalDocSkew_Fallback covers the fallback
// marker path: when OWNERSHIP.yaml is absent for a domain the message
// must include the literal "<fallback: internal/<domain>/**>" marker so
// operators know which manifest to author.
func TestValidateDocsErrorsOnInternalDocSkew_Fallback(t *testing.T) {
	root := t.TempDir()
	// Create the domain directory WITHOUT OWNERSHIP.yaml so loadOwnership
	// returns the synthetic fallback Ownership.
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "domains", "workflow"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := &Result{SubCommand: "docs"}
	source := []string{"internal/workflow/run.go"}
	docs := []string{"CLAUDE.md"}

	checkInternalPackages(r, root, source, docs)

	var issue *Issue
	for i := range r.Issues {
		if r.Issues[i].Name == "internal-docs" {
			issue = &r.Issues[i]
			break
		}
	}
	if issue == nil {
		t.Fatalf("expected internal-docs issue, got %+v", r.Issues)
	}
	if !strings.Contains(issue.Message, "<fallback: internal/workflow/**>") {
		t.Errorf("internal-docs message must include fallback marker; got %q", issue.Message)
	}
}

func TestCheckCmdChanges_WithRelevantDocs(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	source := []string{"cmd/mindspec/validate.go"}
	docs := []string{"CLAUDE.md"}

	checkCmdChanges(r, source, docs)
	if len(r.Issues) > 0 {
		t.Error("expected no issues when CLAUDE.md updated")
	}
}

func TestCheckCmdChanges_WithoutRelevantDocs(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	source := []string{"cmd/mindspec/validate.go"}
	docs := []string{"docs/domains/workflow/overview.md"}

	checkCmdChanges(r, source, docs)
	found := false
	for _, issue := range r.Issues {
		if issue.Name == "cmd-docs" {
			found = true
		}
	}
	if !found {
		t.Error("expected cmd-docs warning")
	}
}

func TestCheckCmdChanges_NoCmdFiles(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	source := []string{"internal/validate/spec.go"}
	docs := []string{}

	checkCmdChanges(r, source, docs)
	if len(r.Issues) > 0 {
		t.Error("expected no issues when no cmd files changed")
	}
}

// TestValidateSpecArtifactSync covers the spec-artifact-sync lane: a
// spec.md change must be accompanied in the same diff by plan.md, an
// ADR, or any sibling artifact under the same spec directory.
func TestValidateSpecArtifactSync(t *testing.T) {
	cases := []struct {
		name      string
		all       []string
		wantError bool
	}{
		{
			name:      "spec.md only -> error",
			all:       []string{".mindspec/docs/specs/086-doc-sync/spec.md"},
			wantError: true,
		},
		{
			name: "spec.md + plan.md -> no error",
			all: []string{
				".mindspec/docs/specs/086-doc-sync/spec.md",
				".mindspec/docs/specs/086-doc-sync/plan.md",
			},
			wantError: false,
		},
		{
			name: "spec.md + sibling notes.md -> no error",
			all: []string{
				".mindspec/docs/specs/086-doc-sync/spec.md",
				".mindspec/docs/specs/086-doc-sync/notes.md",
			},
			wantError: false,
		},
		{
			name: "spec.md + ADR .md -> no error",
			all: []string{
				".mindspec/docs/specs/086-doc-sync/spec.md",
				".mindspec/docs/adr/ADR-0031-doc-sync-gate.md",
			},
			wantError: false,
		},
		{
			name:      "no spec.md change -> no error",
			all:       []string{"internal/validate/spec.go", "docs/core/MODES.md"},
			wantError: false,
		},
		{
			name: "spec.md + unrelated source file (no companion) -> error",
			all: []string{
				".mindspec/docs/specs/086-doc-sync/spec.md",
				"internal/validate/docsync.go",
			},
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Result{SubCommand: "docs"}
			changes := classifiedChanges{All: tc.all}
			validateSpecArtifactSync(r, changes)

			found := false
			for _, issue := range r.Issues {
				if issue.Name == "spec-artifact-sync" {
					if issue.Severity != SevError {
						t.Errorf("spec-artifact-sync severity = %v, want %v", issue.Severity, SevError)
					}
					found = true
				}
			}
			if found != tc.wantError {
				t.Errorf("spec-artifact-sync error present = %v, want %v (issues=%+v)", found, tc.wantError, r.Issues)
			}
		})
	}
}

// TestValidateSpecArtifactSync_MessageMentionsSpecID pins the panel
// CONSENSUS Major 5 contract: the error message must name the specific
// spec ID and the ADR/sibling path hint so operators can act on it.
func TestValidateSpecArtifactSync_MessageMentionsSpecID(t *testing.T) {
	r := &Result{SubCommand: "docs"}
	changes := classifiedChanges{All: []string{".mindspec/docs/specs/086-doc-sync/spec.md"}}
	validateSpecArtifactSync(r, changes)

	var issue *Issue
	for i := range r.Issues {
		if r.Issues[i].Name == "spec-artifact-sync" {
			issue = &r.Issues[i]
			break
		}
	}
	if issue == nil {
		t.Fatalf("expected spec-artifact-sync issue, got %+v", r.Issues)
	}
	if !strings.Contains(issue.Message, "086-doc-sync") {
		t.Errorf("message must name specID; got %q", issue.Message)
	}
	if !strings.Contains(issue.Message, ".mindspec/docs/adr/") {
		t.Errorf("message must hint ADR prefix; got %q", issue.Message)
	}
	if !strings.Contains(issue.Message, ".mindspec/docs/specs/086-doc-sync/") {
		t.Errorf("message must hint sibling prefix; got %q", issue.Message)
	}
}

// TestPromotedLaneSeverities pins the spec-086 Req 7 contract: the
// doc-sync and internal-docs lanes are ERRORS, while the operator-docs
// (cmd-docs) lane stays a WARNING.
func TestPromotedLaneSeverities(t *testing.T) {
	root := t.TempDir()
	writeOwnershipFixture(t, root, "workflow", []string{"internal/validate/**"})

	r := &Result{SubCommand: "docs"}
	checkInternalPackages(r, root, []string{"internal/validate/spec.go"}, nil)
	for _, issue := range r.Issues {
		if issue.Name == "internal-docs" && issue.Severity != SevError {
			t.Errorf("internal-docs severity = %v, want %v", issue.Severity, SevError)
		}
	}

	r2 := &Result{SubCommand: "docs"}
	checkCmdChanges(r2, []string{"cmd/mindspec/validate.go"}, []string{"docs/domains/workflow/overview.md"})
	for _, issue := range r2.Issues {
		if issue.Name == "cmd-docs" && issue.Severity != SevWarning {
			t.Errorf("cmd-docs severity = %v, want %v (Req 7 contract)", issue.Severity, SevWarning)
		}
	}
}

// TestOperatorDocsAdditiveAcceptSet verifies the operator-docs lane accepts
// the spec-086 additive set (.mindspec/docs/user/** and
// .mindspec/docs/core/USAGE.md) in addition to the existing accept set
// (CLAUDE.md, CONVENTIONS.md). Severity stays at warning per Req 7.
func TestOperatorDocsAdditiveAcceptSet(t *testing.T) {
	cases := []struct {
		name        string
		source      []string
		docs        []string
		wantWarning bool
	}{
		{
			name:        "CLAUDE.md satisfies lane (existing, preserved)",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{"CLAUDE.md"},
			wantWarning: false,
		},
		{
			name:        "CONVENTIONS.md satisfies lane (existing, preserved)",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{"docs/CONVENTIONS.md"},
			wantWarning: false,
		},
		{
			name:        ".mindspec/docs/user/** satisfies lane (additive)",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{".mindspec/docs/user/getting-started.md"},
			wantWarning: false,
		},
		{
			name:        ".mindspec/docs/core/USAGE.md satisfies lane (additive)",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{".mindspec/docs/core/USAGE.md"},
			wantWarning: false,
		},
		{
			name:        "no operator-doc touch warns",
			source:      []string{"cmd/mindspec/validate.go"},
			docs:        []string{"docs/domains/workflow/overview.md"},
			wantWarning: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Result{SubCommand: "docs"}
			checkCmdChanges(r, tc.source, tc.docs)

			found := false
			for _, issue := range r.Issues {
				if issue.Name == "cmd-docs" {
					found = true
				}
			}
			if found != tc.wantWarning {
				t.Errorf("warning present = %v, want %v (issues=%+v)", found, tc.wantWarning, r.Issues)
			}
		})
	}
}

// TestValidateDocsWorkingTreeIdiom pins the head=="" diff semantics (PR
// #132 panel C2 medium / mutation M5b): ValidateDocs — and
// ValidateDocsRange with an empty head — MUST use the executor's
// working-tree idiom, ChangedFiles("", base). This is the contract the
// impl-approve gate (internal/approve/impl.go, run from the spec
// worktree) and `mindspec validate docs` rely on; only an explicit head
// switches to the committed range base..head (the per-bead complete
// gate).
func TestValidateDocsWorkingTreeIdiom(t *testing.T) {
	assertSingleDiff := func(t *testing.T, mock *executor.MockExecutor, wantBase, wantHead string) {
		t.Helper()
		calls := mock.CallsTo("ChangedFiles")
		if len(calls) != 1 {
			t.Fatalf("expected exactly 1 ChangedFiles call, got %d", len(calls))
		}
		if calls[0].Args[0] != wantBase || calls[0].Args[1] != wantHead {
			t.Errorf("ChangedFiles(%v, %v), want (%q, %q)", calls[0].Args[0], calls[0].Args[1], wantBase, wantHead)
		}
	}

	t.Run("ValidateDocs diffs working tree vs ref", func(t *testing.T) {
		mock := &executor.MockExecutor{}
		ValidateDocs(t.TempDir(), "BASE", mock)
		assertSingleDiff(t, mock, "", "BASE")
	})

	t.Run("ValidateDocs empty ref defaults to HEAD~1", func(t *testing.T) {
		mock := &executor.MockExecutor{}
		ValidateDocs(t.TempDir(), "", mock)
		assertSingleDiff(t, mock, "", "HEAD~1")
	})

	t.Run("ValidateDocsRange with empty head keeps the working-tree idiom", func(t *testing.T) {
		mock := &executor.MockExecutor{}
		ValidateDocsRange(t.TempDir(), "BASE", "", mock)
		assertSingleDiff(t, mock, "", "BASE")
	})

	t.Run("ValidateDocsRange with explicit head diffs the committed range", func(t *testing.T) {
		mock := &executor.MockExecutor{}
		ValidateDocsRange(t.TempDir(), "FORK", "bead/x-1", mock)
		assertSingleDiff(t, mock, "FORK", "bead/x-1")
	})
}
