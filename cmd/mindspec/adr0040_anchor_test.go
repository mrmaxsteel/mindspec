package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adr0040_anchor_test.go — spec 123 AC-18: the amended ADR-0040 carries
// the consumer-identity clause, and the code that first cites it (the
// managed-content renderers in internal/bootstrap + internal/setup, and
// the models:/commands: config-block sites in internal/doctor +
// internal/config + this package) references ADR-0040 by name — so the
// ADR-divergence gate sees the declared touchpoint (R9's "same bead as
// the first citing code" rule).

// TestADR0040ConsumerIdentityClause_Anchored is the repo-root-relative
// half: the ADR file itself contains the consumer-identity clause.
func TestADR0040ConsumerIdentityClause_Anchored(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootFromTestDir(t)
	adrPath := filepath.Join(repoRoot, ".mindspec", "adr", "ADR-0040-orchestration-layering-ratchet.md")
	data, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("reading ADR-0040: %v", err)
	}
	adr := string(data)
	for _, want := range []string{"consumer-identity", "framework-generic"} {
		if !strings.Contains(adr, want) {
			t.Errorf("ADR-0040 must contain %q (spec 123 R9 consumer-identity clause):\n%s", want, adr)
		}
	}
}

// TestADR0040CitedAtManagedContentAndConfigBlockSites pins AC-18's
// citation-site half: every site R9 names — the managed-block
// renderers (bootstrap's starterAgentsMD/appendAgentsBlock, setup
// codex's agentsMDManagedBlock) and both new config blocks
// (internal/doctor's models:/commands: schema scaffolds) — references
// "ADR-0040" by name in its own source, so the ADR-divergence gate sees
// the declared touchpoint at the code that actually cites the clause.
func TestADR0040CitedAtManagedContentAndConfigBlockSites(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootFromTestDir(t)
	sites := []string{
		filepath.Join("internal", "bootstrap", "bootstrap.go"),
		filepath.Join("internal", "setup", "codex.go"),
		filepath.Join("internal", "doctor", "config.go"),
	}
	for _, rel := range sites {
		data, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		if !strings.Contains(string(data), "ADR-0040") {
			t.Errorf("%s must cite ADR-0040 (the consumer-identity clause it implements)", rel)
		}
	}
}
