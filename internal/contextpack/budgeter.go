// Package contextpack: budgeter.go implements the spec 088 BuildBead
// entry point. BuildBead emits a deterministic markdown bundle for a
// single bead whose estimated token count (per the supplied
// tokenize.Tokenizer) is <= maxTokens, OR returns an explicit error
// when the must-tier alone exceeds the budget. There is no silent
// truncation of essential content.
//
// Section order (fixed, per spec 088 Requirement 9):
//
//  1. # Bead Context: <Title>
//  2. ## Bead          (must-tier; errors on overflow)
//  3. ## Spec
//  4. ## Cited ADRs    (verbatim ## Decision per cited ADR)
//  5. ## Plan          (bead's section of plan.md)
//  6. ## Domain Docs   (overview + interfaces per domain)
//  7. ## File Paths    (only if file_paths non-empty)
//  8. ## Provenance    (SHA-256 of every input artifact)
//
// Tail-shaving on tiers 2-6 is rune-aligned via
// utf8.DecodeLastRuneInString. The truncation marker is the constant
// string "[truncated]" with NO size suffix — the constant length is
// required for the tail-shave to converge as a fixed-point.
//
// The provenance reserve is dynamic. The Provenance block is rendered
// first with a fixed-width estimated_tokens placeholder; its token
// count becomes the headroom for the budget check. No second-pass
// re-render of the block is performed (the placeholder is patched in
// place at exactly the same byte width).
//
// BuildBead requires bead.metadata.spec_id and performs no repo-root
// fallback scan. The package-level walkFn = filepath.Walk seam is a
// negative-recorder asset used by TestContextPackErrorOnMissingSpecID
// to assert that no fallback walk fires on the missing-spec-id path.
//
// This file MUST NOT import os/exec, internal/gitutil, or
// internal/executor — repository reads go through plain os.ReadFile
// and the bd lookup goes through the existing beadShowFn seam in
// beadctx.go.
package contextpack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mrmaxsteel/mindspec/internal/adr"
	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/tokenize"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"gopkg.in/yaml.v3"
)

// truncationMarker is the literal string appended at every
// tail-shave truncation point. No size suffix — see package doc.
const truncationMarker = "[truncated]"

// excludedRoots are first-path-segments that may never appear in a
// file_paths entry per HC-4. The bundler errors on any such entry.
var excludedRoots = map[string]bool{
	"viz":       true,
	"agentmind": true,
	"bench":     true,
}

// walkFn is a test seam mirroring beadShowFn. BuildBead does NOT
// fallback-scan, so this seam normally records zero invocations; the
// missing-spec-id test installs a recorder to confirm.
var walkFn = filepath.Walk

// SetWalkForTest swaps walkFn for testing and returns a restore func.
func SetWalkForTest(fn func(root string, walkFn filepath.WalkFunc) error) func() {
	orig := walkFn
	walkFn = fn
	return func() { walkFn = orig }
}

// planFrontmatter is the minimal YAML shape BuildBead needs from a
// plan.md frontmatter block. Mirrors the relevant subset of
// validate.PlanFrontmatter without importing the validate package.
type planFrontmatter struct {
	ADRCitations []planADRCite `yaml:"adr_citations"`
}

type planADRCite struct {
	ID string `yaml:"id"`
}

// provLine is a single line in the Provenance block: a key (e.g.
// "bead:<id>") and the hex SHA-256 of the input bytes.
type provLine struct {
	key string
	sha string
}

// budAdrEntry holds the per-ADR data BuildBead needs for both the
// Cited ADRs section (Title, Decision) and the Provenance block
// (Bytes for the SHA).
type budAdrEntry struct {
	ID       string
	Title    string
	Decision string
	Bytes    []byte
	Path     string
}

// budDomainDoc holds one domain doc file (overview.md or interfaces.md).
type budDomainDoc struct {
	Domain string
	Kind   string
	Path   string
	Body   []byte
	Found  bool
}

// budFilePathEntry holds one literal file_paths file.
type budFilePathEntry struct {
	Path  string
	Body  []byte
	Found bool
}

func (c *planADRCite) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		c.ID = strings.TrimSpace(node.Value)
		return nil
	}
	type raw planADRCite
	var r raw
	if err := node.Decode(&r); err != nil {
		return err
	}
	*c = planADRCite(r)
	return nil
}

// BuildBead emits a deterministic markdown bundle for the given bead
// id whose tok.Count(output) <= maxTokens, or returns an error when
// the must-tier alone exceeds the budget. See package-level
// doc-comment for the full algorithm.
func BuildBead(beadID string, maxTokens int, tok tokenize.Tokenizer) ([]byte, error) {
	if tok == nil {
		return nil, fmt.Errorf("BuildBead: tokenizer is nil")
	}

	// Gate-all-ids (ADR-0042 §1, round 9): beadID feeds a `bd show` argv
	// build via beadShowFn — validate BEFORE any bd spawn.
	if err := idvalidate.BeadID(beadID); err != nil {
		return nil, fmt.Errorf("invalid bead id %s: %w", beadID, err)
	}

	// 1. Fetch bead JSON via the existing beadShowFn seam.
	beadJSON, err := beadShowFn("show", beadID, "--json")
	if err != nil {
		return nil, fmt.Errorf("fetching bead %s: %w", beadID, err)
	}
	var entries []beadShowEntry
	if err := json.Unmarshal(beadJSON, &entries); err != nil {
		return nil, fmt.Errorf("parsing bead %s: %w", beadID, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("bead %s not found", beadID)
	}
	e := entries[0]

	// 2. Extract spec_id. NO repo-root walk fallback.
	specID, ok := stringFromMeta(e.Metadata, "spec_id")
	if !ok || specID == "" {
		return nil, fmt.Errorf("bead JSON for %s lacks metadata.spec_id; cannot resolve spec", beadID)
	}
	// R6(c) Join-with-ID gate (ADR-0042 §1, round 6 O3): specID is
	// agent-writable bd-metadata reaching a filepath.Join below —
	// idvalidate-then-join, not waist-routed (this package must not
	// import internal/workspace's composition helpers here beyond
	// SpecsDir, the enumeration-root accessor).
	if err := idvalidate.SpecID(specID); err != nil {
		return nil, fmt.Errorf("bead %s carries an invalid spec_id %s: %w", beadID, specID, err)
	}

	// 3. Resolve spec dir + read spec.md, plan.md. The spec dir is resolved
	// tier-aware (spec 106 Req 3) via the Bead-1 specs enumeration-root
	// accessor (flat .mindspec/specs → canonical .mindspec/docs/specs → legacy
	// docs/specs), keyed off the CWD-relative root the bundler already uses for
	// the ADR store below, so a pack assembles identically on flat/canonical/
	// legacy projects. On a canonical/legacy tree this is byte-identical to the
	// pre-spec .mindspec/docs/specs/<id> join.
	const root = "."
	specDir := filepath.Join(workspace.SpecsDir(root), specID)
	specPath := filepath.Join(specDir, "spec.md")
	planPath := filepath.Join(specDir, "plan.md")

	specBytes, _ := os.ReadFile(specPath) // missing files surface as empty content + zero SHA below
	planBytes, _ := os.ReadFile(planPath)

	// 4. Parse spec.md → impacted domains (sorted).
	var domains []string
	if meta, err := ParseSpec(specDir); err == nil && meta != nil {
		domains = append(domains, meta.Domains...)
	}
	sort.Strings(domains)

	// Spec Goal + Acceptance Criteria text for tier 2.
	specGoal, specAccept := extractSpecSections(string(specBytes))

	// 5. Resolve cited ADRs from plan frontmatter; sort by id ascending.
	citedADRs, _ := parseCitedADRs(string(planBytes))
	sort.Strings(citedADRs)

	var adrEntries []budAdrEntry
	store := adr.NewFileStore(".")
	for _, id := range citedADRs {
		a, gerr := store.Get(id)
		if gerr != nil || a == nil {
			continue
		}
		var b []byte
		if a.Path != "" {
			b, _ = os.ReadFile(a.Path)
		}
		adrEntries = append(adrEntries, budAdrEntry{
			ID:       a.ID,
			Title:    a.Title,
			Decision: adr.ExtractDecision(a.Content),
			Bytes:    b,
			Path:     a.Path,
		})
	}

	// 6. Resolve plan bead section.
	planSection := extractPlanBeadSection(string(planBytes), beadID, e.Title)

	// 7. Resolve domain docs.
	var domainDocs []budDomainDoc
	domainsRoot := workspace.DomainsDir(root) // tier-aware (spec 106 Req 3)
	for _, d := range domains {
		for _, kind := range []string{"overview.md", "interfaces.md"} {
			p := filepath.Join(domainsRoot, d, kind)
			body, rerr := os.ReadFile(p)
			dd := budDomainDoc{Domain: d, Kind: kind, Path: p}
			if rerr == nil {
				dd.Body = body
				dd.Found = true
			}
			domainDocs = append(domainDocs, dd)
		}
	}

	// 8. file_paths (sorted ascending; reject excluded roots).
	var filePaths []string
	if raw, ok := e.Metadata["file_paths"]; ok {
		if list, ok := raw.([]interface{}); ok {
			for _, item := range list {
				s, ok := item.(string)
				if !ok {
					continue
				}
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				first := firstPathSegment(s)
				if excludedRoots[first] {
					return nil, fmt.Errorf("file_paths entry %q is under an excluded tree (viz/agentmind/bench)", s)
				}
				filePaths = append(filePaths, s)
			}
		}
	}
	sort.Strings(filePaths)
	var filePathEntries []budFilePathEntry
	for _, p := range filePaths {
		body, rerr := os.ReadFile(p)
		fpe := budFilePathEntry{Path: p}
		if rerr == nil {
			fpe.Body = body
			fpe.Found = true
		}
		filePathEntries = append(filePathEntries, fpe)
	}

	// 9. Compute SHA-256 over each input's raw bytes.
	var provLines []provLine
	provLines = append(provLines, provLine{key: "bead:" + beadID, sha: shaHex(beadJSON)})
	provLines = append(provLines, provLine{key: "spec:" + specPath, sha: shaHex(specBytes)})
	provLines = append(provLines, provLine{key: "plan:" + planPath, sha: shaHex(planBytes)})
	for _, ae := range adrEntries {
		provLines = append(provLines, provLine{key: "adr:" + ae.ID, sha: shaHex(ae.Bytes)})
	}
	for _, dd := range domainDocs {
		provLines = append(provLines, provLine{key: "domain:" + dd.Domain + "/" + dd.Kind, sha: shaHex(dd.Body)})
	}
	for _, fpe := range filePathEntries {
		provLines = append(provLines, provLine{key: "file:" + fpe.Path, sha: shaHex(fpe.Body)})
	}

	// 10. Render the Provenance block first (with placeholder).
	provBlock, placeholderOffsets := renderProvBlock(tok.Name(), maxTokens, provLines)
	provReserve := tok.Count(provBlock)

	// 11. Render tier 1 (must-tier: ## Bead) and the header.
	tier0 := renderHeader(e.Title, e.ID)
	tier1 := renderTier1Bead(e)

	if maxTokens > 0 {
		if tok.Count(tier0+tier1)+provReserve > maxTokens {
			return nil, fmt.Errorf("bead context exceeds --max-tokens %d; raise budget or split bead", maxTokens)
		}
	}

	// 12. Render tiers 2-6 with per-tier tail-shave.
	tier2 := renderTier2Spec(specGoal, domains, specAccept)
	tier3 := renderTier3ADRs(adrEntriesToRender(adrEntries))
	tier4 := renderTier4Plan(planSection)
	tier5pieces := renderTier5DomainDocs(domainDocs) // ordered slices for per-file shave
	tier6pieces := renderTier6FilePaths(filePathEntries)

	// We assemble incrementally so each tier can tail-shave its own
	// most-recently-appended content.
	var out strings.Builder
	out.WriteString(tier0)
	out.WriteString(tier1)

	// helper: total budget headroom for body
	overBudget := func(extra string) bool {
		if maxTokens <= 0 {
			return false
		}
		return tok.Count(out.String()+extra)+provReserve > maxTokens
	}

	appendShaved := func(piece string) {
		if !overBudget(piece) {
			out.WriteString(piece)
			return
		}
		// Tail-shave: reduce piece length on rune boundaries and
		// append marker. Marker is fixed-length so the loop is a
		// convergent fixed-point.
		shaved := shavePieceToFit(piece, func(s string) bool {
			return overBudget(s)
		})
		out.WriteString(shaved)
	}

	appendShaved(tier2)
	appendShaved(tier3)
	appendShaved(tier4)

	// tier 5: per-file shave (within ## Domain Docs section).
	if len(tier5pieces) > 0 {
		out.WriteString("## Domain Docs\n\n")
		for _, p := range tier5pieces {
			appendShaved(p)
		}
	}
	// tier 6: per-file shave (within ## File Paths section); only emit
	// the header if file_paths non-empty.
	if len(tier6pieces) > 0 {
		out.WriteString("## File Paths\n\n")
		for _, p := range tier6pieces {
			appendShaved(p)
		}
	}

	// 13. Append the rendered Provenance block.
	bodyForCount := out.String()
	out.WriteString(provBlock)

	// 14. Patch the estimated_tokens placeholder with the actual body
	// token count (fixed-width, so no byte-length shift).
	estimated := tok.Count(bodyForCount)
	final := []byte(out.String())
	patchEstimatedTokens(final, placeholderOffsets, estimated)

	return final, nil
}

// renderProvBlockForTest is a test-only accessor used by
// TestContextPackProvenanceReserveIsDynamic to compare reserve sizes
// across different input fixtures without exporting renderProvBlock.
func renderProvBlockForTest(tokName string, maxTokens int, provLines []struct{ Key, Sha string }) string {
	conv := make([]provLine, len(provLines))
	for i, l := range provLines {
		conv[i] = provLine{key: l.Key, sha: l.Sha}
	}
	s, _ := renderProvBlock(tokName, maxTokens, conv)
	return s
}

// renderProvBlock renders the Provenance block with a fixed-width
// estimated_tokens placeholder. Returns the block string and the
// byte-offset range of the placeholder digits inside the block (for
// later in-place patching).
//
// Width = number of decimal digits in maxTokens, or 6 when
// maxTokens == 0. Right-justified, space-padded. The actual value is
// at most that width, so the in-place patch does NOT shift any bytes.
func renderProvBlock(tokName string, maxTokens int, provLines []provLine) (string, [2]int) {
	width := 6
	if maxTokens > 0 {
		width = len(fmt.Sprintf("%d", maxTokens))
		if width < 1 {
			width = 1
		}
	}
	placeholder := strings.Repeat(" ", width-1) + "0" // right-justified zero, fixed width

	// Build the block. Track placeholder offsets so the caller can
	// patch in the final value.
	var b strings.Builder
	b.WriteString("## Provenance\n\n")
	b.WriteString(fmt.Sprintf("- tokenizer: %s\n", tokName))
	b.WriteString(fmt.Sprintf("- max_tokens: %d\n", maxTokens))
	prefix := "- estimated_tokens: "
	b.WriteString(prefix)
	startOff := b.Len()
	b.WriteString(placeholder)
	endOff := b.Len()
	b.WriteString("\n")
	b.WriteString("- inputs:\n")

	// Sort prov lines by key for determinism (already mostly sorted
	// by construction, but make it explicit).
	sortedLines := make([]provLine, len(provLines))
	copy(sortedLines, provLines)
	sort.SliceStable(sortedLines, func(i, j int) bool {
		return sortedLines[i].key < sortedLines[j].key
	})
	for _, pl := range sortedLines {
		b.WriteString(fmt.Sprintf("  - %s sha256:%s\n", pl.key, pl.sha))
	}

	return b.String(), [2]int{startOff, endOff}
}

// patchEstimatedTokens writes the decimal representation of n into
// the placeholder span in-place. The span is fixed-width so the byte
// length of the surrounding block is unchanged.
func patchEstimatedTokens(out []byte, span [2]int, n int) {
	// Find the absolute offset of the placeholder inside out by
	// scanning from the end (the block was appended last).
	// We were handed the offset relative to the block; the block was
	// appended at out[len(out)-blockLen:]. We need to locate it.
	// Easiest: search backwards for the prefix "- estimated_tokens: "
	// and overlay digits onto the next span[1]-span[0] bytes.
	const prefix = "- estimated_tokens: "
	idx := lastIndexBytes(out, []byte(prefix))
	if idx < 0 {
		return
	}
	width := span[1] - span[0]
	start := idx + len(prefix)
	if start+width > len(out) {
		return
	}
	val := fmt.Sprintf("%d", n)
	if len(val) > width {
		// Overflow guard: write width '9's rather than corrupting bytes.
		val = strings.Repeat("9", width)
	}
	// Right-justify with leading spaces.
	pad := width - len(val)
	for i := 0; i < pad; i++ {
		out[start+i] = ' '
	}
	for i := 0; i < len(val); i++ {
		out[start+pad+i] = val[i]
	}
}

func lastIndexBytes(haystack, needle []byte) int {
	// Hand-rolled to avoid pulling bytes; small needle so OK.
	n := len(needle)
	if n == 0 || len(haystack) < n {
		return -1
	}
	for i := len(haystack) - n; i >= 0; i-- {
		match := true
		for j := 0; j < n; j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// shavePieceToFit reduces piece length on rune boundaries until
// overBudget(result) returns false, appending the truncation marker
// at the cut point. Returns the shaved piece (with marker) or the
// minimal marker-only string if no positive prefix fits.
func shavePieceToFit(piece string, overBudget func(string) bool) string {
	if !overBudget(piece) {
		return piece
	}
	// Binary-search-ish shrink: halve the piece length until under
	// budget, then linearly grow to the largest rune-aligned prefix.
	// For simplicity and determinism, do a linear shrink by 1 rune at
	// a time from the end of the un-marker prefix.
	prefix := piece
	for utf8.RuneCountInString(prefix) > 0 {
		// Drop the last rune.
		_, size := utf8.DecodeLastRuneInString(prefix)
		if size == 0 {
			break
		}
		prefix = prefix[:len(prefix)-size]
		// Walk back any RuneError-size-1 bytes to land on a valid boundary.
		for len(prefix) > 0 {
			r, sz := utf8.DecodeLastRuneInString(prefix)
			if r == utf8.RuneError && sz == 1 {
				prefix = prefix[:len(prefix)-1]
				continue
			}
			break
		}
		candidate := prefix + truncationMarker + "\n"
		if !overBudget(candidate) {
			return candidate
		}
	}
	// Even the marker alone overflows; emit it anyway to leave evidence.
	return truncationMarker + "\n"
}

// shaHex returns the lowercase hex SHA-256 of b.
func shaHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// stringFromMeta extracts a string value from the bead metadata map.
func stringFromMeta(m map[string]interface{}, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// firstPathSegment returns the first '/'-separated segment of p.
func firstPathSegment(p string) string {
	p = strings.TrimPrefix(p, "./")
	idx := strings.Index(p, "/")
	if idx < 0 {
		return p
	}
	return p[:idx]
}

// extractSpecSections returns (goal, acceptance_criteria_section) from
// a spec.md body. Goal is what's under "## Goal"; AC is what's under
// "## Acceptance Criteria".
func extractSpecSections(body string) (string, string) {
	var goal, ac []string
	section := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			head := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			switch head {
			case "goal":
				section = "goal"
			case "acceptance criteria":
				section = "ac"
			default:
				section = ""
			}
			continue
		}
		switch section {
		case "goal":
			goal = append(goal, line)
		case "ac":
			ac = append(ac, line)
		}
	}
	return strings.TrimSpace(strings.Join(goal, "\n")), strings.TrimSpace(strings.Join(ac, "\n"))
}

// parseCitedADRs extracts ADR ids from a plan.md frontmatter
// adr_citations list. Returns the ids (e.g. "ADR-0033") in source
// order; caller sorts. The block is located via the canonical
// internal/frontmatter.Parse (ARCH-6); YAML ignores `#` comment lines
// natively so no manual comment filtering is needed.
func parseCitedADRs(planBody string) ([]string, error) {
	block, _, ok := frontmatter.Parse([]byte(planBody))
	if !ok {
		return nil, nil
	}
	var pf planFrontmatter
	if err := yaml.Unmarshal(block, &pf); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(pf.ADRCitations))
	for _, c := range pf.ADRCitations {
		id := strings.TrimSpace(c.ID)
		if id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}

// extractPlanBeadSection scans plan.md top-down and returns the first
// level-2-or-deeper heading whose text contains beadID OR title,
// plus all lines until the next heading of the same or shallower
// level. If no heading matches, returns the entire plan body.
func extractPlanBeadSection(planBody, beadID, title string) string {
	// Strip frontmatter via the canonical internal/frontmatter.Parse (ARCH-6):
	// on a well-formed block the post-fence bytes are byte-identical to the
	// prior hand-rolled split/join strip; a space-padded fence now reads as
	// no-frontmatter and the whole body is scanned.
	body := planBody
	if _, bodyOffset, ok := frontmatter.Parse([]byte(planBody)); ok {
		body = planBody[bodyOffset:]
	}

	lines := strings.Split(body, "\n")
	start := -1
	startLevel := 0
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") &&
			!strings.HasPrefix(line, "#### ") && !strings.HasPrefix(line, "##### ") {
			continue
		}
		// Count leading hashes.
		level := 0
		for level < len(line) && line[level] == '#' {
			level++
		}
		if level < 2 {
			continue
		}
		heading := strings.TrimSpace(line[level:])
		if (beadID != "" && strings.Contains(heading, beadID)) ||
			(title != "" && strings.Contains(heading, title)) {
			start = i
			startLevel = level
			break
		}
	}
	if start < 0 {
		return strings.TrimSpace(body)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "#") {
			continue
		}
		lvl := 0
		for lvl < len(line) && line[lvl] == '#' {
			lvl++
		}
		if lvl > 0 && lvl <= startLevel {
			end = i
			break
		}
	}
	return strings.TrimRight(strings.Join(lines[start:end], "\n"), "\n")
}

// renderHeader emits the level-1 header line.
func renderHeader(title, id string) string {
	// R4: title is agent-writable single-line free text (termsafe.Escape);
	// id is an ID-typed position (idrender.Bead).
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Bead Context: %s\n", termsafe.Escape(title)))
	b.WriteString(fmt.Sprintf("**Bead**: %s\n\n", idrender.Bead(id)))
	return b.String()
}

// renderTier1Bead emits the must-tier ## Bead section.
func renderTier1Bead(e beadShowEntry) string {
	var b strings.Builder
	b.WriteString("## Bead\n\n")
	if e.Description != "" {
		b.WriteString("### Description\n\n")
		b.WriteString(strings.TrimSpace(e.Description))
		b.WriteString("\n\n")
	}
	if e.AcceptanceCriteria != "" {
		b.WriteString("### Acceptance Criteria\n\n")
		b.WriteString(strings.TrimSpace(e.AcceptanceCriteria))
		b.WriteString("\n\n")
	}
	if e.Design != "" {
		b.WriteString("### Design\n\n")
		b.WriteString(strings.TrimSpace(e.Design))
		b.WriteString("\n\n")
	}
	return b.String()
}

func renderTier2Spec(goal string, domains []string, ac string) string {
	var b strings.Builder
	b.WriteString("## Spec\n\n")
	if goal != "" {
		b.WriteString("### Goal\n\n")
		b.WriteString(goal)
		b.WriteString("\n\n")
	}
	if len(domains) > 0 {
		b.WriteString("### Impacted Domains\n\n")
		for _, d := range domains {
			b.WriteString(fmt.Sprintf("- %s\n", d))
		}
		b.WriteString("\n")
	}
	if ac != "" {
		b.WriteString("### Acceptance Criteria\n\n")
		b.WriteString(ac)
		b.WriteString("\n\n")
	}
	return b.String()
}

type renderedADR struct {
	ID       string
	Title    string
	Decision string
}

func adrEntriesToRender(in []budAdrEntry) []renderedADR {
	out := make([]renderedADR, 0, len(in))
	for _, a := range in {
		out = append(out, renderedADR{ID: a.ID, Title: a.Title, Decision: a.Decision})
	}
	return out
}

func renderTier3ADRs(adrs []renderedADR) string {
	if len(adrs) == 0 {
		return ""
	}
	// Stable sort by ID ascending.
	sort.SliceStable(adrs, func(i, j int) bool { return adrs[i].ID < adrs[j].ID })
	var b strings.Builder
	b.WriteString("## Cited ADRs\n\n")
	for _, a := range adrs {
		// R4: ADR.ID and ADR.Title are agent-writable frontmatter fields —
		// escape both, matching renderHeader above.
		b.WriteString(fmt.Sprintf("### %s: %s\n\n", termsafe.Escape(a.ID), termsafe.Escape(a.Title)))
		if a.Decision != "" {
			b.WriteString("#### Decision\n\n")
			b.WriteString(a.Decision)
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func renderTier4Plan(section string) string {
	if strings.TrimSpace(section) == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Plan\n\n")
	b.WriteString(section)
	b.WriteString("\n\n")
	return b.String()
}

func renderTier5DomainDocs(docs []budDomainDoc) []string {
	if len(docs) == 0 {
		return nil
	}
	// Per spec: ### <domain> then #### <kind> for each file.
	// Group by domain in order. We emit a string per file so the
	// tail-shave can run per file (per spec Req 10 sub-bullet).
	out := make([]string, 0, len(docs))
	lastDomain := ""
	for _, d := range docs {
		var b strings.Builder
		if d.Domain != lastDomain {
			b.WriteString(fmt.Sprintf("### %s\n\n", d.Domain))
			lastDomain = d.Domain
		}
		b.WriteString(fmt.Sprintf("#### %s\n\n", d.Kind))
		if !d.Found {
			b.WriteString(fmt.Sprintf("<!-- missing: %s -->\n\n", d.Path))
		} else {
			b.WriteString(strings.TrimRight(string(d.Body), "\n"))
			b.WriteString("\n\n")
		}
		out = append(out, b.String())
	}
	return out
}

func renderTier6FilePaths(entries []budFilePathEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("### %s\n\n", e.Path))
		b.WriteString("```\n")
		if !e.Found {
			b.WriteString(fmt.Sprintf("<!-- missing: %s -->\n", e.Path))
		} else {
			body := strings.TrimRight(string(e.Body), "\n")
			b.WriteString(body)
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
		out = append(out, b.String())
	}
	return out
}
