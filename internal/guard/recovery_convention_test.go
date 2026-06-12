package guard

// Recovery-line convention test (spec 092-agent-contract-hardening,
// Req 21; convention defined by Req 12, documented in
// ADR-0035-agent-error-contract).
//
// This test makes the convention enforceable instead of aspirational:
//
//  1. It walks the AST of this package's non-test files and collects
//     every exported guard-failure constructor — any exported,
//     receiver-less function that returns an error or carries "Failure"
//     in its name. A new constructor without a fixture below FAILS the
//     test, so a guard added after spec 092 cannot silently bypass the
//     convention.
//  2. It invokes every fixture with representative inputs and fails
//     when any produced failure message lacks a final
//     `recovery: <command>` line.
//
// Guard-failure CALL SITES converted by spec 092's later beads (the
// dirty-tree guard in `next`, the clean-tree guard in `complete`, the
// phase/bead gates in `impl approve`, ...) live in other packages; as
// each bead routes a site through NewFailure/FormatFailure it must add
// a unit test in that site's package asserting
// guard.HasFinalRecoveryLine on the produced message, mirroring the
// fixtures here. Spec 093 Bead 1 sites follow the same per-site
// pattern: ClaimFailure/WorktreeSetupFailure in internal/next
// (guard_test.go) and adrDivergenceFailure in internal/complete
// (complete_test.go); no new constructor lives in THIS package, so
// conventionFixtures needs no extension (the AST walk below covers
// this package only, and importing those packages here would cycle).

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// conventionExempt lists exported error-returning functions in this
// package that are NOT guard-failure constructors (e.g. plumbing that
// returns I/O errors). Every entry needs a reason; keep it short.
var conventionExempt = map[string]string{}

// failureFixture forces one guard-failure constructor to fail with
// representative inputs and returns the message(s) it produced.
type failureFixture func(t *testing.T) []string

// conventionFixtures maps each exported guard-failure constructor to a
// fixture. The AST walk in TestRecoveryLineConvention fails when an
// exported constructor has neither a fixture nor a conventionExempt
// entry.
func conventionFixtures() map[string]failureFixture {
	return map[string]failureFixture{
		"FormatFailure": func(t *testing.T) []string {
			return []string{
				FormatFailure("blocked: working tree has user-authored changes", "git add -A && git commit"),
				FormatFailure("phase gate failed: stored=implement derived=plan",
					"mindspec complete <bead-id>",
					"mindspec repair phase 092-agent-contract-hardening"),
			}
		},
		"NewFailure": func(t *testing.T) []string {
			return []string{
				NewFailure("blocked: spec worktree has a merge in progress", "git -C /repo/.worktrees/worktree-spec-001-x merge --abort").Error(),
			}
		},
		"CheckCWD": func(t *testing.T) []string {
			stubGuard(t)
			readGuardStateFn = func(root string) (*guardState, error) {
				return &guardState{ActiveWorktree: "/repo/.worktrees/worktree-bead-abc"}, nil
			}
			getwdFn = func() (string, error) { return "/repo", nil }
			err := CheckCWD("/repo")
			if err == nil {
				t.Fatal("fixture failed to force a CheckCWD failure")
			}
			return []string{err.Error()}
		},
		"CheckCWDWithCache": func(t *testing.T) []string {
			stubGuard(t)
			readGuardStateFn = func(root string) (*guardState, error) {
				return &guardState{ActiveWorktree: "/repo/.worktrees/worktree-bead-abc"}, nil
			}
			getwdFn = func() (string, error) { return "/repo", nil }
			// nil cache routes through the stubbed readGuardStateFn.
			err := CheckCWDWithCache(nil, "/repo")
			if err == nil {
				t.Fatal("fixture failed to force a CheckCWDWithCache failure")
			}
			return []string{err.Error()}
		},
	}
}

// checkRecoveryConvention returns "" when msg satisfies the Req 12
// convention, otherwise a description of the violation. Factored out so
// the negative-fixture test below can prove the check actually catches
// offenders.
func checkRecoveryConvention(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return "empty failure message"
	}
	if !HasFinalRecoveryLine(msg) {
		return fmt.Sprintf("failure message lacks a final %q line: %q", RecoveryPrefix, msg)
	}
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(line, RecoveryPrefix) && IsBannedRecoveryCommand(strings.TrimPrefix(line, RecoveryPrefix)) {
			return fmt.Sprintf("recovery line emits a banned `bd update --metadata` command (Req 19): %q", line)
		}
	}
	return ""
}

// TestRecoveryLineConvention is the Req 21 convention test. It runs
// under -short.
func TestRecoveryLineConvention(t *testing.T) {
	fixtures := conventionFixtures()

	// 1. Coverage: every exported guard-failure constructor in this
	// package must have a fixture (or an explicit exemption).
	for _, name := range exportedFailureConstructorNames(t) {
		if _, exempt := conventionExempt[name]; exempt {
			continue
		}
		if _, ok := fixtures[name]; !ok {
			t.Errorf("exported guard-failure constructor %s has no fixture in conventionFixtures; every guard failure must end with a final %q line (spec 092 Req 21)", name, RecoveryPrefix)
		}
	}

	// 2. Convention: every produced failure message ends with a
	// recovery line and emits no banned command.
	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			for i, msg := range fixture(t) {
				if problem := checkRecoveryConvention(msg); problem != "" {
					t.Errorf("%s message %d violates the recovery-line convention: %s", name, i, problem)
				}
			}
		})
	}
}

// TestRecoveryLineConvention_FlagsMissingRecoveryLine is the negative
// fixture required by the bead's verification: a constructor that emits
// no `recovery:` line must be flagged by the convention check.
func TestRecoveryLineConvention_FlagsMissingRecoveryLine(t *testing.T) {
	t.Parallel()
	badConstructor := func() error {
		return errors.New("blocked: working tree is dirty; commit or stash your changes")
	}
	if problem := checkRecoveryConvention(badConstructor().Error()); problem == "" {
		t.Fatal("convention check failed to flag a constructor that emits no recovery line")
	}
	// A recovery line that is not FINAL is also a violation.
	if problem := checkRecoveryConvention("recovery: cd /repo\nblocked: tree dirty"); problem == "" {
		t.Fatal("convention check failed to flag a non-final recovery line")
	}
	// A final recovery line carrying the Req 19 banned command is a violation.
	if problem := checkRecoveryConvention("blocked\nrecovery: bd update x --metadata '{}'"); problem == "" {
		t.Fatal("convention check failed to flag a banned bd update --metadata recovery command")
	}
}

// exportedFailureConstructorNames parses this package's non-test files
// and returns every exported, receiver-less function that returns an
// error or carries "Failure" in its name — the walked set per Req 21.
func exportedFailureConstructorNames(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	var names []string
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}
			if returnsError(fn) || strings.Contains(fn.Name.Name, "Failure") {
				names = append(names, fn.Name.Name)
			}
		}
	}
	if len(names) == 0 {
		t.Fatal("AST walk found no guard-failure constructors; the convention test is miswired")
	}
	return names
}

func returnsError(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, field := range fn.Type.Results.List {
		if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "error" {
			return true
		}
	}
	return false
}
