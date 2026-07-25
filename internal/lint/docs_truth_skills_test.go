// docs_truth_skills_test.go — R2 ground truth: which `/ms-*` and
// `/ms-panel`-style slash references are real. mindspec-ks4u (spec
// w0-docs-truth Bead 2).
//
// # Why three sources, not just plugins/mindspec/skills/
//
// The brief for this bead names a single ground truth:
// `plugins/mindspec/skills/<name>/SKILL.md`. Measuring the real repo
// shows that is necessary but not sufficient — running this lint
// against it produces false positives on true doc claims, which is a
// lint bug, not a finding:
//
//   - `/ms-spec-create`, `/ms-spec-approve`, `/ms-plan-approve`,
//     `/ms-impl-approve` (README.md, CLAUDE.md, all the agent guides)
//     are real and load-bearing, but internal/setup/claude.go's own doc
//     comment says they are "the canonical authority" for these 4 names
//     — shipped as Go raw-string literals from lifecycleSkillFiles(),
//     not as SKILL.md files under plugins/mindspec/skills/. That
//     directory holds only "the 8 plugin skills" per the same comment.
//   - `/ms-panel` (project-docs/user/guides/review-panels.md) is real
//     — plugins/mindspec/workflows/ms-panel.js — but it is a *workflow*
//     adapter, a distinct mechanism from a skill; it has no SKILL.md
//     and never will.
//
// So R2's real ground truth is the union of three structural sources,
// none of them a hand-maintained list in this lint:
//
//  1. plugins/mindspec/skills/<name>/SKILL.md on disk (the embedded
//     plugin skills — pluginmindspec.SkillFiles()'s own source).
//  2. plugins/mindspec/workflows/<name>.js on disk (workflow adapters).
//  3. The string keys of the map literal returned by
//     internal/setup/claude.go's lifecycleSkillFiles() — AST-extracted,
//     not copied by hand, so a 5th gate skill added there is picked up
//     automatically and nothing here needs to change.
//
// Notably absent from all three: `ms-explore`. It exists only as a
// hand-committed `.claude/skills/ms-explore/SKILL.md` /
// `.agents/skills/ms-explore/SKILL.md` pair with no install code path
// anywhere in internal/setup or plugins/mindspec — see the residual
// finding in the bead report. `/ms-explore` in
// project-docs/user/CLAUDE.md is therefore, per this lint's real
// ground truth, an unresolved reference like any other.
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

// skillUniverse is the set of resolvable `/ms-*`-style names, keyed by
// bare name (no leading slash), each mapped to a short provenance tag
// for diagnostics.
type skillUniverse map[string]string

// buildSkillUniverse computes the real ground truth described above,
// rooted at repoRoot.
func buildSkillUniverse(repoRoot string) (skillUniverse, error) {
	out := skillUniverse{}

	pluginSkillsDir := filepath.Join(repoRoot, "plugins", "mindspec", "skills")
	entries, err := os.ReadDir(pluginSkillsDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(pluginSkillsDir, e.Name(), "SKILL.md")); err == nil {
			out[e.Name()] = "plugins/mindspec/skills/" + e.Name() + "/SKILL.md"
		}
	}

	workflowsDir := filepath.Join(repoRoot, "plugins", "mindspec", "workflows")
	if wfEntries, err := os.ReadDir(workflowsDir); err == nil {
		for _, e := range wfEntries {
			if e.IsDir() {
				continue
			}
			name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			if name == "" {
				continue
			}
			out[name] = "plugins/mindspec/workflows/" + e.Name()
		}
	}

	claudeGoPath := filepath.Join(repoRoot, "internal", "setup", "claude.go")
	names, err := extractLifecycleSkillNames(claudeGoPath)
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		out[n] = "internal/setup/claude.go lifecycleSkillFiles()"
	}

	return out, nil
}

// extractLifecycleSkillNames AST-parses internal/setup/claude.go and
// returns the string keys of the map[string]string composite literal
// returned by the top-level func lifecycleSkillFiles — structurally,
// not by copying the 4 names into this lint.
func extractLifecycleSkillNames(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "lifecycleSkillFiles" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return true
			}
			cl, ok := ret.Results[0].(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if s, ok := stringLitValue(kv.Key); ok {
					names = append(names, s)
				}
			}
			return true
		})
	}
	return names, nil
}

// retiredPreRenameNames derives, from skills itself, the set of bare
// names that used to be valid slash commands before the "ms-" prefix
// convention (spec 105) was adopted — e.g. "ms-spec-approve" yields
// "spec-approve". O2-7: the codex/copilot comparison tables used
// exactly this pre-rename spelling (`/spec-approve`), which
// skillRefRe's `ms-` anchor lets escape R2 entirely, since it never
// even gets extracted as a candidate reference. This is a STRUCTURAL
// derivation, not a hand-maintained list of retired verbs: a future
// ms- rename (a 5th gate skill, say) makes its own old bare name
// checked automatically, with no change needed here.
func retiredPreRenameNames(skills skillUniverse) map[string]bool {
	out := map[string]bool{}
	for name := range skills {
		if bare, ok := strings.CutPrefix(name, "ms-"); ok && bare != "" {
			out[bare] = true
		}
	}
	return out
}

func realRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}
