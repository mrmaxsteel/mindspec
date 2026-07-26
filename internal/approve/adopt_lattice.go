package approve

// Spec 127 R1(b): the adopt surface's evidence lattice.
//
// AdoptSpec (adopt.go) must never write a VERIFIED audit marker without
// positive landed evidence, and must never treat an infra failure as
// safety. This file is the aggregation: per-source three-valued results
// (positive / negative / error) over (i) the remote spec-branch ref,
// corroborated via the fetch route ONLY, and (ii) the epic's bead
// branches at bead.AllStatuses breadth — combined into one of five
// outcomes (verified / negative / sources-conflict / evidence-error /
// coverage-unavailable), each carrying its own textually-distinct
// refusal-class marker (F-r5-3) and, when it is a refusal, the
// attestation trigger bucket it belongs to.
//
// Lattice ordering, stated once here (the aggregation's actual
// implementation, evaluateAdoptLattice, follows this exactly):
//
//  1. sources-conflict — the corroborated spec ref (source i) evaluates
//     POSITIVE while some surviving bead branch (source ii) evaluates
//     NEGATIVE. This is a genuine contradiction between two INDEPENDENT
//     evidence classes (not merely "some evidence says not yet landed")
//     and gets checked, and reported, before the plain negative rule
//     below — see the design-decision note on sourcesConflictTakesPriority
//     below for why this ordering is not fully dictated by the spec text
//     and is this bead's own resolution of that ambiguity.
//  2. negative — any literally-evaluated source (source i, or any
//     surviving bead branch) is negative, and rule 1 did not already
//     fire.
//  3. evidence-error — no source is negative, but at least one is an
//     error (a git/infra failure, or a status-set-resolution failure
//     during epic enumeration — F-r5-2).
//  4. coverage-unavailable — no negative, no error, but the epic-coverage
//     clause fails: some bead of the epic (regardless of whether it has
//     a surviving branch) has no positive evidence route at all.
//  5. verified — no negative, no error, and every bead of the epic is
//     positively evidenced.
//
// Every one of AC-2(ix)'s seven table rows is a fixture against this
// exact ordering (adopt_lattice_test.go).

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/lifecycle"
	"github.com/mrmaxsteel/mindspec/internal/workspace"

	"gopkg.in/yaml.v3"
)

// adoptRemote is the corroboration remote name (spec 127 R1b(i)). Every
// other network-touching lifecycle primitive in this tree (FinalizeEpic's
// protected-main probe, RemoteHeadSHA's callers) hardcodes "origin" the
// same way — no flag is added here, matching that house convention.
const adoptRemote = "origin"

// evidenceState is the three-valued per-source result R1(b) pins:
// "NetEffectLanded returns (bool, error) and never guesses an infra
// failure into a boolean; the lattice must not either."
type evidenceState int

const (
	evidencePositive evidenceState = iota
	evidenceNegative
	evidenceError
)

func (s evidenceState) String() string {
	switch s {
	case evidencePositive:
		return "positive"
	case evidenceNegative:
		return "negative"
	case evidenceError:
		return "error"
	default:
		return "unknown-evidence-state"
	}
}

// adoptRefusalClass is the closed set of non-VERIFIED aggregate outcomes,
// each carrying its own textually-distinct marker (F-r5-3).
type adoptRefusalClass int

const (
	// adoptClassNone means the aggregate is VERIFIED — not a refusal.
	adoptClassNone adoptRefusalClass = iota
	adoptClassNegative
	adoptClassSourcesConflict
	adoptClassEvidenceError
	adoptClassCoverageUnavailable
)

// Refusal-class markers (F-r5-3, plan-pinned): distinct literal strings
// so a shared inspection-plus-attestation shape can never hide an
// any-positive-wins aggregation bug behind the coverage clause.
const (
	adoptMarkerNegative            = "adopt-evidence-negative"
	adoptMarkerSourcesConflict     = "adopt-sources-conflict"
	adoptMarkerEvidenceError       = "adopt-evidence-error"
	adoptMarkerCoverageUnavailable = "adopt-coverage-unavailable"
)

// adoptAttestTrigger is R1(c)'s pinned trigger set: the attestation
// escape is honored in EXACTLY these three aggregate states, and the
// audit marker records which one it was exercised in.
type adoptAttestTrigger string

const (
	adoptTriggerNoSource adoptAttestTrigger = "no-source"
	adoptTriggerNegative adoptAttestTrigger = "negative"
	adoptTriggerError    adoptAttestTrigger = "error"
)

// adoptLatticeResult is evaluateAdoptLattice's return value: Class ==
// adoptClassNone means VERIFIED (Marker/Trigger are unset); any other
// Class is a refusal, and — by construction of the four non-None classes
// mapping onto exactly the three pinned triggers below — attestation is
// ALWAYS available whenever Class != adoptClassNone (R1c: "honored in
// exactly three aggregate states ... never honored over an
// all-positive-with-coverage aggregate").
//
//	adoptClassNegative            -> adoptTriggerNegative
//	adoptClassSourcesConflict     -> adoptTriggerNegative (design decision, see below)
//	adoptClassEvidenceError       -> adoptTriggerError
//	adoptClassCoverageUnavailable -> adoptTriggerNoSource
type adoptLatticeResult struct {
	Class   adoptRefusalClass
	Marker  string
	Trigger adoptAttestTrigger
	Detail  string
}

// --- in-package seams (default-pinned to the real functions; tests
// override to drive the lattice from real-git/bd-free fixtures without a
// live repo or `bd`).
var (
	adoptRemoteExistsFn        = lifecycle.RemoteExistsIn
	adoptFetchRemoteBranchFn   = lifecycle.FetchRemoteBranchIn
	adoptEvaluateAgainstMainFn = evaluateAgainstMain
	adoptListEpicBeadsFn       = listEpicBeads
	adoptBranchExistsFn        = lifecycle.BranchExistsIn
	adoptFindLandedMergeFn     = lifecycle.FindLandedMerge
	adoptListBDFn              = bead.RunBD
)

// adoptEpicBead is one bd list --parent result at bead.AllStatuses
// breadth (F-r5-2: never the closed-only enumeration).
type adoptEpicBead struct {
	ID     string
	Status string
}

// evaluateAgainstMain answers "is ref's content already landed in main,
// or would merging it destroy work" via bead 1's shared predicate — the
// SAME primitive R4's merge preflight will consult (bead 6), so the
// adopt surface's evidence and the merge-safety predicate can never
// drift apart. DestructionAncestor/DestructionSuperseded both mean
// "nothing of ref's own work is missing from main" -> positive;
// DestructionClean/DestructionStaleDeletion both mean "ref carries
// content main does not have" -> negative (an ordinary clean merge is,
// for THIS question, exactly as unlanded as a stale-deletion shape — the
// only thing this predicate application cares about is "is it already
// in main", not "would merging it be destructive"); DestructionEvidenceError
// -> error, matching the err != nil case unconditionally (bead 1's
// EvaluateWorkDestruction always returns a non-nil err for that outcome).
func evaluateAgainstMain(root, ref string) (evidenceState, guard.DestructionOutcome, error) {
	outcome, _, err := lifecycle.EvaluateWorkDestruction(root, ref, "main")
	if err != nil {
		return evidenceError, outcome, err
	}
	switch outcome {
	case guard.DestructionAncestor, guard.DestructionSuperseded:
		return evidencePositive, outcome, nil
	default:
		return evidenceNegative, outcome, nil
	}
}

// resolveAdoptStatusSet is F-r5-2's plan-pinned status-set resolution,
// done strictly in internal/approve (never delegated to
// bead.CustomStatuses' own silent degrade): bead.CustomStatuses returns
// nil on BOTH a missing config.yaml (the legitimate no-customs state) AND
// an unreadable/unparseable one (a real evidence failure) — indistinguishable
// from each other by that helper's own return value. This function reads
// and YAML-parses the file itself FIRST so the two cases are told apart,
// then delegates to bead.AllStatuses(root) for the actual union (the
// parse rule keeps one home: this function only classifies "did the read
// succeed", never re-implements the union bead.AllStatuses already
// computes correctly).
func resolveAdoptStatusSet(root string) ([]string, error) {
	cfgPath := filepath.Join(root, ".beads", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Legitimate no-customs state.
			return bead.AllStatuses(root), nil
		}
		return nil, fmt.Errorf("reading %s: %w", cfgPath, err)
	}
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", cfgPath, err)
	}
	return bead.AllStatuses(root), nil
}

// listEpicBeads issues the single comma-joined `bd list --parent <epic>
// --status=<AllStatuses> -n 0 --json` (the phase.fetchChildren precedent,
// minus its cwd-degrade — F-r5-2) and returns every bead under epicID at
// FULL status breadth, built-ins plus every project custom status. A
// status-set resolution failure (a present-but-unreadable/unparseable
// config.yaml) is returned as an error — never a silently narrowed
// enumeration.
func listEpicBeads(root, epicID string) ([]adoptEpicBead, error) {
	// Gate-all-ids (ADR-0042 §1): epicID feeds a `bd list --parent` argv
	// build directly.
	if err := idvalidate.BeadID(epicID); err != nil {
		return nil, fmt.Errorf("invalid epic id %s: %w", idrender.Bead(epicID), err)
	}
	statuses, err := resolveAdoptStatusSet(root)
	if err != nil {
		return nil, fmt.Errorf("resolving epic %s's full status set: %w", idrender.Bead(epicID), err)
	}
	out, err := adoptListBDFn("list", "--parent", epicID, "--status="+strings.Join(statuses, ","), "-n", "0", "--json")
	if err != nil {
		return nil, fmt.Errorf("bd list --parent %s failed: %w", idrender.Bead(epicID), err)
	}
	var items []bead.BeadInfo
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parsing bd list --parent %s output: %w", idrender.Bead(epicID), err)
	}
	beads := make([]adoptEpicBead, 0, len(items))
	for _, it := range items {
		id := strings.TrimSpace(it.ID)
		if id == "" {
			continue
		}
		beads = append(beads, adoptEpicBead{ID: id, Status: strings.ToLower(strings.TrimSpace(it.Status))})
	}
	return beads, nil
}

// evaluateAdoptLattice is R1(b)'s aggregation. root is the workdir every
// git probe runs in; specID/epicID/specBranch are already-resolved facts
// (the caller, adopt.go's AdoptSpec, has already confirmed specBranch
// does NOT exist locally — that is R1(d)'s separate, earlier gate).
func evaluateAdoptLattice(root, specID, epicID, specBranch string) (adoptLatticeResult, error) {
	// Source (i): the remote spec-branch ref, fetch route ONLY. Every
	// evaluation below runs at the freshly fetched tip — the pre-fetch
	// tracking ref is NEVER read first, so a poisoned local cache can
	// never be mistaken for evidence (the AC-2(ix) poisoned-cache row).
	//
	// srcIAbsent (a design decision resolving an underdetermined point in
	// the spec text, flagged honestly): R1b(i) pins "absent remote
	// branch or unreachable remote is error, never positive" — but says
	// nothing about the ordinary case of a repository with NO "origin"
	// remote configured AT ALL (the AC-1 "surviving bead branches
	// covering the epic" fixture form has no reason to configure one).
	// Reading "unreachable remote" and "absent remote branch" as both
	// PRESUPPOSING a configured remote (one that fails to answer, or
	// lacks the branch), an unconfigured remote is source (i) simply not
	// EXISTING as a source — the same "no route" shape a bead with no
	// surviving branch has — never an error. Without this distinction,
	// AC-1's bead-branches-only evidence form would be unsatisfiable:
	// any-error would unconditionally dominate a from-scratch local
	// workspace with no remote at all, even with every bead positively
	// evidenced.
	corroboratedRef := adoptRemote + "/" + specBranch
	var srcIState evidenceState
	var srcIDetail string
	srcIAbsent := !adoptRemoteExistsFn(root, adoptRemote)
	if srcIAbsent {
		srcIDetail = fmt.Sprintf("no %q remote is configured", adoptRemote)
	} else if fetchErr := adoptFetchRemoteBranchFn(root, adoptRemote, specBranch); fetchErr != nil {
		// A CONFIGURED remote that is unreachable, or lacks the wanted
		// branch: error, never positive (R1b(i)) — and, per this
		// function's own ordering, never silently treated as "no
		// source" either; it is a real per-source result that
		// participates in the any-negative/any-error scan below exactly
		// like a bead-branch result would.
		srcIState = evidenceError
		srcIDetail = fmt.Sprintf("fetching %s %s: %v", adoptRemote, specBranch, fetchErr)
	} else {
		st, outcome, evalErr := adoptEvaluateAgainstMainFn(root, corroboratedRef)
		if evalErr != nil {
			srcIState = evidenceError
			srcIDetail = fmt.Sprintf("evaluating freshly-fetched %s against main: %v", corroboratedRef, evalErr)
		} else {
			srcIState = st
			srcIDetail = fmt.Sprintf("the freshly-fetched %s evaluates %s against main (%s)", corroboratedRef, st, outcome)
		}
	}

	// Source (ii): the epic's bead branches at AllStatuses breadth.
	beads, listErr := adoptListEpicBeadsFn(root, epicID)
	if listErr != nil {
		return adoptLatticeResult{
			Class:   adoptClassEvidenceError,
			Marker:  adoptMarkerEvidenceError,
			Trigger: adoptTriggerError,
			Detail:  fmt.Sprintf("could not enumerate epic %s's beads at full status breadth: %v", idrender.Bead(epicID), listErr),
		}, nil
	}

	type beadEval struct {
		id        string
		hasBranch bool
		state     evidenceState
		detail    string
	}
	evals := make([]beadEval, 0, len(beads))
	anyNegative := srcIState == evidenceNegative
	anyError := srcIState == evidenceError
	var firstNegativeDetail, firstErrorDetail string
	if anyNegative {
		firstNegativeDetail = srcIDetail
	}
	if anyError {
		firstErrorDetail = srcIDetail
	}

	for _, b := range beads {
		beadBranch, branchErr := workspace.BeadBranch(b.ID)
		if branchErr != nil {
			// A malformed id from bd is a real evidence failure, never a
			// silently-skipped bead — the coverage quantifier ranges over
			// the bd-enumerated epic, and skipping would narrow it.
			anyError = true
			d := fmt.Sprintf("epic %s's bead %s has a malformed id: %v", idrender.Bead(epicID), idrender.Bead(b.ID), branchErr)
			if firstErrorDetail == "" {
				firstErrorDetail = d
			}
			evals = append(evals, beadEval{id: b.ID, state: evidenceError, detail: d})
			continue
		}
		if !adoptBranchExistsFn(root, beadBranch) {
			evals = append(evals, beadEval{id: b.ID, hasBranch: false})
			continue
		}
		st, outcome, evalErr := adoptEvaluateAgainstMainFn(root, beadBranch)
		if evalErr != nil {
			anyError = true
			d := fmt.Sprintf("evaluating %s against main: %v", beadBranch, evalErr)
			if firstErrorDetail == "" {
				firstErrorDetail = d
			}
			evals = append(evals, beadEval{id: b.ID, hasBranch: true, state: evidenceError, detail: d})
			continue
		}
		if st == evidenceNegative {
			anyNegative = true
			d := fmt.Sprintf("bead %s's surviving branch %s is not landed in main (%s)", idrender.Bead(b.ID), beadBranch, outcome)
			if firstNegativeDetail == "" {
				firstNegativeDetail = d
			}
		}
		evals = append(evals, beadEval{id: b.ID, hasBranch: true, state: st, detail: fmt.Sprintf("%s (%s)", st, outcome)})
	}

	// Rule 1 (checked FIRST, deliberately): sources-conflict. Source i
	// positive while a surviving bead branch is negative is a
	// contradiction between two INDEPENDENT evidence classes — worse than
	// (and textually distinct from) the plain "some evidence says
	// not-yet-landed" signal the negative rule names. DESIGN DECISION,
	// flagged honestly (the spec text pins "any negative -> negative for
	// the whole" AND separately carves out this exact contradiction as
	// "sources conflict", without fully resolving which check runs first
	// when both conditions hold simultaneously): this function checks
	// sources-conflict before the general negative rule, and buckets its
	// attestation trigger as "negative" (the only one of R1c's three
	// pinned triggers a negative-bead-branch-bearing contradiction can
	// honestly be, per the round-5 ruling that negative stays in the
	// trigger set precisely because SOME negative evidence is present).
	if !srcIAbsent && srcIState == evidencePositive {
		for _, e := range evals {
			if e.hasBranch && e.state == evidenceNegative {
				return adoptLatticeResult{
					Class:   adoptClassSourcesConflict,
					Marker:  adoptMarkerSourcesConflict,
					Trigger: adoptTriggerNegative,
					Detail: fmt.Sprintf(
						"%s, but bead %s (%s)",
						srcIDetail, idrender.Bead(e.id), e.detail,
					),
				}, nil
			}
		}
	}

	if anyNegative {
		return adoptLatticeResult{
			Class:   adoptClassNegative,
			Marker:  adoptMarkerNegative,
			Trigger: adoptTriggerNegative,
			Detail:  firstNegativeDetail,
		}, nil
	}
	if anyError {
		return adoptLatticeResult{
			Class:   adoptClassEvidenceError,
			Marker:  adoptMarkerEvidenceError,
			Trigger: adoptTriggerError,
			Detail:  firstErrorDetail,
		}, nil
	}

	// No negative, no error among literally-evaluated sources. Epic
	// coverage (R1b): every bead of the epic — including one with NO
	// surviving branch — must be positively evidenced. A bead with a
	// surviving branch was already positively evaluated above (if it
	// weren't, we would have returned already). A bead with NO surviving
	// branch needs a landed-merge attribution on the corroborated spec
	// ref, evaluated through the SAME shared predicate.
	srcIPositive := !srcIAbsent && srcIState == evidencePositive
	haveAnyRoute := srcIPositive
	for _, e := range evals {
		if e.hasBranch {
			haveAnyRoute = true
			continue
		}
		if !srcIPositive {
			// No positive spec ref either: this bead has no evidence
			// route at all — the zero-source case, per-bead (B-r4-5:
			// "one-of-N surviving-and-landed is never a VERIFIED adopt").
			return adoptLatticeResult{
				Class:   adoptClassCoverageUnavailable,
				Marker:  adoptMarkerCoverageUnavailable,
				Trigger: adoptTriggerNoSource,
				Detail: fmt.Sprintf(
					"bead %s has no surviving bead branch and no corroborated remote spec-ref evidence of a landed merge",
					idrender.Bead(e.id),
				),
			}, nil
		}
		lm, lmErr := adoptFindLandedMergeFn(root, corroboratedRef, e.id)
		if lmErr != nil {
			if errors.Is(lmErr, lifecycle.ErrLandedMergeNotFound) {
				return adoptLatticeResult{
					Class:   adoptClassCoverageUnavailable,
					Marker:  adoptMarkerCoverageUnavailable,
					Trigger: adoptTriggerNoSource,
					Detail: fmt.Sprintf(
						"bead %s has no surviving bead branch and no landed merge identified for it on %s",
						idrender.Bead(e.id), corroboratedRef,
					),
				}, nil
			}
			return adoptLatticeResult{
				Class:   adoptClassEvidenceError,
				Marker:  adoptMarkerEvidenceError,
				Trigger: adoptTriggerError,
				Detail: fmt.Sprintf(
					"identifying a landed merge for bead %s on %s: %v",
					idrender.Bead(e.id), corroboratedRef, lmErr,
				),
			}, nil
		}
		st, outcome, evalErr := adoptEvaluateAgainstMainFn(root, lm.SHA)
		if evalErr != nil {
			return adoptLatticeResult{
				Class:   adoptClassEvidenceError,
				Marker:  adoptMarkerEvidenceError,
				Trigger: adoptTriggerError,
				Detail: fmt.Sprintf(
					"evaluating bead %s's identified landed merge %s against main: %v",
					idrender.Bead(e.id), lm.SHA, evalErr,
				),
			}, nil
		}
		if st != evidencePositive {
			return adoptLatticeResult{
				Class:   adoptClassCoverageUnavailable,
				Marker:  adoptMarkerCoverageUnavailable,
				Trigger: adoptTriggerNoSource,
				Detail: fmt.Sprintf(
					"bead %s's identified landed merge %s is not itself subsumed in main (%s)",
					idrender.Bead(e.id), lm.SHA, outcome,
				),
			}, nil
		}
		haveAnyRoute = true
	}

	if !haveAnyRoute {
		return adoptLatticeResult{
			Class:   adoptClassCoverageUnavailable,
			Marker:  adoptMarkerCoverageUnavailable,
			Trigger: adoptTriggerNoSource,
			Detail:  fmt.Sprintf("no ref-bearing evidence source survives for spec %s: no remote spec-branch ref and no surviving bead branch", idrender.Spec(specID)),
		}, nil
	}

	return adoptLatticeResult{
		Detail: fmt.Sprintf("verified: %s, and every bead of epic %s is positively evidenced", srcIDetail, idrender.Bead(epicID)),
	}, nil
}
