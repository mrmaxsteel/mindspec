package next

import (
	"encoding/json"
	"errors"
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
	"github.com/mrmaxsteel/mindspec/internal/validate/readiness"
)

// BeadInfo represents a work item from Beads.
type BeadInfo struct {
	Parent    string                 `json:"parent"`
	Metadata  map[string]interface{} `json:"metadata"`
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Status    string                 `json:"status"`
	Priority  int                    `json:"priority"`
	IssueType string                 `json:"issue_type"`
	Owner     string                 `json:"owner"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
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

// ReadySet holds the global tracker rows and the ordered selectable plan beads.
type ReadySet struct {
	Filtered   []BeadInfo
	Raw        []BeadInfo
	SpecFlag   string
	EpicID     string
	NearMisses []NearMiss
	PlanNotice string
	Cache      *phase.Cache
}

func (rs ReadySet) ExcludedCount() int { return len(rs.NearMisses) }

// ExclusionNotice names every near-miss, its reason, and its recovery.
func (rs ReadySet) ExclusionNotice() string {
	var lines []string
	if rs.PlanNotice != "" {
		lines = append(lines, rs.PlanNotice)
	}
	for _, miss := range rs.NearMisses {
		lines = append(lines, miss.String())
	}
	return strings.Join(lines, "\n")
}

func containsSpecID(title, specID string) bool { return strings.Contains(title, specID) }

var findEpicForBeadFn = phase.FindEpicLineageWithCache

// BuildReadySet classifies one uncapped global ready set with the same lineage
// resolver and positional plan mapping that readiness uses.
func BuildReadySet(root, specFlag string) (ReadySet, error) {
	rs := ReadySet{SpecFlag: specFlag, Cache: phase.NewCache()}
	if specFlag != "" {
		epic, err := rs.Cache.FindEpicBySpecID(specFlag)
		if err != nil || epic == "" {
			return rs, scopeResolutionRefusal(root, specFlag, err)
		}
		rs.EpicID = epic
	}
	raw, err := QueryReady()
	if err != nil {
		return rs, err
	}
	rs.Raw = raw
	if specFlag == "" {
		rs.Filtered = raw
		return rs, nil
	}
	var plan string
	var loaded, absent bool
	for _, item := range raw {
		if strings.EqualFold(strings.TrimSpace(item.IssueType), "epic") {
			continue
		}
		n, form := bead.PlanBeadIndex(rs.EpicID, item.ID)
		info := lineageInfo{}
		if form {
			var provenance phase.LineageProvenance
			info.epic, info.spec, provenance, err = findEpicForBeadFn(rs.Cache, item.ID)
			info.nonEpicParent = provenance.NonEpicParent
			info.dependency = provenance.ParentChild && info.epic != rs.EpicID
			if err != nil && !errors.Is(err, phase.ErrNoEpicLineage) {
				return rs, fmt.Errorf("could not resolve the lineage of %s: %w\nrecovery: bd show %s --json", idrender.Bead(item.ID), RenderBDError(err), idrender.Bead(item.ID))
			}
			info.none = errors.Is(err, phase.ErrNoEpicLineage)
			// A named parent that supplied no lineage still needs inspection;
			// A missing epic binding must not cause a parent replacement.
			if item.Parent != "" && item.Parent != rs.EpicID && !info.dependency && info.epic != rs.EpicID {
				info.nonEpicParent = true
			}
			if !info.nonEpicParent && !info.dependency {
				if !loaded {
					plan, absent, rs.PlanNotice, err = readPlanForSpec(root, specFlag)
					if err != nil {
						return rs, fmt.Errorf("could not read plan.md for %s: %w\nrecovery: mindspec validate plan %s", idrender.Spec(specFlag), RenderBDError(err), idrender.Spec(specFlag))
					}
					loaded = true
				}
				if absent {
					info.why = "no plan.md exists yet for " + idrender.Spec(specFlag)
				} else {
					info.mapped, info.why = readiness.PlanMapsBead(plan, n)
				}
			}
			if isReadyPlanMember(item, rs.EpicID, info) {
				rs.Filtered = append(rs.Filtered, item)
				continue
			}
		}
		if miss, linked := classifyNearMiss(item, rs.EpicID, specFlag, n, form, info); linked {
			rs.NearMisses = append(rs.NearMisses, miss)
		}
	}
	if len(rs.Filtered) == 0 && rs.HasBlockingNearMisses() {
		return rs, emptyAfterFilterError(rs)
	}
	return rs, nil
}

// isReadyPlanMember applies the member predicate after plan-form validation.
func isReadyPlanMember(item BeadInfo, epic string, info lineageInfo) bool {
	status := strings.TrimSpace(item.Status)
	return !info.nonEpicParent && info.epic == epic && info.mapped && (status == "" || strings.EqualFold(status, "open"))
}

// EmptyAfterFilterError is loud emptiness, with a valid set whose derived state
// the command must still display before returning the refusal.
type EmptyAfterFilterError struct{ notice string }

func (e *EmptyAfterFilterError) Error() string { return e.notice }
func emptyAfterFilterError(rs ReadySet) error {
	return &EmptyAfterFilterError{notice: rs.ExclusionNotice()}
}

func (rs ReadySet) HasBlockingNearMisses() bool {
	for _, miss := range rs.NearMisses {
		if miss.Blocking {
			return true
		}
	}
	return false
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
