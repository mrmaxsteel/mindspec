package setup

// upgrade_refresh_pre8yka_test.go — W0 Bead 9 (mindspec-ng3g).
//
// internal/setup/historical_skills/ms-bead-cycle.pre8yka.md was added
// (213330ce, "docs(w0): truth pass round 2") with no digest pin and no
// dedicated refresh test — verified: `grep -rn pre8yka` returns nothing
// outside that one file itself. previouslyShippedSkills() (skills.go)
// still picks it up generically (any historical_skills/<name>.<tag>.md
// is a valid prior-shipped variant of <name> by construction), so an
// existing install carrying this exact body already upgrades correctly
// in practice — but, per upgrade_refresh_pre126_test.go's own FIRST-leg
// reasoning, that behavioral fact alone cannot prove the snapshot was
// CAPTURED correctly: installSkills trusts whatever bytes sit in
// historical_skills/, so a wrongly-captured snapshot would still seed,
// still matchesShipped, and still classify Refreshed. This file gives
// ms-bead-cycle.pre8yka.md the same two-leg treatment
// upgrade_refresh_pre126_test.go established for the eight pre126
// snapshots, pinned to its own, independent base commit.
//
// Provenance: the snapshot's bytes are identical to
// plugins/mindspec/skills/ms-bead-cycle/SKILL.md as of commit
// e9f46dfe (verified independently: `git show
// e9f46dfe:plugins/mindspec/skills/ms-bead-cycle/SKILL.md | shasum -a
// 256` matches both the digest pinned below and the embedded file's
// own hash) — the version before that same commit's own edit
// (e9f46dfe999f7d768efb5c8fc9f4029a4597786d, referenced by
// upgrade_refresh_pre126_test.go's own header comment as
// "Bead 3's AC-1 sweep") rewrote ms-bead-cycle/SKILL.md into its
// current, much larger, step-0-owning shape.
import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
)

// upgradeRefreshPre8ykaDigest is the independently-computed SHA-256 of
// historical_skills/ms-bead-cycle.pre8yka.md's body — NOT derived from
// the snapshot file under test, per the same non-negotiable rule
// upgrade_refresh_pre126_test.go states for its own table. Captured via
// `git show e9f46dfe:plugins/mindspec/skills/ms-bead-cycle/SKILL.md |
// shasum -a 256`.
const upgradeRefreshPre8ykaDigest = "44feff53e89f3595ce588ca6ada633eb65a5a1c1ae82a1fe77115a4d842bceef"

// TestUpgradeRefreshPre8yka_DigestOracle is the FIRST leg: before any
// seeding, assert the embedded .pre8yka.md body hashes to the pinned
// digest above — what makes this snapshot's coverage non-vacuous,
// exactly as upgrade_refresh_pre126_test.go's own DigestOracle test
// does for its eight snapshots.
func TestUpgradeRefreshPre8yka_DigestOracle(t *testing.T) {
	data, err := historicalSkillsFS.ReadFile("historical_skills/ms-bead-cycle.pre8yka.md")
	if err != nil {
		t.Fatalf("reading historical_skills/ms-bead-cycle.pre8yka.md: %v (snapshot missing or renamed)", err)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != upgradeRefreshPre8ykaDigest {
		t.Errorf("historical_skills/ms-bead-cycle.pre8yka.md digest mismatch:\n  got:  %s\n  want: %s\n(the snapshot was captured incorrectly, or has been altered since capture)", got, upgradeRefreshPre8ykaDigest)
	}
}

// TestUpgradeRefreshPre8yka_RefreshesEditedSkill is the behavioral leg:
// seed the pre8yka body on disk (a stand-in for an existing install
// that predates ms-bead-cycle's step-0-owning rewrite), run
// installSkills with the current canonical `wanted` set, and assert
// the file is classified Refreshed and its bytes now equal the new
// canonical — non-vacuous because the pre8yka body is asserted below
// to differ from the current canonical body.
func TestUpgradeRefreshPre8yka_RefreshesEditedSkill(t *testing.T) {
	const name = "ms-bead-cycle"
	canonical := skillFiles()
	want, ok := canonical[name]
	if !ok {
		t.Fatalf("skillFiles() has no %q entry", name)
	}
	data, err := historicalSkillsFS.ReadFile("historical_skills/ms-bead-cycle.pre8yka.md")
	if err != nil {
		t.Fatalf("reading historical_skills/ms-bead-cycle.pre8yka.md: %v", err)
	}
	prior := string(data)
	if prior == want {
		t.Fatal("ms-bead-cycle.pre8yka.md is IDENTICAL to the current canonical body — the Refreshed assertion below would be vacuous")
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
		t.Errorf("expected Refreshed to record %s; got Refreshed=%v Notices=%v", relPath, r.Refreshed, r.Notices)
	}
	got := readSkill(t, skillsDir, name)
	if got != want {
		t.Errorf("refreshed body does not equal the new canonical content;\ngot:\n%s", got)
	}
}
