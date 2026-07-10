// Package bead is the bd boundary for enforcement packages
// (internal/{validate,approve,complete,state,phase}). Direct
// exec.Command("bd", ...) calls from any of those packages are
// prohibited and must route through this package. Helpers here own
// the os/exec type-switches and JSON parsing so callers stay free of
// process-I/O concerns. See ADR-0030.
package bead

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/trace"
)

// execCommand is a package-level variable for testability.
// Tests override this to capture arguments or return canned output.
var execCommand = exec.Command

// BeadInfo represents a work item from the Beads CLI.
type BeadInfo struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Priority    int                    `json:"priority"`
	IssueType   string                 `json:"issue_type"`
	Owner       string                 `json:"owner"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Preflight checks prerequisites for bead commands:
// git repo, .beads/ directory, bd on PATH.
func Preflight(root string) error {
	// Check git repo
	cmd := execCommand("git", "rev-parse", "--git-dir")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("not a git repository (run 'git init'): %s", string(out))
	}

	// Check .beads/ exists
	beadsDir := filepath.Join(root, ".beads")
	if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
		return fmt.Errorf(".beads/ directory not found (run 'beads init' to initialize)")
	}

	// Check bd on PATH
	if _, err := exec.LookPath("bd"); err != nil {
		return fmt.Errorf("bd not found on PATH (install Beads: https://github.com/beads-project/beads)")
	}

	return nil
}

// RunBD executes a bd command and returns stdout. Stderr is captured for error
// messages but not mixed into the output (important for JSON parsing).
// This is the primary interface for composing with bd per ADR-0012.
func RunBD(args ...string) ([]byte, error) {
	return tracedOutput("run", args)
}

// BeadExists reports whether bead id is present in Beads. Returns
// (true, nil) if `bd show <id> --json` succeeds; (false, nil) if bd
// ran but reported the bead as missing (non-zero exit captured as
// *exec.ExitError); (false, err) only if bd itself is unavailable
// or some other non-exit error occurred. The os/exec type-switch is
// performed inside this package so enforcement-package callers
// (e.g. internal/validate) never import os/exec.
func BeadExists(id string) (bool, error) {
	_, err := RunBD("show", id, "--json")
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
}

// IsUnsupportedFlagError reports whether err (returned by RunBD/RunBDCombined,
// possibly wrapped by a caller with fmt.Errorf's %w) indicates the installed
// bd binary does not recognize flag — e.g. an older bd invoked with a flag
// introduced in a later release (bead mindspec-uopd: `bd show --as-of`,
// added in bd 1.0.4, is not understood by pre-1.0.4 binaries). bd's cobra
// CLI reports this as `Error: unknown flag: --<flag>` on stderr with a
// non-zero exit, captured by os/exec as *exec.ExitError.Stderr. The check is
// intentionally conservative — it requires BOTH "unknown flag" and the flag
// name in the stderr text — so a genuine bd/Dolt failure (missing bead,
// lock contention, network) is never misclassified as an unsupported flag.
// The os/exec type-switch is confined here per this package's doc comment
// (callers stay free of process-I/O concerns).
func IsUnsupportedFlagError(err error, flag string) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	stderr := strings.ToLower(string(exitErr.Stderr))
	return strings.Contains(stderr, "unknown flag") && strings.Contains(stderr, strings.ToLower(strings.TrimPrefix(flag, "--")))
}

// RunBDCombined executes a bd command and returns combined stdout+stderr.
// Use for commands that don't return JSON where stderr output is useful.
func RunBDCombined(args ...string) ([]byte, error) {
	return tracedCombined("run", args)
}

// Export refreshes <workdir>/.beads/issues.jsonl from current Dolt state.
// Run before `git add -A` so the committed JSONL matches Dolt at commit time.
// bd's own pre-commit hook also exports; running here belt-and-braces guards
// against bypassed hooks (--no-verify, non-hook callers).
func Export(workdir string) error {
	start := time.Now()
	cmd := execCommand("bd", "export", "-o", ".beads/issues.jsonl")
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	trace.Emit(trace.NewEvent("bead.cli").
		WithDuration(time.Since(start)).
		WithData(map[string]any{
			"op": "export",
			"ok": err == nil,
		}))
	if err != nil {
		return fmt.Errorf("bd export in %s: %s", workdir, strings.TrimSpace(string(out)))
	}
	return nil
}

// Close closes one or more beads via `bd close`.
func Close(ids ...string) error {
	if len(ids) == 0 {
		return fmt.Errorf("Close requires at least one bead ID")
	}
	args := append([]string{"close"}, ids...)
	out, err := tracedCombined("close", args)
	if err != nil {
		return fmt.Errorf("bd close failed: %s", string(out))
	}
	return nil
}

// DoltCommit forces a Dolt commit of any uncommitted changes in the working
// set via `bd dolt commit` — the strongest available bd durability primitive
// ("Create a Dolt commit from any uncommitted changes in the working set …").
// Spec 098 Req 2 (mindspec-9n2h) calls this after `bd close` to force the
// close to durable Dolt state before completion proceeds.
//
// IDEMPOTENT: a clean working set is SUCCESS, not a failure. `bd dolt commit`
// on a clean set prints "Nothing to commit." and exits 0 (verified: in
// embedded auto-commit mode every write — including `bd close` — auto-commits,
// so the post-close commit is a clean-set no-op). To stay robust against bd
// builds that exit non-zero on a clean set, a "nothing to commit" message is
// ALSO treated as success — only a genuine commit error surfaces as a Go error
// (non-zero exit per ADR-0012).
func DoltCommit() error {
	out, err := tracedCombined("dolt-commit", []string{"dolt", "commit"})
	if err == nil {
		return nil
	}
	// Clean working set: nothing to commit is a no-op success, not a failure.
	if strings.Contains(strings.ToLower(string(out)), "nothing to commit") {
		return nil
	}
	return fmt.Errorf("bd dolt commit failed: %s", strings.TrimSpace(string(out)))
}

// --- Worktree wrappers (genuine multi-step helpers per ADR-0012) ---

// WorktreeListEntry represents a worktree from `bd worktree list --json`.
type WorktreeListEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	IsMain     bool   `json:"is_main"`
	BeadsState string `json:"beads_state"`
}

// WorktreeCreate creates a worktree via `bd worktree create`.
// Beads handles git worktree creation, redirect setup, and .gitignore.
func WorktreeCreate(name, branch string) error {
	args := []string{"worktree", "create", name}
	if branch != "" {
		args = append(args, "--branch="+branch)
	}
	out, err := tracedCombined("worktree-create", args)
	if err != nil {
		return fmt.Errorf("bd worktree create failed: %s", string(out))
	}
	return nil
}

// WorktreeList returns all worktrees via `bd worktree list --json`.
func WorktreeList() ([]WorktreeListEntry, error) {
	args := []string{"worktree", "list", "--json"}
	out, err := tracedOutput("worktree-list", args)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd worktree list failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("bd worktree list failed: %w", err)
	}

	var entries []WorktreeListEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parsing bd worktree list output: %w", err)
	}
	return entries, nil
}

// WorktreeRemove removes a worktree via `bd worktree remove --force`.
// The --force flag skips the "unpushed commits" safety check, which is
// appropriate because mindspec always merges bead work into the spec
// branch before removing the worktree.
func WorktreeRemove(name string) error {
	args := []string{"worktree", "remove", name, "--force"}
	out, err := tracedCombined("worktree-remove", args)
	if err != nil {
		return fmt.Errorf("bd worktree remove failed: %s", string(out))
	}
	return nil
}

// minBdVersionMsg is the bd version mentioned in ListJSON's non-JSON error.
// Keep in sync with doctor.bdVersionFloor — bead must not import doctor (cycle).
const minBdVersionMsg = "1.0.4"

// ListJSON runs `bd list <args> --json` and returns valid JSON bytes (a JSON array).
// Requires `bd` >= 1.0.4 (the floor enforced by `mindspec doctor`). When `bd`
// emits non-JSON output (older versions where --json was ignored), this returns
// a structured error directing the user to upgrade — no human-output scraping.
func ListJSON(args ...string) ([]byte, error) {
	fullArgs := append(append([]string{"list"}, args...), "--json")
	out, err := tracedOutput("list", fullArgs)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(out))

	// Handle empty results
	if trimmed == "" || trimmed == "[]" || trimmed == "No issues found." {
		return []byte("[]"), nil
	}

	// If output is valid JSON, use it directly
	if json.Valid([]byte(trimmed)) {
		return []byte(trimmed), nil
	}

	return nil, fmt.Errorf("bd list --json returned non-JSON output (your bd may be older than the supported floor %s). "+
		"Run `mindspec doctor` and upgrade with `brew upgrade beads`.", minBdVersionMsg)
}

// MergeMetadata reads an issue's existing metadata, merges in the given key-value
// pairs, and writes the merged metadata back. This is the standard pattern for
// updating metadata without losing existing fields (Spec 080).
//
// FAIL-CLOSED (Spec 114 R2/Bead 2, carry-forward #1 / review item 2): a
// read/parse FAILURE of the existing metadata now RETURNS the error instead
// of silently proceeding from an empty map and replace-writing — a prior
// read failure used to ERASE every existing key on the next write. A
// genuinely-absent-but-clean read (the bead exists but carries no metadata
// yet, or `bd show` reports an empty items array) still merges from empty,
// unchanged — only a genuine READ/PARSE failure now aborts. See GetMetadata,
// the read half this shares.
func MergeMetadata(issueID string, updates map[string]interface{}) error {
	existing, err := GetMetadata(issueID)
	if err != nil {
		return fmt.Errorf("bd metadata merge-read failed for %s: %w", issueID, err)
	}

	merged := make(map[string]interface{}, len(existing)+len(updates))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range updates {
		merged[k] = v
	}

	metaJSON, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	_, err = tracedCombined("update", []string{"update", issueID, "--metadata", string(metaJSON)})
	if err != nil {
		// Spec 092 Req 19/HC-5: emitted messages never contain a raw
		// bd metadata-update command line (replace semantics over the
		// whole map); describe the operation without quoting it.
		return fmt.Errorf("bd metadata merge-write failed for %s: %w", issueID, err)
	}
	return nil
}

// GetMetadata reads issueID's current metadata via `bd show <id> --json` and
// returns the parsed map (Spec 114 R2/Bead 2, carry-forward #1). A
// genuinely-absent metadata field (an existing bead with no metadata, or an
// empty `bd show` items array) returns an empty, non-nil map and a nil
// error — NOT an error condition. Only a genuine command failure (bd itself
// erroring) or an unparseable response returns an error — the fail-closed
// signal MergeMetadata above, and the complete-side durable-obligation
// reconciliation (internal/complete), both rely on: a read error is never a
// licence to proceed as if the store were empty.
func GetMetadata(issueID string) (map[string]interface{}, error) {
	out, err := tracedOutput("show", []string{"show", issueID, "--json"})
	if err != nil {
		return nil, fmt.Errorf("bd show %s failed: %w", issueID, err)
	}
	var items []struct {
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing bd show %s output: %w", issueID, err)
	}
	merged := make(map[string]interface{})
	if len(items) > 0 && items[0].Metadata != nil {
		for k, v := range items[0].Metadata {
			merged[k] = v
		}
	}
	return merged, nil
}

// tracedOutput runs a bd command via cmd.Output() with trace instrumentation.
func tracedOutput(op string, args []string) ([]byte, error) {
	start := time.Now()
	cmd := execCommand("bd", args...)
	out, err := cmd.Output()
	trace.Emit(trace.NewEvent("bead.cli").
		WithDuration(time.Since(start)).
		WithData(map[string]any{
			"op":   op,
			"args": args,
			"ok":   err == nil,
		}))
	return out, err
}

// tracedCombined runs a bd command via cmd.CombinedOutput() with trace instrumentation.
func tracedCombined(op string, args []string) ([]byte, error) {
	start := time.Now()
	cmd := execCommand("bd", args...)
	out, err := cmd.CombinedOutput()
	trace.Emit(trace.NewEvent("bead.cli").
		WithDuration(time.Since(start)).
		WithData(map[string]any{
			"op":   op,
			"args": args,
			"ok":   err == nil,
		}))
	return out, err
}
