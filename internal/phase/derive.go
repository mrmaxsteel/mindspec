// Package phase provides beads-based lifecycle phase derivation (ADR-0023).
package phase

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

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
}

// SpecIDFromMetadata constructs a spec ID from num and title.
// The title is slugified (lowercased, spaces/underscores → hyphens) to match
// the original slug format used by spec-init (e.g. "Llm Test Coverage" → "llm-test-coverage").
func SpecIDFromMetadata(specNum int, specTitle string) string {
	return fmt.Sprintf("%03d-%s", specNum, slugify(specTitle))
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
			fmt.Fprintf(os.Stderr, "warning: epic %s: stored phase %q disagrees with child-derived phase %q (trusting stored phase)\n",
				epicID, storedPhase, childPhase)
		}
		return storedPhase, nil
	}

	// Fallback for pre-080 epics: derive from children/status (backward compat).
	if epicStatus == "" {
		epicStatus = queryEpicStatusWithCache(c, epicID)
	}
	if strings.EqualFold(epicStatus, "closed") {
		if hasDoneMarkerWithCache(c, epicID) {
			return state.ModeDone, nil
		}
		children, _ := c.GetChildren(epicID)
		return DerivePhaseFromChildren(children), nil
	}
	children, _ := c.GetChildren(epicID)
	return DerivePhaseFromChildren(children), nil
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
func deriveFromChildrenOrStatusWithCache(c *Cache, epicID, epicStatus string) string {
	if epicStatus == "" {
		epicStatus = queryEpicStatusWithCache(c, epicID)
	}
	if strings.EqualFold(epicStatus, "closed") {
		if hasDoneMarkerWithCache(c, epicID) {
			return state.ModeDone
		}
		children, _ := c.GetChildren(epicID)
		return DerivePhaseFromChildren(children)
	}
	children, _ := c.GetChildren(epicID)
	return DerivePhaseFromChildren(children)
}

// DerivePhaseFromChildren implements the pure logic of phase derivation.
// Exported for direct testing without beads dependency.
func DerivePhaseFromChildren(children []ChildInfo) string {
	if len(children) == 0 {
		return state.ModePlan // epic exists, no children → plan (spec approved, plan being drafted)
	}

	var totalOpen, totalClosed, totalInProgress int
	for _, c := range children {
		switch strings.ToLower(strings.TrimSpace(c.Status)) {
		case "closed":
			totalClosed++
		case "in_progress":
			totalInProgress++
		default: // "open" or anything else
			totalOpen++
		}
	}

	total := len(children)

	// Any child in_progress → implement
	if totalInProgress > 0 {
		return state.ModeImplement
	}

	// All children closed → review
	if totalClosed == total {
		return state.ModeReview
	}

	// Some closed + some open, none in_progress → implement (between beads).
	// Closed beads prove implementation has started; the agent is between
	// completing one bead and claiming the next.
	if totalClosed > 0 {
		return state.ModeImplement
	}

	// All children open (none claimed) → plan
	return state.ModePlan
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

		specID := SpecIDFromMetadata(specNum, specTitle)

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
		children, _ := c.GetChildren(epic.ID)

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
		}
		phase, err := derivePhaseForSpecWithCache(c, ctx.EpicID)
		if err == nil {
			ctx.Phase = phase
		} else {
			ctx.Phase = state.ModeImplement // bead worktree implies implement
		}

	case workspace.WorktreeSpec:
		ctx.SpecID = specID
		epicID, err := FindEpicBySpecIDWithCache(c, specID)
		if err == nil && epicID != "" {
			ctx.EpicID = epicID
			phase, err := DerivePhaseWithCache(c, epicID)
			if err == nil {
				ctx.Phase = phase
			}
		}
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
		if err == nil && len(specs) == 1 {
			ctx.SpecID = specs[0].SpecID
			ctx.EpicID = specs[0].EpicID
			ctx.Phase = specs[0].Phase
		} else if err == nil && len(specs) == 0 {
			ctx.Phase = state.ModeIdle
		}
		// Multiple specs: leave specID empty (caller handles ambiguity)
	}

	return ctx, nil
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
			return fmt.Errorf("spec number %03d is already in use by epic %s (%s)", specNum, epic.ID, epic.Title)
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
func FindEpicBySpecIDWithCache(c *Cache, specID string) (string, error) {
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
func FindEpicForBeadWithCache(c *Cache, beadID string) (epicID, specID string, err error) {
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
		return "", "", fmt.Errorf("bead %s not found", beadID)
	}

	// Try to find the parent epic via dependencies
	for _, dep := range items[0].Dependencies {
		if strings.EqualFold(dep.IssueType, "epic") {
			epic, epicErr := c.FindEpic(dep.ID)
			if epicErr == nil && epic != nil {
				num, title := ExtractSpecMetadata(*epic)
				if num > 0 && title != "" {
					return dep.ID, SpecIDFromMetadata(num, title), nil
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
				if listErr == nil {
					for _, epic := range epics {
						epicNum, epicTitle := ExtractSpecMetadata(epic)
						if epicNum == num {
							return epic.ID, SpecIDFromMetadata(epicNum, epicTitle), nil
						}
					}
				}
			}
		}
	}

	return "", "", fmt.Errorf("no epic found for bead %s", beadID)
}

// FindActiveBeadForEpic returns the ID of an in_progress bead under the given epic, or "".
// Returns "" if zero or multiple beads are in_progress (ambiguous — caller should
// fall back to the spec worktree). Constructs a fresh cache; hot-path callers
// should use FindActiveBeadForEpicWithCache.
func FindActiveBeadForEpic(epicID string) string {
	return FindActiveBeadForEpicWithCache(NewCache(), epicID)
}

// FindActiveBeadForEpicWithCache is the cache-aware variant of FindActiveBeadForEpic.
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
		return beads[0].ID
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
