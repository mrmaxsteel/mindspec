package kernel

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
)

func init() {
	for _, rule := range []struct {
		name string
		mode Mode
		fn   Rule
	}{
		{"retry-attempt", Ordinary, retryAttempts},
		{"unreceipted-close", FlagOnly, unreceiptedFlags},
		{"raw-close", FlagOnly, rawCloseFlags},
		{"forced-close", FlagOnly, forcedCloseFlags},
	} {
		if err := RegisterRule(rule.name, rule.mode, rule.fn); err != nil {
			panic(err)
		}
	}
}

func validClose(s State, r Record) bool {
	t, _, _ := r.Tuple()
	if t.Kind == "bead-gate" || t.Kind == "final-gate" {
		receipt, _, _ := Decode[Receipt](r.Metadata, KeyClose)
		if receipt.Kind == "passed" {
			return passed(s, r)
		}
	}
	receipt, ok, err := Decode[Receipt](r.Metadata, KeyClose)
	return r.Status == "closed" && ok && err == nil && ValidReceipt(s.Root, receipt, r.Metadata) == nil
}

func nextAttempt(id string) (string, error) {
	t, err := kernelid.Parse(id)
	if err != nil {
		return "", err
	}
	t.Attempt++
	if t.Attempt < 1 {
		return "", fmt.Errorf("attempt counter is too large")
	}
	return kernelid.Format(t)
}

// RepairedClose accepts a valid close or a completed retry in its lineage.
// A retry must name its predecessor; an unrelated record is not repair evidence.
func RepairedClose(s State, id string) bool {
	seen := map[string]bool{}
	for !seen[id] {
		seen[id] = true
		if validClose(s, s.Records[id]) {
			return true
		}
		next, err := nextAttempt(id)
		if err != nil {
			return false
		}
		r, exists := s.Records[next]
		from, ok, err := Decode[From](r.Metadata, KeyFrom)
		if !exists || !owned(s, next) || err != nil || !ok || string(from) != id {
			return false
		}
		id = next
	}
	return false
}

// UnreceiptedCloses returns closed kernel records without valid repair evidence.
func UnreceiptedCloses(s State) []string {
	var ids []string
	for _, id := range append(append([]string{}, s.Gates...), s.Steps...) {
		if s.Records[id].Status == "closed" && !RepairedClose(s, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func retryAttempts(s State) (Changes, error) {
	c := Changes{}
	created := map[string]bool{}
	for _, id := range UnreceiptedCloses(s) {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if t.Kind == "review-slot" {
			gate := s.Records[activeGate(s, t.Gate)]
			round, has, err := Decode[GateRound](gate.Metadata, KeyRound)
			if err != nil {
				return c, err
			}
			// Only round lineage can retire a raw close. Slot metadata is not evidence.
			if has && t.Round < round.Round {
				receipt := Receipt{Kind: "superseded", Subject: round.Subject, Repaired: true}
				w := Write{Kind: "close", ID: id, Receipt: &receipt}
				c.Writes = append(c.Writes, w)
				if err := overlay(&s, w); err != nil {
					return c, err
				}
				continue
			}
		}
		next, err := nextAttempt(id)
		if err != nil {
			return c, err
		}
		if _, exists := s.Records[next]; exists {
			from, has, err := Decode[From](s.Records[next].Metadata, KeyFrom)
			if err != nil {
				return c, err
			}
			if has && string(from) == id && owned(s, next) {
				update, err := retryRoundPointer(s, t, id, next)
				if err != nil {
					return c, err
				}
				if update != nil {
					c.Writes = append(c.Writes, *update)
					if err := overlay(&s, *update); err != nil {
						return c, err
					}
				}
			}
			continue
		}
		if options, err := repairEvidence(s, r); err == nil {
			c.Run = append(c.Run, repairRun(options))
			continue
		}
		parent := r.Parent
		for key, record := range s.Records {
			if record.ID == parent {
				parent = key
				break
			}
		}
		w := createStep(next, parent)
		if t.Kind == "decision" {
			w.Create.Type = "decision"
		}
		w.Create.Metadata = Metadata(KeyFrom, From(id))
		// A retry keeps the subject and decision it must resolve, but no old lease or receipt.
		keys := []string{KeySubject, KeyDecision, KeyRound, KeyRegistration}
		switch t.Kind {
		case "fix":
			keys = append(keys, KeyReproduced)
		case "verify":
			keys = append(keys, KeyFindings, KeyRead, KeyVerifyVerdicts)
		}
		for _, key := range keys {
			if value, ok := r.Metadata[key]; ok {
				w.Create.Metadata[key] = value
			}
		}
		c.Writes = append(c.Writes, w)
		if err := overlay(&s, w); err != nil {
			return c, err
		}
		update, err := retryRoundPointer(s, t, id, next)
		if err != nil {
			return c, err
		}
		if update != nil {
			c.Writes = append(c.Writes, *update)
			if err := overlay(&s, *update); err != nil {
				return c, err
			}
		}
		created[next] = true
	}
	if len(created) > 0 {
		entries, err := implement(s)
		if err != nil {
			return c, err
		}
		for _, a := range entries.Actions {
			if created[a.Step] {
				c.Actions = append(c.Actions, a)
			}
		}
		for _, d := range entries.Decisions {
			if created[d.Step] {
				c.Decisions = append(c.Decisions, d)
			}
		}
	}
	return c, nil
}

func retryRoundPointer(s State, t Tuple, id, next string) (*Write, error) {
	if t.Kind != "lens" && t.Kind != "verify" {
		return nil, nil
	}
	gate := activeGate(s, t.Gate)
	round, _, err := Decode[GateRound](s.Records[gate].Metadata, KeyRound)
	if err != nil {
		return nil, err
	}
	current := &round
	if round.Round != t.Round && round.Prior != nil && round.Prior.Round == t.Round {
		current = round.Prior
	}
	pointer := &current.Lens
	if t.Kind == "verify" {
		pointer = &current.Verify
	}
	if current.Round != t.Round || *pointer != id {
		return nil, nil
	}
	*pointer = next
	return &Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)}, nil
}

func unreceiptedFlags(s State) (Changes, error) {
	c := Changes{}
	for _, id := range UnreceiptedCloses(s) {
		c.Flags = append(c.Flags, Flag{Code: "unreceipted-close", Step: id, Repair: []string{"mindspec", "next-action", s.SpecID, "--apply"}})
	}
	return c, nil
}

// RawClose reports a closed, unmerged work item with a passing gate.
func RawClose(s State, id string) bool {
	for _, item := range s.Items {
		if item == id {
			return s.Records[id].Status == "closed" && !s.Merged[id] && passed(s, s.Records[activeGate(s, id+".review")])
		}
	}
	return false
}

// ForcedClose reports a work item closed while a child remains open.
func ForcedClose(s State, id string) bool {
	for _, item := range s.Items {
		if item == id {
			return s.Records[id].Status == "closed" && hasOpenChild(s, id)
		}
	}
	return false
}

func rawCloseFlags(s State) (Changes, error) {
	c := Changes{}
	for _, id := range s.Items {
		if RawClose(s, id) {
			c.Flags = append(c.Flags, Flag{Code: "raw-close", Step: id, Repair: []string{"mindspec", "complete", id}})
		}
	}
	return c, nil
}

func forcedCloseFlags(s State) (Changes, error) {
	c := Changes{}
	for _, id := range s.Items {
		if ForcedClose(s, id) {
			c.Flags = append(c.Flags, Flag{Code: "forced-close", Step: id, Repair: []string{"bd", "reopen", s.Records[id].ID}})
		}
	}
	return c, nil
}

// receiptedSlots reports whether each registered slot has a valid close or repair.
func receiptedSlots(s State, round GateRound) bool {
	if len(round.Slots) == 0 || len(round.Slots) != round.SlotCount {
		return false
	}
	for _, id := range round.Slots {
		if !owned(s, id) || s.Records[id].Status != "closed" || !RepairedClose(s, id) {
			return false
		}
	}
	return true
}

// SlotLineageRoot verifies every retry link back to its registered slot.
func SlotLineageRoot(s State, id string) (string, error) {
	for {
		if !owned(s, id) {
			return "", actorFailure("slot lineage contains a foreign step")
		}
		r := s.Records[id]
		tuple, _, _ := r.Tuple()
		if tuple.Kind != "review-slot" {
			return "", actorFailure("slot lineage contains a different kind")
		}
		from, has, err := Decode[From](r.Metadata, KeyFrom)
		if err != nil {
			return "", err
		}
		if tuple.Attempt == 1 {
			if has {
				return "", actorFailure("root slot has a retry predecessor")
			}
			return id, nil
		}
		predecessor := string(from)
		next, err := nextAttempt(predecessor)
		if !has || err != nil || next != id || !owned(s, predecessor) {
			return "", actorFailure("slot retry has no valid predecessor")
		}
		prior := s.Records[predecessor]
		uncounted, _, _ := Decode[bool](r.Metadata, KeyUncounted)
		if prior.Status != "closed" || (validClose(s, prior) && !(uncounted && passed(s, prior))) {
			return "", actorFailure("slot retry would duplicate a valid vote")
		}
		id = predecessor
	}
}

// SlotLineageHead selects the newest verified retry of a registered slot.
func SlotLineageHead(s State, root string) (Record, error) {
	r, exists := s.Records[root]
	if !exists {
		return Record{}, actorFailure("registered slot is absent")
	}
	var current Record
	for {
		receipt, _, _ := Decode[Receipt](r.Metadata, KeyClose)
		if r.Status != "closed" || receipt.Kind != "superseded" {
			current = r
		}
		next, err := nextAttempt(r.Key())
		if err != nil {
			return Record{}, err
		}
		candidate, exists := s.Records[next]
		if !exists {
			if current.Key() == "" {
				return Record{}, actorFailure("slot has no current attempt")
			}
			return current, nil
		}
		verified, err := SlotLineageRoot(s, next)
		if err != nil || verified != root {
			return Record{}, actorFailure("slot retry lineage is invalid")
		}
		r = candidate
	}
}
