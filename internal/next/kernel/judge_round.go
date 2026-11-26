package kernel

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func openGateJudge(s State, base string) bool {
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if t.Kind == "change-judge" && t.Gate == base && r.Status != "closed" {
			return true
		}
	}
	return false
}

func roundResult(s State, gate string, round GateRound) (*panel.Result, error) {
	reg, _, err := Decode[RegistrationRecord](s.Records[gate].Metadata, KeyRegistration)
	if err != nil {
		return nil, err
	}
	tuple, _, _ := s.Records[gate].Tuple()
	for _, p := range s.Panels {
		if p.Slug() != reg.Slug || p.Panel.Spec != s.SpecID || !gatePanel(tuple, p.Panel) {
			continue
		}
		result, err := TallyPanel(s, p)
		if err != nil {
			return nil, pathFailure(err, "cannot read round panel: ", "restore the panel directory and retry")
		}
		if result.Panel != nil && result.LatestRound == round.Round && result.RoundReviewedHeadSHA == round.Subject {
			return result, nil
		}
	}
	return nil, pathFailure(fmt.Errorf("round needs its registered panel directory"), "", "restore the panel directory and retry")
}

func replaceJudgedVerify(s State, gate string, round GateRound) ([]Write, error) {
	old := round.Verify
	writes := retireVerify(s, old)
	res, err := roundResult(s, gate, round)
	if err != nil {
		return nil, err
	}
	findings, read, err := roundFindings(s, res)
	if err != nil {
		return nil, err
	}
	round.Verify = ""
	if res.Complete() && len(uncountedSlots(res)) == 0 && !res.RoundMismatch && res.Intact() == nil && len(findings) > 0 {
		id := old
		for {
			id, err = nextAttempt(id)
			if err != nil {
				return nil, err
			}
			if _, exists := s.Records[id]; !exists {
				break
			}
		}
		w := createStep(id, gate)
		sources := VerifyVerdictSources{}
		for _, verdict := range append(append([]panel.Verdict{}, res.Verdicts...), res.Unexpected...) {
			sources[verdict.Slot] = VerifyVerdictSource{Verdict: verdict.Verdict, FileSHA256: verdict.FileSHA256}
		}
		w.Create.Metadata = map[string]any{KeyFrom: From(old), KeySubject: Subject{Commit: reviewSubject(round)}, KeyFindings: findings, KeyRead: read, KeyVerifyVerdicts: sources}
		round.Verify = id
		// Publish the identity before creation so a pass can recreate an absent
		// attempt after a crash without leaving an unreferenced open verify.
		writes = append(writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)}, w)
		return writes, nil
	}
	return append(writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)}), nil
}

// roundPrior stores the evidence that authorized the successor without creating
// a Git object. An applied verify is the source even after verdict files change.
func roundPrior(s State, gate string, round GateRound) (GateRound, error) {
	round.Prior = nil
	var findings []Finding
	if pinned, _, ok := verifyReturns(s, round); ok {
		findings = pinned
		round = verifyPrior(s, round)
	} else {
		round.Verify = ""
	}
	// A closed fix uses only the pinned findings and read set. Verb records are
	// already in the state snapshot and do not require the verdict files.
	var p *panel.Panel
	if round.Verify != "" {
		reg, _, _ := Decode[RegistrationRecord](s.Records[gate].Metadata, KeyRegistration)
		tuple, _, _ := s.Records[gate].Tuple()
		for _, candidate := range s.Panels {
			if candidate.Slug() == reg.Slug && candidate.Panel.Spec == s.SpecID && gatePanel(tuple, candidate.Panel) {
				copy := candidate.Panel
				p = &copy
			}
		}
	} else {
		res, err := roundResult(s, gate, round)
		if err != nil {
			return round, err
		}
		findings, round.Read, err = roundFindings(s, res)
		if err != nil {
			return round, err
		}
		data, err := os.ReadFile(filepath.Join(res.Dir, panel.FileName))
		if err != nil {
			return round, pathFailure(err, "cannot read prior panel: ", "restore the panel directory and retry")
		}
		root := s.Root
		if specRoot, ok := specPanelRoot(s); ok {
			root = specRoot
		}
		rel, err := filepath.Rel(root, filepath.Join(res.Dir, panel.FileName))
		if err != nil {
			return round, pathFailure(err, "cannot locate prior panel: ", "restore the panel directory and retry")
		}
		round.Read[filepath.ToSlash(rel)] = recordDigest(data)
		round.Asks = make([]PriorAsk, len(findings))
		for i, f := range findings {
			round.Asks[i].Finding = f
		}
		p = res.Panel
	}
	if p == nil {
		return round, nil
	}
	t, _, _ := s.Records[gate].Tuple()
	filtered := FilterDispositions(s, t.Gate, &panel.Result{Panel: p}).Panel
	for i := range round.Asks {
		f := round.Asks[i].Finding
		for _, ref := range filtered.Refutations {
			if ref.VerifyStep == "" && ref.Slot == f.Slot && ref.Round == f.Round && (ref.Finding == f.Finding && ref.FindingSHA256 == f.FindingSHA256 || ref.Finding == "slot") {
				round.Asks[i].Returns = append(round.Asks[i].Returns, VerifyReturn{Slot: f.Slot, Round: f.Round, Finding: f.Finding, FindingSHA256: f.FindingSHA256, Return: "refuted", Ground: ref.Ground, AC: ref.AC, Reason: ref.Reason, Evidence: ref.Evidence})
			}
		}
		for _, d := range filtered.Deferrals {
			if d.VerifyStep == "" && d.Slot == f.Slot && d.Round == f.Round && d.Finding == f.Finding && d.FindingSHA256 == f.FindingSHA256 {
				round.Asks[i].Returns = append(round.Asks[i].Returns, VerifyReturn{Slot: f.Slot, Round: f.Round, Finding: f.Finding, FindingSHA256: f.FindingSHA256, Return: "deferred", Target: d.Bead})
			}
		}
	}
	return round, nil
}

func scopedRoundWrites(s State, gate string, round GateRound) ([]Write, error) {
	scope, _, err := Decode[RePanel](s.Records[gate].Metadata, KeyRePanel)
	if err != nil {
		return nil, err
	}
	prior, err := roundPrior(s, gate, round)
	if err != nil {
		return nil, err
	}
	count, err := RoundsForGate(s, gate)
	if err != nil {
		return nil, err
	}
	t, _, _ := s.Records[gate].Tuple()
	next := GateRound{Round: count + 1, Subject: reviewSubject(round), SlotCount: scope.SlotCount, Paths: scope.Paths, Prior: &prior, Slots: []string{}}
	for i := 1; i <= next.SlotCount; i++ {
		next.Slots = append(next.Slots, fmt.Sprintf("%s.r%d.R%d", t.Gate, next.Round, i))
	}
	writes := []Write{}
	for _, id := range s.Steps {
		r := s.Records[id]
		st, _, _ := r.Tuple()
		if st.Kind == "review-slot" && st.Gate == t.Gate && st.Round == round.Round && r.Status != "closed" && !hasOpenChild(s, id) && !hasOpenBlocker(s, id) {
			writes = append(writes, Write{Kind: "close", ID: id, Receipt: &Receipt{Kind: "superseded", Subject: next.Subject}})
		}
	}
	writes = append(writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, next)})
	if count >= Allowance(s, gate) {
		writes = append(writes, decisionWrite(s, gate, "limit", round.Round)...)
	}
	return writes, nil
}
