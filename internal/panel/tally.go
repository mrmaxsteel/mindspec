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

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
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

// verdictFileRE matches reviewer verdict files: <slot>-round-<N>.json
// where N >= 1 (rounds are 1-based; a `-round-0.json` file is a
// nonconforming writer and must NOT match — if it did, LatestRound
// would be 0, which reads as "no verdict files", and a directory of
// round-0 APPROVEs would present as a clean, mismatch-free state in
// the gate-passing direction). The consolidated change list
// (consolidated-round-<N>.md) does not match (different extension);
// arbitrary other JSON files without the -round-<N> suffix are
// ignored.
var verdictFileRE = regexp.MustCompile(`^(.+)-round-([1-9][0-9]*)\.json$`)

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
	HardBlock  bool
	Invalid    bool
	Findings   []Finding
	SlotSHA256 string
}

// verdictJSON validates the core verdict fields before finding payload parsing.
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
	PanelErr      error
	IntegrityErr  error
	Unexpected    []Verdict
	Unsubmitted   []string
	NotIn         []ExcludedVerdict
	VerdictSchema int

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
	// that exist but could not be read or parsed. They appear here by
	// name and contribute nothing to Verdicts or the counts. An
	// unparseable roster verdict counts as MISSING.
	Malformed []string

	// HasConsolidated reports whether consolidated-round-<LatestRound>.md
	// exists (the /ms-bead-fix input for this round).
	HasConsolidated bool
}

// ExcludedVerdict identifies a parsed roster file that cannot count.
type ExcludedVerdict struct{ File, Reason string }

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
	if r.ExpectedReviewers() <= 0 || len(r.Verdicts) < r.ExpectedReviewers() {
		return false
	}
	if r.Panel != nil {
		for _, slot := range r.Panel.Roster {
			present := false
			for _, verdict := range r.Verdicts {
				if verdict.Slot == slot.Slot {
					present = true
					break
				}
			}
			if !present {
				return false
			}
		}
	}
	return true
}

// UnresolvedVerdicts returns the latest-round verdicts whose canonical
// Verdict is neither VerdictApprove nor VerdictReject — i.e. REQUEST_CHANGES
// plus anything unrecognized, exactly the "neither" set the Approves/Rejects
// doc comment above names. REJECTs are deliberately excluded: they are leg
// (9)'s business (PanelGateDecision) and must never be treated as merely
// "unresolved" even if leg ordering were ever edited. Verdicts is already
// slot-sorted (see Tally), so the returned slice — and every message built
// from it — is deterministic (Spec 114 R1).
//
// Only REQUEST_CHANGES findings can be discharged. Matching uses the latest
// round and current finding hashes; vetoes remain independently blocking.
func (r *Result) UnresolvedVerdicts() []Verdict {
	var out []Verdict
	for _, v := range r.Verdicts {
		if v.Verdict == VerdictApprove || v.Verdict == VerdictReject {
			continue
		}
		if v.Verdict == VerdictRequestChanges && r.isRefuted(v.Slot) {
			continue
		}
		out = append(out, v)
	}
	return out
}

// isRefuted requires every current finding to have an applicable refutation.
func (r *Result) isRefuted(slot string) bool {
	for _, v := range r.Verdicts {
		if v.Slot == slot {
			return len(r.unresolvedFindings(v)) == 0 && v.Verdict == VerdictRequestChanges && !v.HardBlock && !v.Invalid
		}
	}
	return false
}

// AppliedRefutations reports each effective entry, deduplicated by slot,
// round, finding and hash. A partially resolved slot still has effective entries.
func (r *Result) AppliedRefutations() []Refutation {
	if r.Panel == nil {
		return nil
	}
	var out []Refutation
	seen := map[string]bool{}
	for _, v := range r.Verdicts {
		if v.Verdict != VerdictRequestChanges || v.HardBlock || v.Invalid {
			continue
		}
		for _, ref := range r.Panel.Refutations {
			if !r.refutationApplies(v, ref) {
				continue
			}
			key := fmt.Sprintf("%s:%d:%s:%s", ref.Slot, ref.Round, ref.Finding, ref.FindingSHA256)
			if !seen[key] {
				seen[key] = true
				out = append(out, ref)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Slot < out[j].Slot })
	return out
}

// VoteVerdict is the deterministic vote-only gate outcome of a tally
// (Spec 093 Req 12). It is the subset of the pre-complete hook's decision
// that depends ONLY on fs-derived panel state — registration validity,
// round consistency, verdict completeness, vetoes and discharged findings — and deliberately EXCLUDES the staleness (reviewed_head_sha)
// and dirty-tree checks, which require git the hook owns. The complete-side
// advisory tally (Req 13d) renders this so its "gate would PASS/BLOCK" line
// is computed by the SAME tally the hook consumes and can never disagree on
// the vote portion.
type VoteVerdict int

const (
	// VotePass means all vote terms hold, independently of freshness.
	VotePass VoteVerdict = iota
	// VoteBlock means the vote portion would block (incomplete, REJECT,
	// hard_block, unresolved findings, round mismatch, or malformed registration).
	VoteBlock
	// VoteAbandoned means the panel is a recorded abandonment — the gate
	// passes with a warning, not a block.
	VoteAbandoned
)

// VoteDecision renders the vote-only gate outcome plus a one-line summary
// (Spec 093 Req 13d). Staleness and dirty-tree are NOT considered here — a
// VotePass is necessary but not sufficient for the hook to Pass.
func (r *Result) VoteDecision() (VoteVerdict, string) {
	if r.Panel == nil {
		if r.PanelErr != nil {
			return VoteBlock, "panel registration unreadable"
		}
		return VotePass, "no registered panel"
	}
	p := r.Panel
	if err := r.intactError(); err != nil {
		return VoteBlock, err.Error()
	}
	round := r.LatestRound
	if round == 0 {
		round = p.Round
	}
	if p.Abandoned {
		reason := strings.TrimSpace(p.AbandonReason)
		if reason == "" {
			reason = "(no abandon_reason recorded)"
		}
		// R4: AbandonReason is agent-writable free text from panel.json —
		// escape before rendering.
		return VoteAbandoned, fmt.Sprintf("round %d abandoned: %s", round, termsafe.Escape(reason))
	}
	n := p.ExpectedReviewers
	if r.RoundMismatch {
		return VoteBlock, fmt.Sprintf("panel.json round %d disagrees with verdict files round %d", p.Round, r.LatestRound)
	}
	if !r.Complete() {
		return VoteBlock, fmt.Sprintf("round %d incomplete: %d/%d verdicts present", round, len(r.Verdicts), n)
	}
	if r.Rejects > 0 || len(r.HardBlocks) > 0 || r.InvalidCount() > 0 || r.unexpectedVeto() {
		return VoteBlock, fmt.Sprintf("round %d: %s recorded (%d/%d APPROVE)%s%s", round, r.vetoSummary(), r.Approves, n, r.unexpectedVetoNames(), r.invalidNames())
	}
	if !r.PassRule(true) {
		return VoteBlock, fmt.Sprintf("round %d: unresolved non-APPROVE verdict(s) from %s — %d/%d APPROVE", round, r.UnresolvedNames(), r.Approves, n)
	}
	return VotePass, fmt.Sprintf("round %d: %d/%d APPROVE, %d resolved", round, r.Approves, n, r.ResolvedCount())
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
			field, checkErr := CheckHandEdits(dir, p)
			if checkErr != nil {
				res.IntegrityErr = checkErr
			} else if field != "" {
				res.IntegrityErr = fmt.Errorf("%s", field)
			}
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

	round, schemaErr := RoundSchema(dir)
	if schemaErr != nil {
		res.IntegrityErr = schemaErr
	}
	res.VerdictSchema = round.VerdictSchema
	var submissions map[string]SubmissionRecord
	var attempts map[string][]SlotAttempt
	if round.VerdictSchema == 2 {
		res.RoundMismatch = res.RoundMismatch || res.Panel != nil && res.Panel.Round != round.Round
		submissions, err = LatestSubmissions(dir, res.LatestRound)
		if err != nil {
			return nil, err
		}
		attempts, err = SlotAttempts(dir, res.LatestRound)
		if err != nil {
			return nil, err
		}
	}
	roster := map[string]bool{}
	if res.Panel != nil {
		for _, slot := range res.Panel.Roster {
			roster[slot.Slot] = true
		}
	}
	// Tally the latest round only.
	for _, vf := range vfiles {
		if vf.round != res.LatestRound {
			continue
		}
		offRoster := len(roster) > 0 && !roster[vf.slot]
		data, err := os.ReadFile(filepath.Join(dir, vf.name))
		if err != nil {
			res.Malformed = append(res.Malformed, vf.name)
			continue
		}
		if round.VerdictSchema == 2 && !offRoster {
			sub := submissions[vf.slot]
			if sub.Seq == 0 {
				res.Unsubmitted = append(res.Unsubmitted, vf.name)
				continue
			}
			reason := ""
			switch {
			case findingHash(data) != sub.FileSHA256:
				reason = "file digest differs from the latest submission"
			case sub.Verdict == "VOID":
				reason = "void"
			case sub.Status != "final":
				reason = "draft"
			case !submissionClosed(sub, attempts[vf.slot]):
				reason = "later attempt is not closed"
			}
			if reason != "" {
				res.NotIn = append(res.NotIn, ExcludedVerdict{File: vf.name, Reason: reason})
				continue
			}
		}
		var vj verdictJSON
		if err := json.Unmarshal(data, &vj); err != nil {
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
		var payload FindingVerdict
		payloadErr := json.Unmarshal(data, &payload)
		if round.VerdictSchema == 2 {
			if parsed, parseErr := ParseVerdictV2(data); parseErr == nil {
				payload, payloadErr = parsed.FindingVerdict(), nil
			} else if !offRoster {
				res.Malformed = append(res.Malformed, vf.name)
				continue
			}
		}
		if payloadErr != nil {
			v.Invalid = v.Verdict == VerdictRequestChanges
		} else {
			findings, err := Findings(payload)
			v.Invalid = err != nil
			v.Findings = findings
			if slot, err := SlotFinding(payload); err == nil {
				v.SlotSHA256 = slot.SHA256
			}
		}
		if offRoster {
			res.Unexpected = append(res.Unexpected, v)
			continue
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

// ReadSubmittedFindingVerdict reads only a counted v2 submission. Historical
// rounds retain their file-based finding contract.
func ReadSubmittedFindingVerdict(dir, slot string, round int) (FindingVerdict, error) {
	res, err := Tally(dir)
	if err != nil {
		return FindingVerdict{}, err
	}
	if res.VerdictSchema != 2 {
		return ReadFindingVerdict(dir, slot, round)
	}
	if res.IntegrityErr != nil {
		return FindingVerdict{}, res.IntegrityErr
	}
	for _, v := range res.Verdicts {
		if v.Slot == slot && v.Round == round {
			data, err := os.ReadFile(filepath.Join(dir, v.File))
			if err != nil {
				return FindingVerdict{}, err
			}
			parsed, err := ParseVerdictV2(data)
			return parsed.FindingVerdict(), err
		}
	}
	return FindingVerdict{}, fmt.Errorf("slot has no counted submission in this round")
}
