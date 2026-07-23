package domain

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// setupFlatRoot returns a temp dir bootstrapped as a FLAT tree — a bare
// .mindspec/domains and .mindspec/specs directory, matching a real
// `mindspec init` greenfield tree (spec 106 AC4: new projects are born
// flat). DomainDir/ContextMapPath resolve under .mindspec/ directly.
func setupFlatRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	prev := populatePromptWriter
	populatePromptWriter = io.Discard
	t.Cleanup(func() { populatePromptWriter = prev })

	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "domains"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".mindspec", "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	return root
}

func setupTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Silence the populate prompt Add prints (spec 091 Req 9) so
	// unrelated tests stay quiet; capturePopulatePrompt re-overrides
	// this for the tests that assert on the prompt.
	prev := populatePromptWriter
	populatePromptWriter = io.Discard
	t.Cleanup(func() { populatePromptWriter = prev })

	// Create marker
	os.Mkdir(filepath.Join(root, ".mindspec"), 0755)

	// Create domains dir
	os.MkdirAll(filepath.Join(root, "docs", "domains"), 0755)

	// Create context map
	cm := `# MindSpec Context Map

## Bounded Contexts

### Core

**Owns**: CLI entry point, workspace resolution.

**Domain docs**: [` + "`" + `docs/domains/core/` + "`" + `](domains/core/overview.md)

---

## Relationships
`
	os.WriteFile(filepath.Join(root, "docs", "context-map.md"), []byte(cm), 0644)

	return root
}

func TestAddCreatesTemplateFiles(t *testing.T) {
	root := setupTestRoot(t)

	err := Add(root, "payments")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	domainDir := filepath.Join(root, "docs", "domains", "payments")
	files := []string{"overview.md", "architecture.md", "interfaces.md", "runbook.md"}

	for _, f := range files {
		path := filepath.Join(domainDir, f)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s not created: %v", f, err)
			continue
		}
		content := string(data)
		if !strings.Contains(content, "# Payments Domain") {
			t.Errorf("%s missing title, got:\n%s", f, content)
		}
	}
}

func TestAddOverviewHasCorrectSections(t *testing.T) {
	root := setupTestRoot(t)

	if err := Add(root, "billing"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "docs", "domains", "billing", "overview.md"))
	content := string(data)

	sections := []string{
		"## What This Domain Owns",
		"## Boundaries",
		"## Key Files",
		"## Current State",
	}
	for _, s := range sections {
		if !strings.Contains(content, s) {
			t.Errorf("overview.md missing section %q", s)
		}
	}
}

func TestAddIdempotencyGuard(t *testing.T) {
	root := setupTestRoot(t)

	// First add succeeds
	if err := Add(root, "payments"); err != nil {
		t.Fatalf("first Add() error: %v", err)
	}

	// Second add fails
	err := Add(root, "payments")
	if err == nil {
		t.Fatal("expected error for existing domain, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddInvalidName(t *testing.T) {
	root := setupTestRoot(t)

	tests := []string{"123bad", "Bad-Name", "has spaces", "-leading-dash", "UPPER"}
	for _, name := range tests {
		err := Add(root, name)
		if err == nil {
			t.Errorf("expected error for name %q, got nil", name)
		}
		if !strings.Contains(err.Error(), "invalid domain name") {
			t.Errorf("expected 'invalid domain name' error for %q, got: %v", name, err)
		}
	}
}

func TestAddUpdatesContextMap(t *testing.T) {
	root := setupTestRoot(t)

	if err := Add(root, "payments"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "context-map.md"))
	if err != nil {
		t.Fatalf("reading context map: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "### Payments") {
		t.Error("context map missing ### Payments heading")
	}
	if !strings.Contains(content, "**Owns**: _(fill in)_") {
		t.Error("context map missing Owns placeholder")
	}
	if !strings.Contains(content, "docs/domains/payments/") {
		t.Error("context map missing domain docs link")
	}

	// Entry should appear before the --- separator
	sepIdx := strings.Index(content, "## Relationships")
	paymentsIdx := strings.Index(content, "### Payments")
	if sepIdx >= 0 && paymentsIdx >= sepIdx {
		t.Error("payments entry should appear before ## Relationships")
	}
}

func TestAddHyphenatedName(t *testing.T) {
	root := setupTestRoot(t)

	if err := Add(root, "my-cool-domain"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "docs", "domains", "my-cool-domain", "overview.md"))
	if !strings.Contains(string(data), "# My-Cool-Domain Domain") {
		t.Errorf("expected title-cased hyphenated name, got:\n%s", string(data))
	}

	cm, _ := os.ReadFile(filepath.Join(root, "docs", "context-map.md"))
	if !strings.Contains(string(cm), "### My-Cool-Domain") {
		t.Error("context map missing title-cased heading for hyphenated domain")
	}
}

// capturePopulatePrompt redirects the populate-prompt writer to a
// buffer for the duration of the test.
func capturePopulatePrompt(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := populatePromptWriter
	populatePromptWriter = &buf
	t.Cleanup(func() { populatePromptWriter = prev })
	return &buf
}

func TestAddScaffoldsOwnershipStub(t *testing.T) {
	root := setupTestRoot(t)
	capturePopulatePrompt(t)

	if err := Add(root, "payments"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "docs", "domains", "payments", "OWNERSHIP.yaml"))
	if err != nil {
		t.Fatalf("OWNERSHIP.yaml not created: %v", err)
	}
	content := string(data)

	// Req 8 stub: empty paths body, `domain add` comment variant.
	if !strings.Contains(content, "paths: []") {
		t.Errorf("OWNERSHIP.yaml missing empty `paths: []` stub:\n%s", content)
	}
	if !strings.Contains(content, "# Auto-generated by mindspec domain add payments on ") {
		t.Errorf("OWNERSHIP.yaml comment missing `domain add` command variant:\n%s", content)
	}
	// ZFC: the framework must not pre-fill a claim for the new domain.
	if strings.Contains(content, "internal/payments/**") {
		t.Errorf("OWNERSHIP.yaml contains framework-proposed glob (ZFC violation):\n%s", content)
	}
}

func TestAddPrintsPopulatePrompt(t *testing.T) {
	root := setupTestRoot(t)
	buf := capturePopulatePrompt(t)

	if err := Add(root, "payments"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, ".mindspec/domains/payments/OWNERSHIP.yaml") {
		t.Errorf("populate prompt not printed (missing manifest path):\n%s", out)
	}
	if !strings.Contains(out, "The framework deliberately provides no pattern hints") {
		t.Errorf("populate prompt not printed (missing ZFC sentence):\n%s", out)
	}
}

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"payments", "Payments"},
		{"my-domain", "My-Domain"},
		{"a-b-c", "A-B-C"},
		{"context-system", "Context-System"},
	}
	for _, tt := range tests {
		got := titleCase(tt.input)
		if got != tt.want {
			t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ─── spec 123 R2/AC-2: legacy partial-state convergence ────────────────────

// TestAddConvergesLegacyState_MissingContextMap pins AC-2: the EXACT #207
// aftermath — domains/alpha/ fully scaffolded (all 4 templates +
// OWNERSHIP.yaml present) but context-map.md entirely ABSENT. `domain add
// alpha` must exit 0, create the skeleton, backfill the ### Alpha entry,
// and leave the five existing domain files byte-identical. A second re-run
// refuses "already exists" and leaves the context map byte-identical (no
// duplicate entry). RED on pre-spec-123 main: the dir-exists refusal fired
// before any backfill could run.
func TestAddConvergesLegacyState_MissingContextMap(t *testing.T) {
	root := setupFlatRoot(t)

	domainDir := filepath.Join(root, ".mindspec", "domains", "alpha")
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"overview.md":     "# Alpha Domain — Overview\ncustom overview\n",
		"architecture.md": "# Alpha Domain — Architecture\ncustom architecture\n",
		"interfaces.md":   "# Alpha Domain — Interfaces\ncustom interfaces\n",
		"runbook.md":      "# Alpha Domain — Runbook\ncustom runbook\n",
		"OWNERSHIP.yaml":  "paths: [\"internal/alpha/**\"]\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(domainDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// context-map.md is entirely ABSENT — the exact #207 aftermath.

	if err := Add(root, "alpha"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	cmPath := filepath.Join(root, ".mindspec", "context-map.md")
	cmData, err := os.ReadFile(cmPath)
	if err != nil {
		t.Fatalf("context-map.md not created: %v", err)
	}
	if !strings.Contains(string(cmData), "### Alpha") {
		t.Errorf("context map missing ### Alpha entry:\n%s", cmData)
	}

	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(domainDir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s was modified by backfill; want:\n%s\ngot:\n%s", name, want, got)
		}
	}

	cmAfterFirst, _ := os.ReadFile(cmPath)

	// Second re-run: fully scaffolded AND mapped now — refuses.
	err = Add(root, "alpha")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' on re-run, got: %v", err)
	}
	cmAfterSecond, _ := os.ReadFile(cmPath)
	if string(cmAfterFirst) != string(cmAfterSecond) {
		t.Errorf("context map changed on refused re-run (duplicate entry?); before:\n%s\nafter:\n%s",
			cmAfterFirst, cmAfterSecond)
	}
}

// TestAddBackfillsMissingStandardFiles pins AC-2b: domains/alpha/ present
// but MISSING some standard files (runbook.md and OWNERSHIP.yaml absent,
// others present) and no context-map entry. `domain add alpha` must exit 0,
// write the missing standard files, backfill the ### Alpha entry, and leave
// the already-present files byte-identical. RED on pre-spec-123 main:
// scaffold.go's dir-exists refusal fired before any create-if-missing could
// run, so missing files were never backfilled.
func TestAddBackfillsMissingStandardFiles(t *testing.T) {
	root := setupFlatRoot(t)

	domainDir := filepath.Join(root, ".mindspec", "domains", "alpha")
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		t.Fatal(err)
	}
	present := map[string]string{
		"overview.md":     "# Alpha Domain — Overview\ncustom\n",
		"architecture.md": "# Alpha Domain — Architecture\ncustom\n",
		"interfaces.md":   "# Alpha Domain — Interfaces\ncustom\n",
	}
	for name, content := range present {
		if err := os.WriteFile(filepath.Join(domainDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// runbook.md, OWNERSHIP.yaml, and context-map.md are deliberately absent.

	if err := Add(root, "alpha"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	for name, want := range present {
		got, err := os.ReadFile(filepath.Join(domainDir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s modified during backfill; want:\n%s\ngot:\n%s", name, want, got)
		}
	}

	for _, name := range []string{"runbook.md", "OWNERSHIP.yaml"} {
		if _, err := os.Stat(filepath.Join(domainDir, name)); err != nil {
			t.Errorf("%s not backfilled: %v", name, err)
		}
	}

	cmData, err := os.ReadFile(filepath.Join(root, ".mindspec", "context-map.md"))
	if err != nil {
		t.Fatalf("context-map.md not created: %v", err)
	}
	if !strings.Contains(string(cmData), "### Alpha") {
		t.Errorf("context map missing ### Alpha entry:\n%s", cmData)
	}
}

// TestScaffoldMappedCheckIsSharedHelper is the AC-4 anti-drift identity
// pin: Add's convergence-check seam var must still point at the exact
// exported HasEntry — the SAME helper doctor's unmapped-domain detection
// consumes (its own mirrored seam var, docsMappedCheck, is pinned by
// TestDocsMappedCheckIsSharedHelper in internal/doctor) — never a private
// reimplementation that could silently disagree about what "mapped" means.
func TestScaffoldMappedCheckIsSharedHelper(t *testing.T) {
	got := reflect.ValueOf(scaffoldMappedCheck).Pointer()
	want := reflect.ValueOf(HasEntry).Pointer()
	if got != want {
		t.Fatal("scaffoldMappedCheck has drifted from HasEntry — Add's convergence check and doctor's " +
			"unmapped-domain detection must consume ONE shared 'mapped' predicate (spec 123 R3/AC-4)")
	}
}
