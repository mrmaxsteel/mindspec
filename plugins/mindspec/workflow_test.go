package pluginmindspec

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestWorkflowFiles_EmbedsMsPanel pins embed == plugin copy (spec 111 R8):
// WorkflowFiles() must return the ms-panel.js content, and that content must
// be byte-identical to the on-disk plugin source read independently via
// os.ReadFile (not through the same embed.FS the accessor uses), so a wrong
// embed glob or a stale embedded copy would be caught.
func TestWorkflowFiles_EmbedsMsPanel(t *testing.T) {
	got := WorkflowFiles()["ms-panel.js"]
	if got == "" {
		t.Fatal(`WorkflowFiles()["ms-panel.js"] is empty`)
	}

	want, err := os.ReadFile("workflows/ms-panel.js")
	if err != nil {
		t.Fatalf("reading workflows/ms-panel.js: %v", err)
	}

	if got != string(want) {
		t.Error(`WorkflowFiles()["ms-panel.js"] does not match the on-disk plugin source`)
	}
}

// jsLiteral is one string/template literal extracted from a JS source file,
// with its byte offsets (start = opening quote, end = one past the closing
// quote) so callers can tell whether it falls inside a particular source
// span.
type jsLiteral struct {
	value string // the raw text between the quotes (unescaped as written)
	start int
	end   int
}

// scanJSLiterals extracts every single-quoted, double-quoted, and
// backtick-delimited string/template literal in src, skipping `//` line
// comments and /* */ block comments so a literal mentioned only in
// commentary is never mistaken for code. For a backtick template literal,
// the extracted value is the RAW text between the backticks, including any
// ${...} interpolation syntax verbatim (nested expressions are not
// decomposed) — sufficient for a source-text scan; this file's workflow
// never nests a same-style quote inside a ${...} substitution in a way that
// would confuse this scan.
func scanJSLiterals(src string) []jsLiteral {
	var literals []jsLiteral
	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == '/' && i+1 < n && src[i+1] == '/':
			if j := strings.IndexByte(src[i:], '\n'); j == -1 {
				i = n
			} else {
				i += j + 1
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			if j := strings.Index(src[i+2:], "*/"); j == -1 {
				i = n
			} else {
				i = i + 2 + j + 2
			}
		case c == '"' || c == '\'' || c == '`':
			quote := c
			start := i
			i++
			for i < n {
				if src[i] == '\\' {
					i += 2
					continue
				}
				if src[i] == quote {
					i++
					break
				}
				i++
			}
			end := i
			if end > start+1 && end <= n {
				literals = append(literals, jsLiteral{value: src[start+1 : end-1], start: start, end: end})
			}
		default:
			i++
		}
	}
	return literals
}

// TestMsPanelWorkflow_AllowedCLIExactSet is the carry-forward floor-raise
// (spec 111 AC4, R2/R4/R5): ALLOWED_CLI must admit EXACTLY the four
// permitted commands (not merely include them), the lifecycle merge-terminal
// command must appear nowhere in the file, exactly four verb identifiers
// must be destructured from ALLOWED_CLI in one binding, buildCommand must be
// the file's sole command-construction chokepoint (defined exactly once),
// and a positive enumeration of every mindspec-/codex-bearing string literal
// in the file must show it occurs ONLY inside the ALLOWED_CLI array
// declaration itself, closing both the indirection class (a concatenation
// fragment or template that doesn't equal one of the four exact strings) and
// the exact-match-bypass class (a call site that reuses the literal text
// directly rather than routing through buildCommand).
func TestMsPanelWorkflow_AllowedCLIExactSet(t *testing.T) {
	content := WorkflowFiles()["ms-panel.js"]
	if content == "" {
		t.Fatal(`WorkflowFiles()["ms-panel.js"] is empty`)
	}

	const (
		verbPanelCreate = "mindspec panel create"
		verbCodexExec   = "codex exec --sandbox read-only --skip-git-repo-check"
		verbPanelVerify = "mindspec panel verify"
		verbPanelTally  = "mindspec panel tally"
	)
	wantSet := map[string]bool{
		verbPanelCreate: true,
		verbCodexExec:   true,
		verbPanelVerify: true,
		verbPanelTally:  true,
	}
	mindspecVerbs := map[string]bool{
		verbPanelCreate: true,
		verbPanelVerify: true,
		verbPanelTally:  true,
	}

	// mindspec complete must appear NOWHERE — not as a command, not as a
	// comment — checked over the WHOLE file content, not just extracted
	// literals.
	if strings.Contains(content, "mindspec complete") {
		t.Error(`"mindspec complete" must not appear anywhere in ms-panel.js`)
	}

	// Locate the ALLOWED_CLI array literal's exact source span.
	arrayDeclIdx := strings.Index(content, "const ALLOWED_CLI = [")
	if arrayDeclIdx == -1 {
		t.Fatal("could not find `const ALLOWED_CLI = [` in ms-panel.js")
	}
	closeRel := strings.Index(content[arrayDeclIdx:], "];")
	if closeRel == -1 {
		t.Fatal("could not find the closing `];` of the ALLOWED_CLI array in ms-panel.js")
	}
	arrayStart := arrayDeclIdx
	arrayEnd := arrayDeclIdx + closeRel + len("];")
	arraySpan := content[arrayStart:arrayEnd]

	// The array's own literals must equal EXACTLY the four-element wanted set.
	arrayLiterals := scanJSLiterals(arraySpan)
	if len(arrayLiterals) != 4 {
		t.Fatalf("ALLOWED_CLI array literal has %d string elements, want 4: %+v", len(arrayLiterals), arrayLiterals)
	}
	gotSet := make(map[string]bool, 4)
	for _, lit := range arrayLiterals {
		gotSet[lit.value] = true
	}
	if len(gotSet) != len(wantSet) {
		t.Errorf("ALLOWED_CLI has duplicate or malformed entries: %+v", gotSet)
	}
	for verb := range wantSet {
		if !gotSet[verb] {
			t.Errorf("ALLOWED_CLI is missing required entry %q", verb)
		}
	}
	for verb := range gotSet {
		if !wantSet[verb] {
			t.Errorf("ALLOWED_CLI has an unexpected entry %q (must admit EXACTLY the four permitted commands)", verb)
		}
	}

	// Exactly one destructuring binding of four identifiers from ALLOWED_CLI.
	destructureRe := regexp.MustCompile(`const\s*\[\s*(\w+)\s*,\s*(\w+)\s*,\s*(\w+)\s*,\s*(\w+)\s*\]\s*=\s*ALLOWED_CLI\s*;`)
	matches := destructureRe.FindAllStringSubmatch(content, -1)
	if len(matches) != 1 {
		t.Fatalf("expected exactly one `const [ID1, ID2, ID3, ID4] = ALLOWED_CLI;` destructuring binding, found %d", len(matches))
	}
	if len(matches[0]) != 5 {
		t.Fatalf("destructuring binding did not capture exactly four identifiers: %v", matches[0])
	}

	// buildCommand is defined exactly once — the sole command-construction
	// chokepoint.
	buildCommandDefRe := regexp.MustCompile(`\bfunction\s+buildCommand\s*\(`)
	if n := len(buildCommandDefRe.FindAllString(content, -1)); n != 1 {
		t.Errorf("expected buildCommand to be defined exactly once, found %d definitions", n)
	}

	// Positive enumeration: every mindspec-/codex-bearing literal in the
	// WHOLE file must be exactly one of the four allowed strings AND must
	// occur ONLY inside the ALLOWED_CLI array span above — never routed
	// around buildCommand by a call site that retypes or reuses the literal
	// text.
	allLiterals := scanJSLiterals(content)
	for _, lit := range allLiterals {
		trimmed := strings.TrimSpace(lit.value)
		insideArray := lit.start >= arrayStart && lit.end <= arrayEnd

		if strings.Contains(lit.value, "mindspec") {
			if !insideArray {
				t.Errorf("found a mindspec-bearing literal outside the ALLOWED_CLI array declaration: %q (byte offset %d) — command construction must route through buildCommand, never a retyped/reused literal", lit.value, lit.start)
				continue
			}
			if !mindspecVerbs[trimmed] {
				t.Errorf("mindspec-bearing literal inside the array is not one of the three allowed panel verbs: %q", trimmed)
			}
		}

		if strings.Contains(lit.value, "codex exec") {
			if !insideArray {
				t.Errorf("found a codex-exec-bearing literal outside the ALLOWED_CLI array declaration: %q (byte offset %d)", lit.value, lit.start)
				continue
			}
			if trimmed != verbCodexExec {
				t.Errorf("codex-exec-bearing literal inside the array does not exactly match the allowed sandboxed form: %q", trimmed)
			}
		}
	}

	// Sanity-check the sandboxed codex entry and the claude-sub / .codex.log
	// substrings the spec's own grep-based AC also pins.
	if !strings.Contains(content, "claude-sub") {
		t.Error(`"claude-sub" must appear in ms-panel.js (R4 substitution reviewer_id)`)
	}
	if !strings.Contains(content, ".codex.log") {
		t.Error(`".codex.log" must appear in ms-panel.js (R3b audit artifact)`)
	}
}
