package setup

// upgrade_refresh_presurface_test.go — the shipped skill bodies stopped
// citing mindspec-internal records (ADR numbers, spec numbers, bead IDs,
// other projects). Every edited body has a byte-exact snapshot in
// historical_skills/*.presurface.md so an EXISTING install refreshes in
// place instead of being classified user-modified. Same three legs as
// upgrade_refresh_pre126_test.go: a pinned-digest oracle, a refresh leg,
// and a user-modified-preserved leg.
//
// Map keys are snapshot file stems (<name>.<tag>). Two snapshot sets exist:
//
//   - .presurface.md: the body on origin/main d15787fd, the commit this
//     change is based on:
//     git show d15787fd:plugins/mindspec/skills/<name>/SKILL.md | shasum -a 256
//   - .pre130.md: the v0.16.0 body (8735fbfb) of the five skills that
//     d15787fd changed without a snapshot, so a v0.16.0 install also
//     refreshes:
//     git show 8735fbfb:plugins/mindspec/skills/<name>/SKILL.md | shasum -a 256
//
// The three lifecycle-literal digests come from the base-binary install
// capture: build ./cmd/mindspec from `git archive 8735fbfb` (claude.go is
// unchanged at d15787fd), run `setup claude` in a throwaway `git init`
// repo, then `shasum -a 256` the installed SKILL.md files.
import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

var upgradeRefreshPresurfaceDigests = map[string]string{
	"ms-bead-cycle.presurface":        "e185262fa5f986c30424ec059563f5149cde5cca81bad414af52891fc9d350ff",
	"ms-bead-fix.presurface":          "d10c8ef1424ac200a63fa1bc3a36ceceda3dc444ab6e1128162383d5b0c876e5",
	"ms-bead-impl.presurface":         "17fc9d2b65fccc8a52b45e9b8fd143a6c6307999363b1284368402ca6557aa98",
	"ms-panel-run.presurface":         "bb9ebb9cd89fbc547125a553fb515f3a77554b935396d906cfac53c70b12949c",
	"ms-panel-tally.presurface":       "e1401a0e62a1de3c8a045b4c503c76377bc33b0c30a6cd58dac14ccb172e4a81",
	"ms-spec-autopilot.presurface":    "109f39f8b58baca9e40edf811a2e34d6efdf77a97c688a90cfb328ec2d00af69",
	"ms-spec-final-review.presurface": "7f2dc80fab6661dd149ad7529445c45cc9e2b7f48df8add9e47d993fc2c5910a",
	"ms-spec-grill.presurface":        "794441e4db6cdeaf8aee3ae8e90c175f15ce44a6707ac02b01462873547be21e",
	"ms-spec-approve.presurface":      "08b13b560c9a641251d0cdb3f375447ae21abc03bc89afcb0caccf9481f8d194",
	"ms-plan-approve.presurface":      "0fcc54b092852f4795cdd00927058a32e1f5589f74cfab351083d2c3880c1437",
	"ms-impl-approve.presurface":      "43a2b524b8f8c951aee435769ca298392f1b0f4c45d4a6ea798434f9b91caec2",
	"ms-bead-cycle.pre130":            "8a2e2de2b5037404d0aea95db92c62b2bcd00aa30fd4c7e5c7a3a14988a2a73a",
	"ms-bead-impl.pre130":             "c3e6140a497480656b5212ea6562981017767ede232604a39c14933de544a481",
	"ms-panel-run.pre130":             "26f976a4ab886cac184bd00001a30c3ae4172c07a2e980cef35891e187a02cfb",
	"ms-panel-tally.pre130":           "574e3436aff11776c7fc61a8d799f5d4ae040af16575127220d6718cf416faec",
	"ms-spec-final-review.pre130":     "26112529422fc135420a64e2dfa63a94b72f5d57cbf39348450cb3bb05e3860a",
}

// readPresurfaceSnapshot reads historical_skills/<stem>.md and returns the
// skill name (the stem up to the first dot) and the body.
func readPresurfaceSnapshot(t *testing.T, stem string) (string, string) {
	t.Helper()
	data, err := historicalSkillsFS.ReadFile("historical_skills/" + stem + ".md")
	if err != nil {
		t.Fatalf("reading historical_skills/%s.md: %v (snapshot missing or renamed)", stem, err)
	}
	name, _, _ := strings.Cut(stem, ".")
	return name, string(data)
}

func TestUpgradeRefreshPresurface_DigestOracle(t *testing.T) {
	for stem, want := range upgradeRefreshPresurfaceDigests {
		stem, want := stem, want
		t.Run(stem, func(t *testing.T) {
			_, body := readPresurfaceSnapshot(t, stem)
			sum := sha256.Sum256([]byte(body))
			if got := hex.EncodeToString(sum[:]); got != want {
				t.Errorf("historical_skills/%s.md digest mismatch:\n  got:  %s\n  want: %s", stem, got, want)
			}
		})
	}
}

func TestUpgradeRefreshPresurface_RefreshesEachEditedSkill(t *testing.T) {
	canonical := skillFiles()
	for stem := range upgradeRefreshPresurfaceDigests {
		stem := stem
		t.Run(stem, func(t *testing.T) {
			name, prior := readPresurfaceSnapshot(t, stem)
			want, ok := canonical[name]
			if !ok {
				t.Fatalf("skillFiles() has no %q entry", name)
			}
			if prior == want {
				t.Fatalf("%s: presurface snapshot is identical to the canonical body; the refresh assertion would be vacuous", name)
			}

			skillsDir := filepath.Join(t.TempDir(), ".claude", "skills")
			writeExisting(t, skillsDir, name, prior)

			r := &Result{}
			if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), canonical, false, r); err != nil {
				t.Fatalf("installSkills: %v", err)
			}
			relPath := filepath.Join(".claude", "skills", name, "SKILL.md")
			if !containsPath(r.Refreshed, relPath) {
				t.Errorf("%s: expected Refreshed to record %s; got Refreshed=%v Notices=%v", name, relPath, r.Refreshed, r.Notices)
			}
			if got := readSkill(t, skillsDir, name); got != want {
				t.Errorf("%s: refreshed body does not equal the canonical content", name)
			}
		})
	}
}

func TestUpgradeRefreshPresurface_UserModifiedPreserved(t *testing.T) {
	canonical := skillFiles()
	for stem := range upgradeRefreshPresurfaceDigests {
		stem := stem
		t.Run(stem, func(t *testing.T) {
			name, prior := readPresurfaceSnapshot(t, stem)
			userBody := prior + "\n<!-- local operator note -->\n"

			skillsDir := filepath.Join(t.TempDir(), ".claude", "skills")
			writeExisting(t, skillsDir, name, userBody)

			r := &Result{}
			if err := installSkills(skillsDir, filepath.Join(".claude", "skills"), canonical, false, r); err != nil {
				t.Fatalf("installSkills: %v", err)
			}
			relPath := filepath.Join(".claude", "skills", name, "SKILL.md")
			if containsPath(r.Refreshed, relPath) {
				t.Errorf("%s: a user-modified body must not be refreshed", name)
			}
			if got := readSkill(t, skillsDir, name); got != userBody {
				t.Errorf("%s: a user-modified body must be left in place unmodified", name)
			}
			found := false
			for _, n := range r.Notices {
				if strings.HasPrefix(n, relPath) && strings.Contains(n, "user-modified") {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: expected a user-modified notice for %s; got Notices=%v", name, relPath, r.Notices)
			}
		})
	}
}
