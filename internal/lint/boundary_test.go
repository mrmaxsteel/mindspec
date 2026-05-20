// Package lint hosts AST-based boundary lints that run as ordinary
// Go tests. See ADR-0030 for the doctrine: the enforcement packages
// (internal/{validate,approve,complete,state,phase}) may not shell out
// to git or bd directly; git I/O routes through internal/executor and
// bd routes through internal/bead.
//
// TestEnforcementHasNoGitLeaks walks the AST of each enforcement
// package and fails on:
//
//  1. PRIMARY GATE — banned imports: "os/exec" or
//     "github.com/mrmaxsteel/mindspec/internal/gitutil".
//  2. SECONDARY GATE — banned literal call sites:
//     exec.Command("git", ...), exec.Command("bd", ...),
//     exec.CommandContext(_, "git", ...), exec.CommandContext(_, "bd", ...).
//     One level of constant folding catches
//     `const cmd = "git"; exec.Command(cmd, ...)`. Variable-bound or
//     computed forms are intentionally NOT caught — the import ban is
//     what closes that hole.
//
// Allowlist escape hatch: a file may opt out by placing a file-doc
// comment (the comment group attached to ast.File.Doc, immediately
// before the package clause) containing a line whose stripped text
// begins with the case-sensitive prefix "boundary-allowlisted:". The
// remainder of that line is the reviewer's reason and is not parsed;
// the reviewer signature on the commit is what gates the exemption.
//
// Test-only Go files (*_test.go) are excluded from both gates so test
// fixtures may exercise the historical shapes.
package lint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const allowlistPrefix = "boundary-allowlisted:"

// bannedImports is the set of import paths an enforcement-package
// non-test file may not import.
var bannedImports = map[string]bool{
	`"os/exec"`: true,
	`"github.com/mrmaxsteel/mindspec/internal/gitutil"`: true,
}

// bannedExecLiterals are the first-string-arguments to
// exec.Command / exec.CommandContext that the secondary gate flags.
var bannedExecLiterals = map[string]bool{
	"git": true,
	"bd":  true,
}

// finding describes a single boundary violation surfaced by the
// walker. Pinned by file + line + nearest enclosing func so the
// diagnostic is actionable.
type finding struct {
	File    string
	Line    int
	FuncName string
	Kind    string // "import" or "call"
	Detail  string
}

func (f finding) String() string {
	loc := f.File
	if f.Line > 0 {
		loc = loc + ":" + itoa(f.Line)
	}
	fn := f.FuncName
	if fn == "" {
		fn = "<file-scope>"
	}
	return loc + " (" + fn + "): banned " + f.Kind + " " + f.Detail
}

// itoa avoids pulling strconv for a single int — keeps the test
// file's import surface minimal.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestEnforcementHasNoGitLeaks is the lifetime invariant installed by
// spec 085. See the package doc-comment for the full doctrine.
func TestEnforcementHasNoGitLeaks(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(thisFile)
	repoRoot := filepath.Join(thisDir, "..", "..")
	enforcementPkgs := []string{
		filepath.Join(repoRoot, "internal", "validate"),
		filepath.Join(repoRoot, "internal", "approve"),
		filepath.Join(repoRoot, "internal", "complete"),
		filepath.Join(repoRoot, "internal", "state"),
		filepath.Join(repoRoot, "internal", "phase"),
	}

	var allFindings []finding
	for _, pkgPath := range enforcementPkgs {
		entries, err := os.ReadDir(pkgPath)
		if err != nil {
			t.Fatalf("read enforcement pkg %s: %v", pkgPath, err)
		}
		fset := token.NewFileSet()
		for _, ent := range entries {
			name := ent.Name()
			if ent.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			full := filepath.Join(pkgPath, name)
			file, err := parser.ParseFile(fset, full, nil, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse %s: %v", full, err)
			}
			allFindings = append(allFindings, walkFile(fset, file, full)...)
		}
	}

	if len(allFindings) > 0 {
		t.Fatalf("boundary violations in enforcement packages (%d):\n  %s",
			len(allFindings), joinFindings(allFindings))
	}

	t.Run("seed_fixtures", func(t *testing.T) {
		// Mutation proof: parse the seed fixtures (which simulate
		// the historical leaks removed in Beads 2 and 3) and confirm
		// the walker still flags them. If either fixture stops
		// producing failures, the walker has regressed.
		fset := token.NewFileSet()

		docsyncPath := filepath.Join(thisDir, "testdata", "seed_docsync_leak.go.txt")
		docsyncSrc, err := os.ReadFile(docsyncPath)
		if err != nil {
			t.Fatalf("read seed_docsync_leak: %v", err)
		}
		docsyncFile, err := parser.ParseFile(fset, docsyncPath, docsyncSrc, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse seed_docsync_leak: %v", err)
		}
		docsyncFindings := walkFile(fset, docsyncFile, docsyncPath)
		assertFindingPresent(t, docsyncFindings, "import",
			`"github.com/mrmaxsteel/mindspec/internal/gitutil"`, "")

		beadsPath := filepath.Join(thisDir, "testdata", "seed_beads_leak.go.txt")
		beadsSrc, err := os.ReadFile(beadsPath)
		if err != nil {
			t.Fatalf("read seed_beads_leak: %v", err)
		}
		beadsFile, err := parser.ParseFile(fset, beadsPath, beadsSrc, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse seed_beads_leak: %v", err)
		}
		beadsFindings := walkFile(fset, beadsFile, beadsPath)
		assertFindingPresent(t, beadsFindings, "import", `"os/exec"`, "")
		assertFindingPresent(t, beadsFindings, "call", `exec.Command("bd", ...)`, "CheckBeadExists")
	})

	t.Run("allowlist_marker", func(t *testing.T) {
		// Allowlist proof: a file with the marker comment is
		// exempt from BOTH gates, even when it imports "os/exec"
		// and uses exec.Command("git", ...).
		src := `// boundary-allowlisted: required for foo; reviewed by Max
package validate

import "os/exec"

func leak() error {
	return exec.Command("git", "status").Run()
}
`
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "allowlisted.go", src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse allowlist fixture: %v", err)
		}
		findings := walkFile(fset, file, "allowlisted.go")
		if len(findings) != 0 {
			t.Fatalf("expected no findings on allowlisted file, got %d:\n  %s",
				len(findings), joinFindings(findings))
		}
	})
}

// walkFile applies both gates to a single parsed file. Returns the
// empty slice when the file is allowlisted or clean.
func walkFile(fset *token.FileSet, file *ast.File, path string) []finding {
	if isAllowlisted(file) {
		return nil
	}

	var out []finding

	// PRIMARY GATE: import ban.
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		if bannedImports[imp.Path.Value] {
			out = append(out, finding{
				File:   path,
				Line:   fset.Position(imp.Pos()).Line,
				Kind:   "import",
				Detail: imp.Path.Value,
			})
		}
	}

	// One-level constant fold: collect top-level string consts so we
	// catch `const cmd = "git"; exec.Command(cmd, ...)`.
	consts := collectStringConsts(file)

	// SECONDARY GATE: exec.Command / exec.CommandContext literal walker.
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name != "exec" {
			return true
		}
		var argIdx int
		switch sel.Sel.Name {
		case "Command":
			argIdx = 0
		case "CommandContext":
			argIdx = 1
		default:
			return true
		}
		if len(call.Args) <= argIdx {
			return true
		}
		litValue, okLit := resolveStringArg(call.Args[argIdx], consts)
		if !okLit {
			return true
		}
		if bannedExecLiterals[litValue] {
			out = append(out, finding{
				File:     path,
				Line:     fset.Position(call.Pos()).Line,
				FuncName: enclosingFuncName(file, call.Pos()),
				Kind:     "call",
				Detail:   "exec." + sel.Sel.Name + "(" + quote(litValue) + ", ...)",
			})
		}
		return true
	})

	return out
}

// isAllowlisted reports whether the file's file-doc comment carries
// the boundary-allowlisted marker on any of its lines.
func isAllowlisted(file *ast.File) bool {
	if file.Doc == nil {
		return false
	}
	for _, c := range file.Doc.List {
		text := c.Text
		// Strip // or /* */ framing.
		switch {
		case strings.HasPrefix(text, "//"):
			text = strings.TrimPrefix(text, "//")
		case strings.HasPrefix(text, "/*"):
			text = strings.TrimPrefix(text, "/*")
			text = strings.TrimSuffix(text, "*/")
		}
		// Comment text may contain multiple lines (for /* */ form).
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, allowlistPrefix) {
				return true
			}
		}
	}
	return false
}

// collectStringConsts walks the file's top-level CONST declarations
// and returns a map of constant-name -> unquoted string value for
// string-typed constants. One level only — no transitive folding.
func collectStringConsts(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				bl, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || bl.Kind != token.STRING {
					continue
				}
				if v, ok := unquote(bl.Value); ok {
					out[name.Name] = v
				}
			}
		}
	}
	return out
}

// resolveStringArg returns the string value of a call argument when
// it is a string literal or a one-level-folded string-typed const.
func resolveStringArg(expr ast.Expr, consts map[string]string) (string, bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		return unquote(v.Value)
	case *ast.Ident:
		if val, ok := consts[v.Name]; ok {
			return val, true
		}
	}
	return "", false
}

// unquote strips the surrounding "..." or `...` from a string
// literal's raw form. Returns the unquoted value and ok=true on
// success.
func unquote(raw string) (string, bool) {
	if len(raw) < 2 {
		return "", false
	}
	first, last := raw[0], raw[len(raw)-1]
	if (first == '"' && last == '"') || (first == '`' && last == '`') {
		return raw[1 : len(raw)-1], true
	}
	return "", false
}

// quote wraps a string in double quotes for diagnostic output.
func quote(s string) string {
	return `"` + s + `"`
}

// enclosingFuncName finds the nearest enclosing *ast.FuncDecl for a
// position. Returns the empty string if the position is at file
// scope (e.g. inside a var-initializer).
func enclosingFuncName(file *ast.File, pos token.Pos) string {
	var found string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Pos() <= pos && pos <= fn.End() {
			found = fn.Name.Name
			return found
		}
	}
	return found
}

// assertFindingPresent fails the test if no finding in `findings`
// matches the given kind + detail (substring) + funcName (substring,
// empty to skip).
func assertFindingPresent(t *testing.T, findings []finding, kind, detail, funcName string) {
	t.Helper()
	for _, f := range findings {
		if f.Kind != kind {
			continue
		}
		if !strings.Contains(f.Detail, detail) {
			continue
		}
		if funcName != "" && !strings.Contains(f.FuncName, funcName) {
			continue
		}
		return
	}
	t.Fatalf("expected finding kind=%q detail~=%q func~=%q, got %d findings:\n  %s",
		kind, detail, funcName, len(findings), joinFindings(findings))
}

func joinFindings(fs []finding) string {
	parts := make([]string, len(fs))
	for i, f := range fs {
		parts[i] = f.String()
	}
	return strings.Join(parts, "\n  ")
}
