package kernel

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/panel"
)

const decisionTitle = "decision: halt stops the spec and every work item"

func init() {
	for _, r := range []struct {
		name string
		fn   Rule
	}{{"refused-round", refusedRound}, {"verify-apply", verifyApply}} {
		if err := RegisterRule(r.name, Ordinary, r.fn); err != nil {
			panic(err)
		}
	}
}

func decisionWrite(s State, gate, cause string, round int) []Write {
	t, _ := kernelid.Parse(gate)
	id := fmt.Sprintf("%s.decide.%s.r%d", t.Gate, cause, round)
	if _, exists := s.Records[id]; exists {
		if cause != "uncounted" && cause != "limit" {
			return nil
		}
		for {
			if cause == "limit" {
				r := s.Records[id]
				receipt, _, _ := Decode[Receipt](r.Metadata, KeyClose)
				if r.Status != "closed" || receipt.Kind != "superseded" {
					return nil
				}
			}
			next, err := nextAttempt(id)
			if err != nil {
				return nil
			}
			if _, exists = s.Records[next]; !exists {
				id = next
				break
			}
			id = next
		}
	}
	options := []string{"fix", "halt"}
	if cause == "limit" {
		options = []string{"continue:<n>", "halt"}
	}
	if cause == "uncounted" {
		options = []string{"redispatch", "halt"}
	}
	tuple, _ := kernelid.Parse(id)
	return []Write{{Kind: "create", ID: id, Create: &CreateRecord{ID: id, Parent: gate, Type: "decision", Title: decisionTitle, Tuple: tuple, Metadata: Metadata(KeyDecision, DecisionRecord{Gate: t.Gate, Cause: cause, Options: options})}}}
}

func fixWrite(s State, gate string, round int, subject string) []Write {
	t, _ := kernelid.Parse(gate)
	id := fmt.Sprintf("%s.r%d.fix", t.Gate, round)
	if _, exists := s.Records[id]; exists {
		return nil
	}
	w := createStep(id, gate)
	w.Create.Metadata = Metadata(KeySubject, Subject{Commit: subject})
	return []Write{w}
}

// refusedRound creates a verify only after a complete refused round. An
// uncounted slot instead gets another attempt, bounded by a local decision.
func refusedRound(s State) (Changes, error) {
	c := Changes{}
	for _, gate := range s.Gates {
		g := s.Records[gate]
		t, _, _ := g.Tuple()
		if gate != activeGate(s, t.Gate) || g.Status == "closed" {
			continue
		}
		round, has, err := Decode[GateRound](g.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		if !has {
			continue
		}
		if held, err := tallyHeldForJudge(s, gate, round); err != nil {
			return c, err
		} else if held {
			continue
		}
		if VerifyApplied(s, gate, round) || gateDecision(s, t.Gate, true) {
			continue
		}
		if s.ExcludedStep != nil && s.ExcludedStep.Key() == round.Verify {
			continue
		}
		reg, registered, err := Decode[RegistrationRecord](g.Metadata, KeyRegistration)
		if err != nil {
			return c, err
		}
		if !registered || reg.Round != round.Round || reg.Subject != round.Subject {
			continue
		}
		// A superseded slot cannot supply a refused tally on a later pass.
		// The step a verb omits is still open: its lineage head falls back to
		// a closed predecessor, so its round is not complete in this pass.
		ready := len(round.Slots) > 0 && !omitsRoundSlot(s, t.Gate, round.Round)
		for _, slot := range round.Slots {
			r, err := SlotLineageHead(s, slot)
			ready = ready && err == nil && passed(s, r)
		}
		if !ready {
			continue
		}
		for _, p := range s.Panels {
			if p.Slug() != reg.Slug || p.Panel.Abandoned || p.Panel.Round != round.Round || p.Panel.ReviewedHeadSHA != round.Subject {
				continue
			}
			result, err := panel.Tally(p.Dir)
			if err != nil {
				return c, err
			}
			if result.RoundMismatch || result.Intact() != nil {
				continue
			}
			uncounted := uncountedSlots(result)
			if len(uncounted) > 0 || !result.Complete() {
				if round.Verify != "" {
					c.Writes = append(c.Writes, retireVerify(s, round.Verify)...)
					round.Verify = ""
					c.Writes = append(c.Writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)})
				}
				more, err := retryUncounted(s, gate, round, uncounted)
				if err != nil {
					return c, err
				}
				c.Writes = append(c.Writes, more...)
				continue
			}
			findings, read, err := roundFindings(s, result)
			if err != nil {
				return c, err
			}
			filtered := FilterDispositions(s, t.Gate, result)
			passing := filtered.PassRule(true)
			var sources VerifyVerdictSources
			if round.Verify != "" {
				v := s.Records[round.Verify]
				pin, _, err := Decode[Closing](v.Metadata, KeyClosing)
				if err != nil {
					return c, err
				}
				returns, _, err := Decode[[]VerifyReturn](v.Metadata, KeyReturns)
				if err != nil {
					return c, err
				}
				sub, _, _ := Decode[Subject](v.Metadata, KeySubject)
				if pin.Conflict == nil && sub.Commit == reviewSubject(round) && (!passed(s, v) || returnsCover(findings, returns)) {
					continue
				}
				c.Writes = append(c.Writes, retireVerify(s, round.Verify)...)
				if passing {
					round.Verify = ""
					c.Writes = append(c.Writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)})
					continue
				}
				if pin.Conflict != nil {
					sources, _, err = Decode[VerifyVerdictSources](v.Metadata, KeyVerifyVerdicts)
					if err != nil {
						return c, err
					}
					findings, _, err = Decode[[]Finding](v.Metadata, KeyFindings)
					if err != nil {
						return c, err
					}
					read, _, err = Decode[map[string]string](v.Metadata, KeyRead)
					if err != nil {
						return c, err
					}
				}
			}
			if passing {
				continue
			}
			if len(findings) == 0 {
				return Changes{}, actorFailure("internal error: a complete refused round has no findings")
			}
			id := fmt.Sprintf("%s.r%d.verify", t.Gate, round.Round)
			previous := ""
			for {
				if _, exists := s.Records[id]; !exists {
					break
				}
				previous = id
				id, err = nextAttempt(id)
				if err != nil {
					return c, err
				}
			}
			w := createStep(id, gate)
			if sources == nil {
				sources = VerifyVerdictSources{}
				for _, verdict := range append(append([]panel.Verdict{}, result.Verdicts...), result.Unexpected...) {
					sources[verdict.Slot] = VerifyVerdictSource{Verdict: verdict.Verdict, FileSHA256: verdict.FileSHA256}
				}
			}
			w.Create.Metadata = map[string]any{KeySubject: Subject{Commit: reviewSubject(round)}, KeyFindings: findings, KeyRead: read, KeyVerifyVerdicts: sources}
			if previous != "" {
				w.Create.Metadata[KeyFrom] = From(previous)
			}
			round.Verify = id
			c.Writes = append(c.Writes, w, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)})
		}
	}
	return c, nil
}

// omitsRoundSlot reports whether the pass omits a slot step of the round.
func omitsRoundSlot(s State, gate string, round int) bool {
	if s.ExcludedStep == nil {
		return false
	}
	t, has, err := s.ExcludedStep.Tuple()
	return err == nil && has && t.Kind == "review-slot" && t.Gate == gate && t.Round == round
}

func retireVerify(s State, id string) []Write {
	r, exists := s.Records[id]
	if !exists {
		return nil
	}
	if r.Status == "closed" || hasOpenChild(s, id) || hasOpenBlocker(s, id) {
		return nil
	}
	return []Write{{Kind: "close", ID: id, Receipt: &Receipt{Kind: "superseded", Retally: "retired-verify"}}}
}

func uncountedSlots(res *panel.Result) map[string]bool {
	counted := map[string]bool{}
	for _, v := range res.Verdicts {
		counted[v.Slot] = !v.Invalid && (v.Verdict == panel.VerdictApprove || v.Verdict == panel.VerdictReject || v.Verdict == panel.VerdictRequestChanges)
	}
	uncounted := map[string]bool{}
	for _, slot := range res.Panel.Roster {
		if !counted[slot.Slot] {
			uncounted[slot.Slot] = true
		}
	}
	return uncounted
}

func retryUncounted(s State, gate string, round GateRound, uncounted map[string]bool) ([]Write, error) {
	t, _, _ := s.Records[gate].Tuple()
	if omitted := s.ExcludedStep; omitted != nil {
		d, _, _ := omitted.Tuple()
		if d.Kind == "decision" && d.Cause == "uncounted" && d.Gate == t.Gate && d.Round == round.Round {
			return nil, nil
		}
	}
	baseline := map[string]int{}
	var latest Record
	latestAttempt := 0
	for _, id := range s.Steps {
		r := s.Records[id]
		d, _, _ := r.Tuple()
		if d.Kind != "decision" || d.Cause != "uncounted" || d.Gate != t.Gate || d.Round != round.Round {
			continue
		}
		if d.Attempt > latestAttempt {
			latest, latestAttempt = r, d.Attempt
		}
		if answer, ok := s.Answers()[id]; ok && answer.Option == "redispatch" {
			record, _, _ := Decode[DecisionRecord](r.Metadata, KeyDecision)
			for _, slot := range record.Slots {
				baseline[slot.Slot] = max(baseline[slot.Slot], slot.Attempt)
			}
		}
	}
	if latestAttempt > 0 {
		answer, answered := s.Answers()[latest.Key()]
		if !answered {
			return nil, nil
		}
		if answer.Option == "redispatch" {
			d, _, _ := Decode[DecisionRecord](latest.Metadata, KeyDecision)
			pending := false
			for _, slot := range d.Slots {
				root := fmt.Sprintf("%s.r%d.%s", t.Gate, round.Round, slot.Slot)
				head, err := SlotLineageHead(s, root)
				if err != nil {
					return nil, err
				}
				ht, _, _ := head.Tuple()
				pending = pending || ht.Attempt == slot.Attempt && uncounted[slot.Slot]
			}
			if pending {
				selected := map[string]bool{}
				for _, slot := range d.Slots {
					selected[slot.Slot] = uncounted[slot.Slot]
				}
				return uncountedAttemptWrites(s, gate, round, selected)
			}
		}
	}
	var capped []DecisionSlot
	for _, root := range round.Slots {
		head, err := SlotLineageHead(s, root)
		if err != nil {
			return nil, err
		}
		ht, _, _ := head.Tuple()
		slot := fmt.Sprintf("R%d", ht.Slot)
		if !uncounted[slot] {
			continue
		}
		count := 0
		for _, id := range s.Steps {
			r := s.Records[id]
			st, _, _ := r.Tuple()
			if st.Kind == "review-slot" && st.Gate == t.Gate && st.Round == round.Round && st.Slot == ht.Slot && st.Attempt > baseline[slot] && passed(s, r) {
				count++
			}
		}
		if count >= 3 {
			capped = append(capped, DecisionSlot{Slot: slot, Attempt: ht.Attempt})
		}
	}
	if len(capped) > 0 {
		writes := decisionWrite(s, gate, "uncounted", round.Round)
		for i := range writes {
			d, _, _ := Decode[DecisionRecord](writes[i].Create.Metadata, KeyDecision)
			d.Slots = capped
			writes[i].Create.Metadata[KeyDecision] = d
		}
		return writes, nil
	}
	return uncountedAttemptWrites(s, gate, round, uncounted)
}

func uncountedAttemptWrites(s State, gate string, round GateRound, selected map[string]bool) ([]Write, error) {
	var writes []Write
	for _, root := range round.Slots {
		head, err := SlotLineageHead(s, root)
		if err != nil {
			return nil, err
		}
		t, _, _ := head.Tuple()
		if !selected[fmt.Sprintf("R%d", t.Slot)] {
			continue
		}
		next, err := nextAttempt(head.Key())
		if err != nil {
			return nil, err
		}
		w := createStep(next, gate)
		w.Create.Metadata = map[string]any{KeyFrom: From(head.Key()), KeySubject: Subject{Commit: round.Subject}, KeyUncounted: true}
		writes = append(writes, w)
	}
	return writes, nil
}
