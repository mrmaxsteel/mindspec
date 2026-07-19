package gitutil

// Spec 121 Bead 1 (AC-16 §2 half). The Bead 1 review panel (MAJOR S3)
// found the original AC-16 §2 evidence was a manual `rg` grep, not a
// committed automated test — so a future edit that strips or renumbers a
// §2 clause (breaking Beads 2's and 4's own §2(ii)/§2(i) citations) would
// NOT go red in CI. This test reads ADR-0041's actual committed text and
// asserts both the three roman-numeral clause anchors AND the concrete
// instance/term each clause names are present — living beside
// gitutil.NetEffectLanded, the FIRST code to cite §2(iii) (see
// neteffect.go's doc comment), per R8's "amendment lands with the first
// citing code" rule.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adrAnchorRepoRoot walks up from the test's runtime CWD (this package's
// directory, per `go test`) until go.mod is found — the same convention as
// internal/lifecycle's ownershipRepoRoot / internal/specgate's repoRoot.
func adrAnchorRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "go.mod")
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return dir
		}
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("stat %s: %v", candidate, statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate go.mod walking up from %s (8-level cap reached)", cwd)
	return ""
}

// TestADR0041_Section2ConvergenceCompletenessAnchors is the AC-16 §2 half's
// automated anchor pin. RED if the §2 amendment text is ever stripped,
// renumbered, or paraphrased away from the concrete instances it names.
func TestADR0041_Section2ConvergenceCompletenessAnchors(t *testing.T) {
	root := adrAnchorRepoRoot(t)
	path := filepath.Join(root, ".mindspec", "adr", "ADR-0041-gate-before-mutate.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	text := string(data)
	lower := strings.ToLower(text)

	// The three roman-numeral clause anchors themselves — Beads 2 and 4
	// cite these exact labels (§2(ii), §2(i)) from landed.go and the
	// step-1.6 preflight respectively.
	for _, anchor := range []string{"§2(i)", "§2(ii)", "§2(iii)"} {
		if !strings.Contains(text, anchor) {
			t.Errorf("ADR-0041 must contain the %s clause anchor (Beads 2/4 cite it by this exact label)", anchor)
		}
	}

	// The concrete instance/term each clause names, independent of the
	// numbering: losing these while keeping the bare "§2(i)" label would
	// still gut the amendment's substance, so they are pinned separately.
	instanceChecks := []struct {
		clause string
		terms  []string
	}{
		{clause: "§2(i) (deadlock-free recovery graph)", terms: []string{"tpjn", "all-orphans", "attested-restore"}},
		{clause: "§2(ii) (durable corroboration, revert/reapply-aware)", terms: []string{"q9ea", "revert", "reapply"}},
		{clause: "§2(iii) (content-aware already-merged re-derivation)", terms: []string{"net-effect", "content-aware", "3xqm"}},
	}
	for _, c := range instanceChecks {
		for _, term := range c.terms {
			if !strings.Contains(lower, strings.ToLower(term)) {
				t.Errorf("ADR-0041's %s clause must name %q — an edit that drops this term breaks the citing code's contract", c.clause, term)
			}
		}
	}

	if !strings.Contains(lower, "convergence") {
		t.Error("ADR-0041 must contain the convergence-completeness anchor text (AC-16 §2 half, the rg 'convergence' proof)")
	}
}
