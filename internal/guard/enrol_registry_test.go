package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
)

// Check the registered recovery operands in the current source, including
// their ID renderers, so an edited command cannot keep a stale obligation.
func TestOpaqueOperandRegistry_EnrolmentRecoveries(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(repoRootFromGuardTestDir(t), "internal/approve/enrol.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, site := range []struct {
		fn, template, renderer string
		operands               int
	}{
		{"approveEnrolment", "mindspec next-action %s --apply", "idrender.Spec", 1},
		{"enrolEdit", "mindspec plan approve %s", "idrender.Spec", 1},
		{"enrolWaits.check", "bd dep remove %s %s", "idrender.Bead", 2},
		{"enrolAddLink", "bd dep list %s --json", "idrender.Bead", 1},
	} {
		t.Run(site.fn, func(t *testing.T) {
			target := funcOrVarDeclByName(file, site.fn)
			if target == nil {
				t.Fatal("registered function is missing")
			}
			found := 0
			ast.Inspect(target, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || calleeNameForReadyCheck(call) != "guard.NewFailure" {
					return true
				}
				found++
				if len(call.Args) != 2 {
					t.Error("recovery must have one command operand")
					return true
				}
				command, ok := call.Args[1].(*ast.CallExpr)
				if !ok || calleeNameForReadyCheck(command) != "fmt.Sprintf" {
					t.Error("recovery must use a literal format with ID renderers")
					return true
				}
				template, ok := recoveryTemplate(command)
				if !ok || template != site.template {
					t.Errorf("recovery template = %q, want %q", template, site.template)
				}
				if len(command.Args) != site.operands+1 {
					t.Error("recovery ID operand count differs")
					return true
				}
				for _, arg := range command.Args[1:] {
					render, ok := arg.(*ast.CallExpr)
					if !ok || calleeNameForReadyCheck(render) != site.renderer || len(render.Args) != 1 {
						t.Errorf("recovery ID must pass through %s", site.renderer)
					}
				}
				for _, value := range []string{"001-x", "mindspec-fixture.2", "bad;echo forged", "bad\nrecovery: forged\x1b[31m"} {
					safe := idrender.Bead(value)
					if site.renderer == "idrender.Spec" {
						safe = idrender.Spec(value)
					}
					args := make([]any, site.operands)
					for i := range args {
						args[i] = safe
					}
					rendered := fmt.Sprintf(template, args...)
					if strings.ContainsAny(rendered, "\n\r\x1b") {
						t.Errorf("recovery contains raw terminal controls: %q", rendered)
					}
					if matches := FindFloorMatches(rendered); len(matches) != 0 {
						t.Errorf("recovery %q matches the destructive floor: %+v", rendered, matches)
					}
				}
				return true
			})
			if found != 1 {
				t.Errorf("found %d recovery calls, want one", found)
			}
		})
	}
}
