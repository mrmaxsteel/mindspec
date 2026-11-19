package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

func s134SkillCopies(t *testing.T, name string) map[string]string {
	t.Helper()
	tracked, err := os.ReadFile(filepath.Join(repoRoot(t), ".claude", "skills", name, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"plugin": pluginmindspec.SkillFiles()[name],
		"claude": string(tracked),
	}
}

func s134LeasedCheckError(body string) error {
	start := strings.Index(body, "3. Dispatch by the returned kind:")
	submit := strings.Index(body, "4. Submit the evidence")
	if start < 0 || submit <= start {
		return fmt.Errorf("missing leased dispatch or submission boundary")
	}
	step := body[start:submit]
	for _, required := range []string{
		"After each implementation or fix commit, run `mindspec complete <bead> --check`.",
		"Run this check inside the leased step before submission.",
		"failing gates and recovery lines to the implementer or fixer under the same lease.",
		"Keep the same actor. Do not submit a failed check.",
		"Wait for the fixed commit and run the check again.",
	} {
		if !strings.Contains(step, required) {
			return fmt.Errorf("missing pre-submit leased check rule: %s", required)
		}
	}
	for _, required := range []string{
		"Execute `mindspec complete` only when its command appears in `run`, except for the content check.",
		"The only exception is `mindspec complete <bead> --check` inside a leased implement or fix step.",
		"This is the only `complete` call allowed outside `run`. It must carry `--check`.",
	} {
		if !strings.Contains(body, required) {
			return fmt.Errorf("missing completion authority rule: %s", required)
		}
	}
	// Every example of the exceptional call must retain its read-only flag.
	commands := regexp.MustCompile("mindspec complete <bead>([^`\n]*)").FindAllStringSubmatch(body, -1)
	for _, command := range commands {
		if strings.TrimSpace(command[1]) != "--check" {
			return fmt.Errorf("content check must carry --check: %s", command[0])
		}
	}
	// The diagram has two leased actor steps: implementation and fix.
	parts := strings.SplitN(body, "## Sequence\n", 2)
	if len(parts) != 2 {
		return fmt.Errorf("missing sequence")
	}
	diagram := strings.SplitN(parts[1], "## NOT READY", 2)[0]
	rounds := strings.Split(diagram, "/ms-panel-run")
	if len(rounds) != 3 {
		return fmt.Errorf("expected implementation and fix sequences")
	}
	for _, round := range rounds[:2] {
		commit := strings.LastIndex(round, "implementation commit SHA")
		check := strings.LastIndex(round, "mindspec complete <bead> --check")
		submit := strings.LastIndex(round, "mindspec submit <step> --lease <token> --commit <oid>")
		if commit < 0 || check <= commit || submit <= check {
			return fmt.Errorf("sequence must order commit, content check, submission, panel")
		}
	}
	return nil
}

func TestS134Skills_LeasedContentCheck(t *testing.T) {
	for name, body := range s134SkillCopies(t, "ms-bead-cycle") {
		t.Run(name, func(t *testing.T) {
			if err := s134LeasedCheckError(body); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct{ name, old, replacement string }{
				{"check-after-submit", "   After each implementation or fix commit, run `mindspec complete <bead> --check`.\n", ""},
				{"check-without-flag", "mindspec complete <bead> --check", "mindspec complete <bead>"},
				{"missing-run-authority", "Execute `mindspec complete` only when its command appears in `run`, except for the content check.\n", ""},
				{"failure-submits", "Keep the same actor. Do not submit a failed check.", "Submit a failed check."},
				{"failure-new-lease", "under the same lease.", "under a new lease."},
			} {
				t.Run(tc.name, func(t *testing.T) {
					wrong := strings.ReplaceAll(body, tc.old, tc.replacement)
					if tc.name == "check-after-submit" {
						wrong = strings.Replace(wrong, "5. Execute", tc.old+"5. Execute", 1)
					}
					if wrong == body || s134LeasedCheckError(wrong) == nil {
						t.Fatal("wrong instruction passed the shipped leased-check predicate")
					}
				})
			}
		})
	}
}

func TestS134Skills_CycleLeaseRecovery(t *testing.T) {
	for name, body := range s134SkillCopies(t, "ms-bead-cycle") {
		t.Run(name, func(t *testing.T) {
			lease := strings.SplitN(strings.SplitN(body, "2. For each selected action", 2)[1], "3. Dispatch", 2)[0]
			accept := strings.SplitN(strings.SplitN(body, "- **ACCEPT (the default)**", 2)[1], "- **DISAGREE", 2)[0]
			for _, tc := range []struct{ section, required string }{
				{lease, "Do not take a lease that your own still-running actor holds."},
				{lease, "Wait for that actor or stop it first."},
				{accept, "Return through `mindspec next-action --apply` and `mindspec lease` before a new dispatch."},
				{accept, "Reuse the current lease only while it is valid."},
			} {
				if !strings.Contains(tc.section, tc.required) {
					t.Errorf("missing lease recovery instruction: %s", tc.required)
				}
			}
		})
	}
}

func TestS134Skills_ReadmeAutonomyNavigation(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Governed autonomy\n", "[Governed autonomy](#governed-autonomy)", "[Autonomy](#governed-autonomy)"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing autonomy heading or navigation: %s", want)
		}
	}
	if strings.Contains(strings.ToLower(string(body)), "autonomy ladder") || strings.Contains(string(body), "#the-autonomy-ladder") {
		t.Error("obsolete autonomy ladder heading or anchor")
	}
}
