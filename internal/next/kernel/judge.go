package kernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func init() {
	for _, rule := range []struct {
		name string
		fn   Rule
	}{
		{"judge-apply", judgeApply}, {"judge-supersede", judgeSupersede}, {"judge-create", judgeCreate},
	} {
		if err := RegisterRule(rule.name, Ordinary, rule.fn); err != nil {
			panic(err)
		}
	}
	if err := RegisterArtifactCheck("change-judge", checkJudgement); err != nil {
		panic(err)
	}
}

// AcceptedSubject returns the accepted commit of the highest gate attempt.
// A ledger-backed rebind and a carry-over can extend the frozen passing subject.
func AcceptedSubject(s State, gate string) (string, error) {
	t, err := kernelid.Parse(gate)
	if err != nil {
		return "", err
	}
	g := s.Records[activeGate(s, t.Gate)]
	receipt, _, err := Decode[Receipt](g.Metadata, KeyClose)
	if err != nil {
		return "", err
	}
	accepted := receipt.Subject
	rebound, rebind, err := gateRebind(s, g)
	if err != nil {
		return "", err
	}
	if rebound != "" {
		accepted = rebound
	}
	for _, carry := range receipt.CarryOvers {
		// A carry-over wins until the panel records another rebind.
		if carry.Rebind == rebind {
			accepted = carry.NewSubject
		}
	}
	return accepted, nil
}

func gatePanel(t Tuple, p panel.Panel) bool {
	if t.WorkItem != "" {
		return p.BeadID != nil && *p.BeadID == t.WorkItem
	}
	return p.BeadID == nil && p.Gate == "final_review"
}

// gateRebind reads the matching passing registration and its durable rebind chain.
func gateRebind(s State, g Record) (string, string, error) {
	reg, exists, err := Decode[RegistrationRecord](g.Metadata, KeyRegistration)
	if err != nil || !exists {
		return "", "", err
	}
	receipt, _, err := Decode[Receipt](g.Metadata, KeyClose)
	if err != nil {
		return "", "", err
	}
	t, _, _ := g.Tuple()
	accepted, evidence := receipt.Subject, ""
	for _, p := range s.Panels {
		if p.Slug() != reg.Slug || !gatePanel(t, p.Panel) {
			continue
		}
		for _, line := range s.Ledgers[p.Dir] {
			var event panel.RebindRecord
			if err := json.Unmarshal(line, &event); err != nil {
				return "", "", err
			}
			if event.Record != panel.RecordRebind || event.Spec != s.SpecID || event.Panel != reg.Slug || event.Round != reg.Round || event.FromSHA != accepted {
				continue
			}
			backed := false
			for _, entry := range p.Panel.Rebinds {
				backed = backed || reflect.DeepEqual(entry, event.Rebind)
			}
			if !backed {
				continue
			}
			accepted = event.ToSHA
			evidence = filepath.ToSlash(filepath.Join("reviews", reg.Slug, "dispositions.jsonl")) + "#rebind:" + event.ToSHA
		}
	}
	return accepted, evidence, nil
}

func judgePanelPath(s State, dir string) (string, error) {
	rel, err := filepath.Rel(s.Root, dir)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return actorGit(dir, "rev-parse", "--show-prefix")
	}
	return rel, nil
}

// registeredJudgePanel includes registrations retained only in the ledger.
func registeredJudgePanel(s State, t Tuple, p panel.Registration) bool {
	if p.Panel.Spec == s.SpecID && gatePanel(t, p.Panel) {
		return true
	}
	for _, line := range s.Ledgers[p.Dir] {
		var event panel.RoundRecord
		if json.Unmarshal(line, &event) != nil || event.Record != panel.RecordRound || event.Spec != s.SpecID {
			continue
		}
		if gatePanel(t, panel.Panel{BeadID: event.BeadID, Gate: event.Gate}) {
			return true
		}
	}
	return false
}

// newestJudgeSubject skips commits that change only this gate's panel directories.
func newestJudgeSubject(s State, gate string) (string, error) {
	head := subject(s, gate)
	if head == "" {
		return "", nil
	}
	accepted, err := AcceptedSubject(s, gate)
	if err != nil {
		return "", err
	}
	t, _ := kernelid.Parse(gate)
	dirs := []string{}
	for _, p := range s.Panels {
		if !registeredJudgePanel(s, t, p) {
			continue
		}
		rel, err := judgePanelPath(s, p.Dir)
		if err != nil {
			return "", err
		}
		dirs = append(dirs, filepath.ToSlash(rel)+"/")
	}
	for {
		if head == accepted {
			return head, nil
		}
		line, err := actorGit(s.Root, "rev-list", "--parents", "-n", "1", head, "--")
		if err != nil {
			return "", err
		}
		parents := strings.Fields(line)
		if len(parents) < 2 {
			return head, nil
		}
		data, err := actorGitBytes(s.Root, "diff", "--no-renames", "--name-only", "-z", parents[1], head, "--")
		if err != nil {
			return "", err
		}
		onlyPanel := len(data) > 0
		for _, name := range strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00") {
			excluded := false
			for _, dir := range dirs {
				excluded = excluded || strings.HasPrefix(name, dir)
			}
			onlyPanel = onlyPanel && excluded
		}
		if !onlyPanel {
			return head, nil
		}
		head = parents[1]
	}
}

func judgeEnded(s State, gate string) (bool, error) {
	t, _ := kernelid.Parse(gate)
	if s.Records[s.Epic].Status == "closed" {
		return true, nil
	}
	if t.WorkItem == "" {
		return false, nil
	}
	if s.Merged[t.WorkItem] {
		return true, nil
	}
	// The work branch can move after its merge. Match a historical second parent.
	branch, err := workspace.SpecBranch(s.SpecID)
	if err != nil {
		return false, err
	}
	target := s.Refs["refs/heads/"+branch]
	if target == "" {
		return false, nil
	}
	accepted, err := AcceptedSubject(s, gate)
	if err != nil {
		return false, err
	}
	if !objectID(accepted) {
		return false, nil
	}
	lines, err := actorGit(s.Root, "log", "--first-parent", "--merges", "--format=%P", target, "--")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(lines, "\n") {
		parents := strings.Fields(line)
		if len(parents) != 2 {
			continue
		}
		if _, err := actorGit(s.Root, "merge-base", "--is-ancestor", accepted, parents[1]); err != nil {
			continue
		}
		if _, err := actorGit(s.Root, "merge-base", "--is-ancestor", parents[1], subject(s, gate)); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func judgeCreate(s State) (Changes, error) {
	c := Changes{}
	for _, gate := range s.Gates {
		g := s.Records[gate]
		t, _, _ := g.Tuple()
		if gate != activeGate(s, t.Gate) || !passed(s, g) {
			continue
		}
		ended, err := judgeEnded(s, gate)
		if err != nil {
			return c, err
		}
		if ended {
			continue
		}
		newest, err := newestJudgeSubject(s, gate)
		if err != nil {
			return c, err
		}
		accepted, err := AcceptedSubject(s, gate)
		if err != nil {
			return c, err
		}
		if newest == "" || newest == accepted {
			continue
		}
		attempt, live := 0, false
		for _, id := range s.Steps {
			r := s.Records[id]
			jt, _, _ := r.Tuple()
			if jt.Kind != "change-judge" || jt.Gate != t.Gate {
				continue
			}
			attempt = max(attempt, jt.Attempt)
			live = live || r.Status != "closed"
		}
		if live {
			continue
		}
		id := fmt.Sprintf("%s.judge.a%d", t.Gate, attempt+1)
		w := createStep(id, judgeParent(t))
		w.Create.Metadata = Metadata(KeySubject, Subject{Commit: newest})
		c.Writes = append(c.Writes, w)
	}
	return c, nil
}

func judgeSupersede(s State) (Changes, error) {
	c := Changes{}
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if t.Kind != "change-judge" || r.Status == "closed" {
			continue
		}
		gate := activeGate(s, t.Gate)
		newest, err := newestJudgeSubject(s, gate)
		if err != nil {
			return c, err
		}
		accepted, err := AcceptedSubject(s, gate)
		if err != nil {
			return c, err
		}
		sub, _, err := Decode[Subject](r.Metadata, KeySubject)
		if err != nil {
			return c, err
		}
		if sub.Commit == newest && sub.Commit != accepted {
			continue
		}
		if hasOpenChild(s, id) {
			continue
		}
		_, rebind, err := gateRebind(s, s.Records[gate])
		if err != nil {
			return c, err
		}
		receipt := Receipt{Kind: "superseded", Subject: newest}
		if sub.Commit == accepted {
			receipt.Rebind = rebind
		}
		c.Writes = append(c.Writes, Write{Kind: "close", ID: id, Receipt: &receipt})
	}
	return c, nil
}

func readJudgement(s State, receipt Receipt) (Judgement, error) {
	var j Judgement
	data, err := actorGitBytes(s.Root, "show", receipt.Commit+":"+receipt.Path)
	if err != nil {
		return j, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&j); err != nil {
		return j, actorFailure("invalid judgement artifact")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return j, actorFailure("invalid judgement artifact")
	}
	return j, nil
}

func checkJudgement(s State, r Record, receipt Receipt) error {
	return validateJudgement(s, r, receipt, true)
}

func validateJudgement(s State, r Record, receipt Receipt, current bool) error {
	t, _, _ := r.Tuple()
	gate := activeGate(s, t.Gate)
	g := s.Records[gate]
	if !passed(s, g) {
		return actorFailure("judgement needs a frozen passing gate")
	}
	ended, err := judgeEnded(s, gate)
	if err != nil {
		return err
	}
	if current && ended {
		return actorFailure("review subject has already finished")
	}
	j, err := readJudgement(s, receipt)
	if err != nil {
		return err
	}
	accepted, err := AcceptedSubject(s, gate)
	if err != nil {
		return err
	}
	newest, err := newestJudgeSubject(s, gate)
	if err != nil {
		return err
	}
	sub, _, err := Decode[Subject](r.Metadata, KeySubject)
	if err != nil {
		return err
	}
	if j.OldSubject != accepted || (current && j.NewSubject != newest) || j.NewSubject != sub.Commit || j.OldSubject == j.NewSubject {
		return actorFailure("judgement subject is stale")
	}
	reg, registered, err := Decode[RegistrationRecord](g.Metadata, KeyRegistration)
	if err != nil {
		return err
	}
	inPanel := false
	ceiling := 0
	for _, p := range s.Panels {
		if !registered || p.Slug() != reg.Slug || !gatePanel(t, p.Panel) || p.Panel.Round != reg.Round {
			continue
		}
		rel, err := judgePanelPath(s, p.Dir)
		if err != nil {
			return err
		}
		inPanel = strings.HasPrefix(receipt.Path, filepath.ToSlash(rel)+"/")
		ceiling = p.Panel.ExpectedReviewers
	}
	if !inPanel {
		return actorFailure("judgement must be in its registered panel directory")
	}
	switch j.Value {
	case "carry-over":
		if strings.TrimSpace(j.Reason) == "" || j.SlotCount != 0 || len(j.Paths) != 0 {
			return actorFailure("carry-over needs a reason and no round scope")
		}
	case "re-panel":
		if j.SlotCount < 2 || j.SlotCount > ceiling || len(j.Paths) == 0 {
			return actorFailure("re-panel needs changed paths and a slot count within the passing round")
		}
		for _, name := range j.Paths {
			if strings.TrimSpace(name) == "" || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
				return actorFailure("re-panel paths must be repository-relative files")
			}
		}
	default:
		return actorFailure("judgement must be carry-over or re-panel")
	}
	return nil
}

func judgeApply(s State) (Changes, error) {
	c := Changes{}
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if t.Kind != "change-judge" || !passed(s, r) {
			continue
		}
		receipt, _, err := Decode[Receipt](r.Metadata, KeyClose)
		if err != nil {
			return c, err
		}
		j, err := readJudgement(s, receipt)
		if err != nil {
			return c, err
		}
		gate := activeGate(s, t.Gate)
		g := s.Records[gate]
		frozen, _, err := Decode[Receipt](g.Metadata, KeyClose)
		if err != nil {
			return c, err
		}
		applied := false
		for _, carry := range frozen.CarryOvers {
			applied = applied || carry.Judge == id
		}
		if saved, ok, _ := Decode[Judgement](r.Metadata, KeyJudgement); ok && reflect.DeepEqual(saved, j) {
			continue
		}
		for _, candidate := range s.Gates {
			record := s.Records[candidate]
			prior, _, _ := Decode[Receipt](record.Metadata, KeyClose)
			for _, carry := range prior.CarryOvers {
				applied = applied || carry.Judge == id
			}
			from, hasFrom, _ := Decode[From](record.Metadata, KeyFrom)
			scope, hasScope, _ := Decode[RePanel](record.Metadata, KeyRePanel)
			round, _, _ := Decode[GateRound](record.Metadata, KeyRound)
			applied = applied || (hasFrom && hasScope && sameJudgeGate(string(from), t.Gate) && round.Subject == j.NewSubject && scope.SlotCount == j.SlotCount && reflect.DeepEqual(scope.Paths, j.Paths))
		}
		if applied {
			continue
		}
		// Recheck durable evidence when a crash left only the actor's receipt and close.
		if err := validateJudgement(s, r, receipt, false); err != nil {
			return c, err
		}
		switch j.Value {
		case "carry-over":
			_, rebind, err := gateRebind(s, g)
			if err != nil {
				return c, err
			}
			frozen.CarryOvers = append(frozen.CarryOvers, CarryOver{OldSubject: j.OldSubject, NewSubject: j.NewSubject, Judge: id, Receipt: &receipt, Rebind: rebind})
			c.Writes = append(c.Writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyClose, frozen)})
		case "re-panel":
			gt, _, _ := g.Tuple()
			next := fmt.Sprintf("%s.a%d", gt.Gate, gt.Attempt+1)
			count, err := RoundsForGate(s, gate)
			if err != nil {
				return c, err
			}
			round := GateRound{Round: count + 1, Subject: j.NewSubject, SlotCount: j.SlotCount, Paths: j.Paths, Slots: []string{}}
			for i := 1; i <= j.SlotCount; i++ {
				round.Slots = append(round.Slots, fmt.Sprintf("%s.r%d.R%d", gt.Gate, round.Round, i))
			}
			w := createStep(next, judgeParent(gt))
			w.Create.Metadata = map[string]any{KeyFrom: From(gate), KeyRePanel: RePanel{SlotCount: j.SlotCount, Paths: j.Paths}, KeyRound: round}
			c.Writes = append(c.Writes, w)
		}
		c.Writes = append(c.Writes, Write{Kind: "metadata", ID: id, Metadata: Metadata(KeyJudgement, j)})
	}
	return c, nil
}

func sameJudgeGate(id, base string) bool {
	t, err := kernelid.Parse(id)
	return err == nil && t.Gate == base
}

func judgeParent(t Tuple) string {
	if t.WorkItem != "" {
		return t.WorkItem
	}
	return t.Epic
}
