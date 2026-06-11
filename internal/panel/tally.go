package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Canonical verdict strings as written by panel reviewers
// (plugins/mindspec/skills/ms-panel-run verdict-JSON contract).
// Comparison is whitespace-trimmed and case-insensitive; the
// canonical uppercase forms are what Result stores.
const (
	VerdictApprove        = "APPROVE"
	VerdictRequestChanges = "REQUEST_CHANGES"
	VerdictReject         = "REJECT"
)

// verdictFileRE matches reviewer verdict files: <slot>-round-<N>.json.
// The consolidated change list (consolidated-round-<N>.md) does not
// match (different extension); arbitrary other JSON files without the
// -round-<N> suffix are ignored.
var verdictFileRE = regexp.MustCompile(`^(.+)-round-(\d+)\.json$`)

// ConsolidatedName returns the basename of the consolidated
// concrete_changes_required list for a round, as written by
// /ms-panel-tally: consolidated-round-<N>.md.
func ConsolidatedName(round int) string {
	return fmt.Sprintf("consolidated-round-%d.md", round)
}

// Verdict is one reviewer's parsed verdict file from the latest round.
type Verdict struct {
	// File is the basename, e.g. "codex-correctness-round-2.json".
	File string
	// Slot is the reviewer slot derived from the filename
	// (everything before "-round-<N>.json").
	Slot string
	// Round is the filename-derived round number.
	Round int
	// Verdict is the canonicalized verdict string (VerdictApprove,
	// VerdictRequestChanges, VerdictReject, or the trimmed-uppercased
	// raw value if the reviewer wrote something else).
	Verdict string
	// HardBlock reports the optional `"hard_block": true` flag —
	// evidence-bearing gates (missing measurement artifacts etc.)
	// that no vote count may override.
	HardBlock bool
}

// verdictJSON is the on-disk shape; only the fields the gates consume
// are parsed (reviewers also write confidence, rationale,
// concrete_changes_required — those stay with /ms-panel-tally).
type verdictJSON struct {
	Verdict   string `json:"verdict"`
	HardBlock bool   `json:"hard_block"`
}

// Result is the tally of one panel directory: the parsed registration
// plus the verdict state of the latest round. It is a pure
// fs-derived report — the staleness inputs it carries
// (Panel.ReviewedHeadSHA, Panel.Target, Panel.BeadID) are for the
// CALLER to check against live git state; Result itself renders no
// pass/block decision (the decision matrix is the pre-complete
// hook's, Spec 093 Req 12).
type Result struct {
	// Dir is the absolute panel directory.
	Dir string

	// Panel is the parsed panel.json; nil when the directory carries
	// no panel.json (unregistered — fail-open territory, HC-4).
	Panel *Panel
	// PanelErr records a panel.json read/parse failure (the file
	// exists but is unusable). A registered-but-malformed panel must
	// not be treated as "no panel" by gating consumers.
	PanelErr error

	// LatestRound is the FILENAME-derived latest round: max(N) over
	// `*-round-<N>.json` in the directory. 0 when no verdict files
	// exist. This is authoritative over Panel.Round, which can lag
	// (the writer bumps it in /ms-panel-run step 0; reviewers write
	// files independently).
	LatestRound int
	// RoundMismatch reports Panel.Round != LatestRound, in EITHER
	// direction, whenever both a registration and at least one
	// verdict file exist. Lagging panel.json (files ahead) means
	// step 0 wasn't re-run; a leading panel.json (round bumped, no
	// new files yet) means a re-panel is in flight — in both cases
	// the tally below reflects LatestRound and must not be read as
	// the registered round's outcome.
	RoundMismatch bool

	// Verdicts are the successfully parsed verdicts of LatestRound,
	// sorted by Slot. Verdict files from earlier rounds are never
	// tallied.
	Verdicts []Verdict
	// Approves and Rejects count canonical APPROVE / REJECT verdicts
	// in Verdicts. REQUEST_CHANGES (and anything unrecognized) is
	// neither.
	Approves int
	Rejects  int
	// HardBlocks lists the slots whose verdict set hard_block: true.
	HardBlocks []string

	// Malformed lists the basenames of LatestRound verdict files
	// that exist but could not be parsed (bad JSON or missing
	// "verdict" field). A malformed verdict counts as MISSING — it
	// appears here by name and contributes nothing to Verdicts or
	// the counts.
	Malformed []string

	// HasConsolidated reports whether consolidated-round-<LatestRound>.md
	// exists (the /ms-bead-fix input for this round).
	HasConsolidated bool
}

// ExpectedReviewers returns the registered reviewer count, 0 when
// unregistered/malformed.
func (r *Result) ExpectedReviewers() int {
	if r.Panel == nil {
		return 0
	}
	return r.Panel.ExpectedReviewers
}

// MissingCount is the number of expected reviewers without a valid
// verdict in the latest round (malformed ones included — they are
// named in Malformed). Never negative.
func (r *Result) MissingCount() int {
	n := r.ExpectedReviewers() - len(r.Verdicts)
	if n < 0 {
		return 0
	}
	return n
}

// Complete reports whether every expected reviewer has a valid
// verdict in the latest round. False for unregistered or malformed
// registrations (ExpectedReviewers 0 is never "complete").
func (r *Result) Complete() bool {
	return r.ExpectedReviewers() > 0 && len(r.Verdicts) >= r.ExpectedReviewers()
}

// Tally reads a panel directory and reports its registration plus
// the verdict state of the filename-derived latest round.
//
// fs-only: zero git, zero bd, zero subprocesses. The error return is
// I/O-level only (the directory itself unreadable); per-file problems
// are reported in the Result (PanelErr, Malformed).
func Tally(dir string) (*Result, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("tally panel dir: %w", err)
	}

	res := &Result{Dir: abs}

	// Registration.
	if _, statErr := os.Stat(filepath.Join(dir, FileName)); statErr == nil {
		reg := load(dir)
		if reg.Err != nil {
			res.PanelErr = reg.Err
		} else {
			p := reg.Panel
			res.Panel = &p
		}
	}

	// Filename-derived rounds.
	type vfile struct {
		name, slot string
		round      int
	}
	var vfiles []vfile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := verdictFileRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		round, err := strconv.Atoi(m[2])
		if err != nil {
			continue // unreachable for \d+ short of overflow
		}
		vfiles = append(vfiles, vfile{name: e.Name(), slot: m[1], round: round})
		if round > res.LatestRound {
			res.LatestRound = round
		}
	}

	if res.Panel != nil && res.LatestRound > 0 {
		res.RoundMismatch = res.Panel.Round != res.LatestRound
	}

	// Tally the latest round only.
	for _, vf := range vfiles {
		if vf.round != res.LatestRound {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, vf.name))
		if err != nil {
			res.Malformed = append(res.Malformed, vf.name)
			continue
		}
		var vj verdictJSON
		if err := json.Unmarshal(data, &vj); err != nil || strings.TrimSpace(vj.Verdict) == "" {
			res.Malformed = append(res.Malformed, vf.name)
			continue
		}
		v := Verdict{
			File:      vf.name,
			Slot:      vf.slot,
			Round:     vf.round,
			Verdict:   strings.ToUpper(strings.TrimSpace(vj.Verdict)),
			HardBlock: vj.HardBlock,
		}
		res.Verdicts = append(res.Verdicts, v)
		switch v.Verdict {
		case VerdictApprove:
			res.Approves++
		case VerdictReject:
			res.Rejects++
		}
		if v.HardBlock {
			res.HardBlocks = append(res.HardBlocks, v.Slot)
		}
	}
	sort.Slice(res.Verdicts, func(i, j int) bool { return res.Verdicts[i].Slot < res.Verdicts[j].Slot })
	sort.Strings(res.Malformed)
	sort.Strings(res.HardBlocks)

	if res.LatestRound > 0 {
		if _, err := os.Stat(filepath.Join(dir, ConsolidatedName(res.LatestRound))); err == nil {
			res.HasConsolidated = true
		}
	}

	return res, nil
}
