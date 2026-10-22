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

func TestMsPanelWorkflow_AllowedCLIExactSet(t *testing.T) {
	content := WorkflowFiles()["ms-panel.js"]
	start := strings.Index(content, "const ALLOWED_CLI = [")
	if start < 0 {
		t.Fatal("allowed command array missing")
	}
	end := strings.Index(content[start:], "];")
	if end < 0 {
		t.Fatal("allowed command array incomplete")
	}
	literals := scanJSLiterals(content[start : start+end+2])
	want := map[string]bool{}
	for _, verb := range []string{"create", "verify", "tally", "launch", "checkout", "prompt", "submit", "recover"} {
		want["mindspec panel "+verb] = true
	}
	if len(literals) != len(want) {
		t.Fatalf("allowed commands=%+v", literals)
	}
	for _, lit := range literals {
		if !want[lit.value] {
			t.Fatalf("unexpected or duplicate command %q", lit.value)
		}
		delete(want, lit.value)
	}
	if len(want) != 0 {
		t.Fatalf("missing commands: %v", want)
	}
	if strings.Contains(content, "mindspec complete") {
		t.Fatal("lifecycle command in workflow")
	}
	if len(regexp.MustCompile(`function\s+buildCommand\s*\(`).FindAllString(content, -1)) != 1 {
		t.Fatal("command construction must have one entry point")
	}
	binding := regexp.MustCompile(`const \[([A-Z, ]+)\] = ALLOWED_CLI;`).FindStringSubmatch(content)
	if len(binding) != 2 || len(strings.Split(binding[1], ",")) != 8 {
		t.Fatal("command identifiers are not bound to the allowed set")
	}
	for _, path := range []string{"workflows/ms-panel.js", "../../.claude/workflows/ms-panel.js"} {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if string(raw) != content {
			t.Fatal("workflow copies differ")
		}
		if regexp.MustCompile(`(?i)/tmp|codex|copilot|gemini|claude`).Match(raw) {
			t.Fatal("tool family or scratch literal in workflow")
		}
	}
}

// TestMsPanelWorkflow_ShellMetacharRejectsBareDollar is the spec-113 R2 floor
// pin: SHELL_METACHAR_RE must reject a bare `$` (not merely the `$(`
// digraph), so `$HOME`/`${x}`/`$x` variable-expansion can no longer survive
// validateShellSafe. Go cannot execute the JS regex, so this is a
// string-level pin (same style as TestMsPanelWorkflow_AllowedCLIExactSet)
// asserting the exact declaration line appears exactly once; the behavioral
// match matrix (positive/negative rows) runs under node in verification.
func TestMsPanelWorkflow_ShellMetacharRejectsBareDollar(t *testing.T) {
	content := WorkflowFiles()["ms-panel.js"]
	if content == "" {
		t.Fatal(`WorkflowFiles()["ms-panel.js"] is empty`)
	}

	const wantDecl = `const SHELL_METACHAR_RE = /[\x60;|&\n$]/;`
	if n := strings.Count(content, wantDecl); n != 1 {
		t.Errorf("expected the exact declaration %q exactly once, found %d", wantDecl, n)
	}
}
