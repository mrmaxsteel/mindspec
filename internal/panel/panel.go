// Package panel is the single source of truth for review-panel state
// (Spec 093 Req 6, ADR-0037).
//
// A panel is "registered" by a `panel.json` file inside its review
// directory (`review/<slug>/panel.json`), written by /ms-panel-run
// step 0. Reviewer verdicts land beside it as `<slot>-round-<N>.json`
// files. This package reads BOTH and reports their combined state;
// its read model does not mutate either. Panel verbs use the ledger and
// snapshot writers in this package.
//
// Trust boundary (ADR-0037): every input this package reads is an
// agent-writable repo artifact. Consumers (the pre-complete hook,
// `mindspec complete`'s advisory tally, `instruct --panel-state`) are
// anti-footgun devices, not anti-adversary ones — do not "fix"
// perceived forgeability at this layer.
//
// Boundary: this package is fs-only. It makes zero git, zero bd, and
// zero subprocess calls — staleness (`ReviewedHeadSHA` vs the live
// ref) and dirty-tree checks are the caller's git work. ADR-0037 (Spec 130)
// permits three calls on a SHA mismatch: rev-parse, subject-tree read, and
// porcelain.
package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// FileName is the registration file's basename inside a panel
// directory: review/<slug>/panel.json.
const FileName = "panel.json"

// Panel mirrors the panel.json schema (Spec 093 Req 6):
//
//	{"bead_id": string|null, "spec": string, "target": string,
//	 "round": int, "expected_reviewers": int,
//	 "reviewed_head_sha": string,
//	 "abandoned": bool (optional),
//	 "abandon_reason": string (required when abandoned, with who/why)}
//
// BeadID is null for non-bead targets (final-review/PR panels).
// ReviewedHeadSHA is the `git rev-parse` of the target ref at
// fan-out. On every re-panel, Round and ReviewedHeadSHA are bumped IN
// THE SAME WRITE by /ms-panel-run step 0 — the two fields move
// together by construction; this package therefore treats a
// Round/verdict-filename disagreement as a writer error to report
// (see Result.RoundMismatch), never as something to silently repair.
type Panel struct {
	BeadID            *string `json:"bead_id"`
	Spec              string  `json:"spec"`
	Target            string  `json:"target"`
	Round             int     `json:"round"`
	ExpectedReviewers int     `json:"expected_reviewers"`
	ReviewedHeadSHA   string  `json:"reviewed_head_sha"`
	SubjectDigest     string  `json:"subject_digest,omitempty"`
	Abandoned         bool    `json:"abandoned,omitempty"`
	// AbandonReason is REQUIRED (who/why) when Abandoned is true,
	// but that requirement is deliberately NOT enforced at parse
	// time: flagging a reason-less abandonment as malformed here
	// would set Registration.Err, drop the panel from ForBead, and
	// tip the gate toward fail-open — the wrong direction.
	// Enforcement belongs to the consumers: the Bead 4 decision
	// matrix's abandoned Warn and the complete-side panel_abandoned
	// audit write (Spec 093 Reqs 12/13e) surface the reason and can
	// complain when it is empty.
	AbandonReason string `json:"abandon_reason,omitempty"`

	// ApproveThresholdExpr is an optional recorded override of the N−1
	// threshold (ADR-0037 §3, amended 2026-07-07 spec 109/ADR-0040). Absent
	// or empty means "use the N−1 default" — byte-identical to every
	// pre-existing panel.json, which omits this field entirely. See
	// ApproveThreshold, the sole interpreter of this expression.
	ApproveThresholdExpr string `json:"approve_threshold,omitempty"`

	// Gate records which gate mix the panel was created from — a value
	// drawn from the five-key enum (spec_approve/plan_approve/bead/
	// final_review/adhoc, internal/config's panel.gates keys) BY
	// CONVENTION but parse-lenient like AbandonReason: an unexpected or
	// absent value is a consumer's concern, never a parse error here
	// (flagging it malformed would set Registration.Err and tip the gate
	// toward fail-open, the wrong direction). Subject derives the review roots
	// from Gate (Spec 130 R3); ApproveThreshold remains independent of it.
	// Name ("gate"), type (string), omitempty, and parse-lenience remain the
	// stable schema contract (Spec 112 R9).
	Gate string `json:"gate,omitempty"`

	// Refutations are snapshot copies of audited dismissals. Legacy entries
	// may lack finding hashes; the finding-aware pass rule interprets them.
	// Parse lenience is retained: wrong types fail registration parsing,
	// while missing fields are consumer validation concerns.
	Refutations []Refutation `json:"refutations,omitempty"`

	Roster         []RosterSlot     `json:"roster,omitempty"`
	Deferrals      []Deferral       `json:"deferrals,omitempty"`
	Rebinds        []Rebind         `json:"rebinds,omitempty"`
	Telemetry      string           `json:"telemetry,omitempty"`
	AbandonHistory []AbandonHistory `json:"abandon_history,omitempty"`
}

// Refutation records a dismissal at one round. Finding is ccr[i] or slot;
// FindingSHA256 pins raw finding text. Legacy entries can omit both. Only a
// REQUEST_CHANGES finding is dischargeable; other vetoes remain consumer checks.
type Refutation struct {
	// Slot is the reviewer slot this refutation targets — matched
	// byte-exactly (case-sensitive) against a Verdict.Slot.
	Slot string `json:"slot"`
	// Round is the round this refutation targets — matched exactly
	// against the panel's LatestRound; a round-N entry never clears a
	// later round-N+1 re-RC on the same slot.
	Round int `json:"round"`
	// Reason is the human-authored justification (who/why).
	Reason string `json:"reason,omitempty"`
	// Evidence points to supporting evidence (a commit, a comment, a doc).
	Evidence      string `json:"evidence,omitempty"`
	Finding       string `json:"finding,omitempty"`
	FindingSHA256 string `json:"finding_sha256,omitempty"`
	FindingText   string `json:"finding_text,omitempty"`
	Ground        string `json:"ground,omitempty"`
	AC            string `json:"ac,omitempty"`
	By            string `json:"by,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

// RosterSlot is one ordered reviewer identity in a round.
type RosterSlot struct {
	Slot  string `json:"slot"`
	Model string `json:"model"`
	Lens  string `json:"lens"`
}

// Deferral pins a finding to a follow-up bead. Gate-time checks belong to callers.
type Deferral struct {
	Slot          string `json:"slot"`
	Round         int    `json:"round"`
	Finding       string `json:"finding"`
	FindingSHA256 string `json:"finding_sha256"`
	FindingText   string `json:"finding_text"`
	Bead          string `json:"bead"`
	By            string `json:"by"`
	CreatedAt     string `json:"created_at"`
}

// Rebind records a same-round refresh after tool-artifact-only changes.
type Rebind struct {
	Round     int      `json:"round"`
	FromSHA   string   `json:"from_sha"`
	ToSHA     string   `json:"to_sha"`
	Paths     []string `json:"paths"`
	Reason    string   `json:"reason"`
	CreatedAt string   `json:"created_at"`
}

// AbandonHistory preserves an abandonment when create revives a panel.
type AbandonHistory struct {
	Reason      string `json:"reason"`
	AbandonedAt string `json:"abandoned_at"`
	RevivedAt   string `json:"revived_at"`
}

// ApproveThreshold is the single home of the panel-approval threshold rule
// (Spec 093 DQ5, ADR-0037 §3, amended 2026-07-07 spec 109/ADR-0040): with N
// expected reviewers the default is N−1 (one dissent tolerated) — 5-of-6 for
// the default panel. Consumers must use this method rather than hardcoding a
// second copy of the literal 6 (or 5).
//
// The optional recorded ApproveThresholdExpr can override that default FOR
// THIS PANEL ONLY: absent/empty and "n-1" (case-insensitive) both resolve to
// the N−1 default; an integer string in [1, N] resolves to that integer;
// anything else — an out-of-range integer (0, negative, or > N) or any other
// unparseable value — falls back to N−1, so a recorded 0 never yields a
// free-pass threshold of 0. This is the SOLE interpreter of the expression;
// no consumer re-parses ApproveThresholdExpr or hardcodes a second copy of
// this rule (internal/config's PanelApproveThresholdExpr resolver returns
// the raw, unresolved expression precisely so resolution happens only here).
//
// A non-positive ExpectedReviewers yields 0 regardless of the recorded
// expression (malformed registration; callers should surface it rather than
// treat it as a free pass).
func (p Panel) ApproveThreshold() int {
	if p.ExpectedReviewers <= 0 {
		return 0
	}
	fallback := p.ExpectedReviewers - 1
	expr := strings.TrimSpace(p.ApproveThresholdExpr)
	if expr == "" || strings.EqualFold(expr, "n-1") {
		return fallback
	}
	n, err := strconv.Atoi(expr)
	if err != nil || n < 1 || n > p.ExpectedReviewers {
		return fallback
	}
	return n
}

// ReviewerCountNote returns a pure, config-free advisory string for a
// caller-side surface (`mindspec config show`, the complete-gate advisory —
// spec 109 R8) noting when a panel's recorded reviewer count differs from
// the config's current default. It returns "" when recorded and
// configDefault match (the common case, including every no-panel and
// unchanged-config-default call site — nothing to say).
//
// This helper is advisory only: it never factors into PanelGateDecision (the
// gate's Allow/Block is computed from the recorded panel.json alone) and it
// takes no *config.Config — internal/panel stays a config-free leaf. The
// caller resolves configDefault (e.g. via internal/config's
// PanelExpectedReviewers) and passes it in as a plain int.
func ReviewerCountNote(recorded, configDefault int) string {
	if recorded == configDefault {
		return ""
	}
	return fmt.Sprintf(
		"panel recorded %d expected reviewer(s); current config default is %d — the panel's recorded count governs its own gate decision",
		recorded, configDefault)
}

// IsBead reports whether the panel targets a bead (BeadID non-null
// and non-empty). Final-review/PR panels have a null bead_id and are
// outside v1 hook enforcement (surfaced via --panel-state only).
func (p Panel) IsBead() bool {
	return p.BeadID != nil && *p.BeadID != ""
}

// Registration couples a parsed panel.json with the directory it
// registers. Err is non-nil when panel.json exists but could not be
// read or parsed — the file's presence still registers the panel
// (consumers decide what a malformed registration means; the gate
// must not treat it as "no panel").
type Registration struct {
	// Dir is the absolute panel directory (review/<slug>).
	Dir string
	// Panel is the parsed registration; zero-valued when Err != nil.
	Panel Panel
	// Err records a read/parse failure of panel.json, if any.
	Err error
}

// Slug returns the panel directory's basename (the <slug> in
// review/<slug>).
func (r Registration) Slug() string { return filepath.Base(r.Dir) }

// reviewSegments are the two directory-name conventions a registered
// panel directory can live under (Spec 106 Bead 4): the historical
// repo-root `review/<slug>` and the spec-scoped co-located
// `<spec-dir>/reviews/<slug>` (a sibling of workspace.RecordingDir).
// Scan globs `<root>/<seg>/*/panel.json` for BOTH, so a caller can hand
// it repo-root, worktree, AND spec-dir roots in one call and pick up
// panels under whichever convention each root uses (the literal
// `review/` does not substring-match `reviews/`, so the two are
// independent). The LAYOUT-AWARE choice of WHICH roots to scan — root
// `review/` stays live while the tree is canonical, co-located
// `reviews/` only once flat — is the CALLER's, made from
// workspace.DetectLayout: internal/panel is a dependency-clean leaf and
// performs no layout I/O of its own.
var reviewSegments = []string{"review", "reviews"}

// Scan globs `review/*/panel.json` AND `reviews/*/panel.json` under each
// root and returns one Registration per distinct panel directory,
// deduped across overlapping roots and segments (by symlink-resolved
// absolute path) and sorted by directory for determinism.
//
// Scan is fs-only and forgiving by design: nonexistent roots and
// review/reviews dirs contribute nothing, and legacy panel directories
// (BRIEF.md but no panel.json) are NOT returned — they are
// unregistered, which is exactly the fail-open contract (HC-4):
// no panel.json, no gate.
func Scan(roots ...string) []Registration {
	seen := make(map[string]bool)
	var regs []Registration
	for _, root := range roots {
		if root == "" {
			continue
		}
		for _, seg := range reviewSegments {
			matches, err := filepath.Glob(filepath.Join(root, seg, "*", FileName))
			if err != nil {
				// Only malformed patterns error; root is a literal path.
				continue
			}
			for _, m := range matches {
				dir := filepath.Dir(m)
				key := canonicalPath(dir)
				if seen[key] {
					continue
				}
				seen[key] = true
				regs = append(regs, load(dir))
			}
		}
	}
	sort.Slice(regs, func(i, j int) bool { return regs[i].Dir < regs[j].Dir })
	return regs
}

// ForBead selects registrations by the snapshot or the latest decodable ledger
// round's bead ID. Raw ledger attribution survives invalid ledger lines and an
// unparseable snapshot, so the selected registration reaches the integrity gate.
// Legacy panels without a ledger retain snapshot-only selection.
func ForBead(regs []Registration, beadID string) []Registration {
	if beadID == "" {
		return nil
	}
	var out []Registration
	for _, r := range regs {
		matches := r.Err == nil && r.Panel.IsBead() && *r.Panel.BeadID == beadID
		if !matches {
			lines, err := readDispositionLines(filepath.Join(r.Dir, dispositionsFileName))
			if err == nil {
				var latest *string
				for _, line := range lines {
					var round RoundRecord
					if json.Unmarshal(line, &round) == nil && round.Record == RecordRound {
						latest = round.BeadID
					}
				}
				matches = latest != nil && *latest == beadID
			}
		}
		if matches {
			out = append(out, r)
		}
	}
	return out
}

// load reads and parses dir/panel.json into a Registration.
func load(dir string) Registration {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	reg := Registration{Dir: abs}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		reg.Err = fmt.Errorf("read %s: %w", FileName, err)
		return reg
	}
	if err := json.Unmarshal(data, &reg.Panel); err != nil {
		reg.Err = fmt.Errorf("parse %s: %w", FileName, err)
		reg.Panel = Panel{}
		return reg
	}
	return reg
}

// canonicalPath resolves symlinks where possible so the same panel
// directory reached via different roots dedupes to one entry.
func canonicalPath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = filepath.Clean(dir)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
