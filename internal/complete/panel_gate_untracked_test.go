package complete

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func TestPanelGate_UntrackedIgnoredRegistration(t *testing.T) {
	orig := panelSkipEnvFn
	t.Cleanup(func() { panelSkipEnvFn = orig })
	panelSkipEnvFn = func() bool { return true }
	t.Run("registration", testPanelGateUntrackedIgnoredRegistration)
	if !panelSkipEnvFn() {
		t.Fatal("registration test leaked its panelSkipEnvFn override after cleanup")
	}
}

func testPanelGateUntrackedIgnoredRegistration(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	var messages []string
	for _, tracked := range []bool{true, false} {
		name := "untracked-ignored"
		if tracked {
			name = "tracked"
		}
		t.Run(name, func(t *testing.T) {
			const specID, beadID = "129-review", "mindspec-review.8"
			root, beadSHA := setupPanelGateRepo(t, specID, beadID)
			specDir := makeFlatSpecDir(t, root, specID)
			if err := gitutil.EnsureGitignoreEntriesWithHeader(root, "# local reviews", gitutil.ReviewIgnoreEntries...); err != nil {
				t.Fatal(err)
			}
			gitRun(t, root, "add", ".gitignore")
			gitRun(t, root, "commit", "-q", "-m", "ignore reviews")
			writeCoLocatedPanel(t, specDir, "bd08", panel.Panel{
				BeadID: bp(beadID), Spec: specID, Round: 1, ExpectedReviewers: 6,
				ReviewedHeadSHA: beadSHA,
			}, subThresholdVerdicts())
			panelPath := filepath.Join(".mindspec", "specs", specID, "reviews", "bd08", "panel.json")
			if tracked {
				gitRun(t, root, "add", "-f", filepath.Dir(panelPath))
				gitRun(t, root, "commit", "-q", "-m", "track existing review")
			}
			cmd := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", "--", panelPath)
			err := cmd.Run()
			if tracked && err != nil {
				t.Fatalf("tracked fixture has no tracked registration: %v", err)
			}
			if !tracked {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 1 {
					t.Fatalf("untracked registration must not be in git: %v", err)
				}
				gitRun(t, root, "check-ignore", "-q", "--", panelPath)
			}
			// Equal clean-status facts prevent git dirt from masquerading as
			// the registration-driven threshold Block.
			status, err := exec.Command("git", "-C", root, "status", "--porcelain").CombinedOutput()
			if err != nil || len(status) != 0 {
				t.Fatalf("fixture must have a clean status: %v\n%s", err, status)
			}
			orig := panelSkipEnvFn
			t.Cleanup(func() { panelSkipEnvFn = orig })
			panelSkipEnvFn = func() bool { return false }
			matches, err := panelGate(beadID, panelGateRoots(root, "", specID), "", true, nil)
			reg := firstGateRegistration(matches)
			if reg == nil || reg.Panel.BeadID == nil || *reg.Panel.BeadID != beadID {
				t.Fatalf("gate must match the registration: %+v", reg)
			}
			if err == nil || !strings.Contains(err.Error(), "2/6 APPROVE") {
				t.Fatalf("registration must drive a sub-threshold Block: %v", err)
			}
			messages = append(messages, err.Error())
		})
	}
	if len(messages) != 2 || messages[0] != messages[1] {
		t.Fatalf("tracked and untracked-ignored registrations must yield identical Blocks: %q", messages)
	}
}
