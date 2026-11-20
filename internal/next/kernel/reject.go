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
	}{{"supersede", supersede}, {"refused-round", refusedRound}} {
		if err := RegisterRule(r.name, Ordinary, r.fn); err != nil {
			panic(err)
		}
	}
}

func decisionWrite(s State, gate, cause string, round int) []Write {
	t, _ := kernelid.Parse(gate)
	id := fmt.Sprintf("%s.decide.%s.r%d", t.Gate, cause, round)
	if _, exists := s.Records[id]; exists {
		return nil
	}
	options := []string{"fix", "halt"}
	if cause == "limit" {
		options = []string{"continue:<n>", "halt"}
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

func openFix(s State, gate string) bool {
	if r := s.ExcludedStep; r != nil {
		t, _, _ := r.Tuple()
		if t.Gate == gate && t.Kind == "fix" && r.Status != "closed" {
			return true
		}
	}

	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if t.Gate == gate && t.Kind == "fix" && r.Status != "closed" {
			return true
		}
	}
	return false
}

// supersede replaces an unfinished review when its subject moves outside a fix.
func supersede(s State) (Changes, error) {
	c := Changes{}
	for _, gate := range s.Gates {
		g := s.Records[gate]
		t, _, _ := g.Tuple()
		if gate != activeGate(s, t.Gate) || g.Status == "closed" || openFix(s, t.Gate) || gateDecision(s, t.Gate, true) {
			continue
		}
		round, has, err := Decode[GateRound](g.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		reg, registered, err := Decode[RegistrationRecord](g.Metadata, KeyRegistration)
		if err != nil {
			return c, err
		}
		head := subject(s, gate)
		// Replaying an implementation result cannot replace its existing review.
		if r := s.ExcludedStep; r != nil {
			step, _, _ := r.Tuple()
			if step.Kind == "implement" && step.WorkItem == t.WorkItem {
				continue
			}
		}
		if !has || !registered || reg.Round != round.Round || reg.Subject != round.Subject || head == "" || head == round.Subject {
			continue
		}
		// A fix or decision keeps control of the refused round, even after
		// a raw close or an answer. Only a passed fix can open its round.
		_, hasFix := s.Records[fmt.Sprintf("%s.r%d.fix", t.Gate, round.Round+1)]
		held := hasFix
		if r := s.ExcludedStep; r != nil {
			st, _, _ := r.Tuple()
			held = held || (st.Gate == t.Gate && ((st.Kind == "decision" && st.Round == round.Round) || (st.Kind == "fix" && st.Round == round.Round+1)))
		}
		for _, id := range s.Steps {
			st, _, _ := s.Records[id].Tuple()
			if st.Kind == "decision" && st.Gate == t.Gate && st.Round == round.Round {
				held = true
				break
			}
		}
		if held {
			continue
		}
		superseded := false
		for _, id := range s.Steps {
			r := s.Records[id]
			st, _, _ := r.Tuple()
			if st.Kind != "review-slot" || st.Gate != t.Gate || st.Round != round.Round || r.Status == "closed" {
				continue
			}
			if hasOpenChild(s, id) {
				held = true
				continue
			}
			receipt := Receipt{Kind: "superseded", Subject: head}
			c.Writes = append(c.Writes, Write{Kind: "close", ID: id, Receipt: &receipt})
			superseded = true
		}
		if held || !superseded {
			continue
		}
		count, err := RoundsForGate(s, gate)
		if err != nil {
			return c, err
		}
		if count >= Allowance(s, gate) {
			c.Writes = append(c.Writes, decisionWrite(s, gate, "limit", round.Round)...)
			continue
		}
		next, err := authorisedRound(s, gate, round.Round+1, head)
		if err != nil {
			return c, err
		}
		c.Writes = append(c.Writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, next)})
	}
	return c, nil
}

// refusedRound releases a fix only after a local decision has an answer.
func refusedRound(s State) (Changes, error) {
	c := Changes{}
	answers := s.Answers()
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
		answered := false
		for _, id := range s.Steps {
			d, _, _ := s.Records[id].Tuple()
			answer, ok := answers[id]
			if d.Kind == "decision" && d.Gate == t.Gate && d.Round == round.Round && ok && (answer.Option == "fix" || decisionOption(DecisionRecord{Options: []string{"continue:<n>"}}, answer.Option)) {
				c.Writes = append(c.Writes, fixWrite(s, gate, round.Round+1, round.Subject)...)
				answered = true
			}
		}
		if answered || gateDecision(s, t.Gate, true) {
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
		ready := len(round.Slots) > 0
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
			if !result.Complete() || result.RoundMismatch || result.IntegrityErr != nil || result.PanelErr != nil || result.PassRule(true) {
				continue
			}
			count, err := RoundsForGate(s, gate)
			if err != nil {
				return c, err
			}
			if count >= Allowance(s, gate) {
				c.Writes = append(c.Writes, decisionWrite(s, gate, "limit", round.Round)...)
			} else {
				c.Writes = append(c.Writes, fixWrite(s, gate, round.Round+1, round.Subject)...)
				if result.Rejects > 0 {
					c.Writes = append(c.Writes, decisionWrite(s, gate, "reject", round.Round)...)
				}
			}
		}
	}
	return c, nil
}
