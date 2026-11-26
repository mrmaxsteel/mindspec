package setup

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
)

func TestClaimIDGuidanceRefreshAndPreserve(t *testing.T) {
	const name = "ms-bead-cycle"
	_, prior := readPresurfaceSnapshot(t, name+".preclaimid")
	const digest = "2612462375082d3ba36e0b9efc74df81ad6ccf2a605dcca60c2d6ac42444468b"
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(prior))); got != digest {
		t.Fatalf("snapshot digest=%s, want %s", got, digest)
	}
	canonical := skillFiles()
	if prior == canonical[name] {
		t.Fatal("claim guidance has not changed")
	}
	for _, policy := range []string{"warn", "enforce"} {
		for _, agent := range []string{".agents", ".claude"} {
			for _, modified := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/modified=%t", policy, agent, modified), func(t *testing.T) {
					rel := filepath.Join(agent, "skills")
					root := t.TempDir()
					if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, ".mindspec", "config.yaml"), []byte("enforcement:\n  panel_presence: "+policy+"\n"), 0644); err != nil {
						t.Fatal(err)
					}
					config.ResetCache()
					t.Cleanup(config.ResetCache)
					cfg, err := config.Load(root)
					if err != nil || cfg.Enforcement.PanelPresence != policy {
						t.Fatalf("policy fixture not loaded: %+v %v", cfg, err)
					}
					dir := filepath.Join(root, rel)
					body := prior
					if modified {
						body += "\n<!-- local instruction -->\n"
					}
					writeExisting(t, dir, name, body)
					result := &Result{}
					if err := installSkills(dir, rel, canonical, false, result); err != nil {
						t.Fatal(err)
					}
					want := canonical[name]
					if modified {
						want = body
					}
					if got := readSkill(t, dir, name); got != want {
						t.Fatal("installed body differs from expected")
					}
					if containsPath(result.Refreshed, filepath.Join(rel, name, "SKILL.md")) == modified {
						t.Fatalf("wrong refresh classification: %+v", result)
					}
					if modified && len(result.Notices) == 0 {
						t.Fatal("missing user-modified notice")
					}
				})
			}
		}
	}
}
