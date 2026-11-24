package kernel

import (
	"fmt"
	"reflect"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func init() {
	if err := RegisterRule("lens-create", Ordinary, lensCreate); err != nil {
		panic(err)
	}
	if err := RegisterCloseWriteHook("lens", lensCloseWrites); err != nil {
		panic(err)
	}
}

// RoundReviewerSlots resolves the round roster before assigning its contiguous ids.
// A scoped roster keeps adversaries before filling from the other configured slots.
func RoundReviewerSlots(cfg *config.Config, gate string, round GateRound) ([]config.ReviewerSlot, error) {
	configured, err := cfg.PanelGateReviewerSlots(gate)
	if err != nil {
		return nil, pathFailure(err, "cannot resolve round roster: ", "correct the panel reviewer configuration and retry")
	}
	n := round.SlotCount
	if len(round.Paths) == 0 {
		if n == len(configured) {
			return configured, nil
		}
		roster, err := cfg.PanelGateReviewerSlots(gate, n)
		return roster, pathFailure(err, "cannot resolve round roster: ", "correct the panel reviewer configuration and retry")
	}
	if n < 2 || n > len(configured) {
		return nil, actorFailure("scoped round slot count exceeds its configured roster or is less than two")
	}
	selected := make([]bool, len(configured))
	remaining := n
	for i, slot := range configured {
		if remaining > 0 && slot.Lens == "adversarial" {
			selected[i], remaining = true, remaining-1
		}
	}
	for i := range configured {
		if remaining > 0 && !selected[i] {
			selected[i], remaining = true, remaining-1
		}
	}
	roster := make([]config.ReviewerSlot, 0, n)
	for i, slot := range configured {
		if selected[i] {
			slot.Slot = fmt.Sprintf("R%d", len(roster)+1)
			roster = append(roster, slot)
		}
	}
	return roster, nil
}

// LensAuthorization is the immutable input snapshot stored on a lens attempt.
// Output fields and prior-round evidence do not constitute a new authorization.
func LensAuthorization(round GateRound) GateRound {
	return GateRound{Round: round.Round, Subject: round.Subject, SlotCount: round.SlotCount, Paths: round.Paths}
}

func retireLens(s State, id string) []Write {
	r, exists := s.Records[id]
	if !exists || r.Status == "closed" || hasOpenChild(s, id) || hasOpenBlocker(s, id) {
		return nil
	}
	return []Write{{Kind: "close", ID: id, Receipt: &Receipt{Kind: "superseded", Retally: "round authorization replaced"}}}
}

func lensCreate(s State) (Changes, error) {
	c := Changes{}
	// A verb excludes its target from writes, but its identity remains an input.
	if r := s.ExcludedStep; r != nil {
		t, _, _ := r.Tuple()
		if t.Kind == "lens" {
			s.Records[r.Key()] = *r
		}
	}
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if t.Kind != "lens" || r.Status == "closed" {
			continue
		}
		round, _, err := Decode[GateRound](s.Records[activeGate(s, t.Gate)].Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		if round.Lens != "" && (t.Round < round.Round || t.Round == round.Round && id != round.Lens) {
			c.Writes = append(c.Writes, retireLens(s, id)...)
		}
	}
	for _, gate := range s.Gates {
		g := s.Records[gate]
		t, _, _ := g.Tuple()
		if g.Status == "closed" || gate != activeGate(s, t.Gate) || gateDecision(s, t.Gate, false) || openGateJudge(s, t.Gate) {
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
		if !has || round.Round == 0 || round.Subject == "" || registered && reg.Round >= round.Round {
			continue
		}
		count, err := RoundsForGate(s, gate)
		if err != nil {
			return c, err
		}
		if count >= Allowance(s, gate) {
			continue
		}
		// Attempts share the base gate identity, including across gate successors.
		latest, attempt := "", 0
		for id, r := range s.Records {
			lt, _, _ := r.Tuple()
			if lt.Kind == "lens" && lt.Gate == t.Gate && lt.Round == round.Round && lt.Attempt > attempt && owned(s, id) {
				latest, attempt = id, lt.Attempt
			}
		}
		scope, _, err := Decode[RePanel](g.Metadata, KeyRePanel)
		if err != nil {
			return c, err
		}
		wanted := LensAuthorization(round)
		previous := s.Records[latest]
		// A retry of a raw close is repaired by retry-attempt. Its authorization is
		// that of the attempt it retries, even before its own copy is complete.
		basis := lensRetryBasis(s, previous)
		prior, hasPrior, err := Decode[GateRound](basis.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		priorScope, _, err := Decode[RePanel](basis.Metadata, KeyRePanel)
		if err != nil {
			return c, err
		}
		subject, _, err := Decode[Subject](basis.Metadata, KeySubject)
		if err != nil {
			return c, err
		}
		// A raw close is retried or repaired by retry-attempt, after this rule.
		// A receipt alone must not manufacture a new authorization.
		matching := latest != "" && subject.Commit == round.Subject && (!hasPrior || reflect.DeepEqual(prior, wanted)) && reflect.DeepEqual(priorScope, scope) && gateAttemptOf(s, previous.Parent, t.Gate)
		if matching {
			if round.Lens != latest {
				round.Lens = latest
				c.Writes = append(c.Writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)})
			}
			if passed(s, previous) {
				target := t.WorkItem
				if target == "" {
					target = s.SpecID
				}
				c.Run = append(c.Run, Run{Actor: "binary", Argv: []string{"mindspec", "panel", "create", target}, Paths: round.Paths})
			}
			continue
		}
		c.Writes = append(c.Writes, retireLens(s, latest)...)
		id := fmt.Sprintf("%s.r%d.lens", t.Gate, round.Round)
		if attempt > 0 {
			lt, err := kernelid.Parse(latest)
			if err != nil {
				return c, err
			}
			lt.Attempt++
			id, err = kernelid.Format(lt)
			if err != nil {
				return c, err
			}
		}
		w := createStep(id, gate)
		w.Create.Metadata = map[string]any{KeySubject: Subject{Commit: round.Subject}, KeyRound: wanted}
		if latest != "" {
			w.Create.Metadata[KeyFrom] = From(latest)
		}
		if scope.Judge != "" {
			w.Create.Metadata[KeyRePanel] = scope
		}
		round.Lens, round.Lenses = id, nil
		// Publish the pointer first. A crash before create reconstructs this attempt.
		c.Writes = append(c.Writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)}, w)
	}
	return c, nil
}

// gateAttemptOf reports whether parent is an attempt of the base gate. A gate
// retry keeps its round, so it keeps the lens step that the round names.
func gateAttemptOf(s State, parent, base string) bool {
	for _, id := range s.gateAttempts() {
		g := s.Records[id]
		t, _, err := g.Tuple()
		if err == nil && t.Gate == base && (id == parent || g.ID == parent) {
			return true
		}
	}
	return false
}

// lensRetryBasis follows a retry that has no subject of its own back through
// raw-closed lens attempts. An attempt with its own subject is its own basis.
func lensRetryBasis(s State, r Record) Record {
	for range len(s.Records) + 1 {
		if _, own := r.Metadata[KeySubject]; own {
			return r
		}
		from, has, err := Decode[From](r.Metadata, KeyFrom)
		if err != nil || !has {
			return r
		}
		p, exists := s.Records[string(from)]
		pt, _, err := p.Tuple()
		if !exists || err != nil || pt.Kind != "lens" || p.Status != "closed" || validClose(s, p) {
			return r
		}
		r = p
	}
	return r
}

func lensCloseWrites(s State, r Record, _ Receipt) ([]Write, error) {
	round, err := activeActorRound(s, r)
	if err != nil {
		return nil, err
	}
	data, err := recordBytes(r)
	if err != nil {
		return nil, err
	}
	lens, err := strictRecord[LensRecord](data)
	if err != nil {
		return nil, err
	}
	t, _, _ := r.Tuple()
	round.Lenses = lens.Lenses
	return []Write{{Kind: "metadata", ID: activeGate(s, t.Gate), Metadata: Metadata(KeyRound, round)}}, nil
}

// CheckRoundLens checks only the lens attempt explicitly named by the round.
func CheckRoundLens(s State, round GateRound) error {
	id := round.Lens
	if id == "" {
		id = "the round's lens step"
	}
	r := s.Records[round.Lens]
	t, has, err := r.Tuple()
	if round.Lens == "" || !owned(s, round.Lens) || err != nil || !has || t.Kind != "lens" || t.Round != round.Round || !passed(s, r) {
		return termsafe.NewFailure("lens step "+id+" must be closed passed before panel create", "mindspec next-action")
	}
	subject, _, err := Decode[Subject](r.Metadata, KeySubject)
	if err != nil {
		return pathFailure(err, "cannot read lens subject: ", "mindspec next-action")
	}
	newest, err := effectiveSubject(s, activeGate(s, t.Gate))
	if err != nil {
		return pathFailure(err, "cannot read authorized subject: ", "mindspec next-action")
	}
	if subject.Commit != round.Subject || newest != round.Subject {
		return termsafe.NewFailure("lens step "+id+" is closed passed, but the authorized subject moved after it", "mindspec next-action")
	}
	if openGateJudge(s, t.Gate) {
		return termsafe.NewFailure("lens step "+id+" is closed passed, but a change judge of its gate is open", "mindspec next-action")
	}
	return nil
}
