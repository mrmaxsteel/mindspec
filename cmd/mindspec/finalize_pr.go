package main

// Spec 121 R1-R3 (mindspec-uxl4), ADR-0041 §4, ADR-0042: the finalize-PR
// automation. `impl approve`'s protected-main finalize path
// (internal/executor's FinalizeEpic) already reroutes the epic-close
// export onto a fresh `chore/finalize-<specID>` branch and pushes it —
// this file automates the last manual step that flow left behind: opening
// (and, opt-in, merging) the PR from that branch into main.
//
// ADR-0041 §4 (the machine-owned finalize carrier): the carrier holds ONLY
// the regenerated tracker export (no reviewed code — that already merged
// via the panel-gated impl PR), so machine-OPENING its PR is always safe;
// machine-MERGING it is admissible only behind an explicit config opt-in
// PLUS affirmative green checks PLUS the head/base adoption pin, with a
// true merge commit so ancestry/net-effect consumers (the doctor
// merged-carrier suppression, internal/lifecycle's finalize-orphan scan,
// and internal/gitutil.NetEffectLanded) observe the landing. Every leg's
// failure degrades to the shipped NOTE + doctor surfacing — never fails
// or un-finalizes `impl approve` (ADR-0041 §3: DOCUMENTED-FORWARD-SAFE).
//
// ADR-0030 (executor boundary): this seam lives cmd-side, not behind
// executor.Executor — `gh` is not a git fact, and cmd sits outside the
// enforcement-package import boundary internal/lint/boundary_test.go pins,
// so a process-spawning seam here adds no enforcement-package legs.
//
// ADR-0042 (gate-all-ids + termsafe): every ID operand (specID, epicID)
// that feeds the templated head/title is idvalidate-gated ONCE, at
// runFinalizePRAutomation's entry, before ANY gh argv is constructed — a
// malformed id degrades with ZERO gh invocations (AC-20(ii)). Every
// remote-influenced string this automation surfaces (the PR URL, echoed
// check names, gh stderr/error text) renders termsafe-escaped before
// reaching output (AC-20(i)).
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/gitutil"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// Bounded per-leg timeouts (spec 121 R3; no new config surface, per
// Non-Goals — fixed constants, not user-configurable). Vars, not consts,
// so the fault-injection tests can shrink finalizePRChecksTimeout /
// finalizePRPollInterval without waiting out a real 15-minute bound.
var (
	finalizePRLegTimeout    = 60 * time.Second
	finalizePRChecksTimeout = 15 * time.Minute
	finalizePRPollInterval  = 5 * time.Second
)

// ghRunFn is the injectable gh seam (package-var *Fn, the internal/complete
// seam convention): every gh invocation the automation makes routes
// through this var, so unit tests can script per-leg responses without a
// network, and the harness `gh` recording shim
// (internal/harness/recorder.go's DefaultShimCommands) can pin the
// end-to-end argv of the default implementation below.
var ghRunFn = runGH

// runGH execs the real `gh` binary with a per-call bounded context. Errors
// carry gh's stderr (falling back to stdout, then the raw error) so
// degrade warnings above can name the actual failure — always rendered
// termsafe-escaped by the caller before reaching output.
func runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// ghAvailableFn detects gh's presence on PATH (the AC-3 degrade trigger).
var ghAvailableFn = func() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// fetchRemoteBranchFn is the R2(c) post-merge refresh seam
// (gitutil.FetchRemoteBranch by default) — package-var so tests never
// shell out to the real `origin` remote of whatever repo happens to be
// the test process's cwd.
var fetchRemoteBranchFn = gitutil.FetchRemoteBranch

// finalizePREntry is one `gh pr list --json number,state,url,headRefName,
// baseRefName` result row.
type finalizePREntry struct {
	Number      int    `json:"number"`
	State       string `json:"state"`
	URL         string `json:"url"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
}

// ghCheckEntry is one `gh pr checks --json name,state` result row.
type ghCheckEntry struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// runFinalizePRAutomation is the automation's entry point, called from
// implApproveTail's success path when result.FinalizeBranch is non-empty —
// strictly AFTER the finalize mutation chain (branch committed and pushed,
// epic closed) has durably completed (R3). Never returns an error: every
// failure degrades to a warning + the shipped NOTE (already printed by the
// caller) with the process exiting 0 (ADR-0041 §3).
func runFinalizePRAutomation(stdout, stderr io.Writer, cfg *config.Config, specID, epicID, finalizeBranch string) {
	if finalizeBranch == "" {
		return
	}

	// Gate-all-ids (ADR-0042, AC-20(ii)): gated ONCE here, before head/
	// title — the argv payload every gh call below reuses — are ever
	// computed, so a malformed id degrades with ZERO gh argv constructed.
	if err := idvalidate.SpecID(specID); err != nil {
		fmt.Fprintf(stderr, "warning: finalize-PR automation skipped — spec id %s is invalid: %s\n", termsafe.Escape(specID), termsafe.Escape(err.Error()))
		return
	}
	if epicID == "" {
		fmt.Fprintln(stderr, "warning: finalize-PR automation skipped — no epic id available for the templated PR title")
		return
	}
	if err := idvalidate.BeadID(epicID); err != nil {
		fmt.Fprintf(stderr, "warning: finalize-PR automation skipped — epic id %s is invalid: %s\n", termsafe.Escape(epicID), termsafe.Escape(err.Error()))
		return
	}
	head, err := workspace.FinalizeBranch(specID)
	if err != nil {
		fmt.Fprintf(stderr, "warning: finalize-PR automation skipped — could not derive the finalize branch name: %s\n", termsafe.Escape(err.Error()))
		return
	}

	r := &finalizePRRunner{
		stdout: stdout, stderr: stderr, cfg: cfg,
		specID: specID, epicID: epicID,
		head: head, base: "main",
	}
	r.run()
}

// finalizePRRunner holds one automation invocation's context.
type finalizePRRunner struct {
	stdout, stderr io.Writer
	cfg            *config.Config
	specID, epicID string
	head, base     string
}

// run drives R1 (open/adopt), R2 (opt-in merge), and R3 (degrade +
// reconcile-by-query) in sequence. Every early return leaves the shipped
// NOTE (printed by the caller before this runs) as the operator's manual
// fallback — this function only ever ADDS output, never suppresses it.
func (r *finalizePRRunner) run() {
	if !r.cfg.AutoOpenFinalizePR {
		if r.cfg.AutoMergeFinalizePR {
			// R1/R2 boundary (a): auto_merge_finalize_pr is inert without
			// auto_open_finalize_pr — no PR is opened, adopted, or merged,
			// and no pre-existing operator PR is ever touched.
			fmt.Fprintln(r.stderr, "warning: auto_merge_finalize_pr is inert while auto_open_finalize_pr is false — no finalize PR is opened, adopted, or merged automatically")
		}
		return
	}
	if !ghAvailableFn() {
		fmt.Fprintln(r.stderr, "warning: gh CLI not found on PATH — skipping finalize-PR automation (see the manual command in the NOTE above)")
		return
	}

	entries, lookupErr := r.ghLookup()
	if lookupErr != nil {
		r.degradeAndReconcile("existing-PR lookup", lookupErr)
		return
	}
	sameHeadBase, otherBase := classifyFinalizePREntries(entries, r.head, r.base)
	if sameHeadBase == nil && otherBase != nil {
		// Adoption pin (AC-2): a same-head PR targeting any base other
		// than main is NEVER adopted, created-around, or auto-merged.
		fmt.Fprintf(r.stdout, "NOTE: an existing PR from %s targets %s (not %s) — it is not auto-created or auto-merged; open and merge a PR from %s into %s manually.\n",
			termsafe.Escape(r.head), termsafe.Escape(otherBase.BaseRefName), r.base, termsafe.Escape(r.head), r.base)
		return
	}

	var prURL string
	switch {
	case sameHeadBase != nil && strings.EqualFold(sameHeadBase.State, "OPEN"):
		prURL = sameHeadBase.URL
		fmt.Fprintf(r.stdout, "Finalize PR already open (adopted): %s\n", termsafe.Escape(prURL))
	case sameHeadBase != nil && strings.EqualFold(sameHeadBase.State, "MERGED"):
		fmt.Fprintf(r.stdout, "Finalize PR already merged: %s\n", termsafe.Escape(sameHeadBase.URL))
		r.postMergeRefresh()
		return
	default:
		url, createErr := r.ghCreate()
		if createErr != nil {
			r.degradeAndReconcile("pr create", createErr)
			return
		}
		prURL = url
		fmt.Fprintf(r.stdout, "Auto-opened finalize PR: %s\n", termsafe.Escape(prURL))
	}

	if !r.cfg.AutoMergeFinalizePR {
		return
	}

	// R2 boundary (a): merge only a PR THIS run opened or adopted — both
	// branches above (create, and adoption of an open same-head/main PR)
	// reach here; the other-base case already returned above.
	green, checks, checksErr := r.waitForGreenChecks(prURL)
	if checksErr != nil {
		r.degradeAndReconcile("pr checks", checksErr)
		return
	}
	if !green {
		r.reportChecksNotGreen(prURL, checks)
		return
	}
	if mergeErr := r.ghMerge(prURL); mergeErr != nil {
		r.degradeAndReconcile("pr merge", mergeErr)
		return
	}
	fmt.Fprintf(r.stdout, "Auto-merged finalize PR: %s\n", termsafe.Escape(prURL))
	r.postMergeRefresh()
}

// reportChecksNotGreen renders the R2 boundary (b) outcome: zero checks
// reported is NOT green, same as any non-green check. Check names are
// remote-influenced (AC-20(i)) and rendered termsafe-escaped.
func (r *finalizePRRunner) reportChecksNotGreen(prURL string, checks []ghCheckEntry) {
	if len(checks) == 0 {
		fmt.Fprintf(r.stdout, "Finalize PR checks report none configured — left open for manual merge: %s\n", termsafe.Escape(prURL))
		return
	}
	names := make([]string, 0, len(checks))
	for _, c := range checks {
		names = append(names, fmt.Sprintf("%s=%s", termsafe.Escape(c.Name), termsafe.Escape(c.State)))
	}
	fmt.Fprintf(r.stdout, "Finalize PR checks are not all green (%s) — left open for manual merge: %s\n", strings.Join(names, ", "), termsafe.Escape(prURL))
}

// degradeAndReconcile is R3: after ANY failed leg, warn naming it, then
// query the exact head->base PR state through the SAME seam (itself
// bounded) rather than assert "unmerged" from a leg failure alone — GitHub
// may have created or merged server-side even though the client-side call
// errored (AC-21).
func (r *finalizePRRunner) degradeAndReconcile(leg string, cause error) {
	fmt.Fprintf(r.stderr, "warning: finalize-PR automation leg %q failed: %s\n", leg, termsafe.Escape(cause.Error()))

	entries, qErr := r.ghReconcile()
	if qErr != nil {
		fmt.Fprintf(r.stderr, "warning: finalize-PR reconcile query failed — state UNDETERMINED: %s\n", termsafe.Escape(qErr.Error()))
		fmt.Fprintln(r.stdout, "NOTE: could not determine whether the finalize PR was created or merged after the failure above — check GitHub manually, or open a PR with the command shown above.")
		return
	}
	picked := pickReconcileEntry(entries)
	switch {
	case picked == nil:
		fmt.Fprintln(r.stdout, "NOTE: the finalize PR does not appear to have been created or merged — open a PR with the command shown above.")
	case strings.EqualFold(picked.State, "OPEN"):
		fmt.Fprintf(r.stdout, "Finalize PR reconciled as open despite the failure above: %s\n", termsafe.Escape(picked.URL))
	case strings.EqualFold(picked.State, "MERGED"):
		fmt.Fprintf(r.stdout, "Finalize PR reconciled as merged despite the failure above: %s\n", termsafe.Escape(picked.URL))
		r.postMergeRefresh()
	default:
		fmt.Fprintln(r.stdout, "NOTE: the finalize PR does not appear to have been created or merged — open a PR with the command shown above.")
	}
}

// pickReconcileEntry prefers an OPEN entry, then a MERGED one, over the
// reconcile query's (possibly multi-row, --state all) result — an OPEN
// result is create-success, a MERGED one is merge-success (R3).
func pickReconcileEntry(entries []finalizePREntry) *finalizePREntry {
	var merged *finalizePREntry
	for i := range entries {
		e := &entries[i]
		if strings.EqualFold(e.State, "OPEN") {
			return e
		}
		if strings.EqualFold(e.State, "MERGED") && merged == nil {
			merged = e
		}
	}
	return merged
}

// classifyFinalizePREntries splits entries by head match into the
// same-head/same-base entry (the adoption candidate) and the first
// same-head/OTHER-base entry (the AC-2 adoption-pin negative case).
//
// --state all can return MULTIPLE same-head/same-base rows (a live OPEN
// adoption candidate alongside a historical CLOSED/MERGED one) — panel
// round 1 O1-1: naively keeping the LAST match picks whichever happens
// to sort last, which can be the wrong one (e.g. an old CLOSED PR
// shadowing the live OPEN candidate, missing clean adoption and skipping
// the auto-merge that run — it would still degrade safely via ghCreate's
// own already-exists reconcile, but on the wrong classification). OPEN
// is preferred whenever one is present; otherwise the last match stands
// (a lone MERGED row is itself the unambiguous terminal success state
// the caller checks for explicitly).
func classifyFinalizePREntries(entries []finalizePREntry, head, base string) (sameHeadBase, otherBase *finalizePREntry) {
	for i := range entries {
		e := &entries[i]
		if e.HeadRefName != head {
			continue
		}
		if e.BaseRefName == base {
			if sameHeadBase == nil || strings.EqualFold(e.State, "OPEN") {
				sameHeadBase = e
			}
		} else if otherBase == nil {
			otherBase = e
		}
	}
	return sameHeadBase, otherBase
}

// ghLookup is the R1 idempotency/adoption-pin check: queried by head ONLY
// (no --base filter) so a same-head PR targeting a DIFFERENT base is still
// visible to classifyFinalizePREntries (the reconcile query below,
// R3-pinned to filter by base too, would hide it).
func (r *finalizePRRunner) ghLookup() ([]finalizePREntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), finalizePRLegTimeout)
	defer cancel()
	out, err := ghRunFn(ctx, "pr", "list", "--head", r.head, "--state", "all", "--json", "number,state,url,headRefName,baseRefName")
	if err != nil {
		return nil, err
	}
	return parseFinalizePREntries(out)
}

// ghReconcile is R3's exact pinned reconcile-query shape: head AND base
// filtered, run only after a leg failure.
func (r *finalizePRRunner) ghReconcile() ([]finalizePREntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), finalizePRLegTimeout)
	defer cancel()
	out, err := ghRunFn(ctx, "pr", "list", "--head", r.head, "--base", r.base, "--state", "all", "--json", "number,state,url,headRefName,baseRefName")
	if err != nil {
		return nil, err
	}
	return parseFinalizePREntries(out)
}

// ghCreate opens the R1 templated PR: title carries the epicID
// (approve.ImplResult.EpicID) the shipped NOTE's manual command omits.
func (r *finalizePRRunner) ghCreate() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), finalizePRLegTimeout)
	defer cancel()
	title := fmt.Sprintf("chore(beads): finalize epic %s for spec %s", r.epicID, r.specID)
	body := fmt.Sprintf(
		"Finalizes epic %s for spec %s.\n\nUntil this PR merges, main's committed .beads/issues.jsonl is stale: the bd post-merge hook will keep reverting the epic-close/bead-done state in Dolt on every subsequent merge/FF.\n",
		r.epicID, r.specID,
	)
	out, err := ghRunFn(ctx, "pr", "create", "--head", r.head, "--base", r.base, "--title", title, "--body", body)
	if err != nil {
		return "", err
	}
	return firstNonEmptyLine(string(out)), nil
}

// waitForGreenChecks polls `gh pr checks` until every check is terminal or
// finalizePRChecksTimeout elapses. A checks query reporting "no checks
// reported" (the conventional gh failure on a checkless PR) or an empty
// JSON array both read as (not green, nil error) — R2 boundary (b): zero
// checks is never green, and never a leg failure either.
//
// Bounded to finalizePRChecksTimeout overall (panel round 1, codex
// G1-3 — evidence-refuted, documented here): the deadline check below
// only runs BETWEEN polls, so a slow final poll can overrun the bound by
// at most one finalizePRLegTimeout (60s) — an acceptable, bounded slop,
// never an unbounded watch.
func (r *finalizePRRunner) waitForGreenChecks(prURL string) (bool, []ghCheckEntry, error) {
	deadline := time.Now().Add(finalizePRChecksTimeout)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), finalizePRLegTimeout)
		out, err := ghRunFn(ctx, "pr", "checks", prURL, "--json", "name,state")
		cancel()
		if err != nil {
			if isNoChecksErr(err) {
				return false, nil, nil
			}
			return false, nil, err
		}
		var checks []ghCheckEntry
		trimmed := bytes.TrimSpace(out)
		if len(trimmed) > 0 {
			if uerr := json.Unmarshal(trimmed, &checks); uerr != nil {
				return false, nil, fmt.Errorf("parsing gh pr checks output: %w", uerr)
			}
		}
		if len(checks) == 0 {
			return false, nil, nil
		}
		allDone, allGreen := true, true
		for _, c := range checks {
			state := strings.ToUpper(c.State)
			switch state {
			case "PENDING", "IN_PROGRESS", "QUEUED", "":
				allDone = false
			}
			switch state {
			case "SUCCESS", "SKIPPED", "NEUTRAL":
			default:
				allGreen = false
			}
		}
		if allDone {
			return allGreen, checks, nil
		}
		if time.Now().After(deadline) {
			return false, checks, fmt.Errorf("timed out after %s waiting for finalize PR checks", finalizePRChecksTimeout)
		}
		time.Sleep(finalizePRPollInterval)
	}
}

// ghMerge merges prURL with a TRUE MERGE COMMIT — never squash or rebase
// (R2), so ancestry/net-effect consumers observe the carrier as landed.
func (r *finalizePRRunner) ghMerge(prURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), finalizePRLegTimeout)
	defer cancel()
	_, err := ghRunFn(ctx, "pr", "merge", prURL, "--merge")
	return err
}

// postMergeRefresh is R2 boundary (c): a best-effort origin/main refresh
// (mirroring the FinalizeEpic probe's own fetch discipline) so the doctor
// merged-carrier suppression and the R2(c) stale-tracker classifier both
// evaluate against real refreshed refs, not stale local ancestry. A fetch
// failure degrades to a warning — the doctor finding then clears on a
// later fetch (detection delayed, never lost).
func (r *finalizePRRunner) postMergeRefresh() {
	if err := fetchRemoteBranchFn("origin", "main"); err != nil {
		fmt.Fprintf(r.stderr, "warning: could not refresh origin/main after the finalize-PR merge (the doctor finalize-orphan finding may take another run to clear): %s\n", termsafe.Escape(err.Error()))
	}
}

// parseFinalizePREntries decodes a `gh pr list --json ...` result. An
// empty/blank body (a well-formed "no results" response) decodes to nil,
// not an error.
func parseFinalizePREntries(out []byte) ([]finalizePREntry, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var entries []finalizePREntry
	if err := json.Unmarshal(trimmed, &entries); err != nil {
		return nil, fmt.Errorf("parsing gh pr list output: %w", err)
	}
	return entries, nil
}

// firstNonEmptyLine returns the first non-blank line of s — `gh pr create`
// prints the new PR's URL as its (sole, or last) stdout line.
func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return strings.TrimSpace(s)
}

// isNoChecksErr reports whether err is gh's conventional "no checks
// configured/reported" failure on a checkless PR — degrade to (not green,
// nil error), never a leg failure (R2 boundary (b)).
func isNoChecksErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no checks reported") ||
		strings.Contains(msg, "no commit found") ||
		strings.Contains(msg, "no checks were found")
}
