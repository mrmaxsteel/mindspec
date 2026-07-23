package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bootstrap"
	"github.com/mrmaxsteel/mindspec/internal/domain"
)

// TestDoctorWarnsOnMissingOwnership verifies that checkDomains warns when a
// domain directory lacks OWNERSHIP.yaml and reports OK when present
// (spec-086 Bead 4).
func TestDoctorWarnsOnMissingOwnership(t *testing.T) {
	t.Run("missing manifest emits Warn", func(t *testing.T) {
		root := t.TempDir()
		domainDir := filepath.Join(root, "docs", "domains", "foo")
		if err := os.MkdirAll(domainDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Need at least one file so checkDomains iterates into foo/.
		if err := os.WriteFile(filepath.Join(domainDir, "overview.md"), []byte("# foo"), 0o644); err != nil {
			t.Fatal(err)
		}

		r := &Report{}
		checkDomains(r, root, "docs")

		var found *Check
		for i := range r.Checks {
			if r.Checks[i].Name == "docs/domains/foo/OWNERSHIP.yaml" {
				found = &r.Checks[i]
				break
			}
		}
		if found == nil {
			t.Fatal("expected OWNERSHIP.yaml check, got none")
		}
		if found.Status != Warn {
			t.Errorf("expected Warn status (not Missing/Error per Req 15), got %d", found.Status)
		}
		if !strings.Contains(found.Message, "OWNERSHIP.yaml") {
			t.Errorf("expected message to mention OWNERSHIP.yaml, got %q", found.Message)
		}
	})

	t.Run("present manifest emits OK", func(t *testing.T) {
		root := t.TempDir()
		domainDir := filepath.Join(root, "docs", "domains", "foo")
		if err := os.MkdirAll(domainDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(domainDir, "overview.md"), []byte("# foo"), 0o644); err != nil {
			t.Fatal(err)
		}
		manifest := []byte("domain: foo\nattributes: []\n")
		if err := os.WriteFile(filepath.Join(domainDir, "OWNERSHIP.yaml"), manifest, 0o644); err != nil {
			t.Fatal(err)
		}

		r := &Report{}
		checkDomains(r, root, "docs")

		var found *Check
		for i := range r.Checks {
			if r.Checks[i].Name == "docs/domains/foo/OWNERSHIP.yaml" {
				found = &r.Checks[i]
				break
			}
		}
		if found == nil {
			t.Fatal("expected OWNERSHIP.yaml check, got none")
		}
		if found.Status != OK {
			t.Errorf("expected OK status when manifest exists, got %d (msg=%q)", found.Status, found.Message)
		}
	})
}

// ─── spec 123 R3/AC-3: missing-context-map + unmapped-domain ───────────────

// TestCheckContextMap_MissingFile pins AC-3(i): a repo with domains/alpha/
// and no context-map.md at all reports the missing-context-map finding;
// --fix scaffolds the skeleton and a re-run clears it. RED on pre-spec-123
// main (no such check existed at all).
func TestCheckContextMap_MissingFile(t *testing.T) {
	root := t.TempDir()
	domainDir := filepath.Join(root, "docs", "domains", "alpha")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "overview.md"), []byte("# alpha"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkContextMap(r, root, "docs")

	c := findCheck(r, "docs/context-map.md")
	if c == nil {
		t.Fatal("expected a context-map.md check")
	}
	if c.Status != Missing {
		t.Fatalf("expected Missing, got %v (msg=%q)", c.Status, c.Message)
	}
	if c.FixFunc == nil {
		t.Fatal("expected a --fix FixFunc")
	}
	if err := c.FixFunc(); err != nil {
		t.Fatalf("FixFunc: %v", err)
	}

	r2 := &Report{}
	checkContextMap(r2, root, "docs")
	c2 := findCheck(r2, "docs/context-map.md")
	if c2 == nil || c2.Status != OK {
		t.Fatalf("expected OK after --fix, got %+v", c2)
	}
}

// TestCheckContextMap_UnreadableFileIsError pins FX-3: an EXISTING but
// unreadable context-map.md (mode 000) must NOT be classified Missing with
// the scaffold fixer — that fixer no-ops on an existing file yet
// Report.Fix() would flip the check to Fixed while the read error persists
// ("reports success without fixing"). It must be a concrete Error with NO
// FixFunc, so --fix leaves it visibly Error rather than falsely Fixed.
func TestCheckContextMap_UnreadableFileIsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: mode 000 is still readable, cannot exercise the read-error path")
	}
	root := t.TempDir()
	domainDir := filepath.Join(root, "docs", "domains", "alpha")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "overview.md"), []byte("# alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmPath := filepath.Join(root, "docs", "context-map.md")
	if err := os.WriteFile(cmPath, []byte("# Context Map\n\n## Bounded Contexts\n\n---\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cmPath, 0o644) }) // let TempDir cleanup remove it

	r := &Report{}
	checkContextMap(r, root, "docs")

	c := findCheck(r, "docs/context-map.md")
	if c == nil {
		t.Fatal("expected a context-map.md check")
	}
	if c.Status != Error {
		t.Fatalf("expected Error for an unreadable-but-existing file, got %v (msg=%q)", c.Status, c.Message)
	}
	if c.FixFunc != nil {
		t.Fatal("an unreadable-file Error must NOT carry the scaffold FixFunc (would falsely report Fixed)")
	}

	// Report.Fix() must leave it Error, not flip it to Fixed.
	r.Fix()
	c2 := findCheck(r, "docs/context-map.md")
	if c2.Status != Error {
		t.Errorf("--fix must leave the unreadable-file finding as Error, got %v", c2.Status)
	}
}

// TestCheckContextMap_UnmappedDomain pins AC-3(ii): context-map.md present
// but with no ### Alpha entry produces a Warn naming alpha with the
// `mindspec domain add alpha` recovery; running that command clears it. RED
// on pre-spec-123 main (checkDomains never looked at context-map.md at
// all).
func TestCheckContextMap_UnmappedDomain(t *testing.T) {
	root := t.TempDir()
	domainDir := filepath.Join(root, "docs", "domains", "alpha")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "overview.md"), []byte("# alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "context-map.md"), []byte(domain.ContextMapSkeleton()), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkContextMap(r, root, "docs")

	c := findCheck(r, "context-map.md (alpha)")
	if c == nil {
		t.Fatal("expected an unmapped-domain check for alpha")
	}
	if c.Status != Warn {
		t.Fatalf("expected Warn, got %v", c.Status)
	}
	if !strings.Contains(c.Message, "mindspec domain add alpha") {
		t.Errorf("expected recovery line naming 'mindspec domain add alpha', got %q", c.Message)
	}

	// Recovery: domain.Add backfills the entry, clearing the Warn.
	if err := domain.Add(root, "alpha"); err != nil {
		t.Fatalf("domain.Add: %v", err)
	}
	r2 := &Report{}
	checkContextMap(r2, root, "docs")
	if c2 := findCheck(r2, "context-map.md (alpha)"); c2 != nil {
		t.Errorf("expected the unmapped-domain Warn to clear after domain add, got %+v", c2)
	}
}

// TestCheckContextMap_FullyMapped pins AC-3(iii): a fully-mapped repo
// reports OK for both the context-map presence AND the per-domain mapping
// (no unmapped-domain Warn).
func TestCheckContextMap_FullyMapped(t *testing.T) {
	root := t.TempDir()
	domainDir := filepath.Join(root, "docs", "domains", "alpha")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "overview.md"), []byte("# alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	cm := domain.ContextMapSkeleton()
	cm = strings.Replace(cm, "---", "### Alpha\n\n**Owns**: _(fill in)_\n\n---", 1)
	if err := os.WriteFile(filepath.Join(root, "docs", "context-map.md"), []byte(cm), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Report{}
	checkContextMap(r, root, "docs")

	cmCheck := findCheck(r, "docs/context-map.md")
	if cmCheck == nil || cmCheck.Status != OK {
		t.Fatalf("expected OK for context-map.md presence, got %+v", cmCheck)
	}
	if c := findCheck(r, "context-map.md (alpha)"); c != nil {
		t.Errorf("expected no unmapped-domain Warn for a fully-mapped domain, got %+v", c)
	}
}

// TestDocsMappedCheckIsSharedHelper is the AC-4 anti-drift identity pin: the
// unmapped-domain check's seam var must still point at the exact exported
// domain.HasEntry — the SAME helper scaffold.Add's convergence check
// consumes (its own mirrored seam var, scaffoldMappedCheck, is pinned by
// TestScaffoldMappedCheckIsSharedHelper in internal/domain) — never a
// private reimplementation that could silently disagree about what
// "mapped" means.
func TestDocsMappedCheckIsSharedHelper(t *testing.T) {
	got := reflect.ValueOf(docsMappedCheck).Pointer()
	want := reflect.ValueOf(domain.HasEntry).Pointer()
	if got != want {
		t.Fatal("docsMappedCheck has drifted from domain.HasEntry — the emission (scaffold.Add) and detection (doctor) sides of \"mapped\" must consume ONE shared helper (spec 123 R3/AC-4)")
	}
}

// ─── spec 123 Bead 1 Step 7: lane-scoped greenfield smoke ──────────────────

// TestGreenfieldSmoke_InitThenDomainAddThenDoctor pins the lane-scoped
// AC-1-shape smoke test from the plan's Bead 1 Verification bullet: empty
// dir → git init → mindspec init → mindspec domain add alpha exits 0 with
// the ### Alpha entry under ## Bounded Contexts, and doctor reports no
// Error/Missing from the context-map and gitignore lanes this bead owns.
// (The FULL AC-1 pin — including the R6c/R7c present-Warn legs — lands in
// Bead 3, once those checks exist.) RED on pre-spec-123 main: domain.Add
// errored "reading context map" because init never scaffolded
// .mindspec/context-map.md (#207).
func TestGreenfieldSmoke_InitThenDomainAddThenDoctor(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")

	if _, err := bootstrap.Run(root, false); err != nil {
		t.Fatalf("bootstrap.Run (mindspec init) error: %v", err)
	}
	if err := domain.Add(root, "alpha"); err != nil {
		t.Fatalf("domain.Add(alpha) error: %v", err)
	}

	cmPath := filepath.Join(root, ".mindspec", "context-map.md")
	data, err := os.ReadFile(cmPath)
	if err != nil {
		t.Fatalf("reading context-map.md: %v", err)
	}
	if !strings.Contains(string(data), "### Alpha") {
		t.Errorf("context-map.md missing ### Alpha entry:\n%s", data)
	}

	report := RunWithOptions(root, Options{SkipLocalEnv: true})
	for _, c := range report.Checks {
		if c.Status != Error && c.Status != Missing {
			continue
		}
		if strings.Contains(c.Name, "context-map") || strings.Contains(c.Name, "git tracking") {
			t.Errorf("unexpected %v on the context-map/gitignore lane: %s: %s", c.Status, c.Name, c.Message)
		}
	}
}
