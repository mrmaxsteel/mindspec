// Package phase provides beads-based lifecycle phase derivation (ADR-0023).
package phase

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mindspec/mindspec/internal/bead"
	"github.com/mindspec/mindspec/internal/state"
	"github.com/mindspec/mindspec/internal/workspace"
)

// Package-level function variables for testability.
var (
	runBDFn = bead.RunBD
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
// Implements the phase derivation table from ADR-0023 §3.
func DerivePhase(epicID string) (string, error) {
	return DerivePhaseWithStatus(epicID, "")
}

// DerivePhaseWithStatus determines the lifecycle phase, using a pre-fetched epic status
// if available (to avoid redundant queries). If epicStatus is empty, it is looked up.
func DerivePhaseWithStatus(epicID, epicStatus string) (string, error) {
	if epicStatus == "" {
		epicStatus = queryEpicStatus(epicID)
	}
	if strings.EqualFold(epicStatus, "closed") {
		// Check for explicit done marker set by impl approve.
		// Without this marker, a closed epic means beads auto-closed it
		// when the last child was closed (molecule completion). In that case,
		// derive phase from children: all closed = review (pending impl approve).
		if hasDoneMarker(epicID) {
			return state.ModeDone, nil
		}
		children := queryChildren(epicID)
		return DerivePhaseFromChildren(children), nil
	}
	children := queryChildren(epicID)
	return DerivePhaseFromChildren(children), nil
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

	// All children open (none claimed) → plan
	// Some closed, some open, none in_progress → plan (next bead ready)
	return state.ModePlan
}

// DiscoverActiveSpecs queries beads for all open epics and derives phase for each.
func DiscoverActiveSpecs() ([]ActiveSpec, error) {
	epics, err := queryEpics()
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

		// Query children once and reuse for both phase derivation and orphan check.
		children := queryChildren(epic.ID)

		// Check done marker for closed epics before deriving phase.
		if strings.EqualFold(epic.Status, "closed") {
			if hasDoneMarker(epic.ID) {
				continue // spec lifecycle complete
			}
			if len(children) == 0 {
				continue // orphan: closed epic with no children
			}
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

// ResolveContextFromDir resolves context from a specific directory.
func ResolveContextFromDir(root, dir string) (*Context, error) {
	kind, specID, beadID := workspace.DetectWorktreeContext(dir)

	ctx := &Context{
		WorktreePath: dir,
	}

	switch kind {
	case workspace.WorktreeBead:
		ctx.BeadID = beadID
		epicID, derivedSpecID, err := findEpicForBead(beadID)
		if err == nil && derivedSpecID != "" {
			ctx.SpecID = derivedSpecID
			ctx.EpicID = epicID
		}
		phase, err := derivePhaseForSpec(ctx.EpicID)
		if err == nil {
			ctx.Phase = phase
		} else {
			ctx.Phase = state.ModeImplement // bead worktree implies implement
		}

	case workspace.WorktreeSpec:
		ctx.SpecID = specID
		epicID, err := FindEpicBySpecID(specID)
		if err == nil && epicID != "" {
			ctx.EpicID = epicID
			phase, err := DerivePhase(epicID)
			if err == nil {
				ctx.Phase = phase
			}
		}
		if ctx.Phase == "" {
			ctx.Phase = state.ModeSpec // spec worktree without epic → spec mode
		}
		// Check for active bead
		if ctx.EpicID != "" {
			if activeBead := FindActiveBeadForEpic(ctx.EpicID); activeBead != "" {
				ctx.BeadID = activeBead
			}
		}

	case workspace.WorktreeMain:
		specs, err := DiscoverActiveSpecs()
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

	epics, err := queryEpics()
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
func FindEpicBySpecID(specID string) (string, error) {
	epics, err := queryEpics()
	if err != nil {
		return "", err
	}

	for _, epic := range epics {
		num, title := ExtractSpecMetadata(epic)
		if num > 0 && title != "" {
			if SpecIDFromMetadata(num, title) == specID {
				return epic.ID, nil
			}
		}
	}

	return "", fmt.Errorf("no epic found for spec %s", specID)
}

// --- Internal helpers ---

// hasDoneMarker checks if an epic has the mindspec_done metadata flag,
// which is set by impl approve to distinguish explicitly finalized specs
// from epics auto-closed by beads molecule completion.
func hasDoneMarker(epicID string) bool {
	out, err := runBDFn("show", epicID, "--json")
	if err != nil {
		return false
	}
	var items []EpicInfo
	if err := json.Unmarshal(out, &items); err != nil || len(items) == 0 {
		return false
	}
	if items[0].Metadata == nil {
		return false
	}
	done, ok := items[0].Metadata["mindspec_done"]
	if !ok {
		return false
	}
	b, ok := done.(bool)
	return ok && b
}

func queryEpics() ([]EpicInfo, error) {
	// Query all statuses: bd list --type=epic defaults to open only,
	// but phase derivation needs closed epics too (e.g. impl approve).
	var allEpics []EpicInfo
	seen := map[string]bool{}
	var lastErr error
	for _, status := range []string{"open", "in_progress", "closed"} {
		out, err := runBDFn("list", "--type=epic", "--status="+status, "--json")
		if err != nil {
			lastErr = err
			continue
		}
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" || trimmed == "[]" {
			continue
		}
		var epics []EpicInfo
		if err := json.Unmarshal(out, &epics); err != nil {
			lastErr = err
			continue
		}
		for _, e := range epics {
			if !seen[e.ID] {
				seen[e.ID] = true
				allEpics = append(allEpics, e)
			}
		}
	}
	if len(allEpics) == 0 && lastErr != nil {
		return nil, fmt.Errorf("bd list --type=epic failed: %w", lastErr)
	}
	return allEpics, nil
}

func queryChildren(epicID string) []ChildInfo {
	// Query all statuses: bd list --parent defaults to open only,
	// but phase derivation needs closed beads too.
	var allChildren []ChildInfo
	for _, status := range []string{"open", "in_progress", "closed"} {
		out, err := runBDFn("list", "--parent", epicID, "--status="+status, "--json")
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" || trimmed == "[]" {
			continue
		}
		var children []ChildInfo
		if err := json.Unmarshal(out, &children); err != nil {
			continue
		}
		allChildren = append(allChildren, children...)
	}
	return allChildren
}

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

func findEpicForBead(beadID string) (epicID, specID string, err error) {
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
			epicOut, epicErr := runBDFn("show", dep.ID, "--json")
			if epicErr == nil {
				var epicItems []EpicInfo
				if json.Unmarshal(epicOut, &epicItems) == nil && len(epicItems) > 0 {
					num, title := ExtractSpecMetadata(epicItems[0])
					if num > 0 && title != "" {
						return dep.ID, SpecIDFromMetadata(num, title), nil
					}
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
				epics, listErr := queryEpics()
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
// fall back to the spec worktree).
func FindActiveBeadForEpic(epicID string) string {
	out, err := runBDFn("list", "--parent", epicID, "--status=in_progress", "--json")
	if err != nil {
		return ""
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" || trimmed == "[]" {
		return ""
	}

	var items []ChildInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return ""
	}

	// Filter out epics (only want beads).
	var beads []ChildInfo
	for _, item := range items {
		if !strings.EqualFold(item.IssueType, "epic") {
			beads = append(beads, item)
		}
	}

	// Only return a bead when exactly one is in_progress.
	// Multiple in_progress beads (parallel agents) → ambiguous, return "".
	if len(beads) == 1 {
		return beads[0].ID
	}
	return ""
}

// queryEpicStatus fetches the status of a single epic by ID.
func queryEpicStatus(epicID string) string {
	out, err := runBDFn("show", epicID, "--json")
	if err != nil {
		return ""
	}
	var items []EpicInfo
	if err := json.Unmarshal(out, &items); err != nil || len(items) == 0 {
		return ""
	}
	return items[0].Status
}

func derivePhaseForSpec(epicID string) (string, error) {
	if epicID == "" {
		return state.ModeSpec, nil
	}
	return DerivePhase(epicID)
}
