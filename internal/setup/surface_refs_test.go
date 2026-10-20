package setup

// surface_refs_test.go — the internal-reference guard. Mindspec ships to
// consumers who have none of mindspec's own development records. A shipped
// "see ADR-0045" sends a consumer agent to ITS OWN .mindspec/adr/, where
// that number names a different ADR or none. So no consumer-visible
// surface may cite mindspec's ADR numbers, spec numbers, requirement IDs,
// bead IDs, issue numbers, incidents, handovers, internal Go identifiers,
// or the author's other projects.
//
// This extends the AC-10 portability scan in review_discipline_test.go: it
// reuses that scan's hit type, locator-keyed allowlist entry, and
// validation (scanAllowlistValidate), with a wider pattern set and a wider
// surface set:
//   - every installed skill body (skillFiles(): plugin skills plus the
//     lifecycle literals) and both managed-block literals;
//   - every embedded workflow script;
//   - the tracked files setup or init writes or ships: plugin scripts and
//     README, bootstrap assets, instruct templates, the root README, and
//     the user and installation manuals under project-docs/;
//   - every string literal in non-test Go source under cmd/, internal/, and
//     plugins/ (go/ast; comments, imports, and struct tags are skipped;
//     internal/harness and testdata are skipped).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	pluginmindspec "github.com/mrmaxsteel/mindspec/plugins/mindspec"
)

// surfaceRefClass is one category of internal reference.
type surfaceRefClass struct {
	name string
	re   *regexp.Regexp
	// skip, when set, drops a match that the regex cannot exclude by itself
	// (RE2 has no lookahead). content[loc[1]:] is the text after the match.
	skip func(content string, loc []int) bool
	// codeOnly limits the class to skills, workflows, managed blocks, and Go
	// literals. The prose manuals are not scanned for it (see
	// surfaceRefCodeTermDocsResidual).
	codeOnly bool
}

// followedBySlug reports whether the match continues as a kebab-case slug
// ("spec 001-my-feature"): that is a consumer-format example, not a cite.
func followedBySlug(content string, loc []int) bool {
	return loc[1] < len(content) && content[loc[1]] == '-'
}

// beadIDWords are "mindspec-<4 letters>" forms that are words, not bead IDs.
var beadIDWords = map[string]bool{"root": true, "otel": true, "plan": true, "spec": true, "impl": true, "bead": true, "init": true, "test": true, "docs": true, "main": true}

var surfaceRefClasses = []surfaceRefClass{
	{name: "adr", re: regexp.MustCompile(`\bADR-?[0-9]{4}\b`)},
	{name: "spec-id", re: regexp.MustCompile(`(?i)\bspec[\s-]+[0-9]{2,3}\b`), skip: followedBySlug},
	{name: "spec-number-cite", re: regexp.MustCompile(`\b[01][0-9]{2}\s+(?:R[0-9]+|Req\s*[0-9]+|AC-?[0-9]+|[Bb]ead\s+[0-9]+)\b`)},
	{name: "requirement-id", re: regexp.MustCompile(`\bR[0-9]+(?:\.[0-9]+)+\b|\bR[0-9]+\([a-z]+\)|\bR[0-9]+[a-e]\b|\bReq\s+[0-9]+\b|\b(?:AC|HC)-[0-9]+\b`)},
	{name: "bead-id", re: regexp.MustCompile(`\bmindspec-([a-z0-9]{4})(?:\.[0-9]+)?\b`), skip: func(content string, loc []int) bool {
		return followedBySlug(content, loc) || beadIDWords[content[loc[0]+len("mindspec-"):loc[0]+len("mindspec-")+4]]
	}},
	{name: "issue-number", re: regexp.MustCompile(`(?i)\b(?:PR|issue|GH)\s*#[0-9]+\b|(?:^|[\s(])#[0-9]{3,4}\b`)},
	{name: "incident", re: regexp.MustCompile(`(?i)\bfield evidence\b|\bincident\b|\b2u0u\b|\bq9ea\b`)},
	{name: "handover", re: regexp.MustCompile(`(?i)\bhandover\b`)},
	{name: "other-project", re: regexp.MustCompile(`(?i)\blola\b`)},
	{name: "home-path", re: regexp.MustCompile(`/Users/|/home/[a-z]`)},
	{name: "go-identifier", codeOnly: true, re: regexp.MustCompile(`\binternal/[a-z]+|\b(?:panel|config|phase|resolve|bead|workspace|guard|lifecycle|gitutil|contextpack|tokenize|instruct|approve|complete|setup|executor|validate|readiness|doctor|idrender|termsafe|safeio|layout|ownership|journal)\.[A-Z][A-Za-z]+`)},
}

// surfaceRefCodeTermDocsResidual records why the go-identifier class skips
// the prose manuals: project-docs/user/guides/lifecycle-modes.md documents
// the phase-agreement test contract by Go function name, and a test parses
// that file. Moving that contributor material out of the user manuals is a
// separate change.
const surfaceRefCodeTermDocsResidual = "lifecycle-modes.md is a contributor contract document"

// surfaceRefExcludedGoFiles are non-test Go files whose literals are never
// shown to a consumer. Each entry carries its reason.
var surfaceRefExcludedGoFiles = map[string]string{
	"internal/guard/registries.go":            "test-only registry data (rationales and obligations read only by internal/lint tests)",
	"internal/validate/readiness/fixtures.go": "exported test fixtures in a non-test file so other packages' tests can import them",
	"internal/panel/disposition_migrate.go":   "maintainer-only seed migration paths, used only by tests and never printed",
}

func surfaceRefScanContent(surface, content string, codeSurface bool) []acTenHit {
	var hits []acTenHit
	for _, c := range surfaceRefClasses {
		if c.codeOnly && !codeSurface {
			continue
		}
		for _, loc := range c.re.FindAllStringIndex(content, -1) {
			if c.skip != nil && c.skip(content, loc) {
				continue
			}
			m := strings.TrimLeft(content[loc[0]:loc[1]], " \t\n(")
			hits = append(hits, acTenHit{surface: surface, matched: m, line: acTenLineOf(content, loc[1]-len(m))})
		}
	}
	return hits
}

// surfaceRefIsCode reports whether a surface key names a code-term surface.
func surfaceRefIsCode(surface string) bool {
	return !strings.HasPrefix(surface, "doc:")
}

func surfaceRefScanSurfaces(surfaces map[string]string) []acTenHit {
	var hits []acTenHit
	for surface, content := range surfaces {
		hits = append(hits, surfaceRefScanContent(surface, content, surfaceRefIsCode(surface))...)
	}
	return hits
}

// surfaceRefGoLiterals returns the string-literal values of one parsed Go
// file, one literal per entry. Imports and struct tags are skipped; comments
// are not part of the AST walk.
func surfaceRefGoLiterals(f *ast.File) []string {
	var out []string
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.Field:
			if x.Tag != nil {
				// Walk the field type only; the tag is metadata, not text.
				if x.Type != nil {
					ast.Inspect(x.Type, visit)
				}
				return false
			}
		case *ast.BinaryExpr:
			// A "+" chain is one message; join its literal leaves so a
			// locator can span the pieces. Non-literal operands become "…".
			if x.Op == token.ADD {
				var rest []ast.Expr
				if s, ok := surfaceRefConcat(x, &rest); ok {
					out = append(out, s)
					for _, r := range rest {
						ast.Inspect(r, visit)
					}
					return false
				}
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				if s, err := strconv.Unquote(x.Value); err == nil {
					out = append(out, s)
				}
			}
		}
		return true
	}
	ast.Inspect(f, visit)
	return out
}

// surfaceRefConcat flattens a "+" chain that holds at least one string
// literal. It reports false when the chain has no string literal. It
// appends each non-literal operand to rest, so the caller can still scan
// the literals inside those operands.
func surfaceRefConcat(e ast.Expr, rest *[]ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			if s, err := strconv.Unquote(x.Value); err == nil {
				return s, true
			}
		}
	case *ast.ParenExpr:
		return surfaceRefConcat(x.X, rest)
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			l, lok := surfaceRefConcat(x.X, rest)
			r, rok := surfaceRefConcat(x.Y, rest)
			return l + r, lok || rok
		}
	}
	*rest = append(*rest, e)
	return "…", false
}

// surfaceRefGoSurfaces parses every non-test Go file under cmd/, internal/,
// and plugins/ and returns one surface per file ("go:<rel>"), whose content
// is the file's string literals joined by newlines.
func surfaceRefGoSurfaces(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, top := range []string{"cmd", "internal", "plugins"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if d.Name() == "testdata" || rel == "internal/harness" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			if _, excluded := surfaceRefExcludedGoFiles[rel]; excluded {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			if lits := surfaceRefGoLiterals(f); len(lits) > 0 {
				out["go:"+rel] = strings.Join(lits, "\n")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", top, err)
		}
	}
	return out
}

// surfaceRefFileSurfaces reads the tracked consumer-visible files. Prose
// manuals get the "doc:" prefix; shipped scripts, assets, and templates get
// "file:".
func surfaceRefFileSurfaces(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	add := func(prefix, rel string) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		out[prefix+rel] = string(data)
	}
	walk := func(prefix, dir string) {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			if strings.HasSuffix(rel, ".go") {
				return nil
			}
			add(prefix, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	walk("file:", "plugins/mindspec/scripts")
	walk("file:", "internal/bootstrap/assets")
	walk("file:", "internal/instruct/templates")
	add("doc:", "README.md")
	add("doc:", "plugins/mindspec/README.md")
	walk("doc:", "project-docs/user")
	walk("doc:", "project-docs/installation")
	return out
}

// surfaceRefSurfaces returns every scanned surface, keyed by kind:name.
func surfaceRefSurfaces(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	out := map[string]string{}
	for name, content := range CanonicalGuidanceSurfaces() {
		out["managed:"+name] = content
	}
	for name, content := range pluginmindspec.WorkflowFiles() {
		out["workflow:"+name] = content
	}
	for k, v := range surfaceRefFileSurfaces(t, root) {
		out[k] = v
	}
	for k, v := range surfaceRefGoSurfaces(t, root) {
		out[k] = v
	}
	return out
}

// surfaceRefAllowlist is the explicit, reasoned allowlist. Each entry is
// keyed by surface + a verbatim locator substring of the allowed line +
// the exact matched text + count (the AC-10 entry shape).
var surfaceRefAllowlist = []acTenAllowEntry{
	{
		surface: "go:cmd/mindspec/adr.go",
		locator: "ADR ID to supersede",
		matched: "ADR-0001",
		count:   1,
		reason:  "example in the consumer's own ADR-ID format (--supersedes flag help)",
	},
	{
		surface: "go:internal/panel/disposition.go",
		locator: "/Users/",
		matched: "/Users/",
		count:   1,
		reason:  "the hygiene check's own forbidden-token list (it refuses home paths in stored rows)",
	},
	{
		surface: "go:internal/panel/finding.go",
		locator: "(?:/tmp/|/Users/)",
		matched: "/Users/",
		count:   1,
		reason:  "the finding scrubber's own home-path pattern (it removes local paths from stored text)",
	},
	{
		surface: "go:cmd/mindspec/migrate.go",
		locator: "internal/instruct/templates/",
		matched: "internal/instruct",
		count:   1,
		reason:  "path filter inside the layout migration, never printed",
	},
	{
		surface: "go:internal/ownership/stub.go",
		locator: "globs that this domain owns",
		matched: "internal/ledger",
		count:   1,
		reason:  "example path in a consumer repo (OWNERSHIP.yaml stub)",
	},
	{
		surface: "go:internal/ownership/populate.go",
		locator: "named \"payments\" may correspond to",
		matched: "internal/ledger",
		count:   1,
		reason:  "example path in a consumer repo (ownership populate prompt)",
	},
	{
		surface: "file:internal/instruct/templates/plan.md",
		locator: "`go test ./internal/foo/...` passes",
		matched: "internal/foo",
		count:   1,
		reason:  "placeholder path in the consumer's plan template",
	},
	{
		surface: "file:internal/instruct/templates/plan.md",
		locator: "- internal/foo/foo.go",
		matched: "internal/foo",
		count:   1,
		reason:  "placeholder path in the consumer's plan template",
	},
	{
		surface: "file:internal/instruct/templates/plan.md",
		locator: "- internal/bar/bar.go",
		matched: "internal/bar",
		count:   1,
		reason:  "placeholder path in the consumer's plan template",
	},
	{
		surface: "go:internal/guard/constructor.go",
		locator: "invalid-destructive-command",
		matched: "guard.NewDestructiveCommand",
		count:   1,
		reason:  "programmer-error placeholder for mindspec's own code, never a consumer message",
	},
}

func TestSurfaceRefs_NoInternalReferences(t *testing.T) {
	surfaces := surfaceRefSurfaces(t)
	problems := scanAllowlistValidate("surface-refs", surfaceRefScanSurfaces(surfaces), surfaceRefAllowlist, surfaces)
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

// TestSurfaceRefs_ScansEverySurfaceKind proves the surface set is not
// hollow: each kind is present and non-empty.
func TestSurfaceRefs_ScansEverySurfaceKind(t *testing.T) {
	surfaces := surfaceRefSurfaces(t)
	for _, want := range []string{
		"managed:ms-bead-cycle", "managed:ms-spec-approve", "managed:claudeMDManagedBlock", "managed:agentsMDBlockTemplate",
		"workflow:ms-panel.js",
		"file:plugins/mindspec/scripts/codex_verdict_extract.sh",
		"file:internal/bootstrap/assets/bd-jsonl-merge-driver.sh",
		"file:internal/instruct/templates/idle.md",
		"doc:README.md", "doc:plugins/mindspec/README.md", "doc:project-docs/user/guides/merge-safety.md",
		"go:cmd/mindspec/repair.go", "go:internal/bootstrap/bootstrap.go", "go:internal/setup/claude.go",
	} {
		if strings.TrimSpace(surfaces[want]) == "" {
			t.Errorf("surface %s missing or empty", want)
		}
	}
	for rel := range surfaceRefExcludedGoFiles {
		if _, ok := surfaces["go:"+rel]; ok {
			t.Errorf("excluded Go file %s was scanned", rel)
		}
	}
}

// TestSurfaceRefs_NegativeEachClass injects one sample per class into a
// synthetic surface and requires a RED.
func TestSurfaceRefs_NegativeEachClass(t *testing.T) {
	for _, tc := range []struct{ class, inject string }{
		{"adr", "lifecycle state is derived from beads (ADR-0023)"},
		{"adr-unhyphenated", "see ADR0045 step 4"},
		{"spec-id", "the readiness gate (spec 124) refuses"},
		{"spec-id-hyphen", "the spec-130 rule table"},
		{"spec-number-cite", "092 Bead 5's user-dirt blocking"},
		{"requirement-dotted", "not blocked — R5.1"},
		{"requirement-lettered", "anti-browbeat, R8c"},
		{"requirement-paren", "the R1(b) completeness floor"},
		{"ac-id", "covered by AC-4"},
		{"hc-id", "the HC-3 git-tree guard"},
		{"req-id", "(Req 12)"},
		{"bead-id", "not reflected in bd list, mindspec-bk5t"},
		{"bead-id-child", "claimed mindspec-icaz.6"},
		{"issue-pr", "merged as PR #522"},
		{"issue-gh", "the GH #218 case"},
		{"issue-bare", "closes the #186 wedge"},
		{"incident", "(incident 2026-06-11)"},
		{"field-evidence", "The field evidence runs both ways"},
		{"incident-id", "the 2u0u split"},
		{"handover", "see the handover doc"},
		{"other-project", "On lola, the panel caught it"},
		{"home-path", "wrote to /Users/someone/x"},
		{"go-identifier", "selects a panel with panel.ForBead"},
		{"go-path", "runs internal/panel.Validate"},
	} {
		t.Run(tc.class, func(t *testing.T) {
			fixture := map[string]string{"go:fixture.go": tc.inject}
			if problems := scanAllowlistValidate("surface-refs", surfaceRefScanSurfaces(fixture), nil, fixture); len(problems) == 0 {
				t.Fatalf("expected a RED on %q", tc.inject)
			}
		})
	}
}

// TestSurfaceRefs_NegativeAllowedForms proves the consumer-format examples
// and plain words stay green.
func TestSurfaceRefs_NegativeAllowedForms(t *testing.T) {
	for _, s := range []string{
		"mindspec validate spec 001-my-feature",
		"run mindspec-plan-gate.sh",
		"<mindspec-root>/bin/agentmind",
		"| R4 | Codex | Empirical prober |",
		"`ADR-NNNN`: <why this ADR is relevant>",
		"an incidental choice",
		"`panel.json` and `config.yaml`",
	} {
		fixture := map[string]string{"go:fixture.go": s}
		if problems := scanAllowlistValidate("surface-refs", surfaceRefScanSurfaces(fixture), nil, fixture); len(problems) != 0 {
			t.Errorf("%q must not RED: %v", s, problems)
		}
	}
}

// TestSurfaceRefs_GoLegSkipsCommentsImportsAndTags proves the Go leg reads
// string literals only.
func TestSurfaceRefs_GoLegSkipsCommentsImportsAndTags(t *testing.T) {
	src := `package x

import _ "example.com/adr-0042/spec-124"

// A comment may cite ADR-0042 and spec 124.
type T struct {
	F string ` + "`json:\"f\" doc:\"ADR-0042\"`" + `
}

var msg = "refused (ADR-0042)"
`
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join(surfaceRefGoLiterals(f), "\n")
	if strings.Count(content, "ADR-0042") != 1 || !strings.Contains(content, "refused (ADR-0042)") {
		t.Fatalf("Go leg must see the one string literal only, got %q", content)
	}
	fixture := map[string]string{"go:x.go": content}
	if problems := scanAllowlistValidate("surface-refs", surfaceRefScanSurfaces(fixture), nil, fixture); len(problems) != 1 {
		t.Fatalf("expected exactly one RED, got %v", problems)
	}
}

// TestSurfaceRefs_UnusedAllowlistEntryReds proves a stale allowlist entry
// cannot linger.
func TestSurfaceRefs_UnusedAllowlistEntryReds(t *testing.T) {
	fixture := map[string]string{"go:fixture.go": "clean text\n"}
	stale := []acTenAllowEntry{{surface: "go:fixture.go", locator: "clean text", matched: "ADR-0001", count: 1, reason: "test"}}
	if problems := scanAllowlistValidate("surface-refs", surfaceRefScanSurfaces(fixture), stale, fixture); len(problems) == 0 {
		t.Fatal("expected an unused allowlist entry to RED")
	}
}
