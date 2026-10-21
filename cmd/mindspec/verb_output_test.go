package main

// verb_output_test.go — spec 128 (lifecycle-verb-state-honesty) bead 5.
//
// AC-3: `mindspec spec create <slug>` success output, in order: result
// lines, then guidance whose rendered template is the spec-mode
// template — idle.md's "Available Actions" block is absent.
//
// AC-14: a table-driven test covers every verb in R2.1's mutating-verb
// list, asserting each prints its result summary before any guidance
// output on success; the two tail-less verbs' rows assert the pinned
// result line itself. Anti-drift is mechanized (round-2 G10b): the test
// walks the REGISTERED cobra command tree and requires every runnable
// command to be classified — present in the mutating-verbs row set (or
// delegating BY FUNCTION-POINTER IDENTITY to one, for a hidden alias) or
// in an explicit rationale-carrying non-mutating allowlist — so
// registering any new command without classifying it is red, and
// classifying it as mutating without a row is red.
//
// Hermeticity: every fixture root lives under t.TempDir(); the table
// drives ALL NINE verbs via their real RunE/tail function with seams —
// bd/git stubs, executor.MockExecutor via the newExecutorFn seam, and (for
// `next`/`impl adopt`, round-1 panel G2's four-slot convergence) a
// PURELY CANNED fake `bd` executable on PATH (installFakeBD) — never a
// live lifecycle in this repo, never a real bd process. `next` and
// `impl adopt`'s non-phase-routed bd calls (internal/next's QueryReady/
// ClaimBead; internal/approve's adopt-specific finalize writes) expose no
// Go-level seam reachable from cmd/mindspec, but bd resolves via OS PATH
// lookup regardless of package boundaries, so installFakeBD sidesteps
// that wall entirely without a real bd process or a new exported seam in
// either owning package; their PHASE-routed calls (FindEpicBySpecID,
// DerivePhaseDetail, LifecycleChildIDsForEpic, GetChildren) stay
// Go-seamed via phase.SetListJSONForTest/SetRunBDForTest exactly like
// every other row.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/approve"
	"github.com/mrmaxsteel/mindspec/internal/complete"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/spf13/cobra"
)

// --- AC-3: spec create's tail rides bead 3's draft derivation ---------

// TestEmitInstruct_MainCheckoutSingleDraft_RendersSpecModeNotIdle pins
// AC-3 at the level that actually matters (emitInstruct is the shared
// tail EVERY verb calls, including spec create's own — spec.go itself
// needed no change): from the main checkout, right after `spec create`
// (a pre-epic draft — spec dir + `spec/<id>` branch exist, no epic yet),
// the tail renders the spec-mode template — idle.md's "Available
// Actions" heading is absent, and the spec-mode heading plus the real
// spec ID are present.
func TestEmitInstruct_MainCheckoutSingleDraft_RendersSpecModeNotIdle(t *testing.T) {
	specID := "901-ac3-demo"
	root := newDraftFixtureRoot(t, specID)
	stubNoEpics(t)
	withTestChdir(t, root)

	out, err := captureStdout(t, func() error { return emitInstruct(root) })
	if err != nil {
		t.Fatalf("emitInstruct: %v", err)
	}
	if strings.Contains(out, "Available Actions") {
		t.Errorf("idle.md's Available Actions block leaked into a draft-only spec-create tail:\n%s", out)
	}
	if !strings.Contains(out, "MindSpec — Spec Mode") {
		t.Errorf("expected the spec-mode template heading, got:\n%s", out)
	}
	if !strings.Contains(out, specID) {
		t.Errorf("expected the real spec ID %q in the rendered guidance, got:\n%s", specID, out)
	}
}

// --- AC-14: total cobra-tree classification ----------------------------

// mutatingVerbRows is R2.1's closed list of nine verbs that mutate
// durable lifecycle state, keyed by full cobra command path
// (cmd.CommandPath()).
var mutatingVerbRows = map[string]bool{
	"mindspec spec create":  true,
	"mindspec spec approve": true,
	"mindspec plan approve": true,
	"mindspec next":         true,
	"mindspec complete":     true,
	"mindspec impl approve": true,
	"mindspec impl adopt":   true,
	"mindspec release":      true,
	"mindspec repair phase": true,
}

// mutatingVerbAliases maps a HIDDEN backward-compat alias's full command
// path to the canonical row it delegates to. An alias is classified as
// mutating WITHOUT its own row only because it shares the canonical
// command's RunE BY FUNCTION-POINTER IDENTITY (verified below) — it can
// never diverge in behavior from the row that already covers it.
var mutatingVerbAliases = map[string]string{
	"mindspec approve spec": "mindspec spec approve",
	"mindspec approve plan": "mindspec plan approve",
	"mindspec approve impl": "mindspec impl approve",
	"mindspec spec-init":    "mindspec spec create",
}

// nonMutatingAllowlist is the explicit, rationale-carrying allowlist for
// every OTHER runnable command (RunE or Run set) in the registered
// cobra tree: read-only queries, non-lifecycle-phase mutations (ADR/
// domain/ownership/registry scaffolding, bead/worktree maintenance,
// panel/session/attestation bookkeeping), one-time project setup, and
// deprecated stubs. R2.1's membership test for the row set is durable
// LIFECYCLE-PHASE mutation (spec/plan/implement/review/idle, or the
// epic's mindspec_phase cache) — every entry here either mutates nothing
// or mutates something else entirely.
var nonMutatingAllowlist = map[string]string{
	"mindspec config resolve": "read-only: previews resolved panel slots and their config sources",
	"mindspec panel rebind":   "panel bookkeeping: refreshes the reviewed SHA and digest with an audit entry; no lifecycle phase mutation",
	"mindspec panel defer":    "records a panel deferral and disposition; no lifecycle phase mutation",
	"mindspec panel refute":   "records a panel refutation and disposition; not lifecycle-phase state",
	"mindspec panel abandon":  "records audited panel abandonment; not lifecycle-phase state",
	"mindspec":                "root command: prints help or an unknown-command error, never mutates",

	// --- cobra's own auto-registered built-ins (lazily added by
	// Execute()/InitDefaultHelpCmd/InitDefaultCompletionCmd — present in
	// the tree once any earlier test in this binary has executed a
	// command through cobra) ---
	"mindspec help":                  "cobra built-in: prints help, never mutates",
	"mindspec completion bash":       "cobra built-in: prints a shell completion script, never mutates",
	"mindspec completion fish":       "cobra built-in: prints a shell completion script, never mutates",
	"mindspec completion powershell": "cobra built-in: prints a shell completion script, never mutates",
	"mindspec completion zsh":        "cobra built-in: prints a shell completion script, never mutates",

	// --- read-only queries / reports ---
	"mindspec adr list":                "read-only: lists ADRs",
	"mindspec adr show":                "read-only: prints one ADR",
	"mindspec bead ready-check":        "read-only: evaluates readiness signals, no bd/git write on any path (own Long text)",
	"mindspec config show":             "read-only: prints resolved config",
	"mindspec context bead":            "read-only: prints bead context",
	"mindspec doctor":                  "read-only: health checks",
	"mindspec domain list":             "read-only: lists domains",
	"mindspec domain show":             "read-only: shows one domain",
	"mindspec instruct":                "read-only: prints mode-appropriate guidance (the tail itself)",
	"mindspec otel status":             "read-only: prints OTEL config status",
	"mindspec panel disposition query": "read-only: prints a panel disposition",
	"mindspec report":                  "read-only: consolidates the friction journal into a local report",
	"mindspec report list":             "read-only: lists reports",
	"mindspec spec list":               "read-only: lists specs",
	"mindspec state show":              "read-only: prints derived state (its own honesty is bead 3's territory)",
	"mindspec trace summary":           "read-only: summarizes a trace file",
	"mindspec validate docs":           "read-only: doc-sync validation",
	"mindspec validate plan":           "read-only: plan validation",
	"mindspec validate spec":           "read-only: spec validation",
	"mindspec version":                 "read-only: prints version",

	// --- registry / document scaffolding (mutates a project artifact,
	// never the lifecycle-phase state this contract covers) ---
	"mindspec adr create":         "creates an ADR document; does not transition lifecycle phase",
	"mindspec domain add":         "adds a domain declaration; does not transition lifecycle phase",
	"mindspec ownership populate": "generates OWNERSHIP.yaml; does not transition lifecycle phase",
	"mindspec source populate":    "generates a source registry; does not transition lifecycle phase",
	"mindspec models populate":    "generates a models registry; does not transition lifecycle phase",
	"mindspec commands populate":  "generates a commands registry; does not transition lifecycle phase",

	// --- bead/worktree maintenance (mutates individual beads/worktrees
	// as a recovery/maintenance action, never the epic's lifecycle
	// phase — that transition happens via the plan/next/complete verbs
	// already covered by rows) ---
	"mindspec bead clarify":          "records a readiness-clarification attempt on ONE bead's metadata; does not transition the epic's lifecycle phase",
	"mindspec bead create-from-plan": "manual bead-creation recovery tool; the row-covered `plan approve` is the normal path",
	"mindspec bead hygiene":          "audits/closes stale or oversized beads; does not transition the epic's lifecycle phase",
	"mindspec bead worktree":         "deprecated manual worktree show/create; `next`'s row covers the normal automatic path",
	"mindspec cleanup":               "post-lifecycle worktree/branch janitorial cleanup, after `impl approve` already transitioned to idle",
	"mindspec repair spec-title":     "repairs one epic's spec_title metadata key; never mindspec_phase",

	// --- deprecated no-op/stub commands (print guidance and return,
	// or exit non-zero; never mutate) ---
	"mindspec agentmind":        "hidden deprecated stub, exits non-zero, never mutates",
	"mindspec agentmind replay": "hidden deprecated stub, exits non-zero, never mutates",
	"mindspec agentmind serve":  "hidden deprecated stub, exits non-zero, never mutates",
	"mindspec agentmind setup":  "hidden deprecated stub, exits non-zero, never mutates",
	"mindspec approve":          "hidden deprecated bare dispatcher: always errors naming the canonical noun-verb commands, never mutates",
	"mindspec bead plan":        "deprecated: prints redirect guidance to stderr and returns nil, no mutation",
	"mindspec bead spec":        "deprecated: prints redirect guidance to stderr and returns nil, no mutation",
	"mindspec bench":            "hidden deprecated stub, exits non-zero, never mutates",
	"mindspec state set":        "deprecated no-op: prints a deprecation notice and returns nil",
	"mindspec viz":              "hidden deprecated stub, exits non-zero, never mutates",

	// --- one-time project setup/scaffolding (not a lifecycle-phase
	// transition) ---
	"mindspec init":           "one-time project scaffolding",
	"mindspec migrate":        "one-time data-layout migration tooling",
	"mindspec migrate layout": "one-time data-layout migration tooling",
	"mindspec otel setup":     "one-time OTEL config scaffolding",
	"mindspec setup claude":   "one-time agent config scaffolding",
	"mindspec setup codex":    "one-time agent config scaffolding",
	"mindspec setup copilot":  "one-time agent config scaffolding",

	// --- panel workflow / session / attestation bookkeeping (mutates
	// review-panel artifacts, session-freshness metadata, or a landed-
	// merge attestation marker — never the epic's mindspec_phase) ---
	"mindspec panel create":               "creates panel review bookkeeping artifacts; not lifecycle-phase state",
	"mindspec panel disposition append":   "records a panel disposition entry; not lifecycle-phase state",
	"mindspec panel disposition check":    "checks a panel disposition record; not lifecycle-phase state",
	"mindspec panel disposition validate": "validates panel disposition bookkeeping; not lifecycle-phase state",
	"mindspec panel tally":                "tallies panel verdicts; not lifecycle-phase state",
	"mindspec panel verify":               "verifies a panel round; not lifecycle-phase state",
	"mindspec reattest":                   "records a landed-merge re-attestation marker; not a lifecycle-phase transition",
	"mindspec record start":               "OTEL telemetry-recording config/launcher; writes a recording manifest, not lifecycle-phase state",
	"mindspec state write-session":        "writes session-freshness metadata (session.json); not lifecycle-phase state",

	// --- claude-code hook plumbing (not a user-invoked lifecycle verb) ---
	"mindspec hook": "SessionStart/other hook plumbing; prints instruct guidance, does not itself transition lifecycle phase",
}

// walkRunnableCommands returns the full command path of every command in
// the tree rooted at c that has RunE or Run set (a "leaf" that actually
// does something on invocation) — parent/group nodes with neither (e.g.
// `mindspec spec`, whose bare invocation just prints help) are excluded:
// AC-14's classification obligation is about commands that PRINT A
// RESULT, and a bare group node prints none.
func walkRunnableCommands(c *cobra.Command) []string {
	var out []string
	if c.RunE != nil || c.Run != nil {
		out = append(out, c.CommandPath())
	}
	for _, sub := range c.Commands() {
		out = append(out, walkRunnableCommands(sub)...)
	}
	return out
}

// TestVerbOutput_TotalCobraTreeClassification is AC-14's mechanized
// anti-drift gate (round-2 G10b): every RUNNABLE command registered on
// rootCmd is classified exactly once, either as one of R2.1's nine
// mutating verbs (or a function-pointer-verified alias of one) or in the
// explicit rationale-carrying nonMutatingAllowlist. Registering a new
// command without classifying it here goes RED; classifying a command as
// mutating without a corresponding row also goes RED (mutatingVerbRows'
// nine entries are asserted against R2.1's own list, not just counted).
func TestVerbOutput_TotalCobraTreeClassification(t *testing.T) {
	// cobra lazily adds `help` and `completion <shell>` the first time
	// ANY command executes through it — force both into the tree
	// deterministically so this test's outcome does not depend on
	// whether an earlier test in this binary already triggered them
	// (idempotent: both check for an existing registration first).
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	wantRows := []string{
		"mindspec spec create", "mindspec spec approve", "mindspec plan approve",
		"mindspec next", "mindspec complete", "mindspec impl approve",
		"mindspec impl adopt", "mindspec release", "mindspec repair phase",
	}
	if len(mutatingVerbRows) != len(wantRows) {
		t.Fatalf("mutatingVerbRows has %d entries, want exactly R2.1's %d", len(mutatingVerbRows), len(wantRows))
	}
	for _, w := range wantRows {
		if !mutatingVerbRows[w] {
			t.Errorf("mutatingVerbRows missing R2.1 verb %q", w)
		}
	}

	runnable := walkRunnableCommands(rootCmd)
	if len(runnable) == 0 {
		t.Fatal("no runnable commands found — the walker has regressed")
	}

	seen := map[string]bool{}
	var unclassified []string
	var mutatingNoRow []string
	for _, path := range runnable {
		seen[path] = true
		switch {
		case mutatingVerbRows[path]:
			// Classified: has its own row.
		case mutatingVerbAliases[path] != "":
			// Classified: an alias. Verify it shares the canonical
			// command's RunE by function-pointer identity — an alias
			// that ever diverges to its OWN implementation must be
			// caught here, not silently trusted.
			canonicalPath := mutatingVerbAliases[path]
			canonical := findCommandByPath(t, rootCmd, canonicalPath)
			aliasCmd := findCommandByPath(t, rootCmd, path)
			if canonical.RunE == nil || aliasCmd.RunE == nil {
				mutatingNoRow = append(mutatingNoRow, path+" (alias RunE or canonical RunE is nil)")
				continue
			}
			if reflect.ValueOf(canonical.RunE).Pointer() != reflect.ValueOf(aliasCmd.RunE).Pointer() {
				mutatingNoRow = append(mutatingNoRow, path+" (alias no longer shares RunE with "+canonicalPath+")")
			}
		case nonMutatingAllowlist[path] != "":
			// Classified: rationale-carrying allowlist entry.
		default:
			unclassified = append(unclassified, path)
		}
	}
	if len(unclassified) > 0 {
		sort.Strings(unclassified)
		t.Errorf("UNCLASSIFIED runnable command(s) (add to mutatingVerbRows, mutatingVerbAliases, or nonMutatingAllowlist): %v", unclassified)
	}
	if len(mutatingNoRow) > 0 {
		t.Errorf("alias classification problem(s): %v", mutatingNoRow)
	}

	// Reverse direction: every classified entry must still be present in
	// the tree (a removed command's stale classification is drift too).
	var stale []string
	for path := range mutatingVerbRows {
		if !seen[path] {
			stale = append(stale, "mutatingVerbRows: "+path)
		}
	}
	for path := range mutatingVerbAliases {
		if !seen[path] {
			stale = append(stale, "mutatingVerbAliases: "+path)
		}
	}
	for path := range nonMutatingAllowlist {
		if !seen[path] {
			stale = append(stale, "nonMutatingAllowlist: "+path)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("classified command(s) no longer present in the cobra tree (stale entries): %v", stale)
	}
}

// findCommandByPath locates the command at the exact CommandPath under
// root, failing the test if not found.
func findCommandByPath(t *testing.T, root *cobra.Command, path string) *cobra.Command {
	t.Helper()
	var found *cobra.Command
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if found != nil {
			return
		}
		if c.CommandPath() == path {
			found = c
			return
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	if found == nil {
		t.Fatalf("no command registered at path %q", path)
	}
	return found
}

// --- AC-14: per-verb result-before-guidance fixtures -------------------

// resultBeforeGuidance asserts that want (the pinned result substring)
// appears in out strictly before any of the guidance markers — the
// instruct-tail templates' own section headings. A tail-less verb
// passes a nil markers slice and this degrades to a plain substring
// presence check (AC-14's "the two tail-less verbs' rows assert the
// pinned result line itself").
func resultBeforeGuidance(t *testing.T, label, out, want string, guidanceMarkers ...string) {
	t.Helper()
	resultIdx := strings.Index(out, want)
	if resultIdx < 0 {
		t.Fatalf("%s: result summary %q not found in output:\n%s", label, want, out)
	}
	for _, marker := range guidanceMarkers {
		if gi := strings.Index(out, marker); gi >= 0 && gi < resultIdx {
			t.Errorf("%s: guidance marker %q (offset %d) appears BEFORE the result summary %q (offset %d):\n%s", label, marker, gi, want, resultIdx, out)
		}
	}
}

// mindspecGuidanceMarkers are section headings common to the instruct
// tail templates (idle/spec/plan/implement/review/ambiguous) — any of
// these appearing before a verb's result summary would mean guidance
// beat the result to stdout.
var mindspecGuidanceMarkers = []string{"## MindSpec Lifecycle", "## Next Action", "## Available Actions"}

// withMockNewExecutor swaps newExecutorFn for a canned MockExecutor for
// the duration of one test (spec 128 bead 5's AC-14 seam: drives spec
// create/approve, plan approve, next, and impl approve/adopt's REAL RunE
// functions without a live git/bd lifecycle).
func withMockNewExecutor(t *testing.T, mock *executor.MockExecutor) {
	t.Helper()
	orig := newExecutorFn
	newExecutorFn = func(string) executor.Executor { return mock }
	t.Cleanup(func() { newExecutorFn = orig })
}

// verbOutputTestRepo builds a minimal real git repo with the .mindspec/
// scaffolding ApproveSpec/ApprovePlan's validation requires (mirroring
// internal/harness/lifecycle_scenario_test.go's testRepo/testFixtureADR
// fixture, which already proves this exact shape works end-to-end for
// these two verbs).
func verbOutputTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGitInDir(t, root, "init", "-b", "main")
	runGitInDir(t, root, "config", "user.email", "test@mindspec.invalid")
	runGitInDir(t, root, "config", "user.name", "mindspec-test")

	for _, d := range []string{".mindspec", ".mindspec/specs", ".mindspec/adr"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	writeVerbFixtureFile(t, root, ".mindspec/config.yaml", `protected_branches: [main]
merge_strategy: direct
worktree_root: .worktrees
enforcement:
  pre_commit_hook: true
  cli_guards: true
  agent_hooks: true
`)
	writeVerbFixtureFile(t, root, ".mindspec/adr/ADR-9999.md", `# ADR-9999: Test Fixture ADR

- **Status**: Accepted
- **Domain(s)**: core
- **Supersedes**: n/a
- **Superseded-by**: n/a

## Decision

Test fixture used by cmd/mindspec's AC-14 verb-output table.
`)
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "initial commit")
	return root
}

func writeVerbFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// validVerbFixtureSpecMD returns a spec.md that passes
// validate.ValidateSpec — the exact shape (requirements/criteria counts
// included) internal/harness/lifecycle_scenario_test.go's validSpecMD
// proves works, reproduced here since that helper is unexported in
// package harness.
func validVerbFixtureSpecMD(specID string) string {
	return "---\nspec_id: " + specID + "\nstatus: Draft\nversion: 1\n---\n\n" +
		"# Spec: " + specID + " — Test Spec\n\n" +
		"## Goal\n\nTest specification for the AC-14 verb-output table.\n\n" +
		"## Impacted Domains\n\n- core\n\n" +
		"## ADR Touchpoints\n\nNone currently applicable.\n\n" +
		"## Requirements\n\n1. Requirement one\n2. Requirement two\n3. Requirement three\n\n" +
		"## Scope\n\n### In Scope\n\n- N/A\n\n### Out of Scope\n\n- N/A\n\n" +
		"## Acceptance Criteria\n\n- [ ] Criterion one\n- [ ] Criterion two\n- [ ] Criterion three\n\n" +
		"## Open Questions\n\nNone.\n\n" +
		"## Approval\n\nPending.\n"
}

// validVerbFixturePlanMD returns a plan.md that passes
// validate.ValidatePlan (mirroring validPlanMD).
func validVerbFixturePlanMD(specID string) string {
	return "---\nspec_id: " + specID + "\nstatus: Draft\nversion: 1\nlast_updated: \"2026-02-28\"\nbead_ids: []\nadr_citations:\n  - id: ADR-9999\n    sections: [\"Decision\"]\n---\n\n" +
		"# Plan: " + specID + " — Test Plan\n\n" +
		"## ADR Fitness\n\nCites ADR-9999 (test-fixture ADR covering the core domain).\n\n" +
		"## Testing Strategy\n\nN/A.\n\n" +
		"## Bead 1: Do the work\n\n**Steps**\n\n1. Do it\n\n**Verification**\n\n- [ ] N/A\n\n**Acceptance Criteria**\n\n- [ ] N/A\n\n**Depends on**\n\nNone\n"
}

// stubApproveBeadsForVerbTest is the same stateful epic-registry stub
// internal/harness's lifecycle scenario tests use
// (stubApproveBeadsWithEpicRegistry), reproduced here since that helper
// is unexported in package harness and unreachable from cmd/mindspec:
// ApproveSpec's `bd create --type=epic` call is intercepted and recorded
// in an in-memory registry, and phase's AllEpics query (fetchAllEpics'
// `--type=epic` list) is served from that SAME registry — so a spec this
// test approved earlier (spec approve) is genuinely findable by a later
// call (plan approve), exactly like a real bd epic would be.
func stubApproveBeadsForVerbTest(t *testing.T) {
	t.Helper()

	var mu sync.Mutex
	var epics []map[string]interface{}
	epicByID := map[string]map[string]interface{}{}

	bdFn := func(args ...string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		if len(args) > 0 && args[0] == "create" {
			isEpic := false
			var title, metadataRaw string
			for i, a := range args {
				if a == "--title" && i+1 < len(args) {
					title = args[i+1]
				}
				if a == "--metadata" && i+1 < len(args) {
					metadataRaw = args[i+1]
				}
				if a == "--type=epic" {
					isEpic = true
				}
			}
			if isEpic {
				id := fmt.Sprintf("fake-epic-%d", len(epics)+1)
				var metadata map[string]interface{}
				_ = json.Unmarshal([]byte(metadataRaw), &metadata)
				epic := map[string]interface{}{
					"id": id, "title": title, "status": "open",
					"issue_type": "epic", "metadata": metadata,
				}
				epics = append(epics, epic)
				epicByID[id] = epic
				out, _ := json.Marshal(map[string]string{"id": id})
				return out, nil
			}
		}
		return []byte(`{"id":"fake-test-bead"}`), nil
	}
	t.Cleanup(approve.SetSpecRunBDForTest(bdFn))
	t.Cleanup(approve.SetPlanRunBDForTest(bdFn))
	t.Cleanup(approve.SetPlanRunBDCombinedForTest(bdFn))
	t.Cleanup(approve.SetPlanListJSONForTest(func(args ...string) ([]byte, error) {
		return []byte(`[]`), nil
	}))
	t.Cleanup(phase.SetMergeMetadataForTest(func(string, map[string]interface{}) error { return nil }))
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		for _, a := range args {
			if a == "--type=epic" {
				out, err := json.Marshal(epics)
				if err != nil {
					return nil, err
				}
				return out, nil
			}
		}
		return []byte(`[]`), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		if len(args) >= 2 && args[0] == "show" {
			if epic, ok := epicByID[args[1]]; ok {
				out, err := json.Marshal([]map[string]interface{}{epic})
				if err != nil {
					return nil, err
				}
				return out, nil
			}
		}
		return []byte(`[]`), nil
	}))
}

// TestVerbOutput_SpecCreate drives specCreateCmd.RunE for real (a mocked
// executor stands in for the git-worktree-creating leg — see
// withMockNewExecutor) and asserts "Spec initialized" precedes any
// guidance marker.
func TestVerbOutput_SpecCreate(t *testing.T) {
	root := verbOutputTestRepo(t)
	withTestChdir(t, root)
	stubApproveBeadsForVerbTest(t)

	wsPath := filepath.Join(root, ".worktrees", "worktree-spec-999-verb-create")
	if err := os.MkdirAll(wsPath, 0o755); err != nil {
		t.Fatalf("mkdir workspace dir: %v", err)
	}
	mock := &executor.MockExecutor{
		InitSpecWorkspaceResult: executor.WorkspaceInfo{Path: wsPath, Branch: "spec/999-verb-create"},
	}
	withMockNewExecutor(t, mock)

	out, err := captureStdout(t, func() error {
		return specCreateCmd.RunE(specCreateCmd, []string{"999-verb-create"})
	})
	if err != nil {
		t.Fatalf("spec create: %v\noutput:\n%s", err, out)
	}
	resultBeforeGuidance(t, "spec create", out, "Spec initialized", mindspecGuidanceMarkers...)
}

// TestVerbOutput_SpecApprove drives specApproveCmd.RunE (approveSpecRunE)
// for real, using the same fixture shape internal/harness's
// TestScenario_HappyPath already proves works end-to-end for ApproveSpec.
func TestVerbOutput_SpecApprove(t *testing.T) {
	root := verbOutputTestRepo(t)
	withTestChdir(t, root)
	specID := "998-verb-approve"
	writeVerbFixtureFile(t, root, filepath.Join(".mindspec/specs", specID, "spec.md"), validVerbFixtureSpecMD(specID))
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "spec-init "+specID)

	stubApproveBeadsForVerbTest(t)
	withMockNewExecutor(t, &executor.MockExecutor{})

	out, err := captureStdout(t, func() error {
		return specApproveCmd.RunE(specApproveCmd, []string{specID})
	})
	if err != nil {
		t.Fatalf("spec approve: %v\noutput:\n%s", err, out)
	}
	resultBeforeGuidance(t, "spec approve", out, "approved", mindspecGuidanceMarkers...)
}

// TestVerbOutput_PlanApprove drives planApproveCmd.RunE (approvePlanRunE)
// for real, chaining off a successful ApproveSpec exactly like
// internal/harness's TestScenario_HappyPath.
func TestVerbOutput_PlanApprove(t *testing.T) {
	root := verbOutputTestRepo(t)
	withTestChdir(t, root)
	specID := "997-verb-plan"
	writeVerbFixtureFile(t, root, filepath.Join(".mindspec/specs", specID, "spec.md"), validVerbFixtureSpecMD(specID))
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "spec-init "+specID)

	stubApproveBeadsForVerbTest(t)
	withMockNewExecutor(t, &executor.MockExecutor{})

	if _, err := approve.ApproveSpec(root, specID, "test-user", &executor.MockExecutor{}); err != nil {
		t.Fatalf("fixture ApproveSpec: %v", err)
	}
	writeVerbFixtureFile(t, root, filepath.Join(".mindspec/specs", specID, "plan.md"), validVerbFixturePlanMD(specID))
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-m", "add plan")

	out, err := captureStdout(t, func() error {
		return planApproveCmd.RunE(planApproveCmd, []string{specID})
	})
	if err != nil {
		t.Fatalf("plan approve: %v\noutput:\n%s", err, out)
	}
	resultBeforeGuidance(t, "plan approve", out, "approved", mindspecGuidanceMarkers...)
}

// TestVerbOutput_Complete drives the REAL production completeTail with a
// hand-built success Result (the established cwdsafety_test.go pattern)
// and asserts FormatResult's summary precedes the instruct tail's own
// guidance markers.
func TestVerbOutput_Complete(t *testing.T) {
	root := t.TempDir()
	invocationCwd := root

	var stdout, stderr bytes.Buffer
	completeTail(&stdout, &stderr, root, invocationCwd,
		&complete.Result{
			BeadID:   "bead-ac14",
			NextMode: state.ModeImplement,
			NextBead: "bead-ac14-next",
			NextSpec: "008-ac14",
		},
		func(r string) error {
			stdout.WriteString("## MindSpec Lifecycle\n(stub guidance)\n")
			return nil
		})

	out := stdout.String()
	resultBeforeGuidance(t, "complete", out, "Bead bead-ac14 closed.", "## MindSpec Lifecycle")
}

// TestVerbOutput_ImplApprove drives the REAL production implApproveTail
// with a hand-built success Result and asserts the summary precedes the
// instruct tail's own guidance markers.
func TestVerbOutput_ImplApprove(t *testing.T) {
	root := t.TempDir()
	// implApproveTail unconditionally os.Chdir(root)s as its first act
	// (mirroring the real approveImplRunE); restore the process cwd so
	// this test doesn't strand every OTHER test in this binary inside a
	// t.TempDir() that gets removed at this test's teardown.
	withTestChdir(t, root)
	var stdout, stderr bytes.Buffer
	err := implApproveTail(&stdout, &stderr, root, root, "008-ac14", nil,
		&approve.ImplResult{SpecID: "008-ac14"}, nil,
		func(r string) error {
			stdout.WriteString("## MindSpec Lifecycle\n(stub guidance)\n")
			return nil
		})
	if err != nil {
		t.Fatalf("implApproveTail: %v", err)
	}
	out := stdout.String()
	resultBeforeGuidance(t, "impl approve", out, "approved. Mode: idle.", "## MindSpec Lifecycle")
}

// TestVerbOutput_Release drives the REAL production runRelease with the
// established releaseRecorder deps and asserts the pinned result line
// (release is tail-less, R2.1 — AC-14's "assert the pinned result line
// itself" branch).
func TestVerbOutput_Release(t *testing.T) {
	r := &releaseRecorder{active: "mindspec-ac14", epicID: "mindspec-epic-ac14", haveCursor: true}
	deps := r.deps()
	if err := runRelease(deps, "mindspec-ac14", false); err != nil {
		t.Fatalf("runRelease: %v", err)
	}
	out := deps.stdout.(*bytes.Buffer).String()
	resultBeforeGuidance(t, "release", out, "Released bead")
}

// TestVerbOutput_RepairPhase drives the REAL production repairPhaseRunE
// with the established stubRepairPhaseEnv and asserts the pinned result
// line (repair phase is tail-less, R2.1).
func TestVerbOutput_RepairPhase(t *testing.T) {
	stubRepairPhaseEnv(t, state.ModeReview, func(id string, updates map[string]interface{}) error { return nil })

	out, err := captureStdout(t, func() error {
		return repairPhaseRunE(repairPhaseCmd, []string{"092-agent-contract-hardening"})
	})
	if err != nil {
		t.Fatalf("repair phase: %v\noutput:\n%s", err, out)
	}
	resultBeforeGuidance(t, "repair phase", out, "already consistent")
}

// installFakeBD drops a fake, PURELY CANNED (never forwarding to a real
// bd binary) `bd` executable into a temp dir and prepends it to the live
// process PATH, restored via t.Cleanup (round-1 panel G2: the
// next_ready_gate_test.go PATH-shim MECHANISM, generalized to a
// self-contained fake rather than that file's forward-to-real-bd
// variant — internal/next and internal/approve's adopt-specific bd calls
// expose no Go-level seam reachable from cmd/mindspec, but bd resolves
// via OS PATH lookup regardless of package boundaries, so this sidesteps
// the wall entirely without a real bd process or a new exported seam in
// either owning package). script receives argv (bd's own subcommand and
// flags) as "$@" and must handle every bd invocation the driven verb's
// non-phase-routed call sites make; phase-routed calls
// (phase.FindEpicBySpecID, DerivePhaseDetail, LifecycleChildIDsForEpic,
// GetChildren, etc.) stay Go-seamed via phase.SetListJSONForTest/
// SetRunBDForTest and never reach this fake process at all.
func installFakeBD(t *testing.T, script string) {
	t.Helper()
	fakeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake bd: %v", err)
	}
	origPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", fakeDir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
}

// TestVerbOutput_Next drives the REAL nextCmd.RunE claim path (round-1
// panel G2/G4/R4/R5/R6 four-slot convergence rejected the documented
// scope-limitation this replaces): a fake bd on PATH answers `ready`
// (one claimable task-typed bead) and `update ... --claim` (success);
// phase.SetListJSONForTest/SetRunBDForTest report zero active epics so
// the ambient spec-resolution step degrades harmlessly to the global
// (unscoped) ready query; a MockExecutor stands in for the
// worktree-creating leg (EnsureWorktree -> exec.DispatchBead). Asserts
// the claim/state result lines precede any guidance marker.
func TestVerbOutput_Next(t *testing.T) {
	root := t.TempDir()
	runGitInDir(t, root, "init", "-q", "-b", "main")
	runGitInDir(t, root, "config", "user.email", "test@mindspec.invalid")
	runGitInDir(t, root, "config", "user.name", "mindspec-test")
	runGitInDir(t, root, "commit", "--allow-empty", "-q", "-m", "baseline")
	withTestChdir(t, root)

	const epicID = "epic-ac14next"
	const beadID = epicID + ".1"
	const specID = "900-ac14-next"

	// The readiness gate's EvaluateReadiness reads real spec.md/plan.md
	// files off disk (never seamed) — minimal but present content, with
	// --allow-not-ready tolerating any failing signal so this fixture
	// doesn't also need to satisfy the full MF-1..MF-4 floor.
	specDir := filepath.Join(root, ".mindspec", "specs", specID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# Spec\n\n## Requirements\n\n1. R1\n"), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte("---\nstatus: Approved\nwork_chunks:\n  - id: 1\n---\n\n# Plan\n\n## Bead 1: Fixture\n"), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}
	runGitInDir(t, root, "add", "-A")
	runGitInDir(t, root, "commit", "-q", "-m", "add spec/plan fixture")
	if err := nextCmd.Flags().Set("allow-not-ready", "MF-1,MF-2,MF-3,MF-4"); err != nil {
		t.Fatalf("set --allow-not-ready: %v", err)
	}
	if err := nextCmd.Flags().Set("evidence", nextEvidenceFile(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = nextCmd.Flags().Set("allow-not-ready", "")
		_ = nextCmd.Flags().Set("evidence", "")
		nextCmd.Flags().Lookup("allow-not-ready").Changed = false
	})
	runGitInDir(t, root, "branch", "spec/"+specID)

	installFakeBD(t, "#!/bin/sh\n"+
		"case \"$1\" in\n"+
		"  ready) echo '[{\"id\":\""+beadID+"\",\"title\":\"[IMPL "+specID+".1] AC-14 fixture bead\",\"status\":\"open\",\"priority\":1,\"issue_type\":\"task\"}]' ;;\n"+
		"  update) exit 0 ;;\n"+
		// readiness.fetchBeadRecordReal (a raw bead.RunBD call, NOT
		// phase-seamed) needs a well-formed bd-show shape — no
		// dependencies, so MF-3 trivially passes regardless.
		"  show) echo '[{\"description\":\"AC-14 fixture bead\",\"dependencies\":[]}]' ;;\n"+
		"  *) echo '[]' ;;\n"+
		"esac\n")

	epics := []phase.EpicInfo{{
		ID: epicID, Title: "[SPEC " + specID + "] AC-14 next fixture", Status: "open",
		IssueType: "epic", Metadata: map[string]interface{}{"spec_num": float64(900), "spec_title": "ac14-next"},
	}}
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" && args[1] == beadID {
			// phase.FindEpicForBead's expected shape: a dependency edge
			// to the epic, resolving GateReadiness's owning-spec lookup.
			return []byte(`[{"title":"fixture","dependencies":[{"id":"` + epicID + `","issue_type":"epic"}]}]`), nil
		}
		if len(args) >= 2 && args[0] == "show" {
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	}))

	wtDir := filepath.Join(root, ".worktrees", "worktree-"+beadID)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree dir: %v", err)
	}
	withMockNewExecutor(t, &executor.MockExecutor{
		DispatchBeadResult: executor.WorkspaceInfo{Path: wtDir},
	})

	out, err := captureStdout(t, func() error {
		return nextCmd.RunE(nextCmd, nil)
	})
	if err != nil {
		t.Fatalf("next: %v\noutput:\n%s", err, out)
	}
	// Round-2 panel G2: next's success-path guidance is the bead-context
	// prompt (contextpack.RenderBeadContext), NOT the instruct tail —
	// none of mindspecGuidanceMarkers occurs in it, so passing only
	// those markers made resultBeforeGuidance degrade to a vacuous
	// presence check (no marker found at all -> the loop never fires ->
	// the assertion trivially "passes" regardless of true ordering).
	// The bead-context headings are the REAL markers this fixture's
	// captured output actually contains (verified against the real
	// wiring); mindspecGuidanceMarkers ride along too so the assertion
	// stays meaningful even if a future fixture change makes
	// RenderBeadContext fail and next.go fall back to emitInstruct.
	nextGuidanceMarkers := append([]string{"# Bead Context:", "## Work Chunk", "**When done**"}, mindspecGuidanceMarkers...)
	resultBeforeGuidance(t, "next", out, "Claiming", nextGuidanceMarkers...)
	if !strings.Contains(out, "State updated: mode=implement") {
		t.Errorf("expected the claimed task-typed bead to resolve mode=implement, got:\n%s", out)
	}
}

// TestVerbOutput_ImplAdopt drives the REAL adoptSpecRunE (via a real
// AdoptSpec call) over the attested-evidence path: phase.FindEpicBySpecID/
// DerivePhaseDetail/LifecycleChildIDsForEpic are Go-seamed (a review-phase
// epic with zero lifecycle children); no local spec/<id> branch exists
// (adopt's own precondition — the branch has already been superseded);
// no remote is configured and no surviving bead branches exist, so the
// evidence lattice lands on an unverified trigger, and --attest-unverified
// proceeds through the finalize path. A fake bd on PATH answers the
// remaining non-phase-routed calls (show/list/update/close/export)
// adoptFinalize's real writes make; a MockExecutor stands in for
// exec.CommitPaths.
func TestVerbOutput_ImplAdopt(t *testing.T) {
	root := t.TempDir()
	runGitInDir(t, root, "init", "-q", "-b", "main")
	runGitInDir(t, root, "config", "user.email", "test@mindspec.invalid")
	runGitInDir(t, root, "config", "user.name", "mindspec-test")
	if err := os.MkdirAll(filepath.Join(root, ".mindspec"), 0o755); err != nil {
		t.Fatalf("mkdir .mindspec: %v", err)
	}
	runGitInDir(t, root, "commit", "--allow-empty", "-q", "-m", "baseline")
	withTestChdir(t, root)

	const specID = "901-ac14-adopt"
	const epicID = "epic-ac14-adopt"

	installFakeBD(t, "#!/bin/sh\n"+
		"case \"$1\" in\n"+
		"  show) id=\"$2\"; echo \"[{\\\"id\\\":\\\"$id\\\",\\\"status\\\":\\\"open\\\",\\\"issue_type\\\":\\\"epic\\\",\\\"metadata\\\":{}}]\" ;;\n"+
		"  list) echo '[]' ;;\n"+
		"  update|close|export) exit 0 ;;\n"+
		"  *) echo '[]' ;;\n"+
		"esac\n")

	epics := []phase.EpicInfo{{
		ID: epicID, Title: "[SPEC " + specID + "] AC-14 adopt fixture", Status: "open",
		IssueType: "epic", Metadata: map[string]interface{}{
			"spec_num": float64(901), "spec_title": "ac14-adopt",
			"mindspec_phase": "review", "mindspec_phase_semantics": "v2",
		},
	}}
	t.Cleanup(phase.SetListJSONForTest(func(args ...string) ([]byte, error) {
		for _, a := range args {
			if a == "--type=epic" {
				return json.Marshal(epics)
			}
			if strings.HasPrefix(a, "--parent") {
				return []byte("[]"), nil // zero lifecycle children
			}
		}
		return []byte("[]"), nil
	}))
	t.Cleanup(phase.SetRunBDForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "show" {
			return json.Marshal(epics)
		}
		return []byte("[]"), nil
	}))

	withMockNewExecutor(t, &executor.MockExecutor{})

	if err := implAdoptCmd.Flags().Set("reason", "AC-14 fixture: content already reached main outside the lifecycle"); err != nil {
		t.Fatalf("set --reason: %v", err)
	}
	if err := implAdoptCmd.Flags().Set("attest-unverified", "true"); err != nil {
		t.Fatalf("set --attest-unverified: %v", err)
	}
	t.Cleanup(func() {
		_ = implAdoptCmd.Flags().Set("reason", "")
		_ = implAdoptCmd.Flags().Set("attest-unverified", "false")
	})

	out, err := captureStdout(t, func() error {
		return adoptSpecRunE(implAdoptCmd, []string{specID})
	})
	if err != nil {
		t.Fatalf("impl adopt: %v\noutput:\n%s", err, out)
	}
	resultBeforeGuidance(t, "impl adopt", out, "adopted via attestation", mindspecGuidanceMarkers...)
}
