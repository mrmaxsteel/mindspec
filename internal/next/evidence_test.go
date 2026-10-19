package next

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func evidenceRefusal(t *testing.T, path string, want string) {
	t.Helper()
	root, fx := gateFixture(t)
	gateReport(t, reportWithFailures("MF-3"))
	opts := allOverrideOptions(t)
	opts.EvidencePath = path
	assertGateRefusal(t, root, fx, fx.BeadID, opts, want)
}
func TestAllowNotReady_EvidenceTooLargeRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 65537)), 0600); err != nil {
		t.Fatal(err)
	}
	evidenceRefusal(t, path, "evidence must contain 1 to 65536 bytes")
}
func TestAllowNotReady_EvidenceDirectoryRefuses(t *testing.T) {
	evidenceRefusal(t, t.TempDir(), "evidence must be a readable regular file")
}
func TestAllowNotReady_EvidenceEmptyRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	evidenceRefusal(t, path, "evidence must contain 1 to 65536 bytes")
}
func TestAllowNotReady_EvidenceUnreadableRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	evidenceRefusal(t, path, "no such file")
}
func TestAllowNotReady_EvidenceBounds(t *testing.T) {
	for _, size := range []int{1, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			data := []byte(strings.Repeat("x", size))
			path := filepath.Join(t.TempDir(), "evidence.md")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadOverrideEvidence(path)
			if err != nil {
				t.Fatal(err)
			}
			if got.Path != path || got.Bytes != size || got.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
				t.Fatalf("audit = %+v", got)
			}
		})
	}
}
