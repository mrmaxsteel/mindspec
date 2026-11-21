package kernel

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
)

func init() {
	if err := RegisterRule("final-round", Ordinary, finalRound); err != nil {
		panic(err)
	}
}

func authorisedRound(s State, gate string, n int, head string) (GateRound, error) {
	t, err := kernelid.Parse(gate)
	if err != nil {
		return GateRound{}, err
	}
	cfg, err := config.Load(s.Root)
	if err != nil {
		return GateRound{}, err
	}
	kind := "bead"
	if t.WorkItem == "" {
		kind = "final_review"
	}
	count, err := cfg.PanelGateExpectedReviewers(kind)
	if err != nil {
		return GateRound{}, err
	}
	round := GateRound{Round: n, Subject: head, SlotCount: count, Slots: []string{}}
	for i := 1; i <= count; i++ {
		round.Slots = append(round.Slots, fmt.Sprintf("%s.r%d.R%d", t.Gate, n, i))
	}
	return round, nil
}

func finalRound(s State) (Changes, error) {
	c := Changes{}
	for _, item := range s.Items {
		if !itemDone(s, item) {
			return c, nil
		}
	}
	gate := activeGate(s, s.Epic+".final")
	g := s.Records[gate]
	_, has, err := Decode[GateRound](g.Metadata, KeyRound)
	if err != nil {
		return c, err
	}
	head, err := effectiveSubject(s, gate)
	if err != nil {
		return c, err
	}
	if g.Status == "closed" || has || head == "" {
		return c, nil
	}
	count, err := RoundsForGate(s, gate)
	if err != nil {
		return c, err
	}
	const initialRound = 1
	if count >= Allowance(s, gate) {
		c.Writes = append(c.Writes, decisionWrite(s, gate, "limit", initialRound)...)
		return c, nil
	}
	round, err := authorisedRound(s, gate, initialRound, head)
	if err != nil {
		return c, err
	}
	c.Writes = append(c.Writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)})
	return c, nil
}
