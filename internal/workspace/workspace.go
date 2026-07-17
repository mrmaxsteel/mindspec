package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
)

// ErrNoRoot is returned when no project root marker is found.
var ErrNoRoot = errors.New("no mindspec project root found (looked for .mindspec/, .git)")

// FindRoot walks up from startDir looking for .mindspec/ or .git at each level.
// It checks .mindspec/ first, then .git, to identify the project root.
// If the candidate is a git worktree (.git is a file, not a directory),
// it resolves to the main repository root instead.
func FindRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		if exists(filepath.Join(dir, ".mindspec")) || exists(filepath.Join(dir, ".git")) {
			if resolved := resolveWorktreeRoot(dir); resolved != "" {
				return resolved, nil
			}
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ErrNoRoot
}

// resolveWorktreeRoot checks if dir is a git worktree and returns the main
// repo root. Git worktrees have a .git file (not directory) containing
// "gitdir: <path>". The linked gitdir contains a "commondir" file pointing
// back to the main .git directory, whose parent is the main repo root.
// Returns "" if dir is not a worktree or resolution fails.
func resolveWorktreeRoot(dir string) string {
	gitPath := filepath.Join(dir, ".git")
	fi, err := os.Lstat(gitPath)
	if err != nil || fi.IsDir() {
		return "" // real repo or no .git — not a worktree
	}

	// .git is a file → parse "gitdir: <path>"
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "gitdir: ") {
		return ""
	}
	gitdir := strings.TrimPrefix(content, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dir, gitdir)
	}
	gitdir = filepath.Clean(gitdir)

	// Read commondir to find the shared .git directory
	commondirData, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	if err != nil {
		return ""
	}
	commondir := strings.TrimSpace(string(commondirData))
	if !filepath.IsAbs(commondir) {
		commondir = filepath.Join(gitdir, commondir)
	}
	commondir = filepath.Clean(commondir)

	// commondir is the main .git directory; its parent is the main repo root
	mainRoot := filepath.Dir(commondir)
	if exists(filepath.Join(mainRoot, ".mindspec")) || exists(filepath.Join(mainRoot, ".git")) {
		return mainRoot
	}
	return ""
}

// FindLocalRoot walks up from startDir looking for .mindspec/ or .git at each level.
// Unlike FindRoot, it does NOT resolve worktrees back to the main repo.
// When CWD is inside a worktree, this returns the worktree directory itself.
// This is used for per-worktree focus: each worktree maintains its own .mindspec/focus.
func FindLocalRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		if exists(filepath.Join(dir, ".mindspec")) || exists(filepath.Join(dir, ".git")) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ErrNoRoot
}

// DocsDir returns the path to the docs directory under root.
func DocsDir(root string) string {
	canonical := CanonicalDocsDir(root)
	if exists(canonical) {
		return canonical
	}
	return LegacyDocsDir(root)
}

// CanonicalDocsDir returns the canonical docs root under .mindspec.
func CanonicalDocsDir(root string) string {
	return filepath.Join(root, ".mindspec", "docs")
}

// LegacyDocsDir returns the legacy docs root under project root.
func LegacyDocsDir(root string) string {
	return filepath.Join(root, "docs")
}

// Layout classifies the whole-tree docs layout of a project (Req 2). The flat
// layout keeps lifecycle artifacts directly under .mindspec/; the canonical
// layout nests them under .mindspec/docs/; the legacy layout keeps them under
// a repo-root docs/ directory.
type Layout string

const (
	// LayoutFlat — lifecycle artifacts live directly under .mindspec/
	// (.mindspec/{specs,adr,domains,core}, .mindspec/context-map.md).
	LayoutFlat Layout = "flat"
	// LayoutCanonical — lifecycle artifacts live under .mindspec/docs/.
	LayoutCanonical Layout = "canonical"
	// LayoutLegacy — lifecycle artifacts live under a repo-root docs/ tree.
	LayoutLegacy Layout = "legacy"
	// LayoutGreenfield — an empty tree with no docs layout yet. `mindspec
	// init`/bootstrap creates the flat lifecycle dirs, after which the tree
	// classifies flat and new artifacts are born flat.
	LayoutGreenfield Layout = "greenfield"
	// LayoutMixed — a flat lifecycle tree coexisting with a canonical or
	// legacy one. A hard error outside a recorded migration recovery (Req 2).
	LayoutMixed Layout = "mixed"
)

// LayoutMarkers records which docs layouts are present in a tree. It is the
// pure input to ClassifyLayout: callers populate it from a filesystem probe
// (DetectLayout) or from a git tree listing (the Bead-4 merge guard, via
// executor.TreeDirsAtRef). ClassifyLayout itself does no I/O.
type LayoutMarkers struct {
	Flat      bool // a flat lifecycle child is present directly under .mindspec/
	Canonical bool // a .mindspec/docs/ tree is present
	Legacy    bool // a repo-root docs/ tree is present
}

// ClassifyLayout is the pure layout-signature classifier (plan minor 12): the
// single source of truth that both DetectLayout (filesystem) and the Bead-4
// merge guard (git refs) reuse to fingerprint a tree, so the two never drift.
// A flat tree coexisting with any canonical/legacy tree is mixed; otherwise
// the most-specific present layout wins (flat > canonical > legacy), and an
// empty tree is greenfield.
func ClassifyLayout(m LayoutMarkers) Layout {
	if m.Flat && (m.Canonical || m.Legacy) {
		return LayoutMixed
	}
	switch {
	case m.Flat:
		return LayoutFlat
	case m.Canonical:
		return LayoutCanonical
	case m.Legacy:
		return LayoutLegacy
	default:
		return LayoutGreenfield
	}
}

// LifecycleChildNames lists the four immediate-child directory names —
// specs, adr, domains, and core — that mark a docs tier (flat, canonical, or
// legacy) as populated (ADR-0039 Decision §2). It is the single shared name
// set: filesystem classification below matches it against an IsDir probe on
// each tier's wrapper, and the git-ref classifier in internal/executor
// (Bead 2 — layoutAtRef) matches it against TreeDirsAtRef output from
// .mindspec/docs and root docs, instead of either treating wrapper
// existence (or the .mindspec child name "docs") as a marker.
var LifecycleChildNames = []string{"specs", "adr", "domains", "core"}

// IsLifecycleChildName reports whether name — an immediate child's base name,
// whether read from a filesystem directory listing or a git-ref tree listing
// (e.g. executor.TreeDirsAtRef) — is one of the shared LifecycleChildNames.
// It is the single source of truth for "does this immediate child mark a
// docs tier as populated," reused by filesystem classification (this file,
// via lifecycleChildDirPresent) and by internal/executor's git-ref
// classifier (Bead 2).
func IsLifecycleChildName(name string) bool {
	for _, n := range LifecycleChildNames {
		if name == n {
			return true
		}
	}
	return false
}

// lifecycleChildDirPresent reports whether dir (a docs-tier wrapper such as
// .mindspec, .mindspec/docs, or root docs) has an immediate child, matching
// one of the shared LifecycleChildNames, that is ITSELF a directory
// (IsDir) — not a regular file, and not a name nested more than one level
// below dir. A same-named regular file, a deeper-nested lifecycle name, or
// an unrelated child does not count (ADR-0039 Decision §2 / AC-17, AC-18,
// AC-20). dir itself may be absent or a regular file: isDir on its children
// then simply reports false for each, with no panic.
func lifecycleChildDirPresent(dir string) bool {
	for _, name := range LifecycleChildNames {
		if isDir(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

// LayoutMarkersFromMindspecChildren derives the FLAT marker from the
// immediate child names of a .mindspec/ directory — e.g. the output of
// executor.TreeDirsAtRef(ref, ".mindspec"). It is pure (no I/O).
//
// Bead 2 (spec 118) SUPERSEDES this helper's former canonical-derivation
// behavior: it used to treat the bare child name "docs" as a canonical
// marker, which false-marked canonical for an ordinary `.mindspec/docs/`
// documentation wrapper containing no lifecycle directory and no
// `context-map.md` file (AC-9). A flat list of immediate .mindspec children
// cannot by itself tell a populated `.mindspec/docs/` tree from a bare one —
// that requires descending into `.mindspec/docs/` — so canonical (and
// legacy) derivation now lives in internal/executor's git-ref resolver
// (layoutAtRef / tierMarkerAtRef), which independently descends
// `.mindspec/docs` and root `docs` with TreeDirsAtRef and this package's
// IsLifecycleChildName predicate, plus a type-aware blob probe for each
// tier's context-map.md. This helper is retained ONLY for the flat tier,
// which IS fully determined by .mindspec's own immediate children: an
// immediate child matching IsLifecycleChildName (TreeDirsAtRef already
// restricts these to TREE entries) or a same-named "context-map.md" entry.
func LayoutMarkersFromMindspecChildren(children []string) LayoutMarkers {
	var m LayoutMarkers
	for _, c := range children {
		name := path.Base(strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(c)), "/"))
		if name == "context-map.md" || IsLifecycleChildName(name) {
			m.Flat = true
		}
	}
	return m
}

// ErrMixedLayout is returned by DetectLayout when a flat lifecycle tree
// coexists with a canonical or legacy one outside a recorded migration
// recovery (Req 2). The two shapes must never be live simultaneously.
var ErrMixedLayout = errors.New("mixed docs layout: a flat .mindspec lifecycle tree coexists with a canonical (.mindspec/docs) or legacy (docs/) tree")

// DetectLayout classifies the whole-tree docs layout under root (Req 2). A
// mixed tree is a hard error UNLESS an IN-PROGRESS (non-terminal) migration run
// is recorded under .mindspec/migrations/<run-id>/, in which case the transient
// mixed state of a live recovery is tolerated and returned without error. A
// COMPLETED run's record (which persists past the run; Req 4 / AC9) does NOT
// activate the exception — see migrationRecoveryActive. The classification
// drives the
// write-default (see isFlatTree): a bootstrapped flat tree is born flat;
// existing canonical/legacy projects keep writing their existing form.
func DetectLayout(root string) (Layout, error) {
	kind := detectLayoutKind(root)
	if kind == LayoutMixed && !migrationRecoveryActive(root) {
		return LayoutMixed, ErrMixedLayout
	}
	return kind, nil
}

// detectLayoutKind probes the filesystem under root and classifies it via the
// shared pure classifier. It never errors — the mixed→error decision and the
// recorded-recovery exception live in DetectLayout.
func detectLayoutKind(root string) Layout {
	return ClassifyLayout(LayoutMarkers{
		Flat:      flatTreePresent(root),
		Canonical: canonicalTreePresent(root),
		Legacy:    legacyTreePresent(root),
	})
}

// flatTreePresent reports whether the flat tier is marked directly under
// .mindspec/: an immediate lifecycle child that IS a directory
// (.mindspec/{specs,adr,domains,core}), or .mindspec/context-map.md when
// that path IS a regular file. A same-named regular file, a context-map
// directory, or an otherwise-empty/absent .mindspec sets no marker (ADR-0039
// Decision §2 / AC-19, AC-20).
func flatTreePresent(root string) bool {
	mindspec := MindspecDir(root)
	return lifecycleChildDirPresent(mindspec) || isRegularFile(filepath.Join(mindspec, "context-map.md"))
}

// canonicalTreePresent reports whether the canonical tier is marked under
// .mindspec/docs/: an immediate lifecycle child that IS a directory, or
// .mindspec/docs/context-map.md when that path IS a regular file. A bare or
// empty .mindspec/docs wrapper, an unrelated child, a nested lifecycle name,
// a wrapper-as-file, or a context-map directory sets no marker (ADR-0039
// Decision §2 / AC-2, AC-13, AC-17, AC-18, AC-19, AC-20).
func canonicalTreePresent(root string) bool {
	canonical := CanonicalDocsDir(root)
	return lifecycleChildDirPresent(canonical) || isRegularFile(filepath.Join(canonical, "context-map.md"))
}

// legacyTreePresent reports whether the legacy tier is marked under root
// docs/: an immediate lifecycle child that IS a directory, or
// docs/context-map.md when that path IS a regular file. A bare or empty
// docs/ wrapper, an unrelated child, a nested lifecycle name, a
// wrapper-as-file, or a context-map directory sets no marker (ADR-0039
// Decision §2 / AC-1, AC-14, AC-17, AC-18, AC-19, AC-20).
func legacyTreePresent(root string) bool {
	legacy := LegacyDocsDir(root)
	return lifecycleChildDirPresent(legacy) || isRegularFile(filepath.Join(legacy, "context-map.md"))
}

// migrationRunState is the minimal projection of the layout mover's per-run
// checkpoint record (.mindspec/migrations/<run-id>/state.json — see
// internal/layout/runstate.go State and internal/doctor/migration.go runState)
// needed to tell a LIVE run from a COMPLETED one. Only the stage field matters
// here; the full schema carries crash-resume bookkeeping the resolver ignores.
type migrationRunState struct {
	Stage string `json:"stage"`
}

// migrationTerminalStage is the mover's terminal/finalize stage: a run that
// reached it has fully applied and is NO LONGER a live recovery. It mirrors the
// doctor schema's OK stage (internal/doctor/migration.go) and the mover's
// stageApplied (internal/layout/runstate.go). A completed run still persists
// its .mindspec/migrations/<run-id>/ record (Req 4 / AC9), so keying the
// mixed-layout exception on mere dir existence would let a STALE completed
// record mask a real half-old/half-flat split — hence the predicate keys on a
// non-terminal stage, not dir presence.
const migrationTerminalStage = "applied"

// migrationRecoveryActive reports whether an IN-PROGRESS (non-terminal)
// migration run is recorded under .mindspec/migrations/<run-id>/ — the
// recorded-recovery exception that lets a transient mixed tree pass (Req 2).
//
// The exception is scoped to a LIVE run: a run dir whose state.json parses and
// records a non-empty, non-terminal stage (the mover is mid-flight — crashed,
// interrupted, or stopped at a link-check failure that left the tree mixed). A
// run dir with no readable state, an empty stage, or the terminal "applied"
// stage is a COMPLETED (or unrecognizable) record and does NOT activate the
// exception, so a stale completed record can never silently mask the
// half-old/half-flat split the spec makes a hard error.
func migrationRecoveryActive(root string) bool {
	migrationsDir := filepath.Join(MindspecDir(root), "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if migrationRunInProgress(filepath.Join(migrationsDir, e.Name())) {
			return true
		}
	}
	return false
}

// MigrationRecoveryActive reports whether an IN-PROGRESS (non-terminal) layout
// migration run is recorded under .mindspec/migrations/<run-id>/ — the exported
// accessor for the recorded-recovery exception (Req 2). It is the SAME in-flight
// scoping DetectLayout honors, surfaced for the Bead-4 directional merge guard
// (internal/executor) to EXEMPT a transient cross-layout merge during a live
// recovery, so the guard reuses Bead-1's run-state scoping rather than
// reimplementing it. A stale/completed run record (terminal/empty stage) does
// NOT activate it, so the exemption can never silently mask a real regression.
func MigrationRecoveryActive(root string) bool {
	return migrationRecoveryActive(root)
}

// migrationRunInProgress reports whether the migration run recorded in runDir
// is LIVE: its state.json parses and records a non-empty, non-terminal stage.
// An absent/unreadable/unparseable state.json, an empty stage, or the terminal
// stage all read as NOT in progress.
func migrationRunInProgress(runDir string) bool {
	data, err := os.ReadFile(filepath.Join(runDir, "state.json"))
	if err != nil {
		return false
	}
	var st migrationRunState
	if err := json.Unmarshal(data, &st); err != nil {
		return false
	}
	return st.Stage != "" && st.Stage != migrationTerminalStage
}

// isFlatTree reports whether root is an actual FLAT lifecycle tree — the
// condition under which NEW artifacts are born flat (Req 2). A bare/greenfield
// tree is deliberately NOT treated as flat here: born-flat is realized once
// `mindspec init`/bootstrap has created the flat lifecycle dirs, which makes
// detectLayoutKind report flat. Gating the born-flat write target on an actual
// flat tree (rather than on bare-greenfield) keeps the write-default and the
// resolvers byte-for-byte identical on canonical, legacy, AND greenfield trees
// (Req 15 / AC1) while still delivering born-flat for a bootstrapped project.
func isFlatTree(root string) bool {
	return detectLayoutKind(root) == LayoutFlat
}

// resolveArtifact resolves a docs-relative artifact path with the per-artifact,
// three-tier, flat-first read precedence (Req 1): flat (.mindspec/<rel>) →
// canonical (.mindspec/docs/<rel>) → legacy (docs/<rel>), first-exists-wins.
// "Flat FIRST" is READ-PRECEDENCE, not delivery order: when the flat artifact
// is absent on disk it falls back to the flat tree's own root if (and only if)
// the tree is already flat (so a flat project writes a not-yet-created artifact
// flat too), otherwise to the historical DocsDir join-point. For every
// canonical/legacy/greenfield tree with no flat tree present this is
// byte-for-byte the pre-spec resolution (Req 15). The canonical/legacy fallback
// stays docs-root-keyed (canonical whenever .mindspec/docs/ exists) rather than
// per-artifact-existence-keyed, which is what preserves byte-identity with the
// pre-spec DocsDir behavior.
func resolveArtifact(root, rel string) string {
	if flat := filepath.Join(MindspecDir(root), rel); exists(flat) {
		return flat
	}
	if isFlatTree(root) {
		return filepath.Join(MindspecDir(root), rel)
	}
	return filepath.Join(DocsDir(root), rel)
}

// SpecDir returns the path to a specific spec directory under root.
// Resolution order (ADR-0022 + Req 7), first-exists-wins:
//  1. worktree, flat shape:      root/.worktrees/worktree-spec-<id>/.mindspec/specs/<id>/
//  2. worktree, canonical shape: root/.worktrees/worktree-spec-<id>/.mindspec/docs/specs/<id>/
//  3. flat:                      root/.mindspec/specs/<id>/
//  4. canonical:                 root/.mindspec/docs/specs/<id>/
//  5. legacy:                    root/docs/specs/<id>/
//
// Returns the first path that exists on disk. If none exist, it returns the
// layout-aware write target so callers creating new specs write to the right
// location: born flat for an actual flat tree (a bootstrapped project), and
// the historical canonical default otherwise (Req 2/15). For every
// canonical/legacy/greenfield tree with no flat tree present this is
// byte-for-byte the pre-spec resolution.
//
// The flat worktree tier (1) preserves the mindspec-ew79 cross-worktree
// ADR-visibility fix once a worktree's tree is flat.
//
// SpecDir deliberately scans the default ".worktrees" root rather than
// honoring cfg.WorktreeRoot — it must locate existing on-disk worktrees,
// which may have been created with a different config at the time. A
// follow-up bead may extend this to accept *config.Config and probe both
// the configured root and the default for portability across configs.
//
// Returns an error if specID is not a well-formed spec identifier. This
// turns a previously silent vulnerability (user-controlled IDs reaching
// filepath.Join) into a compile-time obligation for every caller. See
// SEC-1 (bead mindspec-x1qr).
func SpecDir(root, specID string) (string, error) {
	if err := idvalidate.SpecID(specID); err != nil {
		return "", err
	}
	wtBase := filepath.Join(DefaultWorktreesDir(root), SpecWorktreeName(specID))
	// 1. Worktree, flat shape.
	if p := filepath.Join(wtBase, ".mindspec", "specs", specID); exists(p) {
		return p, nil
	}
	// 2. Worktree, canonical shape.
	if p := filepath.Join(wtBase, ".mindspec", "docs", "specs", specID); exists(p) {
		return p, nil
	}
	// 3. Flat (read-precedence first).
	if p := filepath.Join(MindspecDir(root), "specs", specID); exists(p) {
		return p, nil
	}
	// 4. Canonical.
	if p := filepath.Join(CanonicalDocsDir(root), "specs", specID); exists(p) {
		return p, nil
	}
	// 5. Legacy.
	if p := filepath.Join(LegacyDocsDir(root), "specs", specID); exists(p) {
		return p, nil
	}
	// Default (new spec): born flat for an actual flat tree (a bootstrapped
	// greenfield project), otherwise the historical canonical default —
	// byte-identical for canonical/legacy/greenfield trees (Req 15).
	if isFlatTree(root) {
		return filepath.Join(MindspecDir(root), "specs", specID), nil
	}
	return filepath.Join(CanonicalDocsDir(root), "specs", specID), nil
}

// ContextMapPath returns the path to the context-map.md under root, resolved
// with the three-tier flat-first precedence (Req 1): .mindspec/context-map.md
// → .mindspec/docs/context-map.md → docs/context-map.md.
func ContextMapPath(root string) string {
	return resolveArtifact(root, "context-map.md")
}

// ADRDir returns the path to the adr/ directory under root, resolved with the
// three-tier flat-first precedence (Req 1): .mindspec/adr → .mindspec/docs/adr
// → docs/adr.
//
// This takes no user input so its signature is unchanged. However, every
// site that joins ADRDir with a user-supplied ADR ID (e.g.
// `filepath.Join(workspace.ADRDir(root), id+".md")`) must validate the id
// first. Use ADRFilePath, which bundles validation + path construction.
func ADRDir(root string) string {
	return resolveArtifact(root, "adr")
}

// CoreDir returns the path to the core docs directory under root, resolved
// with the three-tier flat-first precedence (Req 1): .mindspec/core →
// .mindspec/docs/core → docs/core.
func CoreDir(root string) string {
	return resolveArtifact(root, "core")
}

// TreeRootForSpecDir returns the root of the checkout tree that a
// SpecDir-resolved spec directory lives in. SpecDir is worktree-aware
// (ADR-0022 resolution order: worktree → canonical → legacy), but
// ADRDir is not — it always resolves under the primary checkout. This
// helper lets validators that received a spec dir inside a spec
// worktree build an ADR store rooted in the SAME tree, so ADRs
// committed only on the spec branch are visible (mindspec-ew79).
//
// Recognized layouts:
//   - <tree>/.mindspec/specs/<id>       → returns <tree> (flat, 3 levels up)
//   - <tree>/.mindspec/docs/specs/<id>  → returns <tree> (canonical, 4 levels up)
//   - <tree>/docs/specs/<id>            → returns <tree> (legacy, 3 levels up)
//
// The flat shape (Req 7) is required so the mindspec-ew79 cross-worktree
// ADR-visibility fix is preserved once a worktree's tree is flattened — the
// pre-spec check (filepath.Base(docs) != "docs") returned "" for a flat dir.
//
// Returns "" when specDir does not match any recognized layout; callers should
// fall back to the primary root.
func TreeRootForSpecDir(specDir string) string {
	abs, err := filepath.Abs(specDir)
	if err != nil {
		return ""
	}
	abs = filepath.Clean(abs)
	specs := filepath.Dir(abs)
	if filepath.Base(specs) != "specs" {
		return ""
	}
	parent := filepath.Dir(specs)
	switch filepath.Base(parent) {
	case "docs":
		// canonical (<tree>/.mindspec/docs/specs/<id>) or legacy
		// (<tree>/docs/specs/<id>).
		grandparent := filepath.Dir(parent)
		if filepath.Base(grandparent) == ".mindspec" {
			return filepath.Dir(grandparent)
		}
		return grandparent
	case ".mindspec":
		// flat (<tree>/.mindspec/specs/<id>).
		return filepath.Dir(parent)
	default:
		return ""
	}
}

// ADRFilePath returns the on-disk path for a single ADR file by ID.
// Returns an error if adrID is not a well-formed ADR identifier.
func ADRFilePath(root, adrID string) (string, error) {
	if err := idvalidate.ADRID(adrID); err != nil {
		return "", err
	}
	return filepath.Join(ADRDir(root), adrID+".md"), nil
}

// DomainDir returns the path to a specific domain's doc directory under root,
// resolved with the three-tier flat-first precedence (Req 1):
// .mindspec/domains/<d> → .mindspec/docs/domains/<d> → docs/domains/<d>.
// Returns an error if domain is not a well-formed domain name.
func DomainDir(root, domain string) (string, error) {
	if err := idvalidate.DomainName(domain); err != nil {
		return "", err
	}
	return resolveArtifact(root, filepath.Join("domains", domain)), nil
}

// SpecsDir returns the flat-aware ENUMERATION root for specs — the directory
// that holds every spec's per-id subdirectory — resolved with the same
// three-tier flat-first precedence as the per-item resolvers (Req 1):
// .mindspec/specs (flat) → .mindspec/docs/specs (canonical) → docs/specs
// (legacy). It is the parent that SpecDir(root, id) resolves an <id> under, so a
// filesystem enumerator can list specs without re-deriving the layout. For
// every canonical/legacy/greenfield tree with no flat tree present this is
// byte-for-byte filepath.Join(DocsDir(root), "specs") — the pre-spec
// enumeration root.
func SpecsDir(root string) string {
	return resolveArtifact(root, "specs")
}

// DomainsDir returns the flat-aware ENUMERATION root for domains — the directory
// that holds every domain's per-name subdirectory — resolved with the same
// three-tier flat-first precedence as the per-item resolvers (Req 1):
// .mindspec/domains (flat) → .mindspec/docs/domains (canonical) → docs/domains
// (legacy). It is the parent that DomainDir(root, d) resolves a <d> under. For
// every canonical/legacy/greenfield tree with no flat tree present this is
// byte-for-byte filepath.Join(DocsDir(root), "domains") — the pre-spec
// enumeration root.
func DomainsDir(root string) string {
	return resolveArtifact(root, "domains")
}

// RecordingDir returns the path to a spec's recording directory.
// Returns an error if specID is not a well-formed spec identifier.
func RecordingDir(root, specID string) (string, error) {
	specDir, err := SpecDir(root, specID)
	if err != nil {
		return "", err
	}
	return filepath.Join(specDir, "recording"), nil
}

// MindspecDir returns the path to the .mindspec directory under root.
func MindspecDir(root string) string {
	return filepath.Join(root, ".mindspec")
}

// SessionPath returns the path to .mindspec/session.json under root.
func SessionPath(root string) string {
	return filepath.Join(root, ".mindspec", "session.json")
}

// WorktreeKind describes the type of worktree context.
const (
	WorktreeMain = "main" // Main repository root
	WorktreeSpec = "spec" // Spec worktree (.worktrees/worktree-spec-<id>)
	WorktreeBead = "bead" // Bead worktree (.worktrees/worktree-<bead-id>)
)

// DetectWorktreeContext identifies the worktree context from a directory path.
// It returns the kind (main/spec/bead) and any extracted spec or bead ID.
// Detection is based on the path containing .worktrees/worktree-spec-<id> or
// .worktrees/worktree-<bead-id>.
//
// This function intentionally does NOT honor cfg.WorktreeRoot: it scans
// path strings that may have been produced with whatever worktree-root
// name was active when the directory was created. Recognizing the
// canonical ".worktrees" token keeps detection robust across configs.
func DetectWorktreeContext(dir string) (kind, specID, beadID string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return WorktreeMain, "", ""
	}

	// Walk the path and use the LAST .worktrees match so that nested worktrees
	// (bead worktree inside spec worktree) resolve to the innermost type.
	parts := strings.Split(filepath.ToSlash(abs), "/")
	lastKind := WorktreeMain
	var lastSpecID, lastBeadID string
	for i, part := range parts {
		if part == ".worktrees" && i+1 < len(parts) {
			wtDir := parts[i+1]
			if strings.HasPrefix(wtDir, SpecWorktreePrefix) {
				lastKind = WorktreeSpec
				lastSpecID = strings.TrimPrefix(wtDir, SpecWorktreePrefix)
				lastBeadID = ""
			} else if strings.HasPrefix(wtDir, BeadWorktreePrefix) {
				lastKind = WorktreeBead
				lastBeadID = strings.TrimPrefix(wtDir, BeadWorktreePrefix)
			}
		}
	}
	return lastKind, lastSpecID, lastBeadID
}

// ContextLine renders the worktree context of dir for guard-failure
// messages (spec 092 Req 8). It names the kind of worktree dir is in
// (via DetectWorktreeContext) and the path the failing check actually
// evaluated — often a different directory. Guard call sites append this
// line to their failure message so an agent that ran a command from the
// wrong directory sees both where it is and what was checked.
//
// The format is fixed:
//
//	you are in the <main|spec|bead> worktree (<dir>); this check evaluated <checkedPath>
func ContextLine(dir, checkedPath string) string {
	kind, _, _ := DetectWorktreeContext(dir)
	return fmt.Sprintf("you are in the %s worktree (%s); this check evaluated %s", kind, dir, checkedPath)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isDir reports whether path exists and is a directory. A path that does not
// exist, cannot be stat'd (e.g. because a path component is itself a regular
// file), or exists as something other than a directory reports false rather
// than panicking or returning an error.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// isRegularFile reports whether path exists and is a regular file (not a
// directory or other non-regular mode). A path that does not exist or cannot
// be stat'd reports false rather than panicking or returning an error.
func isRegularFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}
