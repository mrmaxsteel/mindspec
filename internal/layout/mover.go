// Package layout implements the deterministic, transactional, idempotent
// `migrate layout` mover that flattens the canonical
// .mindspec/docs/{specs,adr,domains,core} + context-map.md tree into the flat
// .mindspec/{specs,adr,domains,core} + context-map.md layout (spec 106, Reqs
// 4/5/11/14).
//
// The mover NEVER mutates the live tree until invoked; all behavior is exercised
// in tests over a captured copy / temp fixture. It stands entirely on the
// net-new git primitives surfaced on the executor boundary (ADR-0030): it does
// not shell out to git itself, and it carries its OWN thin pre/post-flatten
// ref-shape probe (signatureAtRef) so it has no dependency on the Bead-1
// workspace resolvers or layout-signature helper.
//
// Move model: each artifact group lands as TWO commits — a pure 100%-similarity
// `git mv` first, then the finite-pattern link-rewrite second — so
// `git log --follow` and 3-way-merge rename detection stay reliable. Durable
// checkpoints at every boundary make a crashed run resumable; a pre-publish
// failure hard-resets to the pre-run ref; after publish, auto-rollback is
// REFUSED (ADR-0023 forward-only). The run finalizes only after the gating
// doctor link-existence lane reports zero 404s.
package layout

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
)

// GitOps is the subset of executor.Executor the mover drives. The production
// MindspecExecutor satisfies it; tests can use either the real executor against
// a temp git repo (the golden path) or a mock (the precondition scan).
type GitOps interface {
	RevParseRef(workdir, ref string) (string, error)
	Status(workdir string) (string, error)
	GitMv(workdir, src, dst string) error
	ResetHard(workdir, ref string) error
	CleanForce(workdir string) error
	CleanForcePaths(workdir string, paths []string) error
	CommitPaths(workdir, msg string, paths []string) error
	LocalBranchRefs(workdir string) ([]string, error)
	RemoteTrackingRefs(workdir string) ([]string, error)
	MergeBase(a, b string) (string, error)
	TreeDirsAtRef(ref, dirPath string) ([]string, error)
}

// MoveGroup is one artifact-group rename (Src → Dst), repo-relative slash paths.
type MoveGroup struct {
	Src string
	Dst string
}

// DefaultFlattenPlan is the canonical spec-106 move plan: the four lifecycle
// directories plus the context-map file flatten OUT of the `.mindspec/docs/`
// wrapper into `.mindspec/` (the symmetric flatten), and the three dogfood
// directories EVICT out of `.mindspec/` to top-level `project-docs/` (the
// asymmetric depth-change eviction — explicitly NOT `docs/`, which would alias
// the legacy `root/docs` resolver tier). Order is deterministic. The
// content-aware review-co-location moves are NOT static (they depend on each
// review's resolved owning spec) and are appended at run time by the mover.
//
// A group whose source is absent on a given tree (e.g. a project with no
// dogfood docs) is a no-op skip, so this single plan drives every spec-106
// flatten out of the box.
func DefaultFlattenPlan() []MoveGroup {
	return []MoveGroup{
		{Src: ".mindspec/docs/specs", Dst: ".mindspec/specs"},
		{Src: ".mindspec/docs/adr", Dst: ".mindspec/adr"},
		{Src: ".mindspec/docs/domains", Dst: ".mindspec/domains"},
		{Src: ".mindspec/docs/core", Dst: ".mindspec/core"},
		{Src: ".mindspec/docs/context-map.md", Dst: ".mindspec/context-map.md"},
		{Src: ".mindspec/docs/user", Dst: "project-docs/user"},
		{Src: ".mindspec/docs/installation", Dst: "project-docs/installation"},
		{Src: ".mindspec/docs/research", Dst: "project-docs/research"},
	}
}

// DefaultRootDocs are the repo-root docs the rewriter touches (they reference
// INTO the moved trees).
var DefaultRootDocs = []string{"README.md", "AGENTS.md"}

// errSimulatedCrash is the test-only sentinel a crash injection returns: Run
// treats it as a process-death simulation and does NOT roll back, leaving the
// partial on-disk state a real crash would leave, so a fresh re-run resumes.
var errSimulatedCrash = errors.New("layout: simulated crash")

// LinkCheckError is returned when the gating link-existence lane finds any 404
// after the moves+rewrites. It carries the dangling links for the caller.
type LinkCheckError struct {
	Dangling []doctor.DanglingLink
}

func (e *LinkCheckError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "migrate layout: link-check failed — %d dangling link(s) after migration:", len(e.Dangling))
	for _, d := range e.Dangling {
		fmt.Fprintf(&b, "\n  %s → %s (404, in %s)", d.Target, d.Resolved, d.File)
	}
	return b.String()
}

// Mover is the transactional flatten driver.
type Mover struct {
	git      GitOps
	root     string
	runID    string
	plan     []MoveGroup
	rules    []RewriteRule
	rootDocs []string

	// skippedReviews holds the repo-root review/<slug> dirs the
	// review-co-location step could not attribute to a spec; recorded in the
	// lineage manifest rather than failing the run.
	skippedReviews []string

	// linkCheck is the gating link-existence lane; defaults to
	// doctor.CheckMovedTreeLinks. Injectable for tests.
	linkCheck func(root string) ([]doctor.DanglingLink, error)

	state State

	// Test seams. crashAt/failAt fire at the named checkpoint of the given
	// group (group -1 = the global finalize/root-rewrite boundaries):
	// crashAt simulates process death (no rollback); failAt simulates an
	// operational failure (triggers pre-publish rollback).
	crashAt      stage
	crashAtGroup int
	failAt       stage
	failAtGroup  int
}

// NewMover constructs a Mover with the default flatten plan, rules, root docs,
// and the production doctor link-check lane.
func NewMover(git GitOps, root, runID string) *Mover {
	return &Mover{
		git:          git,
		root:         root,
		runID:        runID,
		plan:         DefaultFlattenPlan(),
		rules:        DefaultFlattenRules(),
		rootDocs:     DefaultRootDocs,
		linkCheck:    doctor.CheckMovedTreeLinks,
		crashAtGroup: -2,
		failAtGroup:  -2,
	}
}

// Run drives (or resumes) the migration to completion. It is idempotent:
// re-running on an already-flat tree is a no-op. A pre-publish operational
// failure hard-resets to the pre-run ref; a simulated crash leaves partial
// state for a later resume.
func (m *Mover) Run() error {
	if err := m.ensureInit(); err != nil {
		return err
	}
	if err := m.run(); err != nil {
		if errors.Is(err, errSimulatedCrash) {
			return err // crash: no rollback, partial state preserved for resume
		}
		// Pre-publish failure → hard-reset rollback to the pre-run ref
		// (nothing published; ADR-0023 forward-only blocks post-publish).
		if rbErr := m.rollback(); rbErr != nil {
			return fmt.Errorf("%w; additionally rollback failed: %v", err, rbErr)
		}
		return err
	}
	return nil
}

// ensureInit loads an existing run-state (resume) or records the pre-run ref
// and a fresh state (first run).
func (m *Mover) ensureInit() error {
	s, found, err := loadState(m.root, m.runID)
	if err != nil {
		return err
	}
	if found {
		m.state = s
		return nil
	}
	head, err := m.git.RevParseRef(m.root, "HEAD")
	if err != nil {
		return fmt.Errorf("recording pre-run ref: %w", err)
	}
	m.state = State{RunID: m.runID, Stage: string(stagePreRun), PreRunRef: head}
	return m.writeState()
}

func (m *Mover) run() error {
	// Resolve the FULL move plan once — the static flatten/dogfood groups PLUS
	// the content-aware review-co-location groups — and freeze it so a
	// crash-resume reuses the same group→checkpoint index space (the review
	// sources are gone after they move, so re-deriving them on resume would
	// shift indices). resolvePlan also appends the per-review depth-change
	// rewrite rules to m.rules.
	if err := m.resolvePlan(); err != nil {
		return err
	}

	for i, g := range m.plan {
		if err := m.processGroup(i, g); err != nil {
			return err
		}
	}

	// Affected repo-root docs (README/AGENTS) — rewritten once at the end and
	// committed as a single root-doc commit.
	if err := m.checkpoint(stageRootRewrite, -1); err != nil {
		return err
	}
	if _, err := applyRewritesToFiles(m.rootDocAbsPaths(), m.rules); err != nil {
		return err
	}
	dirty, err := m.nonOperationalDirtyPaths()
	if err != nil {
		return err
	}
	if len(dirty) > 0 {
		if err := m.git.CommitPaths(m.root, "migrate(layout): rewrite root-doc links", dirty); err != nil {
			return err
		}
	}
	m.removeEmptyDocsDir()
	m.removeEmptyReviewDir()

	// Gating link-existence lane: scan EVERY link and FAIL on any 404.
	if err := m.checkpoint(stageFinalize, -1); err != nil {
		return err
	}
	dangling, err := m.linkCheck(m.root)
	if err != nil {
		return fmt.Errorf("link-check: %w", err)
	}
	if len(dangling) > 0 {
		m.state.Stage = string(stageLinkCheckFailed)
		_ = m.writeState()
		return &LinkCheckError{Dangling: dangling}
	}

	if err := m.writeLineage(); err != nil {
		return err
	}
	m.state.Stage = string(stageApplied)
	m.state.Group = -1
	m.state.GroupStage = ""
	return m.writeState()
}

// resolvePlan freezes the full move plan and the active rule set. On the FIRST
// resolve it computes the content-aware review-co-location groups, appends them
// to the static plan, appends their per-group depth-change rewrite rules to the
// base rules, records the un-routable review dirs, and persists the frozen plan
// to the run state. On a RESUME (PlanResolved already set) it reuses the frozen
// plan verbatim and regenerates the review rules deterministically from the
// review groups in it — so the plan is identical to the first run even though
// the review/<slug> sources have already moved.
func (m *Mover) resolvePlan() error {
	if m.state.PlanResolved {
		m.plan = m.state.Plan
		m.skippedReviews = m.state.SkippedReviews
		m.rules = append(m.rules, reviewRules(m.plan)...)
		return nil
	}

	rgs, skipped, err := m.reviewGroups()
	if err != nil {
		return err
	}
	m.plan = append(append([]MoveGroup(nil), m.plan...), rgs...)
	m.rules = append(m.rules, reviewRules(rgs)...)
	m.skippedReviews = skipped

	m.state.PlanResolved = true
	m.state.Plan = m.plan
	m.state.SkippedReviews = skipped
	return m.writeState()
}

// isReviewGroup reports whether a move group is a review-co-location move (its
// source is a repo-root review/<slug> directory).
func isReviewGroup(g MoveGroup) bool {
	return strings.HasPrefix(filepath.ToSlash(g.Src), "review/")
}

// reviewRules generates the finite-pattern depth-change rewrite rule for each
// review-co-location group: an absolute repo-root `review/<slug>/` reference
// rewrites to its co-located `<spec-dir>/reviews/<slug>/` destination. Sibling
// links WITHIN a moved review dir are unchanged (the subtree's internal depth
// is preserved), so only the absolute root-anchored form needs a rule.
func reviewRules(groups []MoveGroup) []RewriteRule {
	var rules []RewriteRule
	for _, g := range groups {
		if isReviewGroup(g) {
			rules = append(rules, RewriteRule{
				Old: filepath.ToSlash(g.Src) + "/",
				New: filepath.ToSlash(g.Dst) + "/",
			})
		}
	}
	return rules
}

// reviewGroups discovers the review-co-location moves: each repo-root
// review/<slug>/ directory is routed to its owning spec's flat
// <spec-dir>/reviews/<slug>/ home. The owning spec is keyed by the review's
// panel.json `spec` field when present and resolvable, else inferred from the
// slug's leading numeric prefix (e.g. 099-final-panel → spec 099-…). A dir that
// cannot be attributed to a spec — and any loose non-directory entry — is
// SKIPPED and recorded (returned in skipped) rather than failing the run.
func (m *Mover) reviewGroups() (groups []MoveGroup, skipped []string, err error) {
	reviewRoot := filepath.Join(m.root, "review")
	entries, err := os.ReadDir(reviewRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil // no root review/ tree → nothing to co-locate
		}
		return nil, nil, err
	}
	specIDs := m.listSpecIDs()
	// Deterministic order so the frozen plan + checkpoint indices are stable.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, ent := range entries {
		slug := ent.Name()
		if !ent.IsDir() {
			skipped = append(skipped, "review/"+slug) // loose file → not routable
			continue
		}
		specID := m.routeReviewSlug(slug, specIDs)
		if specID == "" {
			skipped = append(skipped, "review/"+slug)
			continue
		}
		groups = append(groups, MoveGroup{
			Src: "review/" + slug,
			Dst: ".mindspec/specs/" + specID + "/reviews/" + slug,
		})
	}
	return groups, skipped, nil
}

// routeReviewSlug resolves the owning spec id for a review slug: first the
// review's panel.json `spec` field (when it names an existing spec), then the
// slug's leading numeric prefix matched against the present spec ids. Returns
// "" when neither attributes the slug to a spec.
func (m *Mover) routeReviewSlug(slug string, specIDs []string) string {
	if specID := m.panelSpec(slug); specID != "" && m.specExists(specID) {
		return specID
	}
	if prefix := numericPrefix(slug); prefix != "" {
		for _, id := range specIDs {
			if strings.HasPrefix(id, prefix+"-") {
				return id
			}
		}
	}
	return ""
}

// panelSpec reads review/<slug>/panel.json and returns its `spec` field, or ""
// when the file is absent/unreadable/unparseable (then the slug-prefix fallback
// applies).
//
// Reverse-derivation consumer gate (ADR-0042 §1 reverse, spec 120 round-5
// O3, AC-23 TestPanelSpecRejectsTraversal): the `spec` field is a string
// read back OUT of an agent-writable panel.json — idvalidate.SpecID is
// applied HERE, BEFORE the caller's specExists check (routeReviewSlug,
// below), because specExists is a bare os.Stat that a traversal value like
// "../.." would PASS (it stats a real directory) — the enforcement-was-
// missing proof. An invalid/hostile value returns "" exactly like an
// absent/unparseable file, so the slug-prefix fallback applies and no
// hostile bytes ever reach the MoveGroup.Dst concat below.
func (m *Mover) panelSpec(slug string) string {
	data, err := os.ReadFile(filepath.Join(m.root, "review", slug, "panel.json"))
	if err != nil {
		return ""
	}
	var p struct {
		Spec string `json:"spec"`
	}
	if json.Unmarshal(data, &p) != nil {
		return ""
	}
	spec := strings.TrimSpace(p.Spec)
	if idvalidate.SpecID(spec) != nil {
		return ""
	}
	return spec
}

// specExists reports whether spec id has a directory under any active layout
// tier (flat first, then canonical, then legacy).
func (m *Mover) specExists(specID string) bool {
	for _, root := range []string{".mindspec/specs", ".mindspec/docs/specs", "docs/specs"} {
		if exists(filepath.Join(m.root, filepath.FromSlash(root), specID)) {
			return true
		}
	}
	return false
}

// listSpecIDs returns the spec-id directory names under the active specs root
// (flat → canonical → legacy, first non-empty wins) for slug-prefix routing.
//
// Reverse-derivation consumer gate (ADR-0042 §1 reverse, spec 120 round-4
// G1, AC-23): entry names are enumerated from an agent-creatable directory
// (os.ReadDir) — validate-and-drop: an invalid dir name (e.g.
// ".mindspec/specs/120-x;evil") is never added to the candidate list, so
// routeReviewSlug's numeric-prefix fallback can only ever return a
// validated ID and the MoveGroup.Dst concat above stays clean.
func (m *Mover) listSpecIDs() []string {
	for _, root := range []string{".mindspec/specs", ".mindspec/docs/specs", "docs/specs"} {
		ents, err := os.ReadDir(filepath.Join(m.root, filepath.FromSlash(root)))
		if err != nil {
			continue
		}
		var ids []string
		for _, e := range ents {
			if e.IsDir() && idvalidate.SpecID(e.Name()) == nil {
				ids = append(ids, e.Name())
			}
		}
		if len(ids) > 0 {
			return ids
		}
	}
	return nil
}

// numericPrefix returns the leading run of ASCII digits in s (e.g.
// "099-final-panel" → "099"), or "" when s does not start with a digit.
func numericPrefix(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// processGroup runs one move group's two-commit sequence (pure rename, then
// link-rewrite), with a durable checkpoint at every boundary. Every step is
// idempotent so a resumed run skips already-landed work.
func (m *Mover) processGroup(i int, g MoveGroup) error {
	srcAbs := filepath.Join(m.root, filepath.FromSlash(g.Src))
	dstAbs := filepath.Join(m.root, filepath.FromSlash(g.Dst))

	if err := m.checkpoint(stageBeforeMv, i); err != nil {
		return err
	}
	// MOVE (idempotent: only when the source still exists).
	if exists(srcAbs) {
		if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
			return err
		}
		if err := m.git.GitMv(m.root, g.Src, g.Dst); err != nil {
			return err
		}
	}
	if err := m.checkpoint(stageAfterMv, i); err != nil {
		return err
	}
	// COMMIT the pure rename (whatever `git mv` staged). No-op if nothing is
	// staged (resume after the rename was already committed).
	if err := m.git.CommitPaths(m.root, "migrate(layout): move "+g.Src+" -> "+g.Dst, nil); err != nil {
		return err
	}
	if err := m.checkpoint(stageAfterMoveCommit, i); err != nil {
		return err
	}
	// REWRITE the moved subtree's links to their final targets (idempotent).
	if _, err := applyRewritesInTree(dstAbs, m.rules); err != nil {
		return err
	}
	if err := m.checkpoint(stageAfterRewrite, i); err != nil {
		return err
	}
	// COMMIT the rewrite: stage every pending non-operational change (this
	// commits the working-tree rewrites even on a resume where applyRewrites
	// was already a no-op because the content was rewritten before the crash).
	dirty, err := m.nonOperationalDirtyPaths()
	if err != nil {
		return err
	}
	if len(dirty) > 0 {
		if err := m.git.CommitPaths(m.root, "migrate(layout): rewrite links in "+g.Dst, dirty); err != nil {
			return err
		}
	}
	return m.checkpoint(stageAfterRewriteCommit, i)
}

// checkpoint records the boundary durably, then fires any test injection.
func (m *Mover) checkpoint(s stage, group int) error {
	m.state.Stage = string(s)
	m.state.Group = group
	m.state.GroupStage = string(s)
	if err := m.writeState(); err != nil {
		return err
	}
	if m.failAt == s && m.failAtGroup == group {
		return fmt.Errorf("injected failure at %s (group %d)", s, group)
	}
	if m.crashAt == s && m.crashAtGroup == group {
		return errSimulatedCrash
	}
	return nil
}

// rollback hard-resets to the pre-run ref and cleans the untracked residue the
// run left behind. It REFUSES once the run is published (ADR-0023 forward-only):
// a published run can only move forward.
//
// The clean is SCOPED to the mover's own touched roots (touchedRoots) rather
// than a repo-wide `git clean -fd`. The CLI precondition refuses a dirty idle
// tree before a fresh run, but Run()/Abort() are exported and Bead 5 reuses
// Run() on the LIVE tree (which now also writes project-docs/ and the
// co-located reviews); a repo-wide clean on rollback could then delete
// user-untracked files OUTSIDE the move set. Scoping it to .mindspec /
// project-docs / review / the configured plan roots removes only this run's
// residue.
//
// NOTE for Bead 5: a PUBLISHING run (one that lands the irreversible flatten on
// a shared branch) MUST arm State.Published BEFORE the point of no return, so
// this auto-rollback path correctly REFUSES rather than hard-resetting a
// published cut.
func (m *Mover) rollback() error {
	if m.state.Published {
		return fmt.Errorf("migrate layout: refusing to auto-roll-back a PUBLISHED run (ADR-0023 forward-only)\nrecovery: roll forward — fix the tree on a new branch and rebase onto post-flatten main")
	}
	if m.state.PreRunRef == "" {
		return fmt.Errorf("migrate layout: cannot roll back — no pre-run ref recorded")
	}
	if err := m.git.ResetHard(m.root, m.state.PreRunRef); err != nil {
		return err
	}
	return m.git.CleanForcePaths(m.root, m.touchedRoots())
}

// touchedRoots returns the repo-relative top-level directories the run may have
// created untracked residue under: `.mindspec` (always — run-state + lineage),
// plus the first path segment of every move group's source and destination
// (`.mindspec`, `project-docs`, `review`, and any custom-plan roots). It reads
// the FROZEN plan from run state when available so the scope is correct even on
// a resumed/aborted run whose in-memory plan was not re-resolved.
func (m *Mover) touchedRoots() []string {
	set := map[string]bool{".mindspec": true}
	plan := m.state.Plan
	if len(plan) == 0 {
		plan = m.plan
	}
	for _, g := range plan {
		if r := firstSegment(g.Src); r != "" {
			set[r] = true
		}
		if r := firstSegment(g.Dst); r != "" {
			set[r] = true
		}
	}
	roots := make([]string, 0, len(set))
	for r := range set {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	return roots
}

// firstSegment returns the first slash-separated path segment of a
// repo-relative path (e.g. ".mindspec/docs/specs" → ".mindspec",
// "project-docs/user" → "project-docs").
func firstSegment(p string) string {
	p = filepath.ToSlash(p)
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}

// Abort is the explicit pre-publish `--abort`: hard-reset to the pre-run ref.
// It refuses after publish (forward-only).
func (m *Mover) Abort() error {
	s, found, err := loadState(m.root, m.runID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("migrate layout: no run-state for run-id %q to abort", m.runID)
	}
	m.state = s
	return m.rollback()
}

// writeLineage writes the doctor-schema lineage manifest to
// .mindspec/lineage/manifest.json and a per-run copy under the run dir. It
// records one entry for EVERY move group that actually landed — across ALL
// groups: the symmetric flatten, the dogfood eviction, AND the review
// co-location — by checking that each group's destination exists on disk (so a
// planned-but-absent group, e.g. dogfood docs a project does not have, is not
// falsely recorded, and a group whose move landed in a prior crashed run is
// still recorded after resume). The un-routable review dirs ride along in
// Skipped as provenance.
func (m *Mover) writeLineage() error {
	manifest := LineageManifest{RunID: m.runID, Skipped: m.skippedReviews}
	for _, g := range m.plan {
		if !exists(filepath.Join(m.root, filepath.FromSlash(g.Dst))) {
			continue // source was absent → no move landed → nothing to record
		}
		manifest.Entries = append(manifest.Entries, LineageEntry{
			Source:    g.Src,
			Canonical: g.Dst,
		})
	}
	if err := writeJSON(lineageManifestPath(m.root), manifest); err != nil {
		return err
	}
	return writeJSON(filepath.Join(runDir(m.root, m.runID), "lineage.json"), manifest)
}

func (m *Mover) writeState() error {
	return writeJSON(statePath(m.root, m.runID), m.state)
}

// nonOperationalDirtyPaths returns the repo-relative paths git reports as
// changed (staged or unstaged), EXCLUDING the mover's own untracked run-state /
// lineage residue, so the rewrite commit stages only real doc changes.
func (m *Mover) nonOperationalDirtyPaths() ([]string, error) {
	out, err := m.git.Status(m.root)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		x, y := line[0], line[1]
		p := strings.TrimSpace(line[3:])
		if idx := strings.Index(p, " -> "); idx >= 0 { // rename: keep the dest
			p = strings.TrimSpace(p[idx+4:])
		}
		p = strings.Trim(p, "\"")
		if p == "" || isOperationalPath(p) {
			continue
		}
		if x == '?' && y == '?' {
			continue // untracked (any non-operational untracked is not ours to commit)
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func (m *Mover) rootDocAbsPaths() []string {
	abs := make([]string, 0, len(m.rootDocs))
	for _, rel := range m.rootDocs {
		abs = append(abs, filepath.Join(m.root, filepath.FromSlash(rel)))
	}
	return abs
}

// removeEmptyDocsDir removes the now-empty .mindspec/docs directory left behind
// once every lifecycle child has moved out (git does not track empty dirs).
// Best-effort: a non-empty dir (e.g. residual dogfood trees handled later) is
// left in place.
func (m *Mover) removeEmptyDocsDir() {
	_ = os.Remove(filepath.Join(m.root, ".mindspec", "docs"))
}

// removeEmptyReviewDir removes the now-empty repo-root review/ tree once every
// review/<slug> dir has been co-located under its owning spec (resolving the
// homeless-review friction, adwu). Best-effort: os.Remove only succeeds on an
// EMPTY directory, so a tree still holding SKIPPED (un-routable) review dirs or
// loose files is intentionally left in place — those are recorded in the
// lineage manifest's Skipped list rather than deleted.
func (m *Mover) removeEmptyReviewDir() {
	_ = os.Remove(filepath.Join(m.root, "review"))
}

// isOperationalPath reports whether a repo-relative path is the mover's own
// run-state / lineage output (never committed as a doc change).
func isOperationalPath(p string) bool {
	p = filepath.ToSlash(p)
	return strings.HasPrefix(p, ".mindspec/migrations/") || strings.HasPrefix(p, ".mindspec/lineage/")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// signatureAtRef is the mover's OWN thin pre/post-flatten ref-shape probe (the
// minor-12 intentional residual duplication — the mover keeps its own probe to
// preserve its captured-fixture golden-test independence from the Bead-1
// workspace signature helper). It returns "flat", "canonical", or "unknown"
// from the .mindspec child directories present in ref's tree.
func signatureAtRef(git GitOps, ref string) (string, error) {
	dirs, err := git.TreeDirsAtRef(ref, ".mindspec")
	if err != nil {
		return "", err
	}
	hasDocs := false
	hasFlatChild := false
	for _, d := range dirs {
		switch d {
		case "docs":
			hasDocs = true
		case "specs", "adr", "domains", "core":
			hasFlatChild = true
		}
	}
	switch {
	case hasDocs:
		return "canonical", nil
	case hasFlatChild:
		return "flat", nil
	default:
		return "unknown", nil
	}
}
