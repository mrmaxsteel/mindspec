package next

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/resolve"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
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
	readyJSONFn = bead.ReadyJSON
)

// QueryReady discovers ready work via global bd ready.
func QueryReady() ([]BeadInfo, error) {
	out, err := readyJSONFn()
	if err != nil {
		return nil, fmt.Errorf("bd ready failed: %w", &bdRenderedError{err: err})
	}

	return ParseBeadsJSON(out)
}

// QueryReadyForEpic queries ready work scoped to a specific epic (parent issue).
func QueryReadyForEpic(epicID string) ([]BeadInfo, error) {
	// Gate-all-ids (ADR-0042 §1, round 9): epicID feeds a
	// `bd ready --parent` argv build directly — validate BEFORE any bd
	// spawn.
	if err := idvalidate.BeadID(epicID); err != nil {
		return nil, fmt.Errorf("invalid epic id %s: %w", idrender.Bead(epicID), err)
	}
	out, err := readyJSONFn("--parent", epicID)
	if err != nil {
		return nil, fmt.Errorf("bd ready for epic %s failed: %w", epicID, &bdRenderedError{err: err})
	}

	return ParseBeadsJSON(out)
}

// RenderBDError wraps err for safe terminal-facing rendering identically
// to bdRenderedError (every line of err.Error() escaped individually via
// termsafe.Escape, ADR-0042) — exported so a caller OUTSIDE this package
// (cmd/mindspec/next.go's describePhaseSyncFailure, spec 128 Bead 8's
// folded-in ta30.9 render-leg follow-up) can render an arbitrary
// bd/lock-derived error safely via %v/%s without forking a second
// escaping path. Callers format the RETURNED error (not err directly) —
// e.g. fmt.Sprintf("...: %v", next.RenderBDError(err)). Returns nil for a
// nil err so callers can wrap unconditionally.
func RenderBDError(err error) error {
	if err == nil {
		return nil
	}
	return &bdRenderedError{err: err}
}

// ReadySet is spec 128 R6.1's ONE ready-set construction result — the
// bd-ready query plus the spec-title filter, and the exclusion accounting
// the filter drops — consumed IDENTICALLY by the claim path
// (cmd/mindspec/next.go's RunE) and the --emit-only path (runEmitOnly), so
// `--pick N` always names the same bead on both for the same N (AC-9
// parity) and a filter drop is reported the same way on both (R6.3).
type ReadySet struct {
	// Filtered is the DISPLAYED/selectable list, in bd ready's returned
	// order (R6.2, preserved end-to-end) — the spec-title filter already
	// applied. Callers select from THIS list, never Raw.
	Filtered []BeadInfo
	// Raw is the pre-filter list bd ready / bd ready --parent returned.
	// Never displayed or selected from directly — SelectNamedFromReadySet
	// consults it ONLY to name the true exclusion reason for a positional
	// ID the filter dropped (R6.3(b)/AC-10).
	Raw []BeadInfo
	// SpecFlag is the scope this set was constructed for; "" means
	// unscoped (no filter was applied — Raw and Filtered are the same
	// slice).
	SpecFlag string
}

// ExcludedCount reports how many raw items the spec-title filter dropped.
func (rs ReadySet) ExcludedCount() int {
	return len(rs.Raw) - len(rs.Filtered)
}

// ExclusionNotice renders the R6.3(a) loud-drop line for a partial filter
// exclusion, or "" when the filter dropped nothing (including the
// unscoped case, where ExcludedCount is always 0). The caller decides
// where to print it — this function performs no I/O.
func (rs ReadySet) ExclusionNotice() string {
	n := rs.ExcludedCount()
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d ready bead(s) excluded by spec filter: title does not reference spec %s", n, idrender.Spec(rs.SpecFlag))
}

// containsSpecID checks if a bead title references the given spec ID —
// the step-2.5 filter both the claim and --emit-only paths apply when a
// spec scope is given.
func containsSpecID(title, specID string) bool {
	return strings.Contains(title, specID)
}

// BuildReadySet performs spec 128 R6.1's shared ready-set construction:
// the bd-ready query (global when specFlag is "", scoped to specFlag's
// epic otherwise) plus the spec-title filter — the SAME construction the
// claim path and the --emit-only path both build selection over, so
// `--pick N` names the same bead on either path for the same N (AC-9).
//
// R6.7 scope honesty (round-2 G8): when specFlag != "" and its epic can't
// be resolved — no epic (including a legitimate on-disk draft, which gets
// its own "spec exists but has no epic" refusal) or a genuine bd/discovery
// error — this REFUSES rather than silently widening to the global,
// unscoped `bd ready` query (the removed next.go:166-171/:417-424
// fallback). QueryReady (the global query) is NEVER called in that
// branch — AC-15's "no global bd ready query is issued" assertion holds
// by construction, not by convention. An operator who scoped a command to
// a spec must never receive another spec's bead because the scope
// silently widened.
//
// R6.3(c): a non-empty raw ready set that the spec-title filter reduces
// to EMPTY is a loud error naming the exclusions here — never a silent
// success the caller could mistake for "No ready work found" (AC-10b). A
// genuinely empty raw set (bd ready itself returned nothing) is NOT an
// error — ReadySet.Filtered is simply empty, and the caller's existing
// "No ready work found" handling applies.
func BuildReadySet(root, specFlag string) (ReadySet, error) {
	if specFlag == "" {
		items, err := QueryReady()
		if err != nil {
			return ReadySet{}, err
		}
		return ReadySet{Filtered: items, Raw: items}, nil
	}

	epicID, epicErr := phase.FindEpicBySpecID(specFlag)
	if epicErr != nil || epicID == "" {
		return ReadySet{}, scopeResolutionRefusal(root, specFlag, epicErr)
	}

	raw, err := QueryReadyForEpic(epicID)
	if err != nil {
		return ReadySet{}, err
	}

	var filtered []BeadInfo
	for _, item := range raw {
		if containsSpecID(item.Title, specFlag) {
			filtered = append(filtered, item)
		}
	}

	if len(filtered) == 0 && len(raw) > 0 {
		return ReadySet{}, emptyAfterFilterError(raw, specFlag)
	}

	return ReadySet{Filtered: filtered, Raw: raw, SpecFlag: specFlag}, nil
}

// emptyAfterFilterError renders R6.3(c)'s loud empty-after-filter error
// (AC-10b): the string "No ready work found" must never appear for this
// case, so this is a distinct, non-empty-set error naming every excluded
// bead and the reason — never the empty-set success path.
func emptyAfterFilterError(raw []BeadInfo, specFlag string) error {
	ids := make([]string, 0, len(raw))
	for _, item := range raw {
		ids = append(ids, idrender.Bead(item.ID))
	}
	return fmt.Errorf("%d ready bead(s) excluded by spec filter: title does not reference spec %s (%s)",
		len(raw), idrender.Spec(specFlag), strings.Join(ids, ", "))
}

// scopeResolutionRefusal builds the R6.7 explicit-scope refusal for a
// specFlag whose epic lookup failed. Three cases, distinguished the same
// way resolve.ResolveModeWithCache draws the honesty line between a
// genuine discovery failure and an honest "no epic" result:
//
//  1. A genuine bd/discovery failure (bd unreachable, Dolt down) —
//     refuses naming that failure, never silently treated as "no epic".
//  2. An existing on-disk pre-epic DRAFT (spec 128 Bead 3 made drafts
//     first-class) — refuses with the honest "no epic; not approved yet"
//     steer, naming the actual `spec approve` recovery, never a false
//     "doesn't exist" claim.
//  3. Genuinely no such spec at all — refuses naming the spec (this is
//     the residual case if a caller reaches BuildReadySet with a specFlag
//     that ResolveSpecPrefixProven did not already prove exists).
//
// Never falls back to the global `bd ready` query — the caller returns
// this error directly (AC-15).
func scopeResolutionRefusal(root, specFlag string, epicErr error) error {
	// Final-review round 1 (R5/G3): all three chained errors below can
	// carry wrapBDError'd raw bd stderr (AllEpics / draft discovery /
	// FindEpicBySpecID) — wrap each in bdRenderedError so the rendered
	// text is per-line escaped while errors.Is/As still reach the chain.
	c := phase.NewCache()
	if _, allErr := c.AllEpics(); allErr != nil {
		return fmt.Errorf("resolving --spec %s: querying epics failed: %w", idrender.Spec(specFlag), &bdRenderedError{err: allErr})
	}
	isDraft, draftErr := resolve.IsDraftSpecWithCache(c, root, specFlag)
	if draftErr != nil {
		return fmt.Errorf("resolving --spec %s: %w", idrender.Spec(specFlag), &bdRenderedError{err: draftErr})
	}
	if isDraft {
		return fmt.Errorf("spec %s exists but has no epic; run `mindspec spec approve %s` first", idrender.Spec(specFlag), idrender.Spec(specFlag))
	}
	if epicErr != nil {
		return fmt.Errorf("resolving --spec %s: %w", idrender.Spec(specFlag), &bdRenderedError{err: epicErr})
	}
	return fmt.Errorf("no spec found matching %s; run `mindspec spec list` and re-run with a listed ID", idrender.Spec(specFlag))
}

// bdRenderedError decorates a bd-command-derived error for terminal-facing
// display without severing its Unwrap chain (bead mindspec-ta30.9,
// round-1 panel fix, spec 128 R6.4/AC-11 G1).
//
// Error() returns renderBDError(err) — every line of the underlying
// error's text escaped individually via termsafe.Escape (ADR-0042)
// before rejoining, the DirtyTreeFailure house pattern
// (internal/next/guard.go) — so a hostile/control-bearing bd stderr line
// can't forge terminal control sequences once it reaches a caller's
// rendered message. Unwrap() returns the underlying error UNCHANGED, so
// errors.As/errors.Is still finds whatever the underlying error chain
// carries (a *exec.ExitError; bead.IsUnsupportedFlagError's detection)
// exactly as if this wrapper were absent.
//
// This replaces the bead-9 round-0 approach of interpolating
// renderBDError(err) into a fmt.Errorf format string via %s — that got
// the escaped TEXT right but silently discarded the chain, a regression
// relative to the pre-bead-9 %w-wrapping code that would have broken
// bead.IsUnsupportedFlagError (internal/complete's bd-version graceful
// degradation) and any BeadExists-style errors.As classification
// reached through QueryReady/QueryReadyForEpic/FetchBeadByID/
// FetchBeadAsOf/GateReadiness. Callers embed this type via %w (e.g.
// `fmt.Errorf("bd ready failed: %w", &bdRenderedError{err: err})`) so
// the OUTER error's own Unwrap reaches this wrapper, which reaches the
// original error, preserving the full multi-level chain end to end.
type bdRenderedError struct{ err error }

func (e *bdRenderedError) Error() string { return renderBDError(e.err) }
func (e *bdRenderedError) Unwrap() error { return e.err }

// renderBDError formats a bd command failure's error text for inclusion
// in a terminal-facing message, escaping EVERY line individually through
// termsafe.Escape (ADR-0042) before rejoining (bead mindspec-ta30.9,
// spec 128 R6.4/AC-11).
//
// bd's captured stderr (internal/bead's wrapBDError already folds a
// bounded excerpt into err.Error(), per RunBD/tracedOutput) is what
// makes the #146 schema-error class of failure diagnosable at all — the
// content leg of AC-11 — but that same stderr is agent-writable/
// hostile-adjacent: a compromised or hostile bd (or a shim standing in
// for it) can emit ANSI escapes, carriage returns, or other control
// bytes on stderr. Escaping per line — the DirtyTreeFailure house
// pattern (internal/next/guard.go) — keeps a single hostile line from
// forging additional lines or controlling the terminal, while leaving
// ordinary printable-ASCII diagnostic text (the actual schema-error
// wording) byte-identical. This is the RENDER boundary: the escape
// happens here, in the wrapping error message construction, not inside
// bdcli.go's capture. See bdRenderedError for the chain-preserving
// wrapper that calls this from its Error() method.
//
// Final-review round 1 (R5/G3, found by the real-chain fixture): a
// CLEAN printable-ASCII stderr line is identity under termsafe.Escape,
// so a hostile bd could still forge a standalone machine-greppable
// `recovery: <anything>` line (guard.RecoveryPrefix — the ADR-0035
// convention agents grep and paste). Any line that would read as one is
// force-quoted (strconv.Quote via termsafe's whole-string treatment is
// wrong here — the line is printable — so quote directly), making it
// visibly DATA. A genuine nested recovery line traveling through a
// bd-derived chain degrades to its quoted form: text preserved, paste
// authority stripped — the safe direction.
func renderBDError(err error) string {
	lines := strings.Split(err.Error(), "\n")
	for i, line := range lines {
		escaped := termsafe.Escape(line)
		if strings.HasPrefix(strings.TrimSpace(escaped), guard.RecoveryPrefix) {
			escaped = strconv.Quote(line)
		}
		lines[i] = escaped
	}
	return strings.Join(lines, "\n")
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
		return fmt.Errorf("invalid bead id %s: %w", idrender.Bead(id), err)
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
		return BeadInfo{}, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(id), err)
	}
	out, err := runBDFn("show", id, "--json")
	if err != nil {
		return BeadInfo{}, fmt.Errorf("bd show %s failed: %w", id, &bdRenderedError{err: err})
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
// fmt.Errorf's %w (through bdRenderedError, which preserves the chain
// while escaping the rendered text — bead mindspec-ta30.9 round-1 panel
// fix); use bead.IsUnsupportedFlagError(err, "as-of") to detect that case
// and fall back to FetchBeadByID (see internal/complete.defaultVerifyCommitted).
func FetchBeadAsOf(id, ref string) (BeadInfo, error) {
	// Gate-all-ids (ADR-0042 §1, round 9): id feeds a `bd show` argv
	// build directly — validate BEFORE any bd spawn.
	if err := idvalidate.BeadID(id); err != nil {
		return BeadInfo{}, fmt.Errorf("invalid bead id %s: %w", idrender.Bead(id), err)
	}
	out, err := runBDFn("show", id, "--as-of", ref, "--json")
	if err != nil {
		return BeadInfo{}, fmt.Errorf("bd show %s --as-of %s failed: %w", id, ref, &bdRenderedError{err: err})
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
