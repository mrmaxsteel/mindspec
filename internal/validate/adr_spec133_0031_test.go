package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestADRAmendments_Spec133ADR0031(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(findProjectRoot(t), ".mindspec/adr/ADR-0031-doc-sync-gate.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, body, ok := strings.Cut(string(data), "## Amendment (Spec 133)")
	if !ok {
		t.Fatal("missing amendment")
	}
	for _, anchor := range []string{"--doc-owner", "--no-doc-impact", "--check", "every source change", "spec branch gets no exemption", "open deferral id", "old skew record"} {
		if !strings.Contains(body, anchor) {
			t.Errorf("missing amendment anchor %q", anchor)
		}
	}
}
