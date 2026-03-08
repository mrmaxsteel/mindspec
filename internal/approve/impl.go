package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/recording"
	"github.com/mrmaxsteel/mindspec/internal/state"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

var (
	implRunBDCombinedFn = bead.RunBDCombined
	implRunBDFn         = bead.RunBD
)

// ImplOpts holds options for implementation approval.
type ImplOpts struct{}

// ImplResult holds the result of implementation approval.
type ImplResult struct {
	SpecID      string
	Warnings    []string
	SpecBranch  string
	CommitCount int
	DiffStat    string
	Pushed      bool // true if branch was pushed to remote
}

// ApproveImpl transitions from review mode to idle, completing the spec lifecycle.
// Enforcement logic (phase validation, epic closure, bead status checks) stays here.
// Git operations (merge, push, cleanup) are delegated to the Executor.
func ApproveImpl(root, specID string, exec executor.Executor, opts ...ImplOpts) (*ImplResult, error) {
	if err := validate.SpecID(specID); err != nil {
		return nil, err
	}
	result := &ImplResult{SpecID: specID}

	// Find the epic for this spec directly (ADR-0023).
	epicID, err := phase.FindEpicBySpecID(specID)
	if err != nil {
		return nil, fmt.Errorf("no epic found for spec %s: %w", specID, err)
	}

	// Verify state is review mode: all children closed, pending final merge.
	epicPhase, err := phase.DerivePhase(epicID)
	if err != nil {
		return nil, fmt.Errorf("deriving phase for spec %s: %w", specID, err)
	}
	if epicPhase != state.ModeReview && epicPhase != state.ModeDone {
		return nil, fmt.Errorf("expected review mode, got %q", epicPhase)
	}

	// Close epic and mark as explicitly done.
	if epicID != "" {
		if _, err := implRunBDCombinedFn("close", epicID); err != nil {
			if !isAlreadyClosedErr(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not close lifecycle epic %s: %v", epicID, err))
			}
		}
		doneMetadata := `{"mindspec_done":true}`
		if out, err := implRunBDFn("show", epicID, "--json"); err == nil {
			var items []struct {
				Metadata map[string]interface{} `json:"metadata"`
			}
			if json.Unmarshal(out, &items) == nil && len(items) > 0 && items[0].Metadata != nil {
				merged := items[0].Metadata
				merged["mindspec_done"] = true
				if b, err := json.Marshal(merged); err == nil {
					doneMetadata = string(b)
				}
			}
		}
		if _, err := implRunBDCombinedFn("update", epicID, "--metadata", doneMetadata); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not set done marker on epic %s: %v", epicID, err))
		}
	}

	// Derive spec branch from convention.
	specBranch := state.SpecBranch(specID)

	// Enforcement: verify all plan beads are closed.
	specDir := workspace.SpecDir(root, specID)
	planPath := filepath.Join(specDir, "plan.md")
	beadIDs, planErr := readPlanBeadIDs(planPath)
	if planErr == nil {
		for _, bid := range beadIDs {
			status, err := readBeadStatus(bid)
			if err != nil {
				return nil, fmt.Errorf("checking bead %s status: %w", bid, err)
			}
			if status != "closed" {
				return nil, fmt.Errorf("bead %s is still %q — close all beads before approving implementation", bid, status)
			}
		}
	}

	// Pre-flight: check spec branch has commits (via executor).
	count, countErr := exec.CommitCount("main", specBranch)
	if countErr == nil {
		if count == 0 && (planErr != nil || len(beadIDs) == 0) {
			return nil, fmt.Errorf("preflight check failed: spec branch %s has no commits beyond main — nothing to merge", specBranch)
		}
	}

	// Delegate all git operations to the executor.
	result.SpecBranch = specBranch
	fr, err := exec.FinalizeEpic(epicID, specID, specBranch)
	if err != nil {
		return nil, fmt.Errorf("finalizing epic: %w", err)
	}

	result.CommitCount = fr.CommitCount
	result.DiffStat = fr.DiffStat
	result.Pushed = (fr.MergeStrategy == "pr")

	// Stop recording (best-effort — before transitioning to idle)
	if err := recording.StopRecording(root, specID); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not stop recording: %v", err))
	}

	return result, nil
}

func readBeadStatus(id string) (string, error) {
	out, err := implRunBDFn("show", id, "--json")
	if err != nil {
		return "", err
	}

	var payload []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parsing bd show output for %s: %w", id, err)
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("no bead returned for %s", id)
	}
	return strings.ToLower(strings.TrimSpace(payload[0].Status)), nil
}

// readPlanBeadIDs reads bead_ids from the plan.md YAML frontmatter.
func readPlanBeadIDs(planPath string) ([]string, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("no frontmatter found")
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return nil, fmt.Errorf("no frontmatter end marker")
	}
	fmContent := content[4 : 4+end]

	var fm struct {
		BeadIDs []string `yaml:"bead_ids"`
	}
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return nil, fmt.Errorf("parsing plan frontmatter: %w", err)
	}
	if len(fm.BeadIDs) == 0 {
		return nil, fmt.Errorf("no bead_ids in plan frontmatter")
	}
	return fm.BeadIDs, nil
}

func isAlreadyClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already closed")
}
