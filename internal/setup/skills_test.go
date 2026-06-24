package setup

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSkillInventory_Twelve pins the merged skill surface count: exactly 12 —
// 4 lifecycle gates + 8 plugin skills (the spec-093 thin-down baseline of 7
// plugin skills plus ms-spec-grill).
func TestSkillInventory_Twelve(t *testing.T) {
	all := skillFiles()
	if len(all) != 12 {
		t.Fatalf("skillFiles() must return 12 skills (4 lifecycle + 8 plugin), got %d: %v", len(all), keys(all))
	}
	if n := len(lifecycleSkillFiles()); n != 4 {
		t.Errorf("lifecycleSkillFiles() must return 4 lifecycle gates, got %d", n)
	}

	want := []string{
		"ms-spec-create", "ms-spec-approve", "ms-plan-approve", "ms-impl-approve",
		"ms-bead-cycle", "ms-bead-fix", "ms-bead-impl", "ms-panel-run",
		"ms-panel-tally", "ms-spec-autopilot", "ms-spec-final-review",
		"ms-spec-grill",
	}
	for _, name := range want {
		if _, ok := all[name]; !ok {
			t.Errorf("expected skill %q in skillFiles()", name)
		}
	}

	// The removed/merged skills must be gone.
	for _, gone := range removedSkills {
		if _, ok := all[gone]; ok {
			t.Errorf("removed skill %q must not be in skillFiles()", gone)
		}
	}
}

// TestSkillSurface_NoDeprecatedApproveOrder extends spec-092 Req 11's negative
// assertion (previously over lifecycleSkillFiles() only, claude_test.go:715)
// to the FULL plugin skill surface (skillFiles()): no shipped skill content
// anywhere teaches the deprecated `approve <noun>` order (spec 093 Req 19a).
func TestSkillSurface_NoDeprecatedApproveOrder(t *testing.T) {
	for name, content := range skillFiles() {
		if m := deprecatedApproveOrder.FindString(content); m != "" {
			t.Errorf("skill %s teaches deprecated approve order %q (spec 093 Req 19a); canonical is `mindspec spec approve` / `plan approve` / `impl approve`", name, m)
		}
	}
}

// removedNameToken matches any of the five removed skill names anywhere.
var removedNameToken = regexp.MustCompile(`ms-bead-next|ms-bead-merge|ms-bead-prep|ms-panel-create|ms-spec-status`)

// provenanceLine matches the deliberately-retained "this used to be skill X"
// fold/superseded notes (HC-2 requires recording where deleted prose went).
// These name a removed skill but are NOT live invocation references.
var provenanceLine = regexp.MustCompile(`(?i)previously|folded|superseded|used to carry|no longer a skill|are folded`)

// clauseSplit breaks a physical line into its sentence/clause fragments so the
// HC-2 provenance exemption can be scoped to the SPECIFIC reference rather than
// the whole line. Without this, a live removed-skill reference sharing a
// physical line with an unrelated provenance keyword (e.g. a table row plus a
// trailing fold-note) would be wrongly exempted (bug mindspec-5xnr).
var clauseSplit = regexp.MustCompile(`[;.—\n]|\.\s`)

// liveRemovedRefs returns the removed-skill references on a line that are NOT
// in an HC-2 provenance context. A reference is exempt only when its OWN clause
// carries a provenance keyword; a reference whose clause has no provenance
// keyword is reported as live even if some other clause on the same physical
// line does. Returns the offending clause fragments (trimmed) for diagnostics.
func liveRemovedRefs(line string) []string {
	var live []string
	for _, clause := range clauseSplit.Split(line, -1) {
		if !removedNameToken.MatchString(clause) {
			continue
		}
		if provenanceLine.MatchString(clause) {
			continue // documented fold/superseded mapping (HC-2), scoped to this clause
		}
		live = append(live, strings.TrimSpace(clause))
	}
	return live
}

// TestGrepClean_NoLiveRemovedSkillReferences is the binding grep-clean
// acceptance criterion (spec 093 Req 16): no SURVIVING skill carries a LIVE
// reference (handoff / prerequisite / table row) to a removed skill. The
// deliberately-retained superseded-by / fold-provenance notes (HC-2) are
// exempt — they document the removal, they do not direct a reader to invoke a
// skill that no longer exists.
func TestGrepClean_NoLiveRemovedSkillReferences(t *testing.T) {
	for name, content := range skillFiles() {
		for i, line := range strings.Split(content, "\n") {
			for _, ref := range liveRemovedRefs(line) {
				t.Errorf("skill %s line %d carries a live reference to a removed skill:\n  %s", name, i+1, ref)
			}
		}
	}
}

// acGrepCleanSurfaces lists the NON-skill surfaces the binding grep-clean AC
// (spec.md:815) enumerates beyond skillFiles(): the CLAUDE/AGENTS managed-block
// sources (setup.claudeMDManagedBlock + bootstrap.go's init templates), both
// READMEs, and the instruct templates. Paths are repo-root-relative; the test
// runs from internal/setup, so repo root is two levels up. A removed-skill
// reference reintroduced into any of these would otherwise pass undetected
// (panel R3: the stale bootstrap.go /ms-spec-status table shipped green because
// the AC test stopped at skillFiles()).
var acGrepCleanSurfaces = []string{
	"internal/setup/claude.go",        // claudeMDManagedBlock generator
	"internal/bootstrap/bootstrap.go", // mindspec init CLAUDE/AGENTS/Copilot templates
	"plugins/mindspec/README.md",
	"README.md",
}

// TestGrepClean_NoLiveRemovedSkillReferences_ACSurfaces extends the grep-clean
// AC (spec 093 Req 16/18) past skillFiles() to the AC's other enumerated
// surfaces: the managed-block sources, both READMEs, and the instruct
// templates. The same HC-2 fold-provenance exemption applies (a line that
// documents WHERE a removed skill's prose went is allowed; a line that directs
// a reader to invoke the removed skill is not). This is the regression guard
// for panel R3's demonstrated bootstrap.go /ms-spec-status leak.
func TestGrepClean_NoLiveRemovedSkillReferences_ACSurfaces(t *testing.T) {
	root := repoRoot(t)

	files := append([]string{}, acGrepCleanSurfaces...)
	templates, err := filepath.Glob(filepath.Join(root, "internal", "instruct", "templates", "*.md"))
	if err != nil {
		t.Fatalf("globbing instruct templates: %v", err)
	}
	for _, tmpl := range templates {
		rel, err := filepath.Rel(root, tmpl)
		if err != nil {
			t.Fatalf("relativizing %s: %v", tmpl, err)
		}
		files = append(files, rel)
	}

	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("reading AC surface %s: %v", rel, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, ref := range liveRemovedRefs(line) {
				t.Errorf("%s line %d carries a live reference to a removed skill:\n  %s", rel, i+1, ref)
			}
		}
	}
}

// TestLiveRemovedRefs_PerReferenceExemption pins the mindspec-5xnr tightening:
// the HC-2 provenance exemption is scoped per-REFERENCE (per clause), not
// per-physical-line. A live removed-skill reference sharing a line with an
// unrelated provenance keyword must STILL be FLAGGED, while a genuine
// fold/superseded note remains exempt.
func TestLiveRemovedRefs_PerReferenceExemption(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string // expected live (flagged) references
	}{
		{
			// The bug case: a live invocation reference AND an unrelated
			// provenance keyword on the SAME physical line. The old per-line
			// exemption wrongly skipped this; per-reference must flag it.
			name: "live ref sharing a line with an unrelated provenance keyword is flagged",
			line: "Run `/ms-bead-next` to claim the next bead. Plan files were previously kept elsewhere.",
			want: []string{"Run `/ms-bead-next` to claim the next bead"},
		},
		{
			// Real HC-2 fold-note: the removed skill and its provenance keyword
			// live in the SAME clause; must remain exempt.
			name: "genuine fold-note stays exempt",
			line: "Step 0 was previously the separate `/ms-panel-create` skill; it is folded in here.",
			want: nil,
		},
		{
			// Mixed line: a real fold-note clause (exempt) plus a separate live
			// reference clause (flagged). Only the live clause is reported.
			name: "fold-note clause exempt, live clause in same line flagged",
			line: "These were previously `/ms-bead-merge`, folded in here; now run `/ms-spec-status` to check.",
			want: []string{"now run `/ms-spec-status` to check"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := liveRemovedRefs(tc.line)
			if len(got) != len(tc.want) {
				t.Fatalf("liveRemovedRefs(%q) = %v; want %v", tc.line, got, tc.want)
			}
			for i := range tc.want {
				if !strings.Contains(got[i], strings.TrimSpace(tc.want[i])) {
					t.Errorf("flagged ref %d = %q; want it to contain %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// repoRoot returns the module root by walking up from the test's working
// directory (internal/setup) until it finds go.mod. Keeps the AC-surface scan
// robust to where `go test` is invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod above %s", dir)
		}
		dir = parent
	}
}

// TestInstallSkills_RefreshesPreviouslyShipped covers Req 19b: a skill file
// whose on-disk content byte-matches a PREVIOUSLY-SHIPPED version is refreshed
// in place to the canonical content; the marker-less pre-093 lifecycle skill
// is the canonical historical case.
func TestInstallSkills_RefreshesPreviouslyShipped(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills")

	// Seed an existing install carrying the pre-marker (previously-shipped)
	// ms-spec-approve content.
	canonical := lifecycleSkillFiles()["ms-spec-approve"]
	prior := stripManagedByMarker(canonical)
	if prior == canonical {
		t.Fatal("expected the canonical ms-spec-approve to carry the managed-by marker")
	}
	writeExisting(t, skillsDir, "ms-spec-approve", prior)

	r := &Result{}
	if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), claudeSkillFiles(), false, r); err != nil {
		t.Fatalf("installSkills: %v", err)
	}

	got := readSkill(t, skillsDir, "ms-spec-approve")
	if got != canonical {
		t.Errorf("previously-shipped skill not refreshed to canonical content;\ngot:\n%s", got)
	}
	if !containsPath(r.Refreshed, filepath.Join(".claude", "skills", "ms-spec-approve", "SKILL.md")) {
		t.Errorf("Refreshed should record ms-spec-approve; got %v", r.Refreshed)
	}
}

// TestInstallSkills_RefreshesPriorSpecCreateSnapshot covers the grill
// auto-chain propagation (spec 105): an existing install carries the PRIOR
// canonical ms-spec-create body — with the `managed-by: mindspec` marker but
// WITHOUT the step-5 ms-spec-grill auto-invoke — recorded byte-exact in
// historical_skills/ms-spec-create.md. installSkills must recognize it as a
// shipped snapshot and refresh it in place to the new canonical (which invokes
// ms-spec-grill), NOT leave it as user-modified.
func TestInstallSkills_RefreshesPriorSpecCreateSnapshot(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills")

	// The prior shipped body lives in historical_skills/ms-spec-create.md.
	priorVariants := previouslyShippedSkills()["ms-spec-create"]
	var prior string
	for _, v := range priorVariants {
		if strings.Contains(v, "managed-by: mindspec") && !strings.Contains(v, "ms-spec-grill") {
			prior = v
			break
		}
	}
	if prior == "" {
		t.Fatal("expected a historical ms-spec-create snapshot carrying the managed-by marker without the step-5 ms-spec-grill auto-invoke")
	}
	writeExisting(t, skillsDir, "ms-spec-create", prior)

	r := &Result{}
	if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), claudeSkillFiles(), false, r); err != nil {
		t.Fatalf("installSkills: %v", err)
	}

	canonical := lifecycleSkillFiles()["ms-spec-create"]
	got := readSkill(t, skillsDir, "ms-spec-create")
	if got != canonical {
		t.Errorf("prior ms-spec-create snapshot not refreshed to canonical content;\ngot:\n%s", got)
	}
	if !strings.Contains(got, "ms-spec-grill") {
		t.Errorf("refreshed ms-spec-create must carry the step-5 ms-spec-grill auto-invoke; got:\n%s", got)
	}
	if !containsPath(r.Refreshed, filepath.Join(".claude", "skills", "ms-spec-create", "SKILL.md")) {
		t.Errorf("Refreshed should record ms-spec-create; got %v", r.Refreshed)
	}
}

// TestInstallSkills_RefreshesPre106Snapshot covers the spec 106 (Req 12 /
// AC17) install-refresh path: each path-bearing skill was rewritten to the
// flat .mindspec/{specs,adr,domains} + co-located reviews/ layout, and a
// byte-exact capture of its CANONICAL pre-flatten bytes was added at
// historical_skills/<skill>.pre106.md so an existing pre-106 install refreshes
// in place to the flat content (instead of being treated as user-modified and
// stranded on dead .mindspec/docs/ paths). The multi-snapshot-per-skill keying
// (segment before the first dot) is what makes the .pre106 capture coexist with
// the older frozen pre-093 snapshot.
func TestInstallSkills_RefreshesPre106Snapshot(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills")

	// The pre-flatten canonical bytes live in
	// historical_skills/ms-panel-run.pre106.md and still carry the old
	// repo-root `review/` panel path.
	var pre106 string
	for _, v := range previouslyShippedSkills()["ms-panel-run"] {
		if strings.Contains(v, "<repo>/review/") {
			pre106 = v
			break
		}
	}
	if pre106 == "" {
		t.Fatal("expected a historical ms-panel-run snapshot carrying the pre-flatten repo-root `review/` panel path")
	}
	writeExisting(t, skillsDir, "ms-panel-run", pre106)

	r := &Result{}
	if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), claudeSkillFiles(), false, r); err != nil {
		t.Fatalf("installSkills: %v", err)
	}

	canonical := claudeSkillFiles()["ms-panel-run"]
	got := readSkill(t, skillsDir, "ms-panel-run")
	if got != canonical {
		t.Errorf("pre-106 ms-panel-run snapshot not refreshed to flat canonical content")
	}
	if strings.Contains(got, "<repo>/review/") {
		t.Errorf("refreshed ms-panel-run must NOT retain the pre-flatten repo-root `review/` panel path")
	}
	if !containsPath(r.Refreshed, filepath.Join(".claude", "skills", "ms-panel-run", "SKILL.md")) {
		t.Errorf("Refreshed should record ms-panel-run; got %v", r.Refreshed)
	}
}

// TestInstallSkills_LeavesUserModified covers HC-6: a user-modified skill file
// (matching neither the canonical nor any shipped snapshot) is left untouched
// with a notice.
func TestInstallSkills_LeavesUserModified(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills")

	userContent := "---\nname: ms-spec-approve\n---\n\n# My own version\n"
	writeExisting(t, skillsDir, "ms-spec-approve", userContent)

	r := &Result{}
	if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), claudeSkillFiles(), false, r); err != nil {
		t.Fatalf("installSkills: %v", err)
	}

	if got := readSkill(t, skillsDir, "ms-spec-approve"); got != userContent {
		t.Errorf("user-modified skill must be left untouched; got:\n%s", got)
	}
	if len(r.Notices) == 0 {
		t.Errorf("expected a notice for the user-modified skill")
	}
	if containsPath(r.Refreshed, filepath.Join(".claude", "skills", "ms-spec-approve", "SKILL.md")) {
		t.Errorf("user-modified skill must NOT be refreshed")
	}
}

// TestCleanupRemovedSkills_UnmodifiedRemoved covers Req 18: a retired skill
// dir whose SKILL.md byte-matches a shipped snapshot is removed.
func TestCleanupRemovedSkills_UnmodifiedRemoved(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills")

	// Seed the previously-shipped ms-spec-status (a removed skill).
	shipped := previouslyShippedSkills()["ms-spec-status"]
	if len(shipped) == 0 {
		t.Fatal("expected a historical snapshot for ms-spec-status")
	}
	writeExisting(t, skillsDir, "ms-spec-status", shipped[0])

	r := &Result{}
	if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), claudeSkillFiles(), false, r); err != nil {
		t.Fatalf("installSkills: %v", err)
	}

	if fileExists(filepath.Join(skillsDir, "ms-spec-status", "SKILL.md")) {
		t.Errorf("unmodified retired skill ms-spec-status must be removed")
	}
	if !containsPath(r.Removed, filepath.Join(".claude", "skills", "ms-spec-status")) {
		t.Errorf("Removed should record ms-spec-status; got %v", r.Removed)
	}
}

// TestCleanupRemovedSkills_ModifiedKept covers HC-6 for retired skills: a
// user-modified retired skill is left in place with a notice.
func TestCleanupRemovedSkills_ModifiedKept(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills")

	userContent := "---\nname: ms-spec-status\n---\n\n# I rely on this\n"
	writeExisting(t, skillsDir, "ms-spec-status", userContent)

	r := &Result{}
	if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), claudeSkillFiles(), false, r); err != nil {
		t.Fatalf("installSkills: %v", err)
	}

	if !fileExists(filepath.Join(skillsDir, "ms-spec-status", "SKILL.md")) {
		t.Errorf("user-modified retired skill must be left in place")
	}
	if len(r.Notices) == 0 {
		t.Errorf("expected a notice for the user-modified retired skill")
	}
}

// TestSetupRefresh_EndToEnd_RemovesStatusAndRefreshesApprove exercises the
// real RunClaude path on a simulated pre-093 install.
func TestSetupRefresh_EndToEnd_RemovesStatusAndRefreshesApprove(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills")

	// Pre-093 install: ms-spec-status present (shipped) + ms-spec-approve at
	// the marker-less prior content.
	writeExisting(t, skillsDir, "ms-spec-status", previouslyShippedSkills()["ms-spec-status"][0])
	writeExisting(t, skillsDir, "ms-spec-approve", stripManagedByMarker(lifecycleSkillFiles()["ms-spec-approve"]))

	if _, err := RunClaude(root, false); err != nil {
		t.Fatalf("RunClaude: %v", err)
	}

	if fileExists(filepath.Join(skillsDir, "ms-spec-status", "SKILL.md")) {
		t.Errorf("ms-spec-status should have been removed by setup")
	}
	if got := readSkill(t, skillsDir, "ms-spec-approve"); got != lifecycleSkillFiles()["ms-spec-approve"] {
		t.Errorf("ms-spec-approve should have been refreshed to canonical")
	}
	// And the new plugin skills should now exist.
	if !fileExists(filepath.Join(skillsDir, "ms-bead-cycle", "SKILL.md")) {
		t.Errorf("ms-bead-cycle should be installed")
	}
}

// --- test helpers ---

func writeExisting(t *testing.T, skillsDir, name, content string) {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readSkill(t *testing.T, skillsDir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(skillsDir, name, "SKILL.md"))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func containsPath(list []string, p string) bool {
	for _, x := range list {
		if x == p {
			return true
		}
	}
	return false
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
