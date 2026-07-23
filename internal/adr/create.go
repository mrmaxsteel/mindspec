package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// maxSlugLen caps the derived (or supplied) ADR filename slug at 48
// characters (spec 123 R5(a), plan-level cap choice), truncated at a
// hyphen boundary so a cap mid-word never leaves a partial token
// dangling on disk.
const maxSlugLen = 48

// slugShapePattern is the shape a caller-supplied --slug value must
// already match: lowercase kebab-case, no leading/trailing/consecutive
// hyphens. A raw title is NOT expected to match this (deriveSlug does
// the collapsing); --slug is for a caller who already knows the exact
// filename token they want.
var slugShapePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// deriveSlug converts an ADR title into a kebab-case filename slug:
// lowercase, non-alphanumeric runs collapse to a single hyphen,
// leading/trailing hyphens are trimmed, and the result is capped at
// maxSlugLen characters, backed off to the nearest hyphen boundary so a
// cap mid-word never leaves a partial token. A title with no
// alphanumeric characters at all (e.g. pure punctuation) derives an
// empty slug — callers fall back to the bare "ADR-NNNN.md" filename
// form rather than writing an invalid "ADR-NNNN-.md".
func deriveSlug(title string) string {
	var b strings.Builder
	prevHyphen := true // suppresses a leading hyphen
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case !prevHyphen:
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	slug := strings.TrimRight(b.String(), "-")

	if len(slug) > maxSlugLen {
		truncated := slug[:maxSlugLen]
		if idx := strings.LastIndexByte(truncated, '-'); idx >= 0 {
			truncated = truncated[:idx]
		}
		slug = truncated
	}
	return slug
}

// resolveSlug returns the slug to embed in the new ADR's filename: the
// caller's --slug override when supplied (CreateOpts.SlugOverride
// non-nil), otherwise a slug derived from the title (deriveSlug). An
// override that is present but empty (an explicit empty --slug value) opts out
// of slugging, the same as an empty derived slug — the caller falls back
// to the bare filename. A non-empty override MUST already be
// lowercase-kebab-shaped (slugShapePattern); a malformed override is
// refused with an ADR-0035 recovery line naming the accepted shape,
// never silently corrected.
func resolveSlug(title string, override *string) (string, error) {
	if override == nil {
		return deriveSlug(title), nil
	}
	v := strings.TrimSpace(*override)
	if v == "" {
		return "", nil
	}
	if !slugShapePattern.MatchString(v) {
		return "", fmt.Errorf(
			"--slug %q must be lowercase kebab-case (letters, digits, single internal hyphens; no leading/trailing hyphen)\nrecovery: pass --slug %q or omit --slug to derive one from the title",
			*override, deriveSlug(v),
		)
	}
	return v, nil
}

const adrTemplate = `# ADR-NNNN: <Title>

- **Date**: <YYYY-MM-DD>
- **Status**: Proposed
- **Domain(s)**: <comma-separated list>
- **Deciders**: <who decides>
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Context

<What is the issue that we're seeing that motivates this decision or change?>

## Decision

<What is the change that we're proposing and/or doing?>

## Decision Details

<Detailed breakdown of the decision. Use subsections as needed.>

## Consequences

### Positive

- <Positive consequence 1>
- <Positive consequence 2>

### Negative / Tradeoffs

- <Negative consequence or tradeoff 1>
- <Negative consequence or tradeoff 2>

## Alternatives Considered

### 1. <Alternative name>

<Description and why it was rejected.>

## Validation / Rollout

1. <Validation step 1>
2. <Validation step 2>
`

// CreateOpts configures ADR creation.
type CreateOpts struct {
	Domains    []string
	Supersedes string
	// SlugOverride, when non-nil, replaces slug derivation from the title
	// (spec 123 R5(a), the `--slug` flag). A non-nil-but-empty override
	// (an explicit empty --slug value) opts out of slugging, the same as an
	// empty derived slug — the new ADR gets the bare "ADR-NNNN.md" name.
	// A non-empty override is validated to already be lowercase
	// kebab-case; an invalid value is refused rather than silently
	// corrected (see resolveSlug).
	SlugOverride *string
}

// Create generates a new ADR file from the template under root, numbering
// the new ADR against root's own ADRs. Returns the path to the created file.
func Create(root, title string, opts CreateOpts) (string, error) {
	return create(root, []string{root}, title, opts)
}

// CreateUnion writes the new ADR into writeRoot but allocates its ID over the
// UNION of writeRoot and numberingRoots (e.g. the main checkout), so a new ADR
// authored from a bead/spec worktree cannot collide with a main-only ADR added
// after the branch diverged. The WRITE target (and any --supersedes resolution)
// is writeRoot alone; only the NextID allocation consults the union. With an
// empty numberingRoots this is identical to Create (mindspec-8lzq).
func CreateUnion(writeRoot string, numberingRoots []string, title string, opts CreateOpts) (string, error) {
	roots := append([]string{writeRoot}, numberingRoots...)
	return create(writeRoot, roots, title, opts)
}

// create is the shared ADR-creation body. It writes into root but allocates
// the new ID over numberingRoots (the union of roots to number against).
func create(root string, numberingRoots []string, title string, opts CreateOpts) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("title must not be empty")
	}

	// If superseding, verify old ADR exists and optionally copy domains.
	//
	// SEC-1 hot path (bead mindspec-x1qr): opts.Supersedes is user-controlled
	// and previously flowed straight into filepath.Join, enabling a
	// "../../../tmp/poisoned" traversal that mutated arbitrary *.md files.
	// We validate BEFORE any join so the error message is precise and no
	// path-construction code runs on attacker input.
	//
	// Resolution (spec 123 R5(c)): opts.Supersedes may be a canonical ID
	// whose on-disk file is slugged, so it resolves through
	// workspace.ResolveADRFile (bare-or-slugged, collision-checked)
	// rather than the exact-join ADRFilePath — ResolveADRFile itself
	// returns a "not found" error when the predecessor is absent, so no
	// separate os.Stat existence probe is needed here.
	if opts.Supersedes != "" {
		if err := idvalidate.ADRID(opts.Supersedes); err != nil {
			return "", fmt.Errorf("invalid --supersedes value: %w", err)
		}
		if _, err := workspace.ResolveADRFile(root, opts.Supersedes); err != nil {
			return "", err
		}
		if len(opts.Domains) == 0 {
			domains, err := CopyDomains(root, opts.Supersedes)
			if err != nil {
				return "", fmt.Errorf("copying domains from %s: %w", opts.Supersedes, err)
			}
			opts.Domains = domains
		}
	}

	id, err := NextIDAcross(numberingRoots...)
	if err != nil {
		return "", fmt.Errorf("generating next ID: %w", err)
	}

	// R5(a): derive (or accept the --slug override for) the new file's
	// kebab-case slug. An empty slug (punctuation-only title, or an
	// explicit empty --slug) falls back to the bare "ADR-NNNN" stem.
	slug, err := resolveSlug(title, opts.SlugOverride)
	if err != nil {
		return "", err
	}
	stem := fmt.Sprintf("ADR-%s", id)
	if slug != "" {
		stem = fmt.Sprintf("%s-%s", stem, slug)
	}
	if err := idvalidate.ADRID(stem); err != nil {
		return "", fmt.Errorf("computed ADR filename stem %q is invalid: %w\nrecovery: pass a lowercase kebab-case --slug (letters, digits, single hyphens)", stem, err)
	}

	content := adrTemplate
	content = strings.ReplaceAll(content, "NNNN", id)
	content = strings.ReplaceAll(content, "<Title>", title)
	content = strings.ReplaceAll(content, "<YYYY-MM-DD>", time.Now().Format("2006-01-02"))

	if len(opts.Domains) > 0 {
		content = strings.Replace(content, "<comma-separated list>", strings.Join(opts.Domains, ", "), 1)
	}

	if opts.Supersedes != "" {
		content = strings.Replace(content, "**Supersedes**: n/a", fmt.Sprintf("**Supersedes**: %s", opts.Supersedes), 1)
	}

	adrDir := workspace.ADRDir(root)
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		return "", fmt.Errorf("creating ADR directory: %w", err)
	}

	// R5(a) WRITE-target emission: composes the NEW file's path from the
	// freshly-allocated id + slug; this is never a resolution of an
	// existing file, so it stays a plain filepath.Join (not routed
	// through workspace.ResolveADRFile).
	outPath := filepath.Join(adrDir, stem+".md")
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing ADR file: %w", err)
	}

	// If superseding, update the old ADR
	if opts.Supersedes != "" {
		newID := fmt.Sprintf("ADR-%s", id)
		if err := Supersede(root, opts.Supersedes, newID); err != nil {
			return outPath, fmt.Errorf("updating superseded ADR: %w", err)
		}
	}

	return outPath, nil
}

// CreateWithID generates a new ADR file at a caller-supplied ID
// (e.g. "ADR-0099") instead of auto-allocating via NextID. Used by the
// Spec 087 Bead 3 supersede flow which pre-creates a placeholder ADR
// with Status: Proposed so the divergence-gate skip path is
// deterministic (revision 1 of the panel-revised plan: the file MUST
// exist on disk at the user-supplied ID verbatim).
//
// Contract:
//   - id MUST already have passed idvalidate.ADRID at the call site
//     (the CLI layer enforces this before reaching here).
//   - If a file at workspace.ADRFilePath(root, id) — or any
//     `<id>-<slug>.md` collision — already exists, returns an error
//     whose message contains the substring "already exists" and writes
//     nothing.
//   - Reuses the existing template-fill logic from Create but
//     SUBSTITUTES the user-supplied id verbatim. Status is Proposed,
//     Domains seed from opts.Domains.
//   - The Supersedes/Superseded-by chain update from Create does NOT
//     apply: CreateWithID is for placeholder creation, which has no
//     "old" ADR to update.
func CreateWithID(root, id, title string, opts CreateOpts) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("title must not be empty")
	}

	// Validate the ID defensively in case the caller forgot.
	if err := idvalidate.ADRID(id); err != nil {
		return "", fmt.Errorf("invalid ADR ID: %w", err)
	}

	outPath, err := workspace.ADRFilePath(root, id)
	if err != nil {
		return "", err
	}

	// Collision check: exact path AND slug variants. Mirrors adr.Show's
	// lookup discipline so we don't create ADR-0099.md when an
	// ADR-0099-<slug>.md already exists.
	if _, statErr := os.Stat(outPath); statErr == nil {
		return "", fmt.Errorf("ADR %s already exists at %s", id, outPath)
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	slugMatches, globErr := filepath.Glob(filepath.Join(workspace.ADRDir(root), id+"-*.md"))
	if globErr != nil {
		return "", globErr
	}
	if len(slugMatches) > 0 {
		return "", fmt.Errorf("ADR %s already exists at %s", id, slugMatches[0])
	}

	// Strip the "ADR-" prefix to obtain the bare ID for the template
	// header substitution (the NextID-driven path uses just digits).
	bareID := strings.TrimPrefix(id, "ADR-")

	content := adrTemplate
	content = strings.ReplaceAll(content, "NNNN", bareID)
	content = strings.ReplaceAll(content, "<Title>", title)
	content = strings.ReplaceAll(content, "<YYYY-MM-DD>", time.Now().Format("2006-01-02"))

	if len(opts.Domains) > 0 {
		content = strings.Replace(content, "<comma-separated list>", strings.Join(opts.Domains, ", "), 1)
	}

	adrDir := workspace.ADRDir(root)
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		return "", fmt.Errorf("creating ADR directory: %w", err)
	}

	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing ADR file: %w", err)
	}

	return outPath, nil
}
