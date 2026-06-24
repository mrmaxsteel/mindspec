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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/doctor"
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

// DefaultFlattenPlan is the canonical flatten move plan: the four lifecycle
// directories plus the context-map file. Order is deterministic.
func DefaultFlattenPlan() []MoveGroup {
	return []MoveGroup{
		{Src: ".mindspec/docs/specs", Dst: ".mindspec/specs"},
		{Src: ".mindspec/docs/adr", Dst: ".mindspec/adr"},
		{Src: ".mindspec/docs/domains", Dst: ".mindspec/domains"},
		{Src: ".mindspec/docs/core", Dst: ".mindspec/core"},
		{Src: ".mindspec/docs/context-map.md", Dst: ".mindspec/context-map.md"},
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

// rollback hard-resets to the pre-run ref and cleans untracked residue. It
// REFUSES once the run is published (ADR-0023 forward-only): a published run
// can only move forward.
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
	return m.git.CleanForce(m.root)
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

// writeLineage writes the doctor-schema lineage manifest (one entry per move
// group) to .mindspec/lineage/manifest.json and a per-run copy under the run
// dir.
func (m *Mover) writeLineage() error {
	manifest := LineageManifest{RunID: m.runID}
	for _, g := range m.plan {
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
