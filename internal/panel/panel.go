// Package panel is the single source of truth for review-panel state
// (Spec 093 Req 6, ADR-0037).
//
// A panel is "registered" by a `panel.json` file inside its review
// directory (`review/<slug>/panel.json`), written by /ms-panel-run
// step 0. Reviewer verdicts land beside it as `<slot>-round-<N>.json`
// files. This package reads BOTH and reports their combined state;
// it never writes either.
//
// Trust boundary (ADR-0037): every input this package reads is an
// agent-writable repo artifact. Consumers (the pre-complete hook,
// `mindspec complete`'s advisory tally, `instruct --panel-state`) are
// anti-footgun devices, not anti-adversary ones — do not "fix"
// perceived forgeability at this layer.
//
// Boundary: this package is fs-only. It makes zero git, zero bd, and
// zero subprocess calls — staleness (`ReviewedHeadSHA` vs the live
// ref) and dirty-tree checks are the CALLER's git work (the hook,
// per ADR-0030's at-most-two-subprocesses budget).
package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
}

// ApproveThreshold is the single home of the N−1 threshold rule
// (Spec 093 DQ5, ADR-0037): with N expected reviewers the panel
// passes on N−1 APPROVEs (one dissent tolerated) — 5-of-6 for the
// default panel. Consumers must use this method rather than
// hardcoding a second copy of the literal 6 (or 5).
// A non-positive ExpectedReviewers yields 0 (malformed registration;
// callers should surface it rather than treat it as a free pass).
func (p Panel) ApproveThreshold() int {
	if p.ExpectedReviewers <= 0 {
		return 0
	}
	return p.ExpectedReviewers - 1
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

// Scan globs `review/*/panel.json` under each root and returns one
// Registration per distinct panel directory, deduped across
// overlapping roots (by symlink-resolved absolute path) and sorted by
// directory for determinism.
//
// Scan is fs-only and forgiving by design: nonexistent roots and
// review dirs contribute nothing, and legacy panel directories
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
		matches, err := filepath.Glob(filepath.Join(root, "review", "*", FileName))
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
	sort.Slice(regs, func(i, j int) bool { return regs[i].Dir < regs[j].Dir })
	return regs
}

// ForBead filters registrations to those whose panel targets the
// given bead ID. Malformed registrations (Err != nil) never match —
// the gate's bead-scoped lookup cannot attribute them, but they
// remain visible in the full Scan result for diagnostics.
func ForBead(regs []Registration, beadID string) []Registration {
	if beadID == "" {
		return nil
	}
	var out []Registration
	for _, r := range regs {
		if r.Err == nil && r.Panel.IsBead() && *r.Panel.BeadID == beadID {
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
