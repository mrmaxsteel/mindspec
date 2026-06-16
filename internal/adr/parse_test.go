package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testADR1 = `# ADR-0001: Test Decision

- **Date**: 2026-01-01
- **Status**: Accepted
- **Domain(s)**: core, context-system
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Decision
Some decision.
`

const testADR2 = `# ADR-0002: Another Decision

- **Date**: 2026-01-02
- **Status**: Accepted
- **Domain(s)**: workflow
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Decision
Another.
`

const testADR3 = `# ADR-0003: Superseded One

- **Date**: 2026-01-03
- **Status**: Superseded
- **Domain(s)**: core
- **Supersedes**: n/a
- **Superseded-by**: ADR-0005

## Decision
Old.
`

func setupTestADRs(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	os.MkdirAll(adrDir, 0o755)
	os.WriteFile(filepath.Join(adrDir, "ADR-0001.md"), []byte(testADR1), 0o644)
	os.WriteFile(filepath.Join(adrDir, "ADR-0002.md"), []byte(testADR2), 0o644)
	os.WriteFile(filepath.Join(adrDir, "ADR-0003.md"), []byte(testADR3), 0o644)
	return root
}

func TestParseADR(t *testing.T) {
	root := setupTestADRs(t)
	path := filepath.Join(root, "docs", "adr", "ADR-0001.md")

	a, err := ParseADR(path)
	if err != nil {
		t.Fatalf("ParseADR: %v", err)
	}

	if a.ID != "ADR-0001" {
		t.Errorf("ID = %q, want ADR-0001", a.ID)
	}
	if a.Title != "Test Decision" {
		t.Errorf("Title = %q, want %q", a.Title, "Test Decision")
	}
	if a.Date != "2026-01-01" {
		t.Errorf("Date = %q, want 2026-01-01", a.Date)
	}
	if a.Status != "Accepted" {
		t.Errorf("Status = %q, want Accepted", a.Status)
	}
	if len(a.Domains) != 2 || a.Domains[0] != "core" || a.Domains[1] != "context-system" {
		t.Errorf("Domains = %v, want [core context-system]", a.Domains)
	}
	if a.Supersedes != "" {
		t.Errorf("Supersedes = %q, want empty (n/a)", a.Supersedes)
	}
	if a.SupersededBy != "" {
		t.Errorf("SupersededBy = %q, want empty (n/a)", a.SupersededBy)
	}
}

func TestParseADR_SupersededBy(t *testing.T) {
	root := setupTestADRs(t)
	path := filepath.Join(root, "docs", "adr", "ADR-0003.md")

	a, err := ParseADR(path)
	if err != nil {
		t.Fatalf("ParseADR: %v", err)
	}

	if a.Status != "Superseded" {
		t.Errorf("Status = %q, want Superseded", a.Status)
	}
	if a.SupersededBy != "ADR-0005" {
		t.Errorf("SupersededBy = %q, want ADR-0005", a.SupersededBy)
	}
}

func TestParseADR_StatusQualifiers(t *testing.T) {
	// Authors append provenance qualifiers to the Status line — e.g. the
	// live ADR-0029 case "Accepted (Finalized in spec 090 Bead 1)".
	// Status must normalize to the first token; StatusRaw must preserve
	// the full value.
	cases := []struct {
		name    string
		raw     string
		status  string
		rawWant string
	}{
		{
			name:    "live ADR-0029 case",
			raw:     "Accepted (Finalized in spec 090 Bead 1)",
			status:  "Accepted",
			rawWant: "Accepted (Finalized in spec 090 Bead 1)",
		},
		{
			name:    "withdrawn with supersede note",
			raw:     "Withdrawn (superseded by ADR-0015 — consolidated supply-chain policy)",
			status:  "Withdrawn",
			rawWant: "Withdrawn (superseded by ADR-0015 — consolidated supply-chain policy)",
		},
		{
			name:    "bare accepted",
			raw:     "Accepted",
			status:  "Accepted",
			rawWant: "Accepted",
		},
		{
			name:    "bare proposed",
			raw:     "Proposed",
			status:  "Proposed",
			rawWant: "Proposed",
		},
		{
			name:    "qualifier attached without space",
			raw:     "Accepted(panel round 3)",
			status:  "Accepted",
			rawWant: "Accepted(panel round 3)",
		},
		{
			name:    "trailing punctuation",
			raw:     "Superseded; see ADR-0010",
			status:  "Superseded",
			rawWant: "Superseded; see ADR-0010",
		},
		{
			name:    "proposed with spec qualifier",
			raw:     "Proposed (part of spec 091)",
			status:  "Proposed",
			rawWant: "Proposed (part of spec 091)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			adrDir := filepath.Join(root, "docs", "adr")
			os.MkdirAll(adrDir, 0o755)
			content := "# ADR-0099: Qualifier Test\n\n- **Date**: 2026-06-01\n- **Status**: " + tc.raw + "\n- **Domain(s)**: core\n\n## Decision\nX.\n"
			path := filepath.Join(adrDir, "ADR-0099.md")
			os.WriteFile(path, []byte(content), 0o644)

			a, err := ParseADR(path)
			if err != nil {
				t.Fatalf("ParseADR: %v", err)
			}
			if a.Status != tc.status {
				t.Errorf("Status = %q, want %q", a.Status, tc.status)
			}
			if a.StatusRaw != tc.rawWant {
				t.Errorf("StatusRaw = %q, want %q", a.StatusRaw, tc.rawWant)
			}
		})
	}
}

func TestParseADR_DomainsParenAware(t *testing.T) {
	// mindspec-wgcw: the Domain(s) tokenizer must split only on commas
	// at bracket depth 0 — parenthesized qualifiers with embedded commas
	// (the lola spec-044 case) are single tokens.
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "lola spec-044 case",
			raw:  "core, webapp (`app/`, react navigation native-stack), api/app/...",
			want: []string{"core", "webapp (`app/`, react navigation native-stack)", "api/app/..."},
		},
		{
			name: "simple comma list unchanged",
			raw:  "core, context-system, workflow",
			want: []string{"core", "context-system", "workflow"},
		},
		{
			name: "nested parens",
			raw:  "alpha (outer (inner, deep), tail), beta",
			want: []string{"alpha (outer (inner, deep), tail)", "beta"},
		},
		{
			name: "posix-class brackets and braces",
			raw:  "regex ([[:alpha:]]{2,4}, [[:digit:]]+), core",
			want: []string{"regex ([[:alpha:]]{2,4}, [[:digit:]]+)", "core"},
		},
		{
			name: "square brackets",
			raw:  "matrix [a, b], vector",
			want: []string{"matrix [a, b]", "vector"},
		},
		{
			name: "unbalanced open paren degrades to one token",
			raw:  "broken (no close, here, more",
			want: []string{"broken (no close, here, more"},
		},
		{
			name: "unmatched close bracket clamps at depth 0",
			raw:  "weird), still, splits",
			want: []string{"weird)", "still", "splits"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			adrDir := filepath.Join(root, "docs", "adr")
			os.MkdirAll(adrDir, 0o755)
			content := "# ADR-0098: Domain Tokenizer Test\n\n- **Date**: 2026-06-01\n- **Status**: Accepted\n- **Domain(s)**: " + tc.raw + "\n\n## Decision\nX.\n"
			path := filepath.Join(adrDir, "ADR-0098.md")
			os.WriteFile(path, []byte(content), 0o644)

			a, err := ParseADR(path)
			if err != nil {
				t.Fatalf("ParseADR: %v", err)
			}
			if len(a.Domains) != len(tc.want) {
				t.Fatalf("Domains = %q (len %d), want %q (len %d)", a.Domains, len(a.Domains), tc.want, len(tc.want))
			}
			for i := range tc.want {
				// ParseADR lowercases and trims each token.
				wantNorm := strings.ToLower(strings.TrimSpace(tc.want[i]))
				if a.Domains[i] != wantNorm {
					t.Errorf("Domains[%d] = %q, want %q", i, a.Domains[i], wantNorm)
				}
			}
		})
	}
}

func TestFilterADRs_QualifiedAcceptedStatus(t *testing.T) {
	// A parsed ADR whose Status line carries a qualifier must still be
	// treated as Accepted by FilterADRs (normalization happens at parse
	// time, so the in-memory Status is already bare).
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	os.MkdirAll(adrDir, 0o755)
	content := "# ADR-0029: Supply Chain\n\n- **Date**: 2026-05-01\n- **Status**: Accepted (Finalized in spec 090 Bead 1)\n- **Domain(s)**: workflow\n\n## Decision\nY.\n"
	os.WriteFile(filepath.Join(adrDir, "ADR-0029.md"), []byte(content), 0o644)

	adrs, err := ScanADRs(root)
	if err != nil {
		t.Fatalf("ScanADRs: %v", err)
	}
	filtered := FilterADRs(adrs, []string{"workflow"})
	if len(filtered) != 1 {
		t.Fatalf("got %d filtered ADRs, want 1 (qualified Accepted status must count)", len(filtered))
	}
}

func TestDisplayStatus(t *testing.T) {
	withRaw := ADR{Status: "Accepted", StatusRaw: "Accepted (note)"}
	if got := withRaw.DisplayStatus(); got != "Accepted (note)" {
		t.Errorf("DisplayStatus = %q, want raw form", got)
	}
	bare := ADR{Status: "Proposed"}
	if got := bare.DisplayStatus(); got != "Proposed" {
		t.Errorf("DisplayStatus = %q, want normalized fallback", got)
	}
}

func TestScanADRs_Sorted(t *testing.T) {
	root := setupTestADRs(t)

	adrs, err := ScanADRs(root)
	if err != nil {
		t.Fatalf("ScanADRs: %v", err)
	}

	if len(adrs) != 3 {
		t.Fatalf("got %d ADRs, want 3", len(adrs))
	}

	// Verify sorted by ID
	for i := 1; i < len(adrs); i++ {
		if adrs[i].ID < adrs[i-1].ID {
			t.Errorf("ADRs not sorted: %s before %s", adrs[i-1].ID, adrs[i].ID)
		}
	}

	// Verify all fields populated
	if adrs[0].Title != "Test Decision" {
		t.Errorf("adrs[0].Title = %q, want %q", adrs[0].Title, "Test Decision")
	}
	if adrs[0].Date != "2026-01-01" {
		t.Errorf("adrs[0].Date = %q, want 2026-01-01", adrs[0].Date)
	}
}

func TestScanADRs_EmptyDir(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "adr"), 0o755)

	adrs, err := ScanADRs(root)
	if err != nil {
		t.Fatalf("ScanADRs: %v", err)
	}
	if len(adrs) != 0 {
		t.Errorf("got %d ADRs, want 0", len(adrs))
	}
}

func TestFilterADRs(t *testing.T) {
	adrs := []ADR{
		{ID: "ADR-0001", Status: "Accepted", Domains: []string{"core", "context-system"}},
		{ID: "ADR-0002", Status: "Accepted", Domains: []string{"workflow"}},
		{ID: "ADR-0003", Status: "Superseded", Domains: []string{"core"}},
	}

	filtered := FilterADRs(adrs, []string{"context-system"})
	if len(filtered) != 1 {
		t.Fatalf("got %d filtered ADRs, want 1", len(filtered))
	}
	if filtered[0].ID != "ADR-0001" {
		t.Errorf("filtered[0].ID = %q, want ADR-0001", filtered[0].ID)
	}
}

func TestFilterADRs_MultiDomain(t *testing.T) {
	adrs := []ADR{
		{ID: "ADR-0001", Status: "Accepted", Domains: []string{"core", "context-system"}},
		{ID: "ADR-0002", Status: "Accepted", Domains: []string{"workflow", "context-system"}},
	}

	filtered := FilterADRs(adrs, []string{"context-system", "workflow"})
	if len(filtered) != 2 {
		t.Fatalf("got %d filtered ADRs, want 2", len(filtered))
	}
}

func TestNextID(t *testing.T) {
	root := setupTestADRs(t)

	id, err := NextID(root)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "0004" {
		t.Errorf("NextID = %q, want 0004", id)
	}
}

func TestNextID_Empty(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "adr"), 0o755)

	id, err := NextID(root)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "0001" {
		t.Errorf("NextID = %q, want 0001", id)
	}
}

// setupNextIDFixture writes empty ADR files with the given base names into a
// fresh root's ADR dir and returns the root. NextID only reads filenames, so
// the contents are irrelevant.
func setupNextIDFixture(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	adrDir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(adrDir, name), []byte("# stub\n"), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	return root
}

// TestNextID_SluggedOnly is the RED-on-revert guard for bn3u: a dir whose ADRs
// are ALL slugged (ADR-NNNN-slug.md) must count toward maxNum. The old
// whole-suffix strconv.Atoi("0034-some-slug") fails for every file, leaving
// maxNum at 0 and NextID returning a colliding 0001.
func TestNextID_SluggedOnly(t *testing.T) {
	root := setupNextIDFixture(t,
		"ADR-0025-jsonl-as-build-artifact.md",
		"ADR-0034-some-slug.md",
	)

	id, err := NextID(root)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "0035" {
		t.Errorf("NextID = %q, want 0035 (slugged ADRs must count)", id)
	}
}

func TestNextID_BareSlugged(t *testing.T) {
	root := setupNextIDFixture(t, "ADR-0034.md")

	id, err := NextID(root)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "0035" {
		t.Errorf("NextID = %q, want 0035", id)
	}
}

func TestNextID_SingleDigit(t *testing.T) {
	root := setupNextIDFixture(t, "ADR-1.md")

	id, err := NextID(root)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "0002" {
		t.Errorf("NextID = %q, want 0002", id)
	}
}

func TestNextID_MixedBareAndSlugged(t *testing.T) {
	root := setupNextIDFixture(t,
		"ADR-0007.md",
		"ADR-0025-jsonl-as-build-artifact.md",
		"ADR-0035-agent-error-contract.md",
	)

	id, err := NextID(root)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "0036" {
		t.Errorf("NextID = %q, want 0036 (max leading-integer + 1)", id)
	}
}

func TestNextID_MalformedSkipped(t *testing.T) {
	root := setupNextIDFixture(t,
		"ADR-foo.md",
		"ADR-0009-real.md",
	)

	id, err := NextID(root)
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != "0010" {
		t.Errorf("NextID = %q, want 0010 (malformed ADR-foo.md skipped)", id)
	}
}

// TestParseADR_NonListDomainLine is a REGRESSION LOCK (not a RED pin): the
// parser at parse.go ~L74 keys off strings.Contains(trimmed, "**Domain(s)**:"),
// so it already accepts BOTH the markdown-list form ("- **Domain(s)**: ...")
// and the bare non-list form ("**Domain(s)**: ..."). This test pins that the
// non-list form keeps parsing to the same Domains slice, so a future "tighten
// to list-only" refactor of parse.go can't silently drop the bare form.
func TestParseADR_NonListDomainLine(t *testing.T) {
	cases := []struct {
		name        string
		domainLine  string
		wantDomains []string
	}{
		{
			name:        "non-list form (no leading dash)",
			domainLine:  "**Domain(s)**: foo, bar",
			wantDomains: []string{"foo", "bar"},
		},
		{
			name:        "list form (leading dash)",
			domainLine:  "- **Domain(s)**: foo, bar",
			wantDomains: []string{"foo", "bar"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			adrDir := filepath.Join(root, "docs", "adr")
			os.MkdirAll(adrDir, 0o755)
			content := "# ADR-0099: Domain Line Form\n\n- **Date**: 2026-06-01\n- **Status**: Accepted\n" + tc.domainLine + "\n\n## Decision\nX.\n"
			path := filepath.Join(adrDir, "ADR-0099.md")
			os.WriteFile(path, []byte(content), 0o644)

			a, err := ParseADR(path)
			if err != nil {
				t.Fatalf("ParseADR: %v", err)
			}
			if len(a.Domains) != len(tc.wantDomains) {
				t.Fatalf("Domains = %v, want %v", a.Domains, tc.wantDomains)
			}
			for i, want := range tc.wantDomains {
				if a.Domains[i] != want {
					t.Errorf("Domains[%d] = %q, want %q (full %v)", i, a.Domains[i], want, a.Domains)
				}
			}
		})
	}
}
