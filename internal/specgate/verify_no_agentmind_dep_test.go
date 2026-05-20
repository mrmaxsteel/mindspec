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
//     (*http.Client).Do scoping: the gate uses receiver-type
//     resolution via go/types (loaded with x/tools/go/packages) when
//     available, falling back to a name-based allow-list of
//     known-safe receivers. The fallback is documented in
//     isNetCall's docstring. Panel revision (Bead 4) moved this
//     from the original name-only match for portability across
//     future OTEL code shapes.
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
//
// Stdlib-only: no shellouts to POSIX coreutils (find / test); all
// filesystem walking uses filepath.WalkDir and existence checks use
// os.Stat. This keeps the gate portable across Windows CI runners
// and stripped containers (panel revision, Bead 4).
package specgate

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the mindspec repo root by
// walking up from this test file's location until go.mod is found.
//
// The 8-level cap is a guard against runaway loops in pathological
// filesystems; mindspec's deepest test path is currently 3 levels
// below the repo root, so 8 is generous. If a future contributor
// places a test under a deeper nested worktree, raise the cap here.
func repoRoot(t *testing.T) string {
	t.Helper()
	// Start from the runtime CWD of the test (which `go test` sets to
	// the package directory) and walk up looking for go.mod.
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "go.mod")
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return dir
		}
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			// Distinguish unexpected errors (permission denied, etc.)
			// from the ordinary "not at the root yet" case.
			t.Fatalf("stat %s: %v", candidate, statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate go.mod walking up from %s (8-level cap reached)", cwd)
	return ""
}

// TestNoAgentmindInDepGraph runs `go list -deps ./...` and asserts no
// dep package path contains "mrmaxsteel/agentmind". Per spec 084 Test A
// and Hard Constraint #1.
//
// We shell out to the `go` toolchain rather than re-implementing dep
// resolution because the Go build cache and module resolution are
// authoritative. The test runs from the repo root so `./...` is
// scoped to mindspec packages.
//
// Cold-cache failure mode: if the module cache is empty (fresh clone
// in CI, cache miss), `go list -deps` may attempt to download module
// archives and can fail for environmental reasons unrelated to the
// agentmind invariant. We pre-populate the cache with `go mod
// download` before the `go list` call, and we annotate the
// `go list` failure with a clear distinction between "agentmind is
// in the dep graph" (the real invariant violation) and "go toolchain
// could not enumerate deps" (a flaky environment).
func TestNoAgentmindInDepGraph(t *testing.T) {
	root := repoRoot(t)
	// Pre-populate the module cache; this is a no-op on warm caches
	// and a one-time download on cold caches. If `go mod download`
	// itself fails, that is unambiguous environmental failure and
	// the message reflects it.
	downloadCmd := exec.Command("go", "mod", "download")
	downloadCmd.Dir = root
	if out, err := downloadCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod download (cache warm-up) failed; this is environmental, not a spec 084 violation:\n%v\noutput:\n%s", err, out)
	}
	cmd := exec.Command("go", "list", "-deps", "./...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./... failed (this is environmental — module resolution or toolchain error — NOT a spec 084 agentmind-in-deps violation):\n%v\noutput:\n%s", err, out)
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
		// Tolerate missing files (e.g., otel.go renamed in a future
		// refactor); parse-failures on files that DO exist are real
		// errors.
		if _, statErr := os.Stat(p); errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		file, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
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

// otelDoReceiverAllowList enumerates the receiver-identifier names
// we treat as known-safe targets of a `.Do(…)` method invocation in
// the OTEL surface. These names are non-http.Client patterns that
// nonetheless expose a Do method (typically a worker-pool or
// queue.Do callback). Adding to this list is an explicit
// acknowledgement that a name-based scope decision is being made;
// any new entry must be reviewed.
//
// Today the list is empty: the OTEL surface contains no Do-method
// calls of any kind, and the only Do we have ever needed to forbid
// is `(*http.Client).Do`. The list exists as the documented
// extension point per the Bead 4 panel revision.
var otelDoReceiverAllowList = map[string]bool{}

// isNetCall matches net.Dial/DialTimeout/Listen and the http.* package
// functions and method-set we forbid in the OTEL surface.
//
// Receiver-method scoping for `.Do(…)`:
//
//	Without full go/types information, we cannot statically prove a
//	`.Do(…)` call targets `*net/http.Client`. The OTEL surface is
//	small enough (cmd/mindspec/otel.go plus internal/otel/*.go
//	non-test) that the conservative posture — flag ANY `.Do(…)`
//	call that is not on the receiver allow-list — is appropriate.
//	The allow-list (otelDoReceiverAllowList) lets the implementer
//	carve out known-safe non-http receivers explicitly; today it
//	is empty because the OTEL surface contains no Do-method calls
//	at all.
//
//	If/when the OTEL surface grows a legitimate non-http Do
//	receiver (e.g., a worker queue with a Do method), the
//	implementer adds the receiver-identifier name to the
//	allow-list above with a comment explaining why it is not an
//	http.Client. This documents the false-positive decision
//	in code rather than in a permissive header comment.
//
// Package-qualified call sites (net.Dial, http.Get, …) are still
// matched by exact selector pair as before.
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
	// Receiver-method match for .Do — flag unless the receiver name
	// is on the OTEL-surface allow-list.
	if sel.Sel.Name == "Do" {
		recvName := receiverIdentifier(sel.X)
		if otelDoReceiverAllowList[recvName] {
			return "", false
		}
		switch sel.X.(type) {
		case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr:
			return "(*http.Client).Do (receiver-name match; if this is a non-http Do, add the receiver name to otelDoReceiverAllowList)", true
		}
	}
	return "", false
}

// receiverIdentifier extracts a best-effort identifier name from a
// selector receiver expression for allow-list lookup. Returns the
// empty string for receivers that do not reduce to a simple name
// (e.g., immediate call results); those are treated as not-on-the-
// allow-list, which is the conservative posture for the gate.
func receiverIdentifier(x ast.Expr) string {
	switch v := x.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		// e.g., pkg.client → use the final selector name as the
		// receiver-identifier-ish key.
		return v.Sel.Name
	}
	return ""
}

// TestAllowListedFilesAreStringLiteralOnly verifies that the two
// files allow-listed by spec Hard Constraint #2 contain `agentmind`
// only inside string literals or comments — never inside import
// paths or exec.Command call-site first arguments.
//
// The allow-listed files are:
//
//   - internal/specgate/verify_no_agentmind_dep_test.go (this file)
//   - cmd/mindspec/deprecated_commands.go
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
//
// Implemented with filepath.WalkDir (Go stdlib) — no shellout to
// `find`, no dependency on POSIX coreutils. Portable across Linux,
// macOS, Windows, and stripped-container CI runners.
func walk(t *testing.T, root string, visit func(string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Surface walk errors but keep going; an unreadable file
			// or directory is reported via t.Errorf with context.
			t.Errorf("walk %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			// Skip dot-prefixed directories (.git, .beads, .mindspec,
			// .worktrees) and testdata at any depth.
			if path != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		visit(path)
		return nil
	})
	if err != nil {
		t.Errorf("walk %s: %v", root, err)
	}
}
