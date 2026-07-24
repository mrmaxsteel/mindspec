package setup

// upgrade_refresh_pre126_test.go — spec 126 Bead 4 (mindspec-xurf.4), AC-16:
// the seven byte-exact pre-126 snapshots in historical_skills/*.pre126.md
// (five plugin skills + the two ms-spec-approve/ms-plan-approve lifecycle
// literals) prove an EXISTING pre-126 install upgrades cleanly to the new
// canonical content.
//
// FIRST leg — the provenance-digest oracle (plan-gate correction, plan.md's
// "Snapshot PROVENANCE is pinned by digest, not inferred from history"
// section): the seed/Refreshed legs alone cannot prove a snapshot was
// captured correctly — installSkills trusts whatever bytes sit in
// historical_skills/, so a WRONGLY-CAPTURED snapshot still seeds, still
// matchesShipped (the bad fixture is itself in the shipped-history set by
// construction), and still classifies Refreshed. TestUpgradeRefreshPre126_
// DigestOracle is the external oracle a fixture cannot satisfy by merely
// existing: it hashes each embedded .pre126.md body and compares it against
// a digest PINNED in this file (computed independently, at capture time, by
// `git show <base>:<path> | shasum -a 256` for the five plugin skills and by
// the base-binary install-capture procedure for the two lifecycle literals —
// never derived from the snapshot file under test).
//
// Deleting any one snapshot REDs TWICE, for two DIFFERENT reasons (the
// asymmetry the digest leg exists for): the digest oracle fails because it
// can no longer read the missing embedded body; the seed/Refreshed leg fails
// independently because it can no longer locate that variant to seed either
// — a plain "file not found" Fatal, not a silent pass. A WRONGLY-CAPTURED
// (corrupted) snapshot is caught ONLY by the digest oracle: the seed leg
// would still seed the (wrong) bytes and installSkills would still classify
// them Refreshed, because matchesShipped only asks "does this match
// something shipped" and the bad fixture answers its own question.
//
// Base commit for the five plugin-skill snapshots: 7ec96295 (recorded in
// plan.md as `git merge-base origin/main HEAD` at plan time; re-verified
// identical at this bead's capture time). Capture command:
//
//	git show 7ec96295:plugins/mindspec/skills/<name>/SKILL.md | shasum -a 256
//
// The two lifecycle-literal snapshots (ms-spec-approve, ms-plan-approve) are
// the exact bytes the BASE binary installs — captured per plan.md's recorded
// procedure: `git worktree add --detach <dir> 7ec96295 && cd <dir> && go
// build -o <bin> ./cmd/mindspec && (cd <throwaway-repo> && git init && <bin>
// setup claude)`, then `shasum -a 256` the two installed SKILL.md files.
// Cross-checked byte-for-byte against `git show 7ec96295:internal/setup/
// claude.go`'s lifecycleSkillFiles() literals (the ms-spec-approve.pre0uur.md
// precedent) at capture time.
import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

// upgradeRefreshPre126Digests pins the reviewed SHA-256 digest for each of
// the seven base artifacts. THESE ARE NOT DERIVED FROM THE SNAPSHOT FILE
// UNDER TEST — they are independently-computed values recorded here as the
// proof-of-record, per plan.md's snapshot-provenance block.
var upgradeRefreshPre126Digests = map[string]string{
	// Five plugin skills — git show 7ec96295:plugins/mindspec/skills/<name>/SKILL.md | shasum -a 256
	"ms-panel-run":         "1ae112ef731ee1b489dac7226b73684d5357678a913900fe27e45a1d2afbb11f",
	"ms-panel-tally":       "687dc90613f067b88bc7e3c1b7c490f93a8a99993904bb4489ae55b9c197354a",
	"ms-bead-cycle":        "679b580e33412870fe5449d0ceef8240a4ed5cbbebbd41722bf142ce41796ba8",
	"ms-bead-fix":          "d481f5297960b45f53404ddba3a4ad69be975fb205bb85746f7c9fed005aaf8e",
	"ms-spec-final-review": "26c09c05b9d083495d54de1f0b99393c822037bb6026dd68a2194a45829a9ce3",
	// Two lifecycle literals — base-binary install-capture, cross-checked
	// against git show 7ec96295:internal/setup/claude.go at Bead 4 capture
	// time (this bead).
	"ms-spec-approve": "7eb76f3bd18024b7b2ff462f70e3341ab588331020edfd6e7f05c7f724e6a856",
	"ms-plan-approve": "d52087b6001d6dc256233a8c2d1bf0f712c20a0aa2c1efeb499a7f5fd8fe0e80",
}

// upgradeRefreshPre126Skills is the ordered set of every skill this spec
// edits and for which a byte-exact pre-126 snapshot must exist (AC-16).
var upgradeRefreshPre126Skills = []string{
	"ms-panel-run",
	"ms-panel-tally",
	"ms-bead-cycle",
	"ms-bead-fix",
	"ms-spec-final-review",
	"ms-spec-approve",
	"ms-plan-approve",
}

// readPre126Snapshot reads historical_skills/<name>.pre126.md straight from
// the embedded FS (the same embed.FS previouslyShippedSkills() walks), so a
// `git rm` of the file is picked up by BOTH this helper and the aggregate
// lookup on the next `go test` (go:embed globs are re-evaluated at build
// time; a missing match changes the build).
func readPre126Snapshot(t *testing.T, name string) string {
	t.Helper()
	data, err := historicalSkillsFS.ReadFile("historical_skills/" + name + ".pre126.md")
	if err != nil {
		t.Fatalf("reading historical_skills/%s.pre126.md: %v (snapshot missing or renamed)", name, err)
	}
	return string(data)
}

// TestUpgradeRefreshPre126_DigestOracle is the FIRST leg (plan-gate
// correction): before any seeding, assert each embedded .pre126.md body
// hashes to its PINNED digest above. This is what makes AC-16 non-vacuous —
// without it, a wrongly-captured snapshot passes every downstream leg.
func TestUpgradeRefreshPre126_DigestOracle(t *testing.T) {
	for _, name := range upgradeRefreshPre126Skills {
		name := name
		t.Run(name, func(t *testing.T) {
			body := readPre126Snapshot(t, name)
			sum := sha256.Sum256([]byte(body))
			got := hex.EncodeToString(sum[:])
			want, ok := upgradeRefreshPre126Digests[name]
			if !ok {
				t.Fatalf("no pinned digest recorded for %s", name)
			}
			if got != want {
				t.Errorf("historical_skills/%s.pre126.md digest mismatch:\n  got:  %s\n  want: %s\n(the snapshot was captured incorrectly, or has been altered since capture)", name, got, want)
			}
		})
	}
}

// TestUpgradeRefreshPre126_RefreshesEachEditedSkill is the behavioral leg:
// for EACH of the seven edited skills, seed the pre-126 body on disk (a
// stand-in for an existing install that predates this spec), run
// installSkills with the current canonical `wanted` set, and assert the
// file is classified Refreshed and its bytes now equal the new canonical.
// Non-vacuous per Beads 1–3: every canonical body has DIVERGED from its
// pre-126 snapshot (asserted directly below), so this is a real refresh, not
// a same-content no-op (which installSkills would classify Skipped, never
// Refreshed — skills.go:129-137).
func TestUpgradeRefreshPre126_RefreshesEachEditedSkill(t *testing.T) {
	canonical := skillFiles()
	for _, name := range upgradeRefreshPre126Skills {
		name := name
		t.Run(name, func(t *testing.T) {
			prior := readPre126Snapshot(t, name)
			want, ok := canonical[name]
			if !ok {
				t.Fatalf("skillFiles() has no %q entry", name)
			}
			if prior == want {
				t.Fatalf("%s: pre-126 snapshot is IDENTICAL to the current canonical body — the Refreshed assertion below would be vacuous (expected Beads 1-3 to have diverged every edited surface)", name)
			}

			root := t.TempDir()
			skillsDir := filepath.Join(root, ".claude", "skills")
			writeExisting(t, skillsDir, name, prior)

			r := &Result{}
			if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), canonical, false, r); err != nil {
				t.Fatalf("installSkills: %v", err)
			}

			relPath := filepath.Join(".claude", "skills", name, "SKILL.md")
			if !containsPath(r.Refreshed, relPath) {
				t.Errorf("%s: expected Refreshed to record %s; got Refreshed=%v Notices=%v", name, relPath, r.Refreshed, r.Notices)
			}
			got := readSkill(t, skillsDir, name)
			if got != want {
				t.Errorf("%s: refreshed body does not equal the new canonical content;\ngot:\n%s", name, got)
			}
		})
	}
}

// TestUpgradeRefreshPre126_UserModifiedPreserved is the user-modified-
// preservation leg: a body matching NO shipped snapshot (neither the
// current canonical nor any historical one) is left in place, unmodified,
// with the HC-6 notice — never silently overwritten.
func TestUpgradeRefreshPre126_UserModifiedPreserved(t *testing.T) {
	canonical := skillFiles()
	for _, name := range upgradeRefreshPre126Skills {
		name := name
		t.Run(name, func(t *testing.T) {
			base, ok := canonical[name]
			if !ok {
				t.Fatalf("skillFiles() has no %q entry", name)
			}
			userBody := base + "\n<!-- local operator note: do not touch -->\n"

			root := t.TempDir()
			skillsDir := filepath.Join(root, ".claude", "skills")
			writeExisting(t, skillsDir, name, userBody)

			r := &Result{}
			if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), canonical, false, r); err != nil {
				t.Fatalf("installSkills: %v", err)
			}

			relPath := filepath.Join(".claude", "skills", name, "SKILL.md")
			if containsPath(r.Refreshed, relPath) {
				t.Errorf("%s: user-modified body must NOT be classified Refreshed; got Refreshed=%v", name, r.Refreshed)
			}
			got := readSkill(t, skillsDir, name)
			if got != userBody {
				t.Errorf("%s: user-modified body must be left in place unmodified;\ngot:\n%s", name, got)
			}
			found := false
			for _, n := range r.Notices {
				if strings.HasPrefix(n, relPath) && strings.Contains(n, "user-modified — left in place; delete it to receive the canonical version") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: expected an HC-6 user-modified notice for %s; got Notices=%v", name, relPath, r.Notices)
			}
		})
	}
}
