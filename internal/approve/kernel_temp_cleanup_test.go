package approve

import (
	"os"
	"testing"
)

func TestS134ApproveFixtures_KernelTempCleanup(t *testing.T) {
	caller := t.TempDir()
	t.Setenv("TMPDIR", caller)
	t.Run("document-plan", func(t *testing.T) {
		f := s133Document(t, "enforce", "plan_approve", "pass")
		if _, err := f.approve("plan_approve"); err != nil {
			t.Fatal(err)
		}
	})
	entries, err := os.ReadDir(caller)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("approve fixture leaked into caller TMPDIR: %v", entries)
	}
}
