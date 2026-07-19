package next

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

// BeadInfo represents a work item from Beads.
type BeadInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	IssueType string `json:"issue_type"`
	Owner     string `json:"owner"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Package-level function variables for testability.
var (
	runBDFn     = bead.RunBD
	runBDCombFn = bead.RunBDCombined
	listJSONFn  = bead.ListJSON
)

// QueryReady discovers ready work via global bd ready.
func QueryReady() ([]BeadInfo, error) {
	out, err := runBDFn("ready", "--json")
	if err != nil {
		return nil, fmt.Errorf("bd ready failed: %w", err)
	}

	return ParseBeadsJSON(out)
}

// QueryReadyForEpic queries ready work scoped to a specific epic (parent issue).
func QueryReadyForEpic(epicID string) ([]BeadInfo, error) {
	// Gate-all-ids (ADR-0042 §1, round 9): epicID feeds a
	// `bd ready --parent` argv build directly — validate BEFORE any bd
	// spawn.
	if err := idvalidate.BeadID(epicID); err != nil {
		return nil, fmt.Errorf("invalid epic id %s: %w", epicID, err)
	}
	out, err := runBDFn("ready", "--parent", epicID, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd ready for epic %s failed: %w", epicID, err)
	}

	return ParseBeadsJSON(out)
}

// ParseBeadsJSON parses the JSON output from bd commands into BeadInfo slices.
func ParseBeadsJSON(data []byte) ([]BeadInfo, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var items []BeadInfo
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("parsing beads JSON: %w", err)
		}
		return filterReadyItems(items), nil
	}

	if strings.HasPrefix(trimmed, "{") {
		var payload struct {
			Steps []struct {
				Issue BeadInfo `json:"issue"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, fmt.Errorf("parsing beads ready JSON: %w", err)
		}
		items := make([]BeadInfo, 0, len(payload.Steps))
		for _, step := range payload.Steps {
			items = append(items, step.Issue)
		}
		return filterReadyItems(items), nil
	}

	return nil, fmt.Errorf("parsing beads JSON: unsupported payload shape")
}

func filterReadyItems(items []BeadInfo) []BeadInfo {
	seen := map[string]struct{}{}
	var filtered []BeadInfo
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.IssueType), "epic") {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "closed" {
			continue
		}
		if status != "" && status != "open" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		filtered = append(filtered, item)
	}
	return filtered
}

// ResolveActiveBead finds the currently in-progress bead for a spec by querying
// beads for the spec's epic and then finding in-progress children.
// Returns empty string (no error) if no bead is in progress.
func ResolveActiveBead(root, specID string) (string, error) {
	// Find epic via beads metadata query (ADR-0023).
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil || epicID == "" {
		return "", nil
	}
	// Gate-all-ids (ADR-0042 §1, round 9): epicID feeds a
	// `bd list --parent` argv build directly — validate BEFORE any bd
	// spawn (defense in depth: epicID is already RETURN-gated at
	// phase.FindEpicBySpecID, but no id operand is trusted by
	// provenance).
	if err := idvalidate.BeadID(epicID); err != nil {
		return "", nil
	}

	out, err := listJSONFn("--parent", epicID, "--status=in_progress")
	if err != nil {
		return "", nil // No in-progress beads
	}

	var items []BeadInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return "", nil
	}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id != "" && !strings.EqualFold(strings.TrimSpace(item.IssueType), "epic") {
			return id, nil
		}
	}

	return "", nil
}

// ClaimBead atomically claims a bead via bd update --claim.
// Fails if the bead was already claimed by another agent, preventing
// two concurrent agents from working on the same bead.
//
// On failure, bd's real captured output is surfaced verbatim (R3, ADR-0035
// paste-safe). The earlier "may already be claimed" prefix is deliberately
// gone: it masked non-contention causes — e.g. a stale bd binary emitting
// `column "depends_on_id" could not be found` read as a benign "already
// claimed". The "claim failed:" prefix keeps just enough context to know a
// claim was what failed while letting the true cause through.
func ClaimBead(id string) error {
	// Gate-all-ids (ADR-0042 §1, round 9): id feeds a `bd update --claim`
	// argv build directly — validate BEFORE any bd spawn (this is also
	// the R3 explicit-claim ingress: a malformed claim target refuses
	// convergently).
	if err := idvalidate.BeadID(id); err != nil {
		return fmt.Errorf("invalid bead id %s: %w", id, err)
	}
	out, err := runBDCombFn("update", id, "--claim")
	if err != nil {
		return fmt.Errorf("claim failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// FetchBeadByID retrieves a single bead by its ID via bd show --json. This is
// a SESSION read — it does not distinguish committed Dolt state from
// in-session/uncommitted state. See FetchBeadAsOf for the committed-state
// read (bead mindspec-uopd).
func FetchBeadByID(id string) (BeadInfo, error) {
	// Gate-all-ids (ADR-0042 §1, round 9): id feeds a `bd show` argv
	// build directly — validate BEFORE any bd spawn.
	if err := idvalidate.BeadID(id); err != nil {
		return BeadInfo{}, fmt.Errorf("invalid bead id %s: %w", id, err)
	}
	out, err := runBDFn("show", id, "--json")
	if err != nil {
		return BeadInfo{}, fmt.Errorf("bd show %s failed: %w", id, err)
	}
	return parseBeadShowJSON(out, id)
}

// FetchBeadAsOf retrieves a single bead as it existed at a specific commit
// hash or branch via `bd show <id> --as-of <ref> --json` (bd >= 1.0.4; bead
// mindspec-uopd). Unlike FetchBeadByID, this reads COMMITTED Dolt state —
// bd's embedded engine auto-commits every write, so `--as-of HEAD` is a true
// post-commit re-read distinct from the in-session `bd show`.
//
// Callers on an older bd that predates the --as-of flag get a bd CLI error
// (`unknown flag: --as-of`) wrapped into the returned error's chain via
// fmt.Errorf's %w; use bead.IsUnsupportedFlagError(err, "as-of") to detect
// that case and fall back to FetchBeadByID (see
// internal/complete.defaultVerifyCommitted).
func FetchBeadAsOf(id, ref string) (BeadInfo, error) {
	// Gate-all-ids (ADR-0042 §1, round 9): id feeds a `bd show` argv
	// build directly — validate BEFORE any bd spawn.
	if err := idvalidate.BeadID(id); err != nil {
		return BeadInfo{}, fmt.Errorf("invalid bead id %s: %w", id, err)
	}
	out, err := runBDFn("show", id, "--as-of", ref, "--json")
	if err != nil {
		return BeadInfo{}, fmt.Errorf("bd show %s --as-of %s failed: %w", id, ref, err)
	}
	return parseBeadShowJSON(out, id)
}

// parseBeadShowJSON parses the output of `bd show <id> [--as-of <ref>]
// --json`, which is either a single-element JSON array or (older bd) a bare
// JSON object. Shared by FetchBeadByID and FetchBeadAsOf.
func parseBeadShowJSON(out []byte, id string) (BeadInfo, error) {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return BeadInfo{}, fmt.Errorf("bd show %s returned empty output", id)
	}

	// bd show --json returns an array with one element
	if strings.HasPrefix(trimmed, "[") {
		var items []BeadInfo
		if err := json.Unmarshal(out, &items); err != nil {
			return BeadInfo{}, fmt.Errorf("parsing bead %s JSON: %w", id, err)
		}
		if len(items) == 0 {
			return BeadInfo{}, fmt.Errorf("bead %s not found", id)
		}
		return items[0], nil
	}

	// Single object
	var item BeadInfo
	if err := json.Unmarshal(out, &item); err != nil {
		return BeadInfo{}, fmt.Errorf("parsing bead %s JSON: %w", id, err)
	}
	return item, nil
}

// EnsureWorktree creates or reuses a workspace for the given bead via the
// Executor interface. The specID is passed so the executor can branch from
// the spec branch; the branching strategy is the executor's concern.
// Returns the workspace path.
func EnsureWorktree(root, beadID, specID string, exec executor.Executor) (string, error) {
	ws, err := exec.DispatchBead(beadID, specID)
	if err != nil {
		return "", err
	}
	return ws.Path, nil
}
