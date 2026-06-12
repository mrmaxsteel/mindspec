package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// classifiedChanges groups a diff's changed files by category so doc-sync
// lanes can reason about source, doc, and the raw full list together.
// Package-private: no external consumers (spec-086 panel CONSENSUS Minor 8).
type classifiedChanges struct {
	All    []string
	Source []string
	Docs   []string
}

// missingSourceGlobsMsg is the Req 22(b) migration-status line. It
// mirrors the Req 18 doctor Warn text: names the expected config file
// path, DISCLOSES the active built-in default, and hints the populate
// command. The complete/approve warnings pipe (Req 22(a)) renders it
// as `WARN missing-source-globs: <this message>`.
const missingSourceGlobsMsg = "source_globs not set in .mindspec/config.yaml — " +
	"doc-sync is classifying source with the built-in default " +
	"(.go under cmd/ and internal/, excluding _test.go); " +
	"run 'mindspec source populate' to declare your own"

// ValidateDocs checks for doc-sync compliance by comparing changed source files
// against documentation updates in the same diff. The diff is the working
// tree vs diffRef — the historical semantics every pre-existing call site
// (impl approve run from the spec worktree, `mindspec validate docs`)
// relies on. As a thin wrapper over ValidateDocsRange it inherits the
// spec 091 source_globs override semantics described there.
func ValidateDocs(root, diffRef string, exec executor.Executor) *Result {
	return ValidateDocsRange(root, diffRef, "", exec)
}

// ValidateDocsRange is ValidateDocs over an explicit base..head ref range.
// When head is non-empty the diff is base..head — independent of whatever
// checkout the process runs from. This is the per-bead gate's anchoring
// (mindspec-aqey / mindspec-perm): complete.Run passes the bead branch's
// fork point as base and the bead branch tip as head, so the gate measures
// exactly the bead's work. head == "" preserves the working-tree-vs-base
// semantics of ValidateDocs.
//
// Source classification honors the operator-declared `source_globs`
// from .mindspec/config.yaml with FULL-OVERRIDE semantics (spec 091
// Req 16): a non-empty list is the ONLY classifier (never a union with
// the built-in rule); an empty or absent list leaves the built-in
// isSourceFile classifier running byte-identically to pre-091 (HC-7).
// The override decision point is classifyChangesWithGlobs below.
func ValidateDocsRange(root, base, head string, exec executor.Executor) *Result {
	r := &Result{SubCommand: "docs"}

	if base == "" {
		base = "HEAD~1"
	}

	changed, err := getChangedFiles(exec, base, head)
	if err != nil {
		r.AddError("git-diff", fmt.Sprintf("cannot get changed files: %v", err))
		return r
	}

	if len(changed) == 0 {
		return r // no changes, all good
	}

	globs, cfgOK := sourceGlobs(root)
	if cfgOK && len(globs) == 0 {
		// Req 22(b) migration-status nudge — recurring and STATELESS
		// by construction (HC-2: no marker, no seen-tracking): the
		// warning-severity issue is attached on every invocation while
		// source_globs is empty/absent; the complete/approve warnings
		// pipe prints it (Req 22(a)). Deferred so it rides every
		// return path below and renders after the lane issues.
		defer func() {
			r.AddWarning("missing-source-globs", missingSourceGlobsMsg)
		}()
	}

	sourceChanges, docChanges := classifyChangesWithGlobs(changed, globs)
	changes := classifiedChanges{All: changed, Source: sourceChanges, Docs: docChanges}

	// Spec-artifact sync runs BEFORE the source-empty early-return so a
	// spec.md-only diff (which classifies as docs-only) still gates on
	// having a plan.md / ADR / sibling artifact in the same diff.
	validateSpecArtifactSync(r, changes)

	if len(sourceChanges) == 0 {
		return r // only doc changes, spec-artifact lane already ran
	}

	// Check if any doc files were also changed
	if len(docChanges) == 0 {
		r.AddError("doc-sync", "source files changed but no documentation files updated")
	}

	// Check specific mapping heuristics
	checkInternalPackages(r, root, sourceChanges, docChanges)
	checkCmdChanges(r, sourceChanges, docChanges)

	// Advisory continuous-accuracy lane (spec 091 Req 16): runs only
	// with a non-empty source_globs, AFTER the blocking lanes — when
	// populated globs meet zero domain directories this deliberately
	// double-reports alongside the zero-domains blocking branch above
	// (a specified double-report, not a bug; do not suppress either
	// side).
	checkUnclaimedSource(r, root, globs, sourceChanges)

	return r
}

// sourceGlobs returns the operator-declared source_globs from
// .mindspec/config.yaml under root (spec 091 Req 11) and whether the
// config was readable. On a config load error it returns (nil, false):
// the built-in classifier stays active as the disclosed fallback, and
// the Req 22(b) nudge is suppressed — we cannot honestly claim the
// field is "not set" when the config is unreadable (config errors
// surface through the flows that own config handling).
func sourceGlobs(root string) ([]string, bool) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, false
	}
	return cfg.SourceGlobs, true
}

// classifyChangesWithGlobs splits files into source and doc categories,
// honoring the spec 091 Req 16 override semantics. This is the single
// override decision point:
//
//   - globs NON-EMPTY → the globs are the ONLY classifier (FULL
//     OVERRIDE, never union): a glob-matched file IS source even when
//     the built-in rule would reject it (a .js file, a _test.go file,
//     a file outside cmd/ and internal/); a non-matching file is NOT
//     source even when the built-in rule would accept it. The built-in
//     isSourceFile classifier is fully bypassed.
//   - globs EMPTY → delegates to the unchanged classifyChanges path,
//     so classification is byte-identical to pre-091 (HC-7) and the
//     built-in classifier keeps driving the blocking lanes.
//
// Doc-file precedence is preserved in both branches: isDocFile is
// checked first, so doc files classify as docs even when a glob
// matches them.
func classifyChangesWithGlobs(files, globs []string) (source, docs []string) {
	if len(globs) == 0 {
		return classifyChanges(files)
	}
	for _, f := range files {
		if isDocFile(f) {
			docs = append(docs, f)
		} else if matchesAny(globs, f) {
			source = append(source, f)
		}
	}
	return
}

// checkUnclaimedSource emits the advisory `unclaimed-source` Warn
// (spec 091 Req 16): with a NON-EMPTY source_globs, it fires when the
// diff touches glob-matched source files that no domain's resolved
// `paths` (minus `exclude`) claims — regardless of each domain's
// Source() state. The message lists (a) each unclaimed file, (b) a
// MECHANICAL per-domain state report annotated with Ownership.Source()
// (the framework reports state and never ranks or guesses which domain
// should claim a file — ZFC), and (c) a remedy hint. When EVERY
// domain's Source() is "manifest" (vacuously true at zero domains) the
// message says so explicitly and the hint switches to
// widen-an-existing-manifest or `mindspec domain add` — never commands
// that would do nothing. Advisory only: never blocks the gate. The
// Warn is disabled while source_globs is empty/absent.
func checkUnclaimedSource(r *Result, root string, globs, source []string) {
	if len(globs) == 0 || len(source) == 0 {
		return
	}

	domains, err := listDomainDirs(root)
	if err != nil {
		// checkInternalPackages already reported the enumeration
		// failure as a blocking error; the advisory lane stays quiet.
		return
	}

	type domainState struct {
		name string
		o    *Ownership
	}
	states := make([]domainState, 0, len(domains))
	for _, d := range domains {
		o, derr := LoadOwnership(root, d)
		if derr != nil {
			// Schema violations already surface as blocking errors via
			// the attribution lane; the advisory lane stays quiet
			// rather than reporting against a half-loaded state.
			return
		}
		states = append(states, domainState{name: d, o: o})
	}

	var unclaimed []string
	for _, f := range source {
		claimed := false
		for _, s := range states {
			if matchesAny(s.o.Paths, f) && !matchesAny(s.o.Exclude, f) {
				claimed = true
				break
			}
		}
		if !claimed {
			unclaimed = append(unclaimed, f)
		}
	}
	if len(unclaimed) == 0 {
		return
	}
	sort.Strings(unclaimed)

	// Mechanical per-domain state report (domains arrive sorted from
	// listDomainDirs): report state, never rank candidates.
	allManifest := true
	reports := make([]string, 0, len(states))
	for _, s := range states {
		state := s.o.Source()
		if state != "manifest" {
			allManifest = false
		}
		reports = append(reports, s.name+"="+state)
	}
	stateReport := strings.Join(reports, ", ")
	if len(states) == 0 {
		stateReport = "(no domain directories exist)"
	}

	var hint string
	if allManifest {
		// Every domain is "manifest" (vacuously so at zero domains):
		// `mindspec doctor --fix` would scaffold nothing, so it is NOT
		// named here — pointing at commands that would do nothing is
		// worse than no hint (Req 16).
		hint = "every domain's OWNERSHIP.yaml is already populated — no unpopulated candidates exist; " +
			"the unclaimed files may belong to a domain whose populated manifest needs widening " +
			"(run 'mindspec ownership populate <domain>' — it works for populated domains when named explicitly) " +
			"or to a domain that does not exist yet (run 'mindspec domain add <name>')"
	} else {
		hint = "run 'mindspec doctor --fix' to scaffold missing manifests, " +
			"then 'mindspec ownership populate <domain>' to populate one"
	}

	r.AddWarning("unclaimed-source", fmt.Sprintf(
		"files matching source_globs are claimed by no domain's OWNERSHIP.yaml: %s; domain ownership state: %s; %s",
		strings.Join(unclaimed, ", "), stateReport, hint,
	))
}

// getChangedFiles returns the list of changed files for the requested
// range, routing through the Executor boundary instead of shelling out.
// head == "" means "working tree vs base" (the executor's empty-base
// idiom); a non-empty head means the committed range base..head.
func getChangedFiles(exec executor.Executor, base, head string) ([]string, error) {
	if head == "" {
		files, err := exec.ChangedFiles("", base)
		if err != nil {
			return nil, fmt.Errorf("changed files for %s: %w", base, err)
		}
		return files, nil
	}
	files, err := exec.ChangedFiles(base, head)
	if err != nil {
		return nil, fmt.Errorf("changed files for %s..%s: %w", base, head, err)
	}
	return files, nil
}

// ParseChangedFiles parses a newline-separated list of file paths.
// Exported for testing without shelling out to git.
func ParseChangedFiles(output string) []string {
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// classifyChanges splits files into source and doc categories.
func classifyChanges(files []string) (source, docs []string) {
	for _, f := range files {
		if isDocFile(f) {
			docs = append(docs, f)
		} else if isSourceFile(f) {
			source = append(source, f)
		}
	}
	return
}

// isDocFile returns true for documentation files.
func isDocFile(path string) bool {
	return strings.HasPrefix(path, "docs/") ||
		strings.HasPrefix(path, ".mindspec/docs/") ||
		strings.HasPrefix(path, "CLAUDE.md") ||
		strings.HasPrefix(path, "AGENTS.md")
}

// isSourceFile returns true for Go source files.
func isSourceFile(path string) bool {
	return (strings.HasPrefix(path, "internal/") || strings.HasPrefix(path, "cmd/")) &&
		strings.HasSuffix(path, ".go") &&
		!strings.HasSuffix(path, "_test.go")
}

// listDomainDirs returns the lexicographically-sorted list of domain
// directory names under .mindspec/docs/domains/ in the given root.
// Returns an empty slice (no error) when the domains directory is
// missing — checkInternalPackages then takes its zero-domains
// disclosed default (per-package internal/<pkg>/ attribution). The
// per-domain loader itself has NO fallback: a domain directory whose
// manifest is missing claims nothing (spec 091 Req 13).
func listDomainDirs(root string) ([]string, error) {
	dir := filepath.Join(root, ".mindspec", "docs", "domains")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading domains dir %s: %w", dir, err)
	}
	domains := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			domains = append(domains, e.Name())
		}
	}
	sort.Strings(domains)
	return domains, nil
}

// checkInternalPackages errors when internal/ packages changed without
// the corresponding domain docs being updated in the same diff.
// Attribution uses the ownership machinery (LoadOwnership +
// attributeDomain): each changed source path is resolved to its
// owning domain via .mindspec/docs/domains/<domain>/OWNERSHIP.yaml.
// A domain whose manifest is absent claims NOTHING — the silent
// "internal/<domain>/**" loader fallback was removed by spec 091
// Req 13. The error message NAMES the manifest file that decided
// ownership so the operator knows which OWNERSHIP.yaml to edit. The
// only surviving fallback is the zero-domains disclosed default
// below, which applies when no domain directories exist at all.
func checkInternalPackages(r *Result, root string, source, docs []string) {
	domains, err := listDomainDirs(root)
	if err != nil {
		r.AddError("internal-docs", fmt.Sprintf("cannot enumerate domain dirs: %v", err))
		return
	}

	// Group source files by attributed domain, retaining the
	// manifest path that decided ownership.
	type attribution struct {
		manifest string // o.ManifestPath; never empty post spec 091 (attribution requires non-empty Paths ⇒ manifest-backed load)
		files    []string
	}
	byDomain := map[string]*attribution{}

	// Zero-domains DISCLOSED DEFAULT (spec 091 Req 13): when no
	// domain directories exist at all, attribute changed
	// internal/<pkg>/ files per-package and emit blocking
	// internal-docs errors carrying the literal
	// "<fallback: internal/<pkg>/**>" marker. This branch is the
	// deliberate no-domains default — NOT a leftover of the removed
	// per-domain loader fallback — and is the only drift coverage
	// for bare checkouts with no domain docs. The marker in the
	// error text is the disclosure.
	if len(domains) == 0 {
		pkgs := map[string][]string{}
		for _, f := range source {
			if !strings.HasPrefix(f, "internal/") {
				continue
			}
			parts := strings.SplitN(f, "/", 3)
			if len(parts) < 2 {
				continue
			}
			pkgs[parts[1]] = append(pkgs[parts[1]], f)
		}
		if len(pkgs) == 0 {
			return
		}
		hasDomainDocs := false
		for _, f := range docs {
			if strings.HasPrefix(f, "docs/domains/") || strings.HasPrefix(f, ".mindspec/docs/domains/") {
				hasDomainDocs = true
				break
			}
		}
		if hasDomainDocs {
			return
		}
		names := make([]string, 0, len(pkgs))
		for p := range pkgs {
			names = append(names, p)
		}
		sort.Strings(names)
		for _, p := range names {
			r.AddError("internal-docs", fmt.Sprintf(
				"internal sources in domain %q changed (%s) but no doc updates under %s/; ownership decided by <fallback: internal/%s/**>",
				p, strings.Join(pkgs[p], ", "),
				filepath.Join(".mindspec", "docs", "domains", p),
				p,
			))
		}
		return
	}

	for _, f := range source {
		// Only consider files that could plausibly be owned by a
		// domain. attributeDomain returns "" when nothing matches —
		// in that case the file is silently skipped (it is not the
		// internal-docs lane's job to police unmapped trees).
		domain, o, derr := attributeDomain(root, f, domains)
		if derr != nil {
			r.AddError("internal-docs", fmt.Sprintf("attributing %s: %v", f, derr))
			continue
		}
		if domain == "" {
			continue
		}
		manifest := ""
		if o != nil {
			manifest = o.ManifestPath
		}
		a, ok := byDomain[domain]
		if !ok {
			a = &attribution{manifest: manifest}
			byDomain[domain] = a
		}
		a.files = append(a.files, f)
	}

	if len(byDomain) == 0 {
		return
	}

	// Walk domains in sorted order for deterministic emit.
	domainNames := make([]string, 0, len(byDomain))
	for d := range byDomain {
		domainNames = append(domainNames, d)
	}
	sort.Strings(domainNames)

	for _, domain := range domainNames {
		a := byDomain[domain]
		hasDomainDocs := false
		mindspecPrefix := ".mindspec/docs/domains/" + domain + "/"
		legacyPrefix := "docs/domains/" + domain + "/"
		for _, f := range docs {
			if strings.HasPrefix(f, mindspecPrefix) || strings.HasPrefix(f, legacyPrefix) {
				hasDomainDocs = true
				break
			}
		}
		if hasDomainDocs {
			continue
		}
		// a.manifest is always non-empty here: attributeDomain only
		// returns a domain whose Paths matched, and post spec 091
		// (Req 13) a non-empty Paths implies a manifest-backed load.
		// The old empty-ManifestPath "<fallback: internal/<domain>/**>"
		// marker branch was therefore dead and has been removed (panel
		// V2-4); TestPerDomainMarkerNamesManifest pins this outcome.
		r.AddError("internal-docs", fmt.Sprintf(
			"internal sources in domain %q changed (%s) but no doc updates under %s/; ownership decided by %s",
			domain, strings.Join(a.files, ", "),
			filepath.Join(".mindspec", "docs", "domains", domain),
			a.manifest,
		))
	}
}

// checkCmdChanges warns if cmd/ files changed without CLAUDE.md or CONVENTIONS.md updates.
func checkCmdChanges(r *Result, source, docs []string) {
	hasCmdChanges := false
	for _, f := range source {
		if strings.HasPrefix(f, "cmd/") {
			hasCmdChanges = true
			break
		}
	}

	if !hasCmdChanges {
		return
	}

	hasRelevantDoc := false
	for _, f := range docs {
		// Existing operator-docs accept set (preserved):
		if f == "CLAUDE.md" || strings.Contains(f, "CONVENTIONS.md") {
			hasRelevantDoc = true
			break
		}
		// Spec-086 additive operator-docs accept set (Requirement 10):
		// any user-facing doc or the core USAGE manual also satisfies the lane.
		if strings.HasPrefix(f, ".mindspec/docs/user/") ||
			f == ".mindspec/docs/core/USAGE.md" {
			hasRelevantDoc = true
			break
		}
	}

	if !hasRelevantDoc {
		r.AddWarning("cmd-docs", "cmd/ changes without operator-docs update (one of CLAUDE.md, CONVENTIONS.md, .mindspec/docs/user/**, .mindspec/docs/core/USAGE.md)")
	}
}

// validateSpecArtifactSync enforces that any modification to a
// .mindspec/docs/specs/<id>/spec.md file is accompanied in the same
// diff by at least one supporting artifact: the sibling plan.md, any
// other file under .mindspec/docs/specs/<id>/, or any ADR file under
// .mindspec/docs/adr/**.md. A spec.md change made in isolation is
// rejected with the "spec-artifact-sync" lane error so the doctrine
// that "a spec change is never atomic" is enforced by the gate.
//
// NOTE on ADR-sibling matching (panel CONSENSUS Minor 9): any
// modification under .mindspec/docs/adr/**.md currently satisfies the
// sibling requirement. This is deliberately loose — spec edits in
// practice routinely add or cite ADRs as the load-bearing artifact,
// and the gate's purpose here is to prevent zero-companion spec.md
// commits, not to police ADR-citation graphs. A stricter "cited ADR"
// check is deferred to spec 087's ADR-divergence lane.
func validateSpecArtifactSync(r *Result, changes classifiedChanges) {
	// Collect spec IDs whose spec.md was touched in this diff.
	touched := make(map[string]bool)
	for _, f := range changes.All {
		if id := specMDID(f); id != "" {
			touched[id] = true
		}
	}
	if len(touched) == 0 {
		return
	}

	// Sort touched spec IDs for deterministic emit order (panel
	// CONSENSUS Major 6).
	ids := make([]string, 0, len(touched))
	for id := range touched {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		prefix := ".mindspec/docs/specs/" + id + "/"
		specMD := prefix + "spec.md"
		hasCompanion := false
		for _, f := range changes.All {
			if f == specMD {
				continue
			}
			if strings.HasPrefix(f, prefix) {
				hasCompanion = true
				break
			}
			if strings.HasPrefix(f, ".mindspec/docs/adr/") && strings.HasSuffix(f, ".md") {
				hasCompanion = true
				break
			}
		}
		if !hasCompanion {
			r.AddError("spec-artifact-sync", fmt.Sprintf(
				"spec %s/spec.md change requires plan.md, ADR (.mindspec/docs/adr/**.md), or sibling artifact (.mindspec/docs/specs/%s/**) update in same diff",
				id, id,
			))
		}
	}
}

// specMDID returns the spec ID iff path is .mindspec/docs/specs/<id>/spec.md.
// Returns "" otherwise.
func specMDID(path string) string {
	const prefix = ".mindspec/docs/specs/"
	const suffix = "/spec.md"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimSuffix(rest, suffix)
	// Reject nested paths — must be exactly one segment.
	if rest == "" || strings.Contains(rest, "/") {
		return ""
	}
	return rest
}
