package kernel

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func init() {
	for _, r := range []struct {
		name string
		fn   Rule
	}{
		{"recreate", recreate}, {"missing-link", missingLinks}, {"tally", tally},
		{"round-open", roundOpen}, {"slots", slots}, {"implement", implement}, {"complete-entry", completeEntry},
	} {
		if err := RegisterRule(r.name, Ordinary, r.fn); err != nil {
			panic(err)
		}
	}
}

func createStep(id, parent string) Write {
	t, _ := kernelid.Parse(id)
	return Write{Kind: "create", ID: id, Create: &CreateRecord{ID: id, Parent: parent, Type: "step", Title: t.Kind, Tuple: t}}
}
func owned(s State, id string) bool {
	if id == s.Epic {
		return true
	}
	for _, item := range s.Items {
		if id == item {
			return true
		}
	}
	r, ok := s.Records[id]
	t, err := kernelid.Parse(id)
	have, present, e := r.Tuple()
	return ok && err == nil && e == nil && present && have == t && t.Epic == s.Epic
}
func passed(s State, r Record) bool {
	receipt, ok, err := Decode[Receipt](r.Metadata, KeyClose)
	if r.Status != "closed" || err != nil || !ok || receipt.Kind != "passed" || ValidReceipt(s.Root, receipt) != nil {
		return false
	}
	t, _, _ := r.Tuple()
	if t.Kind == "bead-gate" || t.Kind == "final-gate" {
		accepted, valid, err := gatePassEvidence(s, r)
		return err == nil && valid && receipt.Subject != "" && receipt.Subject == accepted && receipt.Retally != "" && receipt.Path == "" && receipt.Commit == "" && receipt.Blob == "" && receipt.Answer == nil
	}
	return true
}
func activeGate(s State, base string) string {
	best, attempt := base, 0
	for _, id := range s.Gates {
		t, _, _ := s.Records[id].Tuple()
		if t.Gate == base && t.Attempt > attempt {
			best, attempt = id, t.Attempt
		}
	}
	return best
}
func itemDone(s State, id string) bool {
	completed, _ := s.Records[id].Metadata[KeyCompletion].(string)
	accepted, err := AcceptedSubject(s, id+".review")
	return completed != "" && completed == accepted && err == nil && s.Records[id].Status == "closed" && !ForcedClose(s, id) && s.Merged[id] && CheckWorkItem(s, id) == nil && CheckPassedGate(s, id+".review") == nil
}
func upstreamsDone(s State, item string) bool {
	for _, link := range s.Links {
		if link.ID == item {
			for _, up := range link.Upstream {
				if !itemDone(s, up) {
					return false
				}
			}
		}
	}
	return true
}
func subject(s State, gate string) string {
	t, _ := kernelid.Parse(gate)
	if t.WorkItem != "" {
		branch, err := workspace.BeadBranch(t.WorkItem)
		if err != nil {
			return ""
		}
		if head := s.Refs["refs/heads/"+branch]; head != "" {
			return head
		}
		if s.Merged[t.WorkItem] {
			head, _ := s.Records[t.WorkItem].Metadata["mindspec_landed_second_parent"].(string)
			return head
		}
		return ""
	}
	branch, err := workspace.SpecBranch(s.SpecID)
	if err != nil {
		return ""
	}
	return s.Refs["refs/heads/"+branch]
}
func recreate(s State) (Changes, error) {
	c := Changes{}
	for _, item := range s.Items {
		id := item + ".review"
		if _, ok := s.Records[id]; !ok {
			c.Writes = append(c.Writes, createStep(id, item))
		}
	}
	id := s.Epic + ".final"
	if _, ok := s.Records[id]; !ok {
		c.Writes = append(c.Writes, createStep(id, s.Epic))
	}
	// A registered round retains the identities of its slots after deletion.
	for _, gate := range s.Gates {
		r := s.Records[gate]
		round, ok, err := Decode[GateRound](r.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		reg, registered, err := Decode[RegistrationRecord](r.Metadata, KeyRegistration)
		if err != nil {
			return c, err
		}
		if !ok || !registered || r.Status == "closed" || reg.Round != round.Round || reg.Subject != round.Subject {
			continue
		}
		for _, slot := range round.Slots {
			if _, exists := s.Records[slot]; !exists {
				c.Writes = append(c.Writes, createStep(slot, gate))
			}
		}
	}
	return c, nil
}
func roundOpen(s State) (Changes, error) {
	c := Changes{}
	cfg, err := config.Load(s.Root)
	if err != nil {
		return c, err
	}
	for _, gate := range s.Gates {
		g := s.Records[gate]
		t, _, _ := g.Tuple()
		if gate != activeGate(s, t.Gate) || g.Status == "closed" || gateDecision(s, t.Gate, false) {
			continue
		}
		round, has, err := Decode[GateRound](g.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		wanted := round.Round
		for _, step := range s.Steps {
			r := s.Records[step]
			st, _, _ := r.Tuple()
			if !passed(s, r) {
				continue
			}
			if st.Kind == "implement" && st.WorkItem == t.WorkItem && t.WorkItem != "" {
				wanted = max(wanted, 1)
			}
			if st.Kind == "fix" && st.Gate == t.Gate {
				wanted = max(wanted, st.Round)
			}
		}
		if wanted == 0 {
			continue
		}
		count, err := RoundsForGate(s, gate)
		if err != nil {
			return c, err
		}
		if count >= Allowance(s, gate) {
			continue
		}
		reg, registered, err := Decode[RegistrationRecord](g.Metadata, KeyRegistration)
		if err != nil {
			return c, err
		}
		if registered && reg.Round >= wanted {
			continue
		}
		head := subject(s, gate)
		resetScope := false
		if _, repanel, err := Decode[RePanel](g.Metadata, KeyRePanel); err != nil {
			return c, err
		} else if repanel && has && wanted == round.Round {
			newest, err := newestJudgeSubject(s, gate)
			if err != nil {
				return c, err
			}
			if newest == round.Subject {
				// The judgement authorizes this scope only on the subject the judge saw.
				head = round.Subject
			} else {
				// A later subject needs a full round. The scoped round is superseded.
				head = newest
				resetScope = true
			}
		}

		if head == "" {
			continue
		}
		if !has || wanted != round.Round || resetScope {
			kind := "bead"
			if t.WorkItem == "" {
				kind = "final_review"
			}
			n, err := cfg.PanelGateExpectedReviewers(kind)
			if err != nil {
				return c, err
			}
			round = GateRound{Round: wanted, Subject: head, SlotCount: n, Slots: []string{}}
			for i := 1; i <= n; i++ {
				round.Slots = append(round.Slots, fmt.Sprintf("%s.r%d.R%d", t.Gate, wanted, i))
			}
		} else {
			round.Subject = head
		}
		if old, _, _ := Decode[GateRound](g.Metadata, KeyRound); !reflect.DeepEqual(old, round) {
			c.Writes = append(c.Writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)})
		}
		target := t.WorkItem
		if target == "" {
			target = s.SpecID
		}
		c.Run = append(c.Run, Run{Actor: "binary", Argv: []string{"mindspec", "panel", "create", target}, Paths: round.Paths})
	}
	return c, nil
}
func slots(s State) (Changes, error) {
	c := Changes{}
	for _, gate := range s.Gates {
		g := s.Records[gate]
		if g.Status == "closed" {
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
		if !has || !registered || reg.Round != round.Round || reg.Subject != round.Subject || len(reg.Slots) != round.SlotCount {
			continue
		}
		t, _, _ := g.Tuple()
		for i, slot := range reg.Slots {
			if slot != fmt.Sprintf("R%d", i+1) {
				return c, fmt.Errorf("registered slots must use ordered slot identities")
			}
			id := fmt.Sprintf("%s.r%d.%s", t.Gate, round.Round, slot)
			if _, ok := s.Records[id]; !ok {
				c.Writes = append(c.Writes, createStep(id, gate))
			}
		}
	}
	return c, nil
}
func implement(s State) (Changes, error) {
	c := Changes{}
	for _, item := range s.Items {
		gate := activeGate(s, item+".review")
		g := s.Records[gate]
		_, hasRound, err := Decode[GateRound](g.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		if s.Records[item].Status == "closed" || !upstreamsDone(s, item) || hasRound || passed(s, g) || blocked(s, item) {
			continue
		}
		_, exists := s.Records[item+".impl.a1"]
		for _, id := range s.Steps {
			t, _, _ := s.Records[id].Tuple()
			if t.Kind == "implement" && t.WorkItem == item {
				exists = true
			}
		}
		id := item + ".impl.a1"
		if !exists {
			w := createStep(id, item)
			c.Writes = append(c.Writes, w)
			if err := overlay(&s, w); err != nil {
				return c, err
			}
		}
	}
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if r.Status == "closed" || t.Kind == "decision" || blocked(s, id) || gateDecision(s, t.Gate, t.Kind == "fix") {
			continue
		}
		lease, ok, err := Decode[LeaseRecord](r.Metadata, KeyLease)
		if err != nil {
			return c, err
		}
		if ok && s.At.Before(lease.Expires) {
			c.Waiting = append(c.Waiting, Waiting{Step: id, Lease: LeaseView{Generation: lease.Generation, Expires: lease.Expires}})
			continue
		}
		sub := subject(s, t.Gate)
		if t.Kind == "implement" {
			branch, err := workspace.BeadBranch(t.WorkItem)
			sub = ""
			if err == nil {
				sub = s.Refs["refs/heads/"+branch]
			}
		}
		if v, ok, _ := Decode[Subject](r.Metadata, KeySubject); ok {
			sub = v.Commit
		}
		target := t.WorkItem
		if target == "" {
			target = id
		}
		brief := []string{"mindspec", "next", target, "--emit-only"}
		if t.Kind == "review-slot" {
			gate := activeGate(s, t.Gate)
			reg, _, err := Decode[RegistrationRecord](s.Records[gate].Metadata, KeyRegistration)
			if err != nil {
				return c, err
			}
			brief = []string{"mindspec", "panel", "prompt", reg.Slug, "--slot", fmt.Sprintf("R%d", t.Slot), "--spec", s.SpecID}
		}
		flag, placeholder := "--artifact", "<path>"
		if t.Kind == "implement" || t.Kind == "fix" {
			flag, placeholder = "--commit", "<commit>"
		}
		c.Actions = append(c.Actions, Action{Step: id, Kind: t.Kind, Round: t.Round, Subject: Subject{Commit: sub}, Claim: []string{"mindspec", "lease", id}, Brief: brief, Submit: []string{"mindspec", "submit", id, "--lease", "<token>", flag, placeholder}})
	}
	for _, r := range s.OpenDecisions() {
		d, _, err := Decode[DecisionRecord](r.Metadata, KeyDecision)
		if err != nil {
			return c, err
		}
		c.Decisions = append(c.Decisions, Decision{Step: r.Key(), Options: d.Options, Submit: []string{"mindspec", "submit", r.Key(), "--answer", "<option>"}})
	}
	return c, nil
}
func gateDecision(s State, gate string, fix bool) bool {
	for _, r := range s.OpenDecisions() {
		t, _, _ := r.Tuple()
		if t.Gate == gate && (t.Cause == "reject" || fix) {
			return true
		}
	}
	return false
}
func tally(s State) (Changes, error) {
	c := Changes{}
	for _, gate := range s.Gates {
		g := s.Records[gate]
		if g.Status == "closed" || hasOpenChild(s, gate) {
			continue
		}
		accepted, ok, err := gatePassEvidence(s, g)
		if err != nil {
			return c, err
		}
		if ok && subject(s, gate) == accepted {
			receipt := Receipt{Kind: "passed", Subject: accepted, Retally: "passing-tally"}
			c.Writes = append(c.Writes, Write{Kind: "close", ID: gate, Receipt: &receipt})
		}
	}
	return c, nil
}
func completeEntry(s State) (Changes, error) {
	c := Changes{}
	for _, item := range s.Items {
		gate := activeGate(s, item+".review")
		if !passed(s, s.Records[gate]) || itemDone(s, item) || hasOpenChild(s, item) || blocked(s, item) {
			continue
		}
		judge := false
		for _, id := range s.Steps {
			r := s.Records[id]
			t, _, _ := r.Tuple()
			if t.Kind == "change-judge" && t.Gate == item+".review" && r.Status != "closed" {
				judge = true
			}
		}
		if !judge {
			c.Run = append(c.Run, Run{Actor: "binary", Argv: []string{"mindspec", "complete", item}})
		}
	}
	final := s.Epic + ".final"
	if s.Records[s.Epic].Status == "closed" || blocked(s, activeGate(s, final)) || CheckPassedGate(s, final) != nil {
		return c, nil
	}
	for _, item := range s.Items {
		if !itemDone(s, item) {
			return c, nil
		}
	}
	c.Run = append(c.Run, Run{Actor: "binary", Argv: []string{"mindspec", "impl", "approve", s.SpecID}})
	return c, nil
}

// gatePassEvidence is the common evidence check for gate writers and readers.
// A close reason alone cannot replace a registered, fully receipted passing panel.
func gatePassEvidence(s State, gate Record) (string, bool, error) {
	round, has, err := Decode[GateRound](gate.Metadata, KeyRound)
	if err != nil || !has {
		return "", false, err
	}
	reg, has, err := Decode[RegistrationRecord](gate.Metadata, KeyRegistration)
	if err != nil || !has {
		return "", false, err
	}
	if round.Subject == "" || reg.Round != round.Round || reg.Subject != round.Subject || len(reg.Slots) != round.SlotCount || !receiptedSlots(s, round) {
		return "", false, nil
	}
	tuple, _, _ := gate.Tuple()
	for i, id := range round.Slots {
		slot, ok, err := s.Records[id].Tuple()
		if err != nil || !ok || slot.Gate != tuple.Gate || slot.Round != round.Round || slot.Slot != i+1 || reg.Slots[i] != fmt.Sprintf("R%d", i+1) {
			return "", false, nil
		}
	}
	for _, candidate := range s.Panels {
		dir := candidate.Dir
		if candidate.Slug() != reg.Slug {
			continue
		}
		var result *panel.Result
		if gate.Status == "closed" {
			matchingRound := false
			for _, line := range s.Ledgers[dir] {
				var event panel.RoundRecord
				if err := json.Unmarshal(line, &event); err != nil {
					return "", false, err
				}
				matchingRound = matchingRound || (event.Record == panel.RecordRound && event.Spec == s.SpecID && event.Round == round.Round && event.ReviewedHeadSHA == round.Subject && gatePanel(tuple, panel.Panel{BeadID: event.BeadID, Gate: event.Gate}))
			}
			if !matchingRound {
				continue
			}
			result, err = panel.TallyRegisteredRound(dir, s.SpecID, round.Round, round.Subject)
		} else {
			if candidate.Panel.Abandoned || candidate.Panel.Round != round.Round || candidate.Panel.ReviewedHeadSHA != round.Subject || candidate.Panel.Spec != s.SpecID {
				continue
			}
			result, err = panel.Tally(dir)
		}
		if err != nil {
			return "", false, err
		}
		if result.Panel == nil || !gatePanel(tuple, *result.Panel) || result.Panel.Abandoned || result.Panel.Round != round.Round || result.RoundReviewedHeadSHA != round.Subject {
			continue
		}
		if result.PassRule(true) {
			return round.Subject, true, nil
		}
	}
	return "", false, nil
}

// ItemDone reports whether completion has landed the accepted work item subject.
func ItemDone(s State, item string) bool { return itemDone(s, item) }
