package contextpack

import (
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/tokenize"
)

// Spec 106 Bead 2 (AC6): a context pack assembles byte-identical CONTENT
// sections on a flat tree vs an otherwise-identical canonical tree — same
// Bead/Spec/Cited ADRs/Plan/Domain Docs sections, same bytes — so the flatten
// silently drops no spec, domain, or ADR. The budgeter resolves the spec dir
// and domain docs through the Bead-1 tier-aware accessors (Req 3); the ADR
// store is already tier-aware via workspace.ADRDir.

// flatFixture mirrors standardFixture under the FLAT layout: lifecycle docs
// live directly under .mindspec/ (no /docs/ segment).
func flatFixture() []fixtureFile {
	return []fixtureFile{
		{rel: ".mindspec/specs/099-test/spec.md", body: fixtureSpecBody},
		{rel: ".mindspec/specs/099-test/plan.md", body: fixturePlanBody},
		{rel: ".mindspec/adr/ADR-9001.md", body: fixtureADR1},
		{rel: ".mindspec/adr/ADR-9002.md", body: fixtureADR2},
		{rel: ".mindspec/domains/context-system/overview.md", body: fixtureOverview},
		{rel: ".mindspec/domains/context-system/interfaces.md", body: fixtureInterfaces},
	}
}

// sectionsBeforeProvenance returns the bundle up to (but excluding) the
// ## Provenance block. The provenance block embeds the resolved spec/plan file
// PATHS (which differ by layout by design); every CONTENT section before it is
// layout-neutral, so AC6 is asserted over this prefix.
func sectionsBeforeProvenance(t *testing.T, out []byte) string {
	t.Helper()
	s := string(out)
	idx := strings.Index(s, "## Provenance")
	if idx < 0 {
		t.Fatalf("output missing ## Provenance block:\n%s", s)
	}
	return s[:idx]
}

func TestContextPackFlatVsCanonicalByteIdentical(t *testing.T) {
	build := func(files []fixtureFile) []byte {
		root := buildFixtureRepo(t, files)
		withChdir(t, root)
		restore := SetBeadShowForTest(mkBeadShow(standardBeadEntry()))
		defer restore()
		out, err := BuildBead("bead-fixture", 8000, tokenize.Approx{})
		if err != nil {
			t.Fatalf("BuildBead: %v", err)
		}
		return out
	}

	canonical := build(standardFixture())
	flat := build(flatFixture())

	cSec := sectionsBeforeProvenance(t, canonical)
	fSec := sectionsBeforeProvenance(t, flat)
	if cSec != fSec {
		t.Fatalf("content sections differ flat vs canonical (AC6 byte-identity).\n--- canonical ---\n%s\n--- flat ---\n%s", cSec, fSec)
	}

	// Sanity: the load-bearing content actually rendered (not two empty packs).
	for _, want := range []string{"## Cited ADRs", "ADR-9001", "## Domain Docs", "context-system", "## Plan"} {
		if !strings.Contains(cSec, want) {
			t.Errorf("expected assembled sections to contain %q; got:\n%s", want, cSec)
		}
	}
}
