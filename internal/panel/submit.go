package panel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// SubmitTarget is resolved by the adapter. File binds a direct submission to
// its attempt directory; internal callers set Via and Attempt themselves.
type SubmitTarget struct {
	Dir         string
	Spec        string
	Panel       string
	Slot        string
	Round       int
	File        string
	SlotDir     string
	Via         string
	Attempt     int
	Reason      string
	AfterAppend func() error // crash-injection seam, after append and before rename
}
type SubmitResolution struct {
	Family       string
	Model        string
	RunnerKind   string
	RunnerSource string
	Landed       bool
}
type SubmitResolver func(Panel, RosterSlot) (SubmitResolution, error)

func submitRefusal(message string) error { return verdictError("", message) }
func modelSafe(model string) bool {
	if model == "" || strings.HasPrefix(model, "-") {
		return false
	}
	for _, r := range model {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// Submit checks and mutates under one ledger lock. The resolver must not take
// that lock. The selected seq is returned even for a same-bytes repair.
func Submit(target SubmitTarget, data []byte, resolve SubmitResolver) (SubmissionRecord, error) {
	var selected SubmissionRecord
	err := withPanelDispositionLock(target.Dir, func() error {
		if submitLockHeldHook != nil {
			submitLockHeldHook()
		}
		if !layoutComponent(target.Slot) {
			return submitRefusal("invalid slot name")
		}
		v, err := ParseVerdictV2(data)
		if err != nil {
			return err
		}
		reg := load(target.Dir)
		if reg.Err != nil {
			return reg.Err
		}
		p := reg.Panel
		if p.Abandoned {
			return submitRefusal("panel is abandoned; create a new round")
		}
		if target.Panel != "" && target.Panel != filepath.Base(target.Dir) {
			return submitRefusal("panel target disagrees with its directory")
		}
		if target.Spec != "" && target.Spec != p.Spec {
			return submitRefusal("spec differs from the panel")
		}
		if target.Round != p.Round {
			return submitRefusal("round differs from the panel's current round")
		}
		if v.Slot != target.Slot {
			return verdictError("/slot", "differs from the target slot")
		}
		if v.Round != target.Round {
			return verdictError("/round", "differs from the target round")
		}
		var roster *RosterSlot
		for i := range p.Roster {
			if p.Roster[i].Slot == target.Slot {
				roster = &p.Roster[i]
				break
			}
		}
		if roster == nil {
			return submitRefusal("slot is not on the roster")
		}
		lines, err := ReadLedger(target.Dir)
		if err != nil {
			return err
		}
		var round RoundRecord
		for _, line := range lines {
			var r RoundRecord
			if err := json.Unmarshal(line, &r); err != nil {
				return err
			}
			if r.Record == RecordRound {
				round = r
			}
		}
		if round.Round != target.Round || round.VerdictSchema != 2 {
			return submitRefusal(fmt.Sprintf("round does not accept submissions; use mindspec panel create %s --round %d", filepath.Base(target.Dir), p.Round+1))
		}
		if resolve == nil {
			return submitRefusal("runner resolver is required")
		}
		effective, err := resolve(p, *roster)
		if err != nil {
			return err
		}
		if effective.Landed {
			return submitRefusal(fmt.Sprintf("panel round has landed; use mindspec panel create %s --round %d", filepath.Base(target.Dir), p.Round+1))
		}
		if !modelSafe(v.Model) || v.Model != effective.Model {
			return verdictError("/model", "must be the slot's effective model and contain no whitespace, control byte or leading dash")
		}
		via := target.Via
		if via == "" {
			via = "direct"
		}
		if via != "direct" && via != "launch" && via != "recover" {
			return submitRefusal("invalid submission source")
		}
		if via == "direct" && effective.RunnerKind == "process" {
			return submitRefusal("process slot requires mindspec panel launch or mindspec panel recover")
		}
		latest, err := LatestSubmissions(target.Dir, target.Round)
		if err != nil {
			return err
		}
		previous := latest[target.Slot]
		hash := findingHash(data)
		same := previous.Seq > 0 && previous.FileSHA256 == hash
		if !same && v.ReviewedSHA != p.ReviewedHeadSHA {
			return verdictError("/reviewed_sha", "differs from the current reviewed head")
		}
		attempts, err := SlotAttempts(target.Dir, target.Round)
		if err != nil {
			return err
		}
		events := attempts[target.Slot]
		k := 0
		var start LaunchRecord
		outcomes := map[int]LaunchRecord{}
		for _, e := range events {
			if e.Status == "started" || e.Status == "dispatched" {
				if e.Attempt >= k {
					k = e.Attempt
					start = e.LaunchRecord
				}
			} else {
				outcomes[e.Attempt] = e.LaunchRecord
			}
		}
		attempt := target.Attempt
		if via == "direct" {
			attempt = 0
			j, err := AttemptFromPath(target.SlotDir, target.File)
			if err != nil {
				return err
			}
			if j > 0 {
				if j < k {
					return submitRefusal(fmt.Sprintf("attempt %d was superseded by %d", j, k))
				}
				if j > k || k == 0 {
					return submitRefusal(fmt.Sprintf("attempt %d was not dispatched", j))
				}
				if outcome, ok := outcomes[j]; ok && outcome.Status != "submitted" {
					return submitRefusal(fmt.Sprintf("attempt %d ended with %s; use mindspec panel checkout --slot %s --dispatch --replace", j, outcome.Status, target.Slot))
				}
			}
			if start.Status == "dispatched" {
				attempt = k
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if same {
			selected = previous
		} else {
			selected = SubmissionRecord{Record: RecordVerdict, Spec: p.Spec, Panel: filepath.Base(target.Dir), Slot: target.Slot, Round: target.Round, Seq: previous.Seq + 1, Status: v.Status, Verdict: v.Verdict, Family: effective.Family, Model: effective.Model, RunnerKind: effective.RunnerKind, RunnerSource: effective.RunnerSource, Via: via, Attempt: attempt, Reason: target.Reason, FileSHA256: hash, CreatedAt: now}
			raw, err := json.Marshal(selected)
			if err != nil {
				return err
			}
			if err := appendRecordsLocked(target.Dir, raw); err != nil {
				return err
			}
		}
		if target.AfterAppend != nil {
			if err := target.AfterAppend(); err != nil {
				return err
			}
		}
		if err := writeVerdictBytes(target.Dir, target.Slot, target.Round, data); err != nil {
			return err
		}
		if via == "direct" && start.Status == "dispatched" {
			if _, ended := outcomes[k]; !ended {
				outcome := start
				outcome.Status = "submitted"
				outcome.Seq = selected.Seq
				outcome.CreatedAt = now
				raw, _ := json.Marshal(outcome)
				return appendRecordsLocked(target.Dir, raw)
			}
		}
		return nil
	})
	return selected, err
}
func writeVerdictBytes(dir, slot string, round int, data []byte) error {
	// A slot comes from the roster but is still repo-authored input.
	if !layoutComponent(slot) {
		return submitRefusal("invalid slot name")
	}
	f, err := os.CreateTemp(dir, ".verdict-*.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, fmt.Sprintf("%s-round-%d.json", slot, round)))
}
