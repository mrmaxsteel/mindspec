package validate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
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

	checkInternalPackages(r, nil, root, "", source, docs)
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

	checkInternalPackages(r, nil, root, "", source, docs)
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

	checkInternalPackages(r, nil, root, "", source, docs)

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

// TestValidateDocsErrorsOnInternalDocSkew_Fallback covers the
// post-spec-091 (Req 13) semantics of a manifest-less domain: when
// OWNERSHIP.yaml is absent the domain claims NOTHING, so a source
// change under internal/<domain>/ produces NO internal-docs error —
// the old synthetic "internal/<domain>/**" fallback claim is removed.
func TestValidateDocsErrorsOnInternalDocSkew_Fallback(t *testing.T) {
	root := t.TempDir()
	// Create the domain directory WITHOUT OWNERSHIP.yaml so
	// LoadOwnership returns the claims-nothing Ownership
	// (Source() == "missing").
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "domains", "workflow"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := &Result{SubCommand: "docs"}
	source := []string{"internal/workflow/run.go"}
	docs := []string{"CLAUDE.md"}

	checkInternalPackages(r, nil, root, "", source, docs)

	for _, issue := range r.Issues {
		if issue.Name == "internal-docs" {
			t.Fatalf("manifest-less domain must claim nothing (no internal-docs error); got %+v", issue)
		}
	}
	if r.HasFailures() {
		t.Errorf("expected no failures for manifest-less domain, got %+v", r.Issues)
	}
}

// TestCheckInternalPackages_ZeroDomainsDisclosedDefault pins the
// zero-domains DISCLOSED DEFAULT branch (spec 091 Req 13 disclosure
// obligation, panel V1-1): when NO domain directories exist at all,
// changed internal/<pkg>/ files are still attributed per-package and
// emit a BLOCKING internal-docs error carrying the literal
// "<fallback: internal/<pkg>/**>" marker. No other test reaches
// len(domains)==0 — this coverage is part of the disclosure.
func TestCheckInternalPackages_ZeroDomainsDisclosedDefault(t *testing.T) {
	root := t.TempDir() // no .mindspec/docs/domains/ at all

	r := &Result{SubCommand: "docs"}
	source := []string{"internal/foo/bar.go"}
	docs := []string{"CLAUDE.md"} // not a domain doc

	checkInternalPackages(r, nil, root, "", source, docs)

	var issue *Issue
	for i := range r.Issues {
		if r.Issues[i].Name == "internal-docs" {
			issue = &r.Issues[i]
			break
		}
	}
	if issue == nil {
		t.Fatalf("expected internal-docs error from zero-domains disclosed default, got %+v", r.Issues)
	}
	if issue.Severity != SevError {
		t.Errorf("zero-domains internal-docs severity = %v, want SevError (blocking)", issue.Severity)
	}
	if !strings.Contains(issue.Message, "<fallback: internal/foo/**>") {
		t.Errorf("zero-domains message must carry the literal disclosure marker; got %q", issue.Message)
	}
	if !r.HasFailures() {
		t.Error("result should report failures")
	}
}

// TestPerDomainMarkerNamesManifest pins the spec 091 Req 13 marker
// audit outcome (panel V2-4): the per-domain empty-ManifestPath
// "<fallback: internal/<domain>/**>" marker branch was DEAD after the
// loader fallback removal (attribution requires non-empty Paths,
// which implies a manifest-backed load) and has been DELETED. Every
// per-domain internal-docs message names the manifest path that
// decided ownership and never carries a fallback marker.
func TestPerDomainMarkerNamesManifest(t *testing.T) {
	root := t.TempDir()
	manifest := writeOwnershipFixture(t, root, "workflow", []string{"internal/validate/**"})

	r := &Result{SubCommand: "docs"}
	source := []string{"internal/validate/spec.go"}
	docs := []string{"CLAUDE.md"} // not a domain doc

	checkInternalPackages(r, nil, root, "", source, docs)

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
	if !strings.Contains(issue.Message, manifest) {
		t.Errorf("per-domain message must name manifest %q; got %q", manifest, issue.Message)
	}
	if strings.Contains(issue.Message, "<fallback:") {
		t.Errorf("per-domain message must never carry a fallback marker (deleted as dead code); got %q", issue.Message)
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
	checkInternalPackages(r, nil, root, "", []string{"internal/validate/spec.go"}, nil)
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

// --- Spec 091 Bead 2: source_globs override semantics + unclaimed-source Warn + Req 22(b) nudge ---

// writeSourceGlobsConfig writes .mindspec/config.yaml under root with
// the given source_globs entries (an empty slice writes
// `source_globs: []`) and resets the per-process config cache (the
// Load cache at internal/config/config.go — every test that mutates
// config.yaml on disk must reset it).
func writeSourceGlobsConfig(t *testing.T, root string, globs []string) {
	t.Helper()
	dir := filepath.Join(root, ".mindspec")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body := "source_globs:"
	if len(globs) == 0 {
		body += " []\n"
	} else {
		body += "\n"
		for _, g := range globs {
			body += "  - " + g + "\n"
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	config.ResetCache()
	t.Cleanup(config.ResetCache)
}

// findIssue returns the first issue with the given name, or nil.
func findIssue(r *Result, name string) *Issue {
	for i := range r.Issues {
		if r.Issues[i].Name == name {
			return &r.Issues[i]
		}
	}
	return nil
}

// mockChanged builds an executor whose ChangedFiles returns files.
func mockChanged(files []string) *executor.MockExecutor {
	return &executor.MockExecutor{ChangedFilesResult: files}
}

// TestClassifyChangesWithGlobs_EmptyDelegatesByteIdentically pins the
// HC-7 empty side of the override-semantics gate at the decision
// point: with empty/absent source_globs, classification over a mixed
// fixture (cmd/+internal/ .go files, _test.go, docs, non-Go) is
// EXACTLY what the unchanged isSourceFile-based classifyChanges
// selects.
func TestClassifyChangesWithGlobs_EmptyDelegatesByteIdentically(t *testing.T) {
	files := []string{
		"cmd/mindspec/validate.go",
		"internal/validate/spec.go",
		"internal/validate/spec_test.go",
		"docs/domains/workflow/interfaces.md",
		".mindspec/docs/core/MODES.md",
		"scripts/build.sh",
		"go.mod",
	}

	wantSource, wantDocs := classifyChanges(files)

	for _, globs := range [][]string{nil, {}} {
		gotSource, gotDocs := classifyChangesWithGlobs(files, globs)
		if !reflect.DeepEqual(gotSource, wantSource) {
			t.Errorf("globs=%v: source = %v, want isSourceFile selection %v", globs, gotSource, wantSource)
		}
		if !reflect.DeepEqual(gotDocs, wantDocs) {
			t.Errorf("globs=%v: docs = %v, want %v", globs, gotDocs, wantDocs)
		}
	}

	// Explicit pin of the expected selection so a mutation of
	// classifyChanges itself cannot satisfy the identity vacuously.
	if !reflect.DeepEqual(wantSource, []string{"cmd/mindspec/validate.go", "internal/validate/spec.go"}) {
		t.Errorf("isSourceFile selection drifted: %v", wantSource)
	}
}

// TestValidateDocs_EmptyGlobsPreservesPre091Outcome pins the HC-7
// empty side end-to-end: with NO source_globs declared, ValidateDocs
// over a mixed fixture produces the identical blocking outcome the
// pre-091 gate produced (zero-domains disclosed default fires on the
// internal/ file; HasFailures true), and no unclaimed-source Warn
// exists anywhere.
func TestValidateDocs_EmptyGlobsPreservesPre091Outcome(t *testing.T) {
	root := t.TempDir() // no config.yaml, no domains dir
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	changed := []string{
		"internal/foo/bar.go",
		"internal/foo/bar_test.go",
		"scripts/build.sh",
	}
	r := ValidateDocs(root, "HEAD~1", mockChanged(changed))

	// Pre-091 outcome: source with no docs → doc-sync error + the
	// zero-domains internal-docs error with the disclosure marker.
	if !r.HasFailures() {
		t.Fatalf("expected failures identical to pre-091 outcome, got %+v", r.Issues)
	}
	if findIssue(r, "doc-sync") == nil {
		t.Errorf("expected doc-sync error (pre-091 outcome), got %+v", r.Issues)
	}
	internalDocs := findIssue(r, "internal-docs")
	if internalDocs == nil || !strings.Contains(internalDocs.Message, "<fallback: internal/foo/**>") {
		t.Errorf("expected zero-domains internal-docs error with disclosure marker, got %+v", r.Issues)
	}
	if findIssue(r, "unclaimed-source") != nil {
		t.Errorf("unclaimed-source must be DISABLED while source_globs is empty, got %+v", r.Issues)
	}
}

// TestValidateDocs_PopulatedGlobsFullyOverride pins the populated side
// of the override-semantics gate, two-sided (never union):
// with source_globs: [pkg/**] —
//
//	(a) pkg/foo.js IS source (the built-in .go-only rule does NOT
//	    apply), so the doc-sync lane errors on a source-only diff;
//	(b) internal/foo/bar.go is NOT source (the built-in
//	    cmd/+internal/ rule does NOT apply), so the same lanes stay
//	    silent.
func TestValidateDocs_PopulatedGlobsFullyOverride(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, []string{"pkg/**"})

	// (a) glob-matched non-Go file IS source.
	rA := ValidateDocs(root, "HEAD~1", mockChanged([]string{"pkg/foo.js"}))
	if findIssue(rA, "doc-sync") == nil {
		t.Errorf("pkg/foo.js must classify as source under override (doc-sync error expected), got %+v", rA.Issues)
	}

	// (b) built-in-acceptable file is NOT source when no glob matches.
	rB := ValidateDocs(root, "HEAD~1", mockChanged([]string{"internal/foo/bar.go"}))
	if findIssue(rB, "doc-sync") != nil || findIssue(rB, "internal-docs") != nil {
		t.Errorf("internal/foo/bar.go must NOT classify as source under [pkg/**] override (never union), got %+v", rB.Issues)
	}
	if rB.HasFailures() {
		t.Errorf("non-matching diff must not fail under override, got %+v", rB.Issues)
	}
}

// TestClassifyChangesWithGlobs_OverrideIsTotal pins the full-override
// contract at the decision point: under a populated glob set the
// built-in classifier contributes NOTHING — a union mutant (globs OR
// isSourceFile) or a filtered mutant (globs AND the built-in
// _test.go/.go restrictions) both fail this test. Doc-file precedence
// is preserved under override.
func TestClassifyChangesWithGlobs_OverrideIsTotal(t *testing.T) {
	files := []string{
		"pkg/foo.js",                   // glob-matched non-Go → source (built-in would reject)
		"pkg/foo_test.go",              // glob-matched _test.go → source (built-in exclusion bypassed)
		"internal/foo/bar.go",          // built-in would accept; no glob → NOT source
		"cmd/mindspec/main.go",         // built-in would accept; no glob → NOT source
		".mindspec/docs/core/MODES.md", // doc precedence preserved under override
		"go.mod",                       // matches nothing → neither
	}

	source, docs := classifyChangesWithGlobs(files, []string{"pkg/**", ".mindspec/**"})

	wantSource := []string{"pkg/foo.js", "pkg/foo_test.go"}
	if !reflect.DeepEqual(source, wantSource) {
		t.Errorf("override source = %v, want %v (full override, never union)", source, wantSource)
	}
	wantDocs := []string{".mindspec/docs/core/MODES.md"}
	if !reflect.DeepEqual(docs, wantDocs) {
		t.Errorf("override docs = %v, want %v (isDocFile precedence under override)", docs, wantDocs)
	}
}

// TestUnclaimedSourceWarn_StateReport covers spec AC "unclaimed-source
// Warn" cases (a)/(b)/(c): with source_globs: [internal/**] and a diff
// touching internal/contextpack/foo.go, the Warn fires regardless of
// the context-system domain's Source() state, and the mechanical state
// report annotates the domain with the derived state.
func TestUnclaimedSourceWarn_StateReport(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(t *testing.T, root string)
		wantState string
	}{
		{
			// (a) domain dir exists, NO manifest → "missing".
			name: "missing manifest",
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "domains", "context-system"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "context-system=missing",
		},
		{
			// (b) empty-stub manifest (paths: []) → "empty-stub".
			name: "empty-stub manifest",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, ".mindspec", "docs", "domains", "context-system")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "OWNERSHIP.yaml"), []byte("paths: []\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "context-system=empty-stub",
		},
		{
			// (c) hand-populated manifest pointing at an EXISTING
			// other dir → "manifest" (honesty note: the
			// wrong-but-resolving manifest itself is flagged by
			// nothing; the misclaim surfaces only as this Warn on the
			// orphaned file, and the domain is reported — never
			// presented as a populate candidate).
			name: "populated manifest claiming elsewhere",
			setup: func(t *testing.T, root string) {
				writeOwnershipFixture(t, root, "context-system", []string{"internal/something-else/**"})
				other := filepath.Join(root, "internal", "something-else")
				if err := os.MkdirAll(other, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(other, "x.go"), []byte("package somethingelse\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantState: "context-system=manifest",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeSourceGlobsConfig(t, root, []string{"internal/**"})
			tc.setup(t, root)

			r := ValidateDocs(root, "HEAD~1", mockChanged([]string{"internal/contextpack/foo.go"}))

			issue := findIssue(r, "unclaimed-source")
			if issue == nil {
				t.Fatalf("expected unclaimed-source Warn, got %+v", r.Issues)
			}
			if issue.Severity != SevWarning {
				t.Errorf("unclaimed-source severity = %v, want SevWarning (advisory)", issue.Severity)
			}
			if !strings.Contains(issue.Message, "internal/contextpack/foo.go") {
				t.Errorf("message must list the unclaimed file, got %q", issue.Message)
			}
			if !strings.Contains(issue.Message, tc.wantState) {
				t.Errorf("state report must annotate %q, got %q", tc.wantState, issue.Message)
			}
		})
	}
}

// TestUnclaimedSourceWarn_DefaultHint pins the (c) hint text for the
// not-all-populated case: the message carries the doctor --fix +
// ownership populate remedy chain verbatim.
func TestUnclaimedSourceWarn_DefaultHint(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, []string{"internal/**"})
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "docs", "domains", "context-system"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{"internal/contextpack/foo.go"}))

	issue := findIssue(r, "unclaimed-source")
	if issue == nil {
		t.Fatalf("expected unclaimed-source Warn, got %+v", r.Issues)
	}
	want := "run 'mindspec doctor --fix' to scaffold missing manifests, then 'mindspec ownership populate <domain>' to populate one"
	if !strings.Contains(issue.Message, want) {
		t.Errorf("message must carry the spec hint %q, got %q", want, issue.Message)
	}
}

// TestUnclaimedSourceWarn_AllPopulatedHintVariant covers the spec AC
// "unclaimed-source with zero unpopulated domains names the right
// remedies": when EVERY domain's Source() is "manifest", the message
// says so explicitly, hints widening (ownership populate) or
// `mindspec domain add`, and does NOT hint `doctor --fix` (which would
// scaffold nothing in that state).
func TestUnclaimedSourceWarn_AllPopulatedHintVariant(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, []string{"internal/**"})
	writeOwnershipFixture(t, root, "alpha", []string{"internal/alpha/**"})
	writeOwnershipFixture(t, root, "beta", []string{"internal/beta/**"})

	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{"internal/orphan/foo.go"}))

	issue := findIssue(r, "unclaimed-source")
	if issue == nil {
		t.Fatalf("expected unclaimed-source Warn, got %+v", r.Issues)
	}
	if !strings.Contains(issue.Message, "alpha=manifest") || !strings.Contains(issue.Message, "beta=manifest") {
		t.Errorf("state report must annotate every domain as manifest, got %q", issue.Message)
	}
	if !strings.Contains(issue.Message, "no unpopulated candidates exist") {
		t.Errorf("message must state explicitly that no unpopulated domains exist, got %q", issue.Message)
	}
	if !strings.Contains(issue.Message, "mindspec ownership populate") {
		t.Errorf("message must hint widening via ownership populate, got %q", issue.Message)
	}
	if !strings.Contains(issue.Message, "mindspec domain add") {
		t.Errorf("message must hint mindspec domain add, got %q", issue.Message)
	}
	if strings.Contains(issue.Message, "doctor --fix") {
		t.Errorf("all-populated variant must NOT hint doctor --fix (it would do nothing), got %q", issue.Message)
	}
}

// TestUnclaimedSourceWarn_NeverBlocks pins the spec AC "Warn does NOT
// block the gate": unclaimed source files with no other doc-sync error
// leave HasFailures() false.
func TestUnclaimedSourceWarn_NeverBlocks(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, []string{"pkg/**"})

	// pkg/ source + a doc file: the doc-sync lane is satisfied, no
	// blocking lane fires, only the advisory Warn remains.
	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{"pkg/foo.js", "docs/notes.md"}))

	issue := findIssue(r, "unclaimed-source")
	if issue == nil {
		t.Fatalf("expected unclaimed-source Warn, got %+v", r.Issues)
	}
	if r.HasFailures() {
		t.Errorf("unclaimed-source is advisory and must never block, got %+v", r.Issues)
	}
}

// TestUnclaimedSourceWarn_PurelyDocsDiff pins the spec AC "Warn does
// NOT fire for purely-docs diffs": a diff touching only
// .mindspec/docs/** never fires the Warn even with populated globs
// that would match those paths (doc-file precedence).
func TestUnclaimedSourceWarn_PurelyDocsDiff(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, []string{"internal/**", ".mindspec/**"})

	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{
		".mindspec/docs/core/MODES.md",
		".mindspec/docs/domains/workflow/overview.md",
	}))

	if findIssue(r, "unclaimed-source") != nil {
		t.Errorf("purely-docs diff must not fire unclaimed-source, got %+v", r.Issues)
	}
}

// TestUnclaimedSourceWarn_DoubleReportAtZeroDomains pins the SPECIFIED
// double-report (Req 16 — not a bug, must not be "fixed"): populated
// globs + zero domain directories fires BOTH the blocking zero-domains
// legacy branch (with the disclosure marker, governing pass/fail) AND
// the advisory unclaimed-source Warn on the same files; the
// all-"manifest" hint variant is vacuously triggered and already
// includes `mindspec domain add`.
func TestUnclaimedSourceWarn_DoubleReportAtZeroDomains(t *testing.T) {
	root := t.TempDir() // no domains dir at all
	writeSourceGlobsConfig(t, root, []string{"internal/**"})

	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{"internal/foo/bar.go"}))

	blocking := findIssue(r, "internal-docs")
	if blocking == nil || blocking.Severity != SevError {
		t.Fatalf("zero-domains blocking branch must still fire (governs pass/fail), got %+v", r.Issues)
	}
	if !strings.Contains(blocking.Message, "<fallback: internal/foo/**>") {
		t.Errorf("blocking error must keep the disclosure marker, got %q", blocking.Message)
	}

	warn := findIssue(r, "unclaimed-source")
	if warn == nil {
		t.Fatalf("advisory Warn must fire alongside the blocking branch (specified double-report), got %+v", r.Issues)
	}
	if !strings.Contains(warn.Message, "internal/foo/bar.go") {
		t.Errorf("Warn must name the same file, got %q", warn.Message)
	}
	if !strings.Contains(warn.Message, "mindspec domain add") {
		t.Errorf("vacuous all-manifest variant must hint mindspec domain add, got %q", warn.Message)
	}
	if !r.HasFailures() {
		t.Error("pass/fail must be governed by the blocking branch")
	}
}

// TestUnclaimedSourceWarn_ClaimedFilesSilent: a glob-matched file that
// a domain's resolved paths DOES claim fires nothing (companion case —
// the Warn is about unclaimed files only).
func TestUnclaimedSourceWarn_ClaimedFilesSilent(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, []string{"internal/**"})
	writeOwnershipFixture(t, root, "workflow", []string{"internal/validate/**"})

	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{
		"internal/validate/spec.go",
		".mindspec/docs/domains/workflow/interfaces.md",
	}))

	if findIssue(r, "unclaimed-source") != nil {
		t.Errorf("claimed file must not fire unclaimed-source, got %+v", r.Issues)
	}
	if r.HasFailures() {
		t.Errorf("expected clean result, got %+v", r.Issues)
	}
}

// TestUnclaimedSourceWarn_RespectsExclude: a file matching a domain's
// paths but subtracted by its exclude list is NOT claimed by that
// domain, so the Warn fires (resolved paths = paths minus exclude).
func TestUnclaimedSourceWarn_RespectsExclude(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, []string{"internal/**"})
	dir := filepath.Join(root, ".mindspec", "docs", "domains", "workflow")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "paths:\n  - internal/**\nexclude:\n  - internal/contextpack/**\n"
	if err := os.WriteFile(filepath.Join(dir, "OWNERSHIP.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{"internal/contextpack/foo.go"}))

	issue := findIssue(r, "unclaimed-source")
	if issue == nil {
		t.Fatalf("excluded file is unclaimed — Warn must fire, got %+v", r.Issues)
	}
	if !strings.Contains(issue.Message, "internal/contextpack/foo.go") {
		t.Errorf("Warn must name the excluded-thus-unclaimed file, got %q", issue.Message)
	}
}

// TestMissingSourceGlobsNudge_RecursStatelessly covers the spec AC
// "migration-status line recurs statelessly" (Req 22(b), validator
// half): with source_globs absent, the warning-severity
// missing-source-globs issue is on the *Result on EVERY invocation,
// and no marker/state file is created anywhere under root (HC-2).
func TestMissingSourceGlobsNudge_RecursStatelessly(t *testing.T) {
	root := t.TempDir() // no config.yaml at all (the brownfield state)
	config.ResetCache()
	t.Cleanup(config.ResetCache)

	exec := mockChanged([]string{"internal/foo/bar.go", "docs/notes.md"})

	for run := 1; run <= 2; run++ {
		r := ValidateDocs(root, "HEAD~1", exec)
		issue := findIssue(r, "missing-source-globs")
		if issue == nil {
			t.Fatalf("run %d: expected missing-source-globs issue on every invocation, got %+v", run, r.Issues)
		}
		if issue.Severity != SevWarning {
			t.Errorf("run %d: severity = %v, want SevWarning", run, issue.Severity)
		}
		if !strings.Contains(issue.Message, ".mindspec/config.yaml") {
			t.Errorf("run %d: message must name .mindspec/config.yaml, got %q", run, issue.Message)
		}
		if !strings.Contains(issue.Message, "built-in default") ||
			!strings.Contains(issue.Message, ".go under cmd/ and internal/, excluding _test.go") {
			t.Errorf("run %d: message must DISCLOSE the built-in default, got %q", run, issue.Message)
		}
		if !strings.Contains(issue.Message, "mindspec source populate") {
			t.Errorf("run %d: message must hint 'mindspec source populate', got %q", run, issue.Message)
		}
	}

	// HC-2: stateless by construction — validation created no
	// marker/state file of any kind.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("validator must persist NO marker/state file (HC-2); found %v", entries)
	}
}

// TestMissingSourceGlobsNudge_EmptyListAlsoFires: an explicit
// `source_globs: []` collapses to the same unset state (Req 18/22(b)).
func TestMissingSourceGlobsNudge_EmptyListAlsoFires(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, nil) // writes source_globs: []

	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{"internal/foo/bar.go"}))
	if findIssue(r, "missing-source-globs") == nil {
		t.Errorf("explicit empty list must fire the nudge, got %+v", r.Issues)
	}
}

// TestMissingSourceGlobsNudge_AbsentWhenPopulated: with populated
// source_globs the nudge issue is absent.
func TestMissingSourceGlobsNudge_AbsentWhenPopulated(t *testing.T) {
	root := t.TempDir()
	writeSourceGlobsConfig(t, root, []string{"internal/**"})

	r := ValidateDocs(root, "HEAD~1", mockChanged([]string{"internal/foo/bar.go"}))
	if findIssue(r, "missing-source-globs") != nil {
		t.Errorf("populated source_globs must not fire the nudge, got %+v", r.Issues)
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
		ValidateDocsRange(t.TempDir(), "BASE", "", "", mock)
		assertSingleDiff(t, mock, "", "BASE")
	})

	t.Run("ValidateDocsRange with explicit head diffs the committed range", func(t *testing.T) {
		mock := &executor.MockExecutor{}
		ValidateDocsRange(t.TempDir(), "FORK", "bead/x-1", "", mock)
		assertSingleDiff(t, mock, "FORK", "bead/x-1")
	})
}

// TestInternalDocsHintLayoutAware — spec 122 AC-9 (R4a), the doc-sync
// half: the internal-docs "no doc updates under .../" hint must print
// the domains root that ACTUALLY resolves in the operator's workspace.
// A FLATTENED workspace (.mindspec/domains/ present, .mindspec/docs/
// domains/ absent) prints .mindspec/domains/... with no
// .mindspec/docs/domains substring; a PRE-flatten (canonical) workspace
// prints .mindspec/docs/domains/....
func TestInternalDocsHintLayoutAware(t *testing.T) {
	t.Run("flattened", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, ".mindspec", "domains", "workflow")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "OWNERSHIP.yaml"), []byte("paths:\n  - internal/validate/**\n"), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		r := &Result{SubCommand: "docs"}
		source := []string{"internal/validate/spec.go"}
		docs := []string{"CLAUDE.md"}

		checkInternalPackages(r, nil, root, "", source, docs)

		var msg string
		for _, issue := range r.Issues {
			if issue.Name == "internal-docs" {
				msg = issue.Message
			}
		}
		if msg == "" {
			t.Fatalf("expected internal-docs error, got %+v", r.Issues)
		}
		if !strings.Contains(msg, ".mindspec/domains/workflow") {
			t.Errorf("expected the flat domains root in the hint, got: %q", msg)
		}
		if strings.Contains(msg, ".mindspec/docs/domains") {
			t.Errorf("flat workspace hint must NOT contain the canonical literal, got: %q", msg)
		}
	})

	t.Run("pre-flatten (canonical)", func(t *testing.T) {
		root := t.TempDir()
		writeOwnershipFixture(t, root, "workflow", []string{"internal/validate/**"})

		r := &Result{SubCommand: "docs"}
		source := []string{"internal/validate/spec.go"}
		docs := []string{"CLAUDE.md"}

		checkInternalPackages(r, nil, root, "", source, docs)

		var msg string
		for _, issue := range r.Issues {
			if issue.Name == "internal-docs" {
				msg = issue.Message
			}
		}
		if msg == "" {
			t.Fatalf("expected internal-docs error, got %+v", r.Issues)
		}
		if !strings.Contains(msg, ".mindspec/docs/domains/workflow") {
			t.Errorf("expected the canonical domains root in the hint, got: %q", msg)
		}
	})
}

// hintLiteralGuardBannedSubstrings are the hard-coded domains-root
// literals spec 122 AC-10's sweep guard forbids inside any
// AddError/AddWarning operator-facing format string in this package:
// the flat path, the canonical pre-flatten path, and the legacy path.
// Every gate message must render its root through domainsRootLabel
// (hint_root.go) instead of embedding one of these literally.
var hintLiteralGuardBannedSubstrings = []string{
	".mindspec/docs/domains",
	".mindspec/domains",
	"docs/domains",
}

// scanHintLiterals parses a single Go source (filename is cosmetic
// metadata for parser.ParseFile/position reporting; src holds the
// actual bytes — either a []byte, string, or nil to read from disk) and
// returns one description per AddError/AddWarning CALL SITE whose
// argument subtree contains (a) a string literal directly embedding a
// banned domains-root literal (e.g. the pre-fix divergence.go pattern:
// a literal baked straight into a fmt.Sprintf format string), or (b) a
// `<pkg>.Join(...)` call (e.g. filepath.Join/path.Join) whose LEADING
// string-literal arguments, joined with "/", reconstruct a banned
// literal — the pre-fix docsync.go pattern: `filepath.Join(".mindspec",
// "docs", "domains", p)`, where no SINGLE argument embeds the full
// literal but the leading three do once joined; a trailing non-literal
// argument (the dynamic domain/package name) does not defeat this,
// since the guard only requires the ROOT PREFIX to be literal (spec 122
// AC-10). Only call-site subtrees are inspected — not the whole file —
// so a doc COMMENT explaining the three-tier precedence (e.g.
// hint_root.go's own package doc) is deliberately exempt: go/ast's
// expression tree never sees comments. This is a pure SYNTAX parse
// (go/parser, no type-checking), so a throwaway sample source doesn't
// need real Result/fmt scaffolding to resolve — only to parse.
func scanHintLiterals(t *testing.T, filename string, src interface{}) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", filename, err)
	}

	var findings []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "AddError" && sel.Sel.Name != "AddWarning") {
			return true
		}
		// Inspect the WHOLE call subtree — including any nested
		// fmt.Sprintf and its format-string literal, plus any nested
		// <pkg>.Join(...) call — for a banned substring.
		ast.Inspect(call, func(inner ast.Node) bool {
			switch v := inner.(type) {
			case *ast.BasicLit:
				if v.Kind != token.STRING {
					return true
				}
				value, uqErr := strconv.Unquote(v.Value)
				if uqErr != nil {
					return true
				}
				for _, banned := range hintLiteralGuardBannedSubstrings {
					if strings.Contains(value, banned) {
						findings = append(findings, fmt.Sprintf("%s: %s call embeds hard-coded domains-root literal %q", fset.Position(call.Pos()), sel.Sel.Name, banned))
					}
				}
			case *ast.CallExpr:
				joinSel, ok := v.Fun.(*ast.SelectorExpr)
				if !ok || joinSel.Sel.Name != "Join" {
					return true
				}
				var segs []string
				for _, arg := range v.Args {
					lit, ok := arg.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						break // stop at the first non-literal (dynamic) segment
					}
					seg, uqErr := strconv.Unquote(lit.Value)
					if uqErr != nil {
						break
					}
					segs = append(segs, seg)
				}
				if len(segs) == 0 {
					return true
				}
				joined := strings.Join(segs, "/")
				for _, banned := range hintLiteralGuardBannedSubstrings {
					if strings.Contains(joined, banned) {
						findings = append(findings, fmt.Sprintf("%s: %s call's Join(%q...) reconstructs hard-coded domains-root literal %q", fset.Position(call.Pos()), sel.Sel.Name, joined, banned))
					}
				}
			}
			return true
		})
		return true
	})
	return findings
}

// TestHintLiteralSweep_PackageClean is spec 122 AC-10's guard proper:
// every non-test .go file in this package must have ZERO
// AddError/AddWarning call sites embedding a hard-coded domains-root
// literal — every gate message renders its root through
// domainsRootLabel (hint_root.go) instead (R4a).
func TestHintLiteralSweep_PackageClean(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected to find package source files — glob pattern or cwd is wrong")
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		findings := scanHintLiterals(t, f, nil)
		if len(findings) > 0 {
			t.Errorf("%s: hard-coded domains-root literal(s) in gate message(s):\n%s", f, strings.Join(findings, "\n"))
		}
	}
}

// TestHintLiteralSweep_CatchesReintroducedLiteral is AC-10's
// fixture-of-the-guard: a deliberately reintroduced hard-coded
// domains-root literal in a test-local sample source (never written to
// a real file, never compiled into the package) must turn the sweep
// guard's scanner red, proving it actually detects the class of
// regression it exists to catch.
func TestHintLiteralSweep_CatchesReintroducedLiteral(t *testing.T) {
	const sample = `package validate

import "fmt"

func reintroducedLiteralSample(r *Result) {
	r.AddError("adr-divergence-unowned", fmt.Sprintf(
		"create a new domain dir at .mindspec/docs/domains/%s/OWNERSHIP.yaml", "example"))
}
`
	findings := scanHintLiterals(t, "sample.go", sample)
	if len(findings) == 0 {
		t.Fatal("expected the guard to flag the deliberately reintroduced literal, got none — the guard is not detecting the class it exists to catch")
	}
}

// TestHintLiteralSweep_CatchesReintroducedJoinLiteral is a second
// fixture-of-the-guard: the pre-fix REAL shape this spec closed in
// docsync.go was not a single embedded literal but
// `filepath.Join(".mindspec", "docs", "domains", domain)` — no ONE
// argument contains the full literal, but the leading three
// reconstruct it once joined, with a trailing DYNAMIC segment (the
// domain/package name) that must not defeat detection. Confirms the
// guard's Join-reconstruction leg (not just its direct-literal leg)
// actually catches the class of regression that shipped here.
func TestHintLiteralSweep_CatchesReintroducedJoinLiteral(t *testing.T) {
	const sample = `package validate

import (
	"fmt"
	"path/filepath"
)

func reintroducedJoinLiteralSample(r *Result, domain string) {
	r.AddError("internal-docs", fmt.Sprintf(
		"no doc updates under %s/", filepath.Join(".mindspec", "docs", "domains", domain)))
}
`
	findings := scanHintLiterals(t, "sample.go", sample)
	if len(findings) == 0 {
		t.Fatal("expected the guard to flag the reintroduced filepath.Join-reconstructed literal, got none — the guard is not detecting the class it exists to catch")
	}
}
