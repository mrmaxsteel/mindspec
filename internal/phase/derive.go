// Package phase provides beads-based lifecycle phase derivation (ADR-0023).
package phase

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/shellquote"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// ErrNoEpicLineage is the typed sentinel FindEpicForBead(WithCache) wraps
// when every lookup SUCCEEDED but the bead genuinely has no discoverable
// epic/spec lineage (the bead does not exist, has no epic-typed parent
// dependency carrying spec metadata, and no title-number epic match).
// Callers that fail closed on lineage lookup ERRORS (spec 119 final-review
// finding A: a transient bd/Dolt failure must never silently demote
// resolution to cwd guessing) use errors.Is(err, ErrNoEpicLineage) to
// distinguish this genuinely-epic-less result — where a legacy fallback is
// legitimate — from a real infra failure, which propagates as-is. Never
// classify by error text.
var ErrNoEpicLineage = errors.New("no epic lineage")

// ErrMalformedSpecMetadata is the spec 120 D1 sentinel SpecIDFromMetadata
// returns when the composed spec ID fails idvalidate.SpecID (ADR-0042 §1):
// agent-writable epic spec_title/title metadata is only lowercased and
// hyphenated by slugify — it PRESERVES shell metacharacters and `..` — so
// the derived ID must validate before any caller treats it as a real spec
// ID. errors.Is-DISTINCT from ErrNoEpicLineage: a lineage lookup that
// SUCCEEDS with hostile/malformed metadata is not "no lineage" — it must
// never be silently demoted to cwd-derived fallback resolution (that would
// let the D1 preflight refusal, spec 120 AC-4, be bypassed).
var ErrMalformedSpecMetadata = errors.New("malformed spec metadata: derived spec id fails idvalidate.SpecID")

// Package-level function variables for testability.
var (
	runBDFn    = bead.RunBD
	listJSONFn = bead.ListJSON
)

// RunBDFunc is the function signature for bd command execution.
type RunBDFunc func(args ...string) ([]byte, error)

// SetRunBDForTest allows tests in other packages to stub the bd runner.
// Returns a restore function that should be called in t.Cleanup.
func SetRunBDForTest(fn RunBDFunc) func() {
	orig := runBDFn
	runBDFn = fn
	return func() { runBDFn = orig }
}

// SetListJSONForTest allows tests to stub the ListJSON runner.
func SetListJSONForTest(fn RunBDFunc) func() {
	orig := listJSONFn
	listJSONFn = fn
	return func() { listJSONFn = orig }
}

// ListJSONForTest returns the currently installed ListJSON stub. Test-only:
// it lets a helper in another package COMPOSE onto the existing stub —
// handling the `bd list` queries it cares about (e.g. `--parent` children)
// while delegating the rest (e.g. `--type=epic` epic resolution) back to the
// stub already installed. The complete children query and its spec→epic
// resolution now share this single seam (spec 107 wave 1).
func ListJSONForTest() RunBDFunc {
	return listJSONFn
}

// EpicInfo represents a beads epic with spec metadata.
type EpicInfo struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Status    string                 `json:"status"`
	IssueType string                 `json:"issue_type"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ChildInfo represents a bead child of an epic.
type ChildInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	IssueType string `json:"issue_type"`
	// Parent is the bd parent issue ID (the owning epic for lifecycle
	// beads). bd emits it in every list/show JSON shape; it is empty for
	// top-level issues. Consumed by Cache.OpenBeads callers (the lifecycle
	// aggregate scan) to attribute a globally-enumerated bead to its epic
	// without a per-epic children query.
	Parent string `json:"parent"`
}

// ActiveSpec holds discovered spec information derived from beads.
type ActiveSpec struct {
	SpecID  string `json:"spec_id"`
	EpicID  string `json:"epic_id"`
	Phase   string `json:"phase"`
	SpecNum int    `json:"spec_num"`
}

// Context holds the resolved context for a given working directory.
type Context struct {
	SpecID       string `json:"spec_id"`
	BeadID       string `json:"bead_id"`
	Phase        string `json:"phase"`
	WorktreePath string `json:"worktree_path"`
	EpicID       string `json:"epic_id"`
	// DerivationFailed is true when a GENUINE bd/Dolt lookup failure was
	// encountered while resolving Phase — never when a lookup simply and
	// successfully found nothing (a pre-epic draft, or a fresh worktree
	// with no active spec, both leave this false). Spec 128 R2.3 scope
	// extension: ResolveContextFromDirWithCache's per-case fallbacks
	// (e.g. "bead worktree implies implement") used to silently back-fill
	// Phase on error, making a real discovery failure indistinguishable
	// from a legitimately-derived value. Phase keeps its historical
	// fallback VALUE for callers that only read it (state show, the
	// bead-4 agreement-table fixtures) — this field is the new, additive
	// signal that lets an instruct-tail caller render the honest
	// indeterminate form instead of trusting that fallback.
	DerivationFailed bool `json:"derivation_failed,omitempty"`
	// DerivationError is the human-readable cause when DerivationFailed
	// is true.
	DerivationError string `json:"derivation_error,omitempty"`
}

// SpecIDFromMetadata constructs a spec ID from num and title, checked
// (spec 120 D1, ADR-0042 §1, OQ2 — (string, error), the SpecDir SEC-1
// precedent). The title is slugified (lowercased, spaces/underscores →
// hyphens) to match the original slug format used by spec-init (e.g.
// "Llm Test Coverage" → "llm-test-coverage"), but slugify only lowercases
// and hyphenates — it PRESERVES shell metacharacters and ".." verbatim
// from the agent-writable spec_title/title metadata. The composed ID must
// therefore pass idvalidate.SpecID before ANY caller treats it as a real
// ID; a failure returns ErrMalformedSpecMetadata (errors.Is-distinct from
// ErrNoEpicLineage — see its doc comment) wrapping the idvalidate error.
func SpecIDFromMetadata(specNum int, specTitle string) (string, error) {
	id := fmt.Sprintf("%03d-%s", specNum, slugify(specTitle))
	if err := idvalidate.SpecID(id); err != nil {
		return "", fmt.Errorf("%w: %v", ErrMalformedSpecMetadata, err)
	}
	return id, nil
}

// slugify converts a title to a URL-safe slug: lowercase, spaces/underscores → hyphens,
// collapse runs of hyphens, trim leading/trailing hyphens.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer(" ", "-", "_", "-").Replace(s)
	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

// DerivePhase determines the lifecycle phase from an epic's status and children statuses.
// Implements the phase derivation table from ADR-0023 §3. Constructs a fresh cache;
// hot-path callers should use DerivePhaseWithCache to share bd queries.
func DerivePhase(epicID string) (string, error) {
	return DerivePhaseWithCache(NewCache(), epicID)
}

// DerivePhaseWithCache is the cache-aware variant of DerivePhase.
func DerivePhaseWithCache(c *Cache, epicID string) (string, error) {
	return DerivePhaseWithStatusWithCache(c, epicID, "")
}

// DerivePhaseWithStatus determines the lifecycle phase, using a pre-fetched epic status
// if available (to avoid redundant queries). If epicStatus is empty, it is looked up.
//
// Spec 080: metadata-first approach. If the epic has a valid mindspec_phase in metadata,
// that is returned directly. Child-based derivation runs as a consistency check; if it
// disagrees, a warning is emitted to stderr but the stored phase is trusted.
func DerivePhaseWithStatus(epicID, epicStatus string) (string, error) {
	return DerivePhaseWithStatusWithCache(NewCache(), epicID, epicStatus)
}

// DerivePhaseWithStatusWithCache is the cache-aware variant of DerivePhaseWithStatus.
// All four bd-touching helpers (readStoredPhase, queryEpicStatus, hasDoneMarker,
// queryChildren) now route through the cache, so a warm path that already has
// the epic loaded incurs zero extra bd calls.
func DerivePhaseWithStatusWithCache(c *Cache, epicID, epicStatus string) (string, error) {
	// Spec 080: check metadata-stored phase first (O(1)).
	if storedPhase := readStoredPhaseWithCache(c, epicID); storedPhase != "" {
		// Run child-based derivation as consistency check.
		childPhase := deriveFromChildrenOrStatusWithCache(c, epicID, epicStatus)
		if childPhase != "" && childPhase != storedPhase {
			// Spec 128 Bead 2 (plan step 7/8): a marker-absent epic
			// whose stored/children match the pre-v2 legacy signature
			// (LegacyPhaseSignature, semantics_v2.go) gets a REPLACEMENT
			// notice instead of the general mismatch warning below —
			// its recovery (`mindspec repair phase`) is made correct by
			// EnsurePhaseSemanticsV2 also running at repair phase's own
			// entry (repair.go), which reconciles legacy -> plan + marker
			// BEFORE the guard evaluates. This branch never takes the
			// R3.3 write lock (LegacyPhaseSignature and the marker read
			// below are both lock-free, round-2 panel G4).
			if !hasV2MarkerWithCache(c, epicID) {
				if children, cerr := c.GetChildren(epicID); cerr == nil && LegacyPhaseSignature(storedPhase, children) {
					fmt.Fprintf(os.Stderr, "warning: epic %s: legacy phase semantics detected (pre-v2 stored implement)\nrecovery: mindspec repair phase %s\n",
						epicID, shellquote.Quote(specIDForEpicWithCache(c, epicID)))
					return storedPhase, nil
				}
			}
			// Spec 092 Req 2 / spec 128 AC-5(iv): the consistency warning
			// carries the recovery-line convention (ADR-0035) so agents
			// can grep and paste the fix. Direction-conditional (bead 2,
			// round-1 G5 disposition): a FORWARD disagreement (the
			// child-derived phase is AHEAD of stored in lifecycle order)
			// keeps the existing `mindspec repair phase <spec-id>`
			// recovery — safe and correct there. A BACKWARD disagreement
			// (derived is BEHIND stored) never emits that command —
			// writing it would move the epic backward — and instead ends
			// with an inspection action (`bd show <epic-id>`), belt (this
			// line) alongside suspenders (the R3.3 guard itself refuses
			// an unforced backward write regardless). Never a raw `bd
			// update` metadata command, which has replace semantics over
			// the entire metadata map (Req 19/HC-5). Hand-formatted here
			// because internal/guard imports this package (import
			// cycle); the external-package test derive_recovery_test.go
			// keeps the format aligned with guard.HasFinalRecoveryLine.
			if IsBackwardMove(storedPhase, childPhase) {
				fmt.Fprintf(os.Stderr, "warning: epic %s: stored phase %q disagrees with child-derived phase %q (trusting stored phase) — the derived phase is BEHIND stored in lifecycle order; writing it would move the epic backward\nrecovery: bd show %s\n",
					idrender.Bead(epicID), storedPhase, childPhase, shellquote.Quote(idrender.Bead(epicID)))
			} else {
				fmt.Fprintf(os.Stderr, "warning: epic %s: stored phase %q disagrees with child-derived phase %q (trusting stored phase)\nrecovery: mindspec repair phase %s\n",
					epicID, storedPhase, childPhase, shellquote.Quote(specIDForEpicWithCache(c, epicID)))
			}
		}
		return storedPhase, nil
	}

	// Fallback for pre-080 epics: derive from children/status (backward compat).
	//
	// Final-review round 1 (R6 F2, the AC-4b class): a failed child read
	// PROPAGATES here instead of feeding DerivePhaseFromChildren a nil set
	// — which would RETURN a fabricated `plan` with a nil error, the exact
	// error-discarding derivation this spec exists to kill (spec.md: "a
	// lifecycle-discovery failure is never serialized as a successful
	// empty or idle mode"). Callers already map a non-nil error to the
	// indeterminate machinery (Context.DerivationFailed / state show's
	// discovery refusal).
	if epicStatus == "" {
		epicStatus = queryEpicStatusWithCache(c, epicID)
	}
	if strings.EqualFold(epicStatus, "closed") {
		if hasDoneMarkerWithCache(c, epicID) {
			return state.ModeDone, nil
		}
		children, cerr := c.GetChildren(epicID)
		if cerr != nil {
			return "", fmt.Errorf("deriving epic %s's phase from its children: %w", idrender.Bead(epicID), cerr)
		}
		return DerivePhaseFromChildren(children), nil
	}
	children, cerr := c.GetChildren(epicID)
	if cerr != nil {
		return "", fmt.Errorf("deriving epic %s's phase from its children: %w", idrender.Bead(epicID), cerr)
	}
	return DerivePhaseFromChildren(children), nil
}

// PhaseDetail exposes the stored (metadata-cached) phase alongside the
// child-derived phase for one epic. Spec 092 Req 1: child bead statuses
// are the ground truth of the lifecycle phase (ADR-0023 §3/§5);
// `mindspec_phase` is a trusted CACHE of that derivation (ADR-0034
// amendment). Gate callers use this to evaluate gates on the derived
// value read-only and defer the forward reconcile write;
// `mindspec repair phase` uses it to re-derive before its merge-write.
type PhaseDetail struct {
	EpicID  string
	Stored  string // mindspec_phase metadata value; "" when absent or invalid
	Derived string // phase derived from children/epic status; "" when derivation fails
}

// DerivePhaseDetail returns the stored vs child-derived phase for an
// epic so callers do not duplicate the cache plumbing. Constructs a
// fresh cache; hot-path callers should use DerivePhaseDetailWithCache.
func DerivePhaseDetail(epicID string) (PhaseDetail, error) {
	return DerivePhaseDetailWithCache(NewCache(), epicID)
}

// DerivePhaseDetailWithCache is the cache-aware variant of
// DerivePhaseDetail. The derivation is deterministic and read-only —
// it never writes metadata (the reconcile write is the caller's
// decision, sequenced per spec 092 Req 1).
func DerivePhaseDetailWithCache(c *Cache, epicID string) (PhaseDetail, error) {
	return PhaseDetail{
		EpicID:  epicID,
		Stored:  readStoredPhaseWithCache(c, epicID),
		Derived: deriveFromChildrenOrStatusWithCache(c, epicID, ""),
	}, nil
}

// specIDForEpicWithCache resolves the spec ID for an epic so emitted
// recovery commands are copy-pastable. Falls back to the "<spec-id>"
// placeholder when the epic carries no resolvable spec metadata.
func specIDForEpicWithCache(c *Cache, epicID string) string {
	epic, err := c.FindEpic(epicID)
	if err == nil && epic != nil {
		if num, title := ExtractSpecMetadata(*epic); num > 0 && title != "" {
			// D1 (ADR-0042): a malformed derivation falls back to the
			// existing "<spec-id>" placeholder, same as no metadata at
			// all — this is a recovery-line rendering helper, never a
			// gate.
			if specID, err := SpecIDFromMetadata(num, title); err == nil {
				return specID
			}
		}
	}
	return "<spec-id>"
}

// readStoredPhaseWithCache reads the mindspec_phase metadata field from an epic.
// Returns "" if the field is missing, empty, or not a valid mode.
func readStoredPhaseWithCache(c *Cache, epicID string) string {
	epic, err := c.FindEpic(epicID)
	if err != nil || epic == nil || epic.Metadata == nil {
		return ""
	}
	raw, ok := epic.Metadata["mindspec_phase"]
	if !ok {
		return ""
	}
	phase, ok := raw.(string)
	if !ok || !state.IsValidPhase(phase) {
		return ""
	}
	return phase
}

// deriveFromChildrenOrStatusWithCache runs child-based phase derivation for consistency checking.
// Returns the derived phase, or "" if derivation fails.
//
// Final-review round 1 (R6 F2): a failed child read returns "" — the
// documented derivation-failed sentinel, which makes the AC-5(iv)
// consistency check SKIP (DerivePhaseWithStatusWithCache's `childPhase
// != ""` guard) — never a phase fabricated from the nil child set a
// failed `bd list --parent` yields. Pre-fix, a partial bd outage made
// the consistency warning assert `child-derived phase "plan"` for a
// derivation that never ran (reproduced live by the final panel's R6).
func deriveFromChildrenOrStatusWithCache(c *Cache, epicID, epicStatus string) string {
	if epicStatus == "" {
		epicStatus = queryEpicStatusWithCache(c, epicID)
	}
	if strings.EqualFold(epicStatus, "closed") {
		if hasDoneMarkerWithCache(c, epicID) {
			return state.ModeDone
		}
		children, cerr := c.GetChildren(epicID)
		if cerr != nil {
			return ""
		}
		return DerivePhaseFromChildren(children)
	}
	children, cerr := c.GetChildren(epicID)
	if cerr != nil {
		return ""
	}
	return DerivePhaseFromChildren(children)
}

// childClass partitions a child of a spec epic for phase derivation.
type childClass int

const (
	// childLifecycle is a lifecycle implementation bead — created with
	// `--type task` by `plan approve` (internal/approve/plan.go:341), or an
	// untyped child defaulted to lifecycle (the conservative choice: an
	// unknown child keeps its blocking weight, so a real lifecycle bead is
	// never mistaken for a follow-up).
	childLifecycle childClass = iota
	// childNonLifecycle is a follow-up filed against the spec epic — a `bug`
	// or any other explicit non-`task`, non-`epic` type. It never blocks
	// `review` or forces `implement` (spec 095 / ry73).
	childNonLifecycle
	// childEpic is a structural epic child — neither a lifecycle bead nor a
	// follow-up; ignored entirely by the derivation.
	childEpic
)

// classifyChild partitions a child by its issue_type for phase derivation
// (spec 095 / ry73). Only epics use a non-task lifecycle type, so
// `task` (and an empty/unknown type) is treated as a lifecycle bead while
// any other explicit type (e.g. `bug`) is a non-lifecycle follow-up. See
// ADR-0023 §3 for the refined rule and the plan.go:341 `--type task`
// invariant it relies on.
func classifyChild(issueType string) childClass {
	switch strings.ToLower(strings.TrimSpace(issueType)) {
	case "epic":
		return childEpic
	case "", "task":
		return childLifecycle
	default:
		return childNonLifecycle
	}
}

// IsLifecycleIssueType reports whether issueType classifies as a
// LIFECYCLE child — task-typed or untyped (empty), the ADR-0023
// discriminator classifyChild already encodes for phase derivation.
// Exported for cmd/mindspec/next.go's claim-path phase advance (spec 128
// Bead 2, OQ-3 option 1): claiming a non-lifecycle bead (e.g. a `bug`)
// must never advance stored mindspec_phase.
func IsLifecycleIssueType(issueType string) bool {
	return classifyChild(issueType) == childLifecycle
}

// DerivePhaseFromChildren implements the pure logic of phase derivation.
// Exported for direct testing without beads dependency.
//
// Spec 095 (mindspec-ry73): `review` is derived over LIFECYCLE children
// ONLY. A non-lifecycle follow-up child (e.g. a `bug` filed as a child of
// the spec epic post-implementation) must not strand the spec short of
// `review`, nor force it back to `implement`. Critically, an EMPTY
// lifecycle set (no children at all, OR only non-lifecycle / epic
// children) falls back to `plan`, NEVER `review` — this closes the
// vacuous-review hazard where a bug filed as an epic child during plan
// mode would otherwise force `review` on an unimplemented spec.
func DerivePhaseFromChildren(children []ChildInfo) string {
	var lifecycleClosed, lifecycleInProgress, lifecycleTotal int
	for _, c := range children {
		switch classifyChild(c.IssueType) {
		case childEpic, childNonLifecycle:
			// Structural epics and non-lifecycle follow-ups are excluded
			// from the review/implement/plan computation entirely.
			continue
		}
		lifecycleTotal++
		switch strings.ToLower(strings.TrimSpace(c.Status)) {
		case "closed":
			lifecycleClosed++
		case "in_progress":
			lifecycleInProgress++
		}
	}

	// Empty lifecycle set → plan (spec approved / plan being drafted, or only
	// follow-up children exist). NEVER derive `review` here — that would be a
	// vacuous review over zero implementation work.
	if lifecycleTotal == 0 {
		return state.ModePlan
	}

	// Any lifecycle child in_progress → implement
	if lifecycleInProgress > 0 {
		return state.ModeImplement
	}

	// All lifecycle children closed → review
	if lifecycleClosed == lifecycleTotal {
		return state.ModeReview
	}

	// Some closed + some open lifecycle, none in_progress → implement (between
	// beads). Closed beads prove implementation has started; the agent is
	// between completing one bead and claiming the next.
	if lifecycleClosed > 0 {
		return state.ModeImplement
	}

	// All lifecycle children open (none claimed) → plan
	return state.ModePlan
}

// OpenNonLifecycleChildren returns the children that are both open (not
// closed) AND non-lifecycle — follow-up `bug`/other children that the
// lifecycle-only `review` derivation deliberately ignores. Spec 095:
// `impl approve` uses this to name the offending child(ren) in its
// advisory guard hint so the operator can re-file or detach if they
// disagree (ADR-0035 recovery-line convention).
func OpenNonLifecycleChildren(children []ChildInfo) []ChildInfo {
	var out []ChildInfo
	for _, c := range children {
		if classifyChild(c.IssueType) != childNonLifecycle {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(c.Status), "closed") {
			continue
		}
		out = append(out, c)
	}
	return out
}

// OpenNonLifecycleChildrenForEpic resolves the open non-lifecycle children
// of an epic via the shared cache (one `bd list --parent` call), for the
// `impl approve` guard hint. A query failure yields nil — the hint is
// advisory and must never block.
func OpenNonLifecycleChildrenForEpic(epicID string) []ChildInfo {
	children, err := NewCache().GetChildren(epicID)
	if err != nil {
		return nil
	}
	return OpenNonLifecycleChildren(children)
}

// LifecycleChildIDsForEpic resolves the LIFECYCLE (task / empty-type)
// children of an epic via the shared cache (one `bd list --parent` call,
// same query as OpenNonLifecycleChildrenForEpic above) and returns their
// bead IDs.
//
// Spec 119 Bead 3 (P3): a FAIL-CLOSED sibling of the advisory
// OpenNonLifecycleChildren(ForEpic) — those swallow a query failure to nil
// because they feed a never-blocking hint. This function feeds
// FinalizeEpic's destructive bead-branch enumerations (the scoping
// allow-set, R6), so a bd query failure must be returned as an error
// rather than silently read as "no lifecycle children" (which would
// under-scope the allow-set and either strand real lifecycle beads or, if
// misread as "everything is out of scope", merge nothing). bd-only, no
// git I/O — TestEnforcementHasNoGitLeaks (ADR-0030) is unaffected.
func LifecycleChildIDsForEpic(epicID string) ([]string, error) {
	children, err := NewCache().GetChildren(epicID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, c := range children {
		if classifyChild(c.IssueType) == childLifecycle {
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

// DiscoverActiveSpecs queries beads for open/in_progress epics and derives phase for each.
// Constructs a fresh cache; hot-path callers should use DiscoverActiveSpecsWithCache
// to share the underlying `bd list --type=epic` call with other parts of the same invocation.
func DiscoverActiveSpecs() ([]ActiveSpec, error) {
	return DiscoverActiveSpecsWithCache(NewCache())
}

// DiscoverActiveSpecsWithCache is the cache-aware variant of DiscoverActiveSpecs.
// Filters AllEpics down to open + in_progress in-process (no additional bd call).
func DiscoverActiveSpecsWithCache(c *Cache) ([]ActiveSpec, error) {
	epics, err := c.ActiveEpics()
	if err != nil {
		return nil, err
	}

	var active []ActiveSpec
	for _, epic := range epics {

		specNum, specTitle := ExtractSpecMetadata(epic)
		if specNum == 0 && specTitle == "" {
			continue // no spec metadata, skip
		}

		specID, err := SpecIDFromMetadata(specNum, specTitle)
		if err != nil {
			// D1 ambient-enumeration degrade (ADR-0042, AC-9): a hostile
			// spec_title/title must never brick this best-effort scan for
			// every OTHER valid spec — skip this one epic with a single
			// escaped warning naming the convergent repair lever.
			epicRef := epic.ID
			if idvalidate.BeadID(epicRef) != nil {
				epicRef = "<epic-id>"
			}
			fmt.Fprintf(os.Stderr, "warning: epic %s has malformed spec metadata (%s); skipping this spec — run `mindspec repair spec-title %s \"<corrected-title>\"` to fix\n",
				termsafe.Escape(epic.ID), termsafe.Escape(err.Error()), epicRef)
			continue
		}

		// Spec 080: check metadata-stored phase first.
		if storedPhase := extractPhaseFromMetadata(epic); storedPhase != "" {
			if storedPhase == state.ModeDone {
				continue // spec lifecycle complete
			}
			active = append(active, ActiveSpec{
				SpecID:  specID,
				EpicID:  epic.ID,
				Phase:   storedPhase,
				SpecNum: specNum,
			})
			continue
		}

		// Fallback for pre-080 epics: derive from children.
		//
		// Final-review round 1 (R6 F2): a failed child read PROPAGATES —
		// skipping just this epic could serialize a false `idle` when it
		// is the only active spec, and deriving from the nil set would
		// fabricate `plan`; neither is an honest answer to a discovery
		// failure. (The D1 degrade above is for hostile per-epic DATA;
		// this is an infrastructure read failure.)
		children, cerr := c.GetChildren(epic.ID)
		if cerr != nil {
			return nil, fmt.Errorf("deriving epic %s's phase from its children: %w", idrender.Bead(epic.ID), cerr)
		}

		phase := DerivePhaseFromChildren(children)
		if phase == state.ModeDone {
			continue
		}

		active = append(active, ActiveSpec{
			SpecID:  specID,
			EpicID:  epic.ID,
			Phase:   phase,
			SpecNum: specNum,
		})
	}

	return active, nil
}

// ResolveContext combines worktree path conventions with beads queries
// to determine the current spec, bead, phase, and worktree path.
func ResolveContext(root string) (*Context, error) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		cwd = "."
	}
	return ResolveContextFromDir(root, cwd)
}

// ResolveContextWithCache is the cache-aware variant of ResolveContext.
func ResolveContextWithCache(c *Cache, root string) (*Context, error) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		cwd = "."
	}
	return ResolveContextFromDirWithCache(c, root, cwd)
}

// ResolveContextFromDir resolves context from a specific directory.
func ResolveContextFromDir(root, dir string) (*Context, error) {
	return ResolveContextFromDirWithCache(NewCache(), root, dir)
}

// ResolveContextFromDirWithCache is the cache-aware variant of ResolveContextFromDir.
func ResolveContextFromDirWithCache(c *Cache, root, dir string) (*Context, error) {
	kind, specID, beadID := workspace.DetectWorktreeContext(dir)

	ctx := &Context{
		WorktreePath: dir,
	}

	switch kind {
	case workspace.WorktreeBead:
		ctx.BeadID = beadID
		epicID, derivedSpecID, err := FindEpicForBeadWithCache(c, beadID)
		if err == nil && derivedSpecID != "" {
			ctx.SpecID = derivedSpecID
			ctx.EpicID = epicID
		} else if err != nil && !errors.Is(err, ErrNoEpicLineage) {
			// A genuine lookup failure (bd/Dolt), not "this bead
			// honestly has no epic lineage" — record it so the tail
			// can render indeterminate instead of trusting the
			// ModeImplement fallback below as if it were derived.
			ctx.DerivationFailed = true
			ctx.DerivationError = err.Error()
		}
		phase, phaseErr := derivePhaseForSpecWithCache(c, ctx.EpicID)
		if phaseErr == nil {
			ctx.Phase = phase
		} else {
			ctx.Phase = state.ModeImplement // bead worktree implies implement
			ctx.DerivationFailed = true
			ctx.DerivationError = phaseErr.Error()
		}

	case workspace.WorktreeSpec:
		ctx.SpecID = specID
		epicID, err := FindEpicBySpecIDWithCache(c, specID)
		if err == nil && epicID != "" {
			ctx.EpicID = epicID
			phase, phaseErr := DerivePhaseWithCache(c, epicID)
			if phaseErr == nil {
				ctx.Phase = phase
			} else {
				ctx.DerivationFailed = true
				ctx.DerivationError = phaseErr.Error()
			}
		} else if err != nil && !BeadsDirMissing(root) {
			// Distinguish "bd healthy, no epic yet" (a pre-epic draft —
			// ModeSpec below is the honest, non-error answer) from a
			// genuine bd/Dolt failure, the same probe
			// resolve.ResolveModeWithCache applies to its own epic
			// lookup (round-1 panel G2/F2a precedent). BeadsDirMissing
			// short-circuits first: a workspace with no .beads/ directory
			// at all (bead.Preflight's own "run 'beads init'" precondition,
			// checked the same on-disk way, never by bd's error TEXT) has
			// never used the beads lifecycle — that is honestly ModeSpec
			// below, not a health failure worth an extra bd probe.
			if _, epicsErr := c.AllEpics(); epicsErr != nil {
				ctx.DerivationFailed = true
				ctx.DerivationError = epicsErr.Error()
			}
		}
		// Round-1 panel G1/G3 (hard block): this backfill is UNCONDITIONAL,
		// matching WorktreeBead's ModeImplement fallback above — it fires
		// even when ctx.DerivationFailed is true. Honest consumers
		// (cmd/mindspec/instruct_tail.go, internal/instruct/run.go) check
		// DerivationFailed BEFORE ever reading Phase, so this does not
		// resurrect the "failure presented as idle" bug the field exists
		// to close. Gating this on !DerivationFailed left Phase=="" on a
		// genuine failure, which silently flipped internal/cleanup's
		// active-spec refusal (cleanup.go: `ctx.Phase != ""`) from
		// fail-closed to fail-open for a bd outage in a spec worktree —
		// exactly the "additive, values unchanged" claim this field's own
		// doc comment makes, now actually true for every arm.
		if ctx.Phase == "" {
			ctx.Phase = state.ModeSpec // spec worktree without epic → spec mode
		}
		// Check for active bead
		if ctx.EpicID != "" {
			if activeBead := FindActiveBeadForEpicWithCache(c, ctx.EpicID); activeBead != "" {
				ctx.BeadID = activeBead
			}
		}

	case workspace.WorktreeMain:
		specs, err := DiscoverActiveSpecsWithCache(c)
		switch {
		case err == nil && len(specs) == 1:
			ctx.SpecID = specs[0].SpecID
			ctx.EpicID = specs[0].EpicID
			ctx.Phase = specs[0].Phase
		case err == nil && len(specs) == 0:
			ctx.Phase = state.ModeIdle
		case err != nil && BeadsDirMissing(root):
			// No .beads/ directory at all: this workspace has never used
			// the beads lifecycle (bead.Preflight's own "run 'beads
			// init'" precondition, re-checked here the same on-disk way —
			// never by matching bd's error TEXT). That is honestly idle,
			// not a bd/Dolt health failure.
			ctx.Phase = state.ModeIdle
		case err != nil:
			ctx.DerivationFailed = true
			ctx.DerivationError = err.Error()
		}
		// Multiple specs: leave specID empty (caller handles ambiguity)
	}

	return ctx, nil
}

// BeadsDirMissing reports whether root has no .beads/ directory at all —
// the same on-disk precondition bead.Preflight checks before ever
// shelling out to bd (internal/bead/bdcli.go's "run 'beads init'" case).
// Spec 128 R2.3: a workspace that has never used the beads lifecycle
// makes every bd query fail (real bd: "Error: no beads database found"),
// which is NOT a health failure worth reporting as indeterminate — it is
// the ordinary, permanently-legitimate "nothing has started here yet"
// idle state. This is checked via the filesystem, never by matching bd's
// error TEXT (the same discipline ErrNoEpicLineage's doc comment states:
// "never classify by error text"). Exported (round-1 panel G5.1) so
// internal/instruct's own idle-vs-indeterminate arrival points apply the
// identical precondition rather than a second, potentially-drifting copy.
func BeadsDirMissing(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".beads"))
	return os.IsNotExist(err)
}

// CheckSpecNumberCollision checks if a spec number is already in use.
// It pulls from Dolt remote first to ensure freshness.
func CheckSpecNumberCollision(specNum int) error {
	// Pull latest from Dolt
	_, _ = runBDFn("dolt", "pull")

	epics, err := fetchAllEpics()
	if err != nil {
		return fmt.Errorf("querying epics: %w", err)
	}

	for _, epic := range epics {
		num, _ := ExtractSpecMetadata(epic)
		if num == specNum {
			// epic.ID/epic.Title are bd-sourced and agent-writable (an
			// epic's title can be edited via `bd update`); this error
			// reaches the terminal raw via cobra's default unhandled-
			// error printer, so escape both before interpolation
			// (spec 120 R4).
			return fmt.Errorf("spec number %03d is already in use by epic %s (%s)", specNum, idrender.Bead(epic.ID), termsafe.Escape(epic.Title))
		}
	}

	return nil
}

// FindEpicBySpecID finds the epic ID for a given spec ID by querying metadata.
// Constructs a fresh cache; hot-path callers should use FindEpicBySpecIDWithCache.
func FindEpicBySpecID(specID string) (string, error) {
	return FindEpicBySpecIDWithCache(NewCache(), specID)
}

// FindEpicBySpecIDWithCache is the cache-aware variant of FindEpicBySpecID.
//
// Semantic-lookup boundary gate (ADR-0042 §6 inertness theorem, round 5):
// this is pure in-process string equality against SpecIDFromMetadata — an
// invalid specID can only fail to match, so this one-line validation is
// posture (closing re-litigation of the class), not a live-bug fix. Invalid
// → the existing not-found error, byte-identical in shape to a genuine
// no-match.
func FindEpicBySpecIDWithCache(c *Cache, specID string) (string, error) {
	if err := idvalidate.SpecID(specID); err != nil {
		return "", fmt.Errorf("no epic found for spec %s", idrender.Spec(specID))
	}
	return c.FindEpicBySpecID(specID)
}

// --- Internal helpers ---

// hasDoneMarkerWithCache checks if an epic has been explicitly finalized.
// Checks both mindspec_phase: done (Spec 080) and legacy mindspec_done: true
// for backward compatibility. Routes through c.FindEpic so the same bd show
// call is shared with readStoredPhase / queryEpicStatus in a warm path.
func hasDoneMarkerWithCache(c *Cache, epicID string) bool {
	epic, err := c.FindEpic(epicID)
	if err != nil || epic == nil || epic.Metadata == nil {
		return false
	}
	// Spec 080: check mindspec_phase: done
	if phase, ok := epic.Metadata["mindspec_phase"]; ok {
		if s, ok := phase.(string); ok && s == state.ModeDone {
			return true
		}
	}
	// Legacy: check mindspec_done: true
	done, ok := epic.Metadata["mindspec_done"]
	if !ok {
		return false
	}
	b, ok := done.(bool)
	return ok && b
}

// hasDoneMarker is the cache-free wrapper retained for backward compatibility
// and the existing test suite (which invokes it directly via SetRunBDForTest stubs).
func hasDoneMarker(epicID string) bool {
	return hasDoneMarkerWithCache(nil, epicID)
}

// extractPhaseFromMetadata reads mindspec_phase from an already-loaded epic's metadata.
// Returns "" if the field is missing or invalid.
func extractPhaseFromMetadata(epic EpicInfo) string {
	if epic.Metadata == nil {
		return ""
	}
	raw, ok := epic.Metadata["mindspec_phase"]
	if !ok {
		return ""
	}
	phase, ok := raw.(string)
	if !ok || !state.IsValidPhase(phase) {
		return ""
	}
	return phase
}

// (Per-status fan-out helpers queryActiveEpics / queryEpics / queryChildren
// were collapsed into Cache.AllEpics / Cache.ActiveEpics / Cache.GetChildren,
// backed by single `bd list … --status=open,in_progress,closed -n 0` calls.
// See internal/phase/cache.go.)

// ExtractSpecMetadata gets spec_num and spec_title from epic metadata or title.
func ExtractSpecMetadata(epic EpicInfo) (int, string) {
	if epic.Metadata != nil {
		numRaw, hasNum := epic.Metadata["spec_num"]
		titleRaw, hasTitle := epic.Metadata["spec_title"]
		if hasNum && hasTitle {
			var num int
			switch v := numRaw.(type) {
			case float64:
				num = int(v)
			case int:
				num = v
			}
			title, _ := titleRaw.(string)
			if num > 0 && title != "" {
				return num, title
			}
		}
	}

	// Fallback: parse from title convention [SPEC NNN-slug]
	return ParseSpecFromTitle(epic.Title)
}

// ParseSpecFromTitle extracts spec_num and spec_title from "[SPEC NNN-slug] ..." title format.
func ParseSpecFromTitle(title string) (int, string) {
	start := strings.Index(title, "[SPEC ")
	if start < 0 {
		return 0, ""
	}
	end := strings.Index(title[start:], "]")
	if end < 0 {
		return 0, ""
	}

	specPart := title[start+6 : start+end] // after "[SPEC " and before "]"
	dashIdx := strings.Index(specPart, "-")
	if dashIdx < 0 {
		return 0, ""
	}

	numStr := specPart[:dashIdx]
	slug := specPart[dashIdx+1:]

	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, ""
	}

	return num, slug
}

// FindEpicForBead looks up the parent epic for a bead and returns the epic ID
// and derived spec ID. Used by complete to resolve the spec from just a bead ID.
// Constructs a fresh cache; hot-path callers should use FindEpicForBeadWithCache.
func FindEpicForBead(beadID string) (epicID, specID string, err error) {
	return FindEpicForBeadWithCache(NewCache(), beadID)
}

// FindEpicForBeadWithCache is the cache-aware variant of FindEpicForBead.
// The `bd show <beadID>` call is not memoized in Cache (bead IDs are not
// tracked); the resolved parent epic, however, is fetched via cache.FindEpic
// so a subsequent bd show on the same epic ID is a no-op. The fallback
// epic-list path uses cache.AllEpics so it shares with the wider invocation.
//
// Error contract (spec 119 final-review finding A): a genuinely-no-lineage
// result — every lookup succeeded but no epic/spec lineage exists for this
// bead — is returned as an error wrapping ErrNoEpicLineage. Any REAL lookup
// failure (the `bd show` itself, its JSON decode, a parent-epic `bd show`
// via cache.FindEpic, or the title-fallback cache.AllEpics list) PROPAGATES
// as-is, never silently degraded into "no lineage": pre-final-review the
// dependency/title fallbacks swallowed those inner errors and fell through
// to the terminal not-found return, which let a transient bd/Dolt failure
// masquerade as an epic-less bead.
func FindEpicForBeadWithCache(c *Cache, beadID string) (epicID, specID string, err error) {
	// Gate-all-ids (ADR-0042 §1, round 9): beadID feeds a `bd show` argv
	// build directly — validated here, doubly covered with the R3
	// explicit-ingress gate at complete.Run's beadID argument (no id
	// operand is trusted by provenance, even one already gated upstream).
	if err := idvalidate.BeadID(beadID); err != nil {
		return "", "", fmt.Errorf("invalid bead id %s: %w", idrender.Bead(beadID), err)
	}
	out, err := runBDFn("show", beadID, "--json")
	if err != nil {
		return "", "", fmt.Errorf("bd show %s failed: %w", beadID, err)
	}

	var items []struct {
		Title        string `json:"title"`
		Dependencies []struct {
			ID        string `json:"id"`
			IssueType string `json:"issue_type"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		// The show succeeded and definitively answered "no such bead" —
		// a genuine no-lineage result, not an infra failure.
		return "", "", fmt.Errorf("bead %s not found: %w", idrender.Bead(beadID), ErrNoEpicLineage)
	}

	// Try to find the parent epic via dependencies
	for _, dep := range items[0].Dependencies {
		if strings.EqualFold(dep.IssueType, "epic") {
			epic, epicErr := c.FindEpic(dep.ID)
			if epicErr != nil {
				// A real epic-lookup failure must propagate — skipping it
				// (the pre-final-review behavior) silently reclassified a
				// transient bd/Dolt error as "no lineage".
				return "", "", fmt.Errorf("resolving parent epic %s for bead %s: %w", idrender.Bead(dep.ID), idrender.Bead(beadID), epicErr)
			}
			if epic != nil {
				num, title := ExtractSpecMetadata(*epic)
				if num > 0 && title != "" {
					// D1 (ADR-0042, AC-4): a malformed derivation
					// PROPAGATES as an ErrMalformedSpecMetadata-wrapping
					// error — errors.Is-distinct from ErrNoEpicLineage —
					// so complete.Run's preflight refuses before any
					// mutation instead of silently falling back to
					// cwd-derived resolution.
					specID, specErr := SpecIDFromMetadata(num, title)
					if specErr != nil {
						return dep.ID, "", specErr
					}
					return dep.ID, specID, nil
				}
			}
		}
	}

	// Fallback: parse spec num from bead title [NNN] and find matching epic
	title := items[0].Title
	if idx := strings.Index(title, "["); idx >= 0 {
		endIdx := strings.Index(title[idx:], "]")
		if endIdx > 0 {
			numStr := title[idx+1 : idx+endIdx]
			var num int
			if _, scanErr := fmt.Sscanf(numStr, "%d", &num); scanErr == nil {
				epics, listErr := c.AllEpics()
				if listErr != nil {
					// Same rule as the dependency path: a failed epic list
					// is an infra error, not evidence of an epic-less bead.
					return "", "", fmt.Errorf("listing epics for bead %s title fallback: %w", beadID, listErr)
				}
				for _, epic := range epics {
					epicNum, epicTitle := ExtractSpecMetadata(epic)
					if epicNum == num {
						// D1 (ADR-0042, AC-4): same propagation rule as
						// the dependency-path leg above.
						specID, specErr := SpecIDFromMetadata(epicNum, epicTitle)
						if specErr != nil {
							return epic.ID, "", specErr
						}
						return epic.ID, specID, nil
					}
				}
			}
		}
	}

	return "", "", fmt.Errorf("no epic found for bead %s: %w", beadID, ErrNoEpicLineage)
}

// FindActiveBeadForEpicWithCache returns the ID of an in_progress bead under the
// given epic, or "". Returns "" if zero or multiple beads are in_progress
// (ambiguous — caller should fall back to the spec worktree).
// Routes through cache.GetChildren (single all-status `bd list --parent` call) and
// filters to in_progress in-process, so it shares its bd call with DerivePhase.
func FindActiveBeadForEpicWithCache(c *Cache, epicID string) string {
	kids, err := c.GetChildren(epicID)
	if err != nil {
		return ""
	}

	// Filter out epics and non-in_progress entries.
	var beads []ChildInfo
	for _, item := range kids {
		if strings.EqualFold(item.IssueType, "epic") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.Status), "in_progress") {
			continue
		}
		beads = append(beads, item)
	}

	// Only return a bead when exactly one is in_progress.
	// Multiple in_progress beads (parallel agents) → ambiguous, return "".
	if len(beads) == 1 {
		// R3 ambient ingress gate (ADR-0042, spec 120 AC-6): the in_progress
		// child's ID is bd-sourced and therefore agent-writable — a
		// malformed candidate is not selected (existing empty-result
		// semantics), never returned as ctx.BeadID.
		if idvalidate.BeadID(beads[0].ID) == nil {
			return beads[0].ID
		}
	}
	return ""
}

// queryEpicStatusWithCache fetches the status of a single epic by ID via the cache.
func queryEpicStatusWithCache(c *Cache, epicID string) string {
	epic, err := c.FindEpic(epicID)
	if err != nil || epic == nil {
		return ""
	}
	return epic.Status
}

func derivePhaseForSpecWithCache(c *Cache, epicID string) (string, error) {
	if epicID == "" {
		return state.ModeSpec, nil
	}
	return DerivePhaseWithCache(c, epicID)
}
