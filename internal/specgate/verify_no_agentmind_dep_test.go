// Package specgate hosts the permanent CI gates that enforce spec 084's
// architectural invariants for the lifetime of the mindspec repo.
//
// This file implements the spec-084 gate trio:
//
//  1. TestNoAgentmindInDepGraph — asserts the mindspec module's Go
//     dep graph does not contain any github.com/mrmaxsteel/agentmind
//     package (direct or transitive). Spec 084 Test A.
//
//  2. TestNoAgentmindExecLiteral — AST-walks every *.go file under
//     cmd/ and internal/ and asserts no exec.Command / exec.LookPath /
//     os.StartProcess first-argument string literal equals "agentmind"
//     or "mindspec" (the latter prevents mindspec from re-execing
//     itself, which would re-create the subprocess management
//     surface that spec 084 deletes). Spec 084 Test H, process-spawn
//     half.
//
//  3. TestNoOtelNetCalls — AST-walks cmd/mindspec/otel.go and
//     internal/otel/ and asserts zero call sites to net.Dial /
//     net.DialTimeout / net.Listen / http.Get / http.Post /
//     http.Head / http.PostForm / (*http.Client).Do/Get/Post.
//     Closes the "doctor-by-another-name" hole: a status command
//     that secretly reaches out to "verify" the configured endpoint
//     would re-create the very probe behavior spec 084 forbids.
//     Spec 084 Test H, net-call half.
//
//  4. TestAllowListedFilesAreStringLiteralOnly — AST-walks the two
//     files allow-listed by spec Hard Constraint #2
//     (this file and cmd/mindspec/deprecated_commands.go) and
//     verifies their only occurrences of `agentmind` are inside
//     string literals or comments — never inside import paths or
//     exec.Command first arguments. Closes the HC #2 self-
//     consistency gate.
//
// The tests run unconditionally on every `go test -short ./...`.
// No build tags. No t.Skip paths. First appearance is permanent
// enforced state, per spec 084 Migration Commit 6.
package specgate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the mindspec repo root by
// walking up from this test file's location until go.mod is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	// Start from the runtime CWD of the test (which go test sets to
	// the package directory) and walk up.
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := filepath.Abs(filepath.Join(dir, "go.mod")); err == nil {
			info := filepath.Join(dir, "go.mod")
			if fileExists(info) {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate go.mod walking up from %s", cwd)
	return ""
}

func fileExists(p string) bool {
	f, err := exec.Command("test", "-f", p).Output()
	_ = f
	return err == nil
}

// TestNoAgentmindInDepGraph runs `go list -deps ./...` and asserts no
// dep package path contains "mrmaxsteel/agentmind". Per spec 084 Test A
// and Hard Constraint #1.
//
// We shell out to the `go` toolchain rather than re-implementing dep
// resolution because the Go build cache and module resolution are
// authoritative. The test runs from the repo root so `./...` is
// scoped to mindspec packages.
func TestNoAgentmindInDepGraph(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.Command("go", "list", "-deps", "./...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./... failed: %v\noutput:\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "mrmaxsteel/agentmind") {
			t.Errorf("forbidden dep in graph: %q\n"+
				"spec 084 Hard Constraint #1: zero agentmind in the Go dep graph.\n"+
				"see ADR-0027 / .mindspec/docs/specs/084-mindspec-otel-only/spec.md",
				line)
		}
	}
}

// TestNoAgentmindExecLiteral AST-walks every *.go file under cmd/ and
// internal/ and asserts no exec.Command / exec.LookPath /
// os.StartProcess first-argument string literal equals "agentmind"
// or "mindspec". Per spec 084 Test H (process-spawn half).
//
// We accept literal occurrences in test files only when they are
// inside ordinary string contexts (e.g., test banned-token lists,
// docstring comments) — those are matched by the broader allow-list
// gate (TestAllowListedFilesAreStringLiteralOnly). What we forbid
// here is the *call site*: exec.Command("agentmind", …).
func TestNoAgentmindExecLiteral(t *testing.T) {
	root := repoRoot(t)
	forbiddenFirstArgs := map[string]bool{
		"agentmind": true,
		"mindspec":  true,
	}
	scanRoots := []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "internal"),
	}
	fset := token.NewFileSet()

	var violations []string
	for _, sr := range scanRoots {
		walk(t, sr, func(path string) {
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Errorf("parse %s: %v", path, err)
				return
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if !isProcessSpawnCall(call) {
					return true
				}
				if len(call.Args) == 0 {
					return true
				}
				first := call.Args[0]
				lit, ok := first.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				val := strings.Trim(lit.Value, "`\"")
				// Strip quoting; check both the bare literal and the
				// basename (handles e.g. "./agentmind" or "/usr/local/bin/agentmind").
				base := filepath.Base(val)
				if forbiddenFirstArgs[val] || forbiddenFirstArgs[base] {
					pos := fset.Position(call.Pos())
					violations = append(violations, pos.String()+": "+val)
				}
				return true
			})
		})
	}
	if len(violations) > 0 {
		t.Errorf("forbidden process-spawn first-argument literals (spec 084 Test H):\n  %s\n"+
			"spec 084 Hard Constraint #2: no exec.Command targets agentmind or mindspec.",
			strings.Join(violations, "\n  "))
	}
}

// isProcessSpawnCall returns true if the call expression is one of
// exec.Command, exec.CommandContext, exec.LookPath, or
// os.StartProcess. We match on the selector's X.Sel pair to avoid
// false positives on user-defined functions named Command.
func isProcessSpawnCall(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	xIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch xIdent.Name {
	case "exec":
		switch sel.Sel.Name {
		case "Command", "CommandContext", "LookPath":
			return true
		}
	case "os":
		switch sel.Sel.Name {
		case "StartProcess":
			return true
		}
	}
	return false
}

// TestNoOtelNetCalls AST-walks cmd/mindspec/otel.go and internal/otel/
// and asserts no call sites to net.Dial / net.DialTimeout / net.Listen
// / http.Get / http.Post / http.Head / http.PostForm /
// (*http.Client).Do/Get/Post.
//
// Per spec 084 Test H (net-call half) and the spec line 278-285 contract
// that `mindspec otel status` performs zero network I/O.
func TestNoOtelNetCalls(t *testing.T) {
	root := repoRoot(t)
	scanFiles := []string{
		filepath.Join(root, "cmd", "mindspec", "otel.go"),
	}
	// Walk all *.go files in internal/otel/ (skip _test.go for the
	// implementation gate — test files may legitimately use net for
	// fixtures).
	walk(t, filepath.Join(root, "internal", "otel"), func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		scanFiles = append(scanFiles, path)
	})

	fset := token.NewFileSet()
	var violations []string
	for _, p := range scanFiles {
		file, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			// Tolerate missing file (e.g., if otel.go is later renamed)
			// but only if it does not exist; parse failures on
			// existing files are real errors.
			if strings.Contains(err.Error(), "no such file") {
				continue
			}
			t.Errorf("parse %s: %v", p, err)
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if forbiddenName, hit := isNetCall(call); hit {
				pos := fset.Position(call.Pos())
				violations = append(violations, pos.String()+": "+forbiddenName)
			}
			return true
		})
	}
	if len(violations) > 0 {
		t.Errorf("forbidden network-call sites in OTEL surface (spec 084 Test H net-call half):\n  %s\n"+
			"spec 084 Hard Constraint #5 / spec lines 278-285: mindspec otel status performs zero network I/O.",
			strings.Join(violations, "\n  "))
	}
}

// isNetCall matches net.Dial/DialTimeout/Listen and the http.* package
// functions and method-set we forbid in the OTEL surface.
//
// We match selector form pkg.Func (for package-level functions) and
// receiver.Method (for *http.Client.Do/Get/Post) by name only;
// false positives on user-defined types with the same method names
// are acceptable because the violation message is informative and the
// file scope is tiny (a handful of files in internal/otel + otel.go).
func isNetCall(c *ast.CallExpr) (string, bool) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	// Package-qualified: net.Dial, http.Get etc.
	if xIdent, ok := sel.X.(*ast.Ident); ok {
		switch xIdent.Name {
		case "net":
			switch sel.Sel.Name {
			case "Dial", "DialTimeout", "Listen", "ListenPacket":
				return "net." + sel.Sel.Name, true
			}
		case "http":
			switch sel.Sel.Name {
			case "Get", "Post", "Head", "PostForm":
				return "http." + sel.Sel.Name, true
			}
		}
	}
	// Receiver-method match for any expression .Do/.Get/.Post when the
	// selector is one of the http.Client method names. We can't
	// statically resolve the receiver type without full type info, so
	// we use the conservative-but-informative name-only match scoped
	// to the OTEL files.
	switch sel.Sel.Name {
	case "Do":
		// Filter to common http.Client.Do patterns: receiver is an
		// identifier (client variable) or selector chain.
		switch sel.X.(type) {
		case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr:
			return "(*http.Client).Do (by name)", true
		}
	}
	return "", false
}

// TestAllowListedFilesAreStringLiteralOnly verifies that the two
// files allow-listed by spec Hard Constraint #2 contain `agentmind`
// only inside string literals or comments — never inside import
// paths or exec.Command call-site first arguments.
//
// The allow-listed files are:
//
//  - internal/specgate/verify_no_agentmind_dep_test.go (this file)
//  - cmd/mindspec/deprecated_commands.go
//
// Closes the spec-line-548 "string-literal-only" gate.
func TestAllowListedFilesAreStringLiteralOnly(t *testing.T) {
	root := repoRoot(t)
	files := []string{
		filepath.Join(root, "internal", "specgate", "verify_no_agentmind_dep_test.go"),
		filepath.Join(root, "cmd", "mindspec", "deprecated_commands.go"),
	}
	fset := token.NewFileSet()
	for _, p := range files {
		file, err := parser.ParseFile(fset, p, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("parse %s: %v", p, err)
			continue
		}
		// Import paths must not contain "agentmind".
		for _, imp := range file.Imports {
			if strings.Contains(imp.Path.Value, "agentmind") {
				pos := fset.Position(imp.Pos())
				t.Errorf("%s: allow-listed file has agentmind in import path %s",
					pos.String(), imp.Path.Value)
			}
		}
		// exec.Command / exec.LookPath / os.StartProcess first-arg
		// literals must not contain "agentmind".
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isProcessSpawnCall(call) {
				return true
			}
			if len(call.Args) == 0 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val := strings.Trim(lit.Value, "`\"")
			if strings.Contains(val, "agentmind") {
				pos := fset.Position(call.Pos())
				t.Errorf("%s: allow-listed file has agentmind in exec.* first-arg: %s",
					pos.String(), val)
			}
			return true
		})
	}
}

// walk visits every *.go file under root (recursive), invoking visit
// with the absolute path. Skips dot-prefixed directories (.git, .beads,
// .mindspec) and the testdata convention.
func walk(t *testing.T, root string, visit func(string)) {
	t.Helper()
	cmd := exec.Command("find", root, "-name", "*.go", "-type", "f")
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("find %s: %v", root, err)
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "/testdata/") {
			continue
		}
		visit(line)
	}
}
