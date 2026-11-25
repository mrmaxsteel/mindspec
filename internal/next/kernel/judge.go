package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
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
	return acceptedGateSubject(s, s.Records[activeGate(s, t.Gate)])
}

func acceptedGateSubject(s State, g Record) (string, error) {
	if g.Status != "closed" {
		round, _, err := Decode[GateRound](g.Metadata, KeyRound)
		if err != nil {
			return "", err
		}
		if scopedPending(g, round) {
			if round.Prior != nil {
				return reviewSubject(*round.Prior), nil
			}
			from, _, err := Decode[From](g.Metadata, KeyFrom)
			if err != nil {
				return "", err
			}
			if prior, ok := s.Records[string(from)]; ok {
				return acceptedGateSubject(s, prior)
			}
		}
		return reviewSubject(round), nil
	}
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

// effectiveSubject skips commits that change only this gate's panel directories.
// A merge is compared with its first parent. An empty commit moves the subject.
func effectiveSubject(s State, gate string) (string, error) {
	return effectiveSubjectAt(s, gate, rawSubject(s, gate))
}

func effectiveSubjectAt(s State, gate, head string) (string, error) {
	if head == "" {
		return "", nil
	}
	accepted, err := AcceptedSubject(s, gate)
	if err != nil {
		return "", err
	}
	for {
		if head == accepted {
			return head, nil
		}
		dirs := RegisteredPanelPaths(s, gate)
		if len(dirs) == 0 {
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
			if name == "" {
				continue
			}
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

// lexicalPanelPath accepts only panel locations with no symlink components.
// The repository root itself can have a system path alias.
func lexicalPanelPath(s State, dir string) (string, bool) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", false
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	location := filepath.Dir(dir)
	segment := filepath.Base(location)
	if segment != "review" && segment != "reviews" {
		return "", false
	}
	base := filepath.Dir(location)
	if base == filepath.Clean(s.SpecDir) {
		var ok bool
		root, ok = specPanelRoot(s)
		if !ok {
			return "", false
		}
	} else if base != root && base != filepath.Join(root, ".mindspec") {
		return "", false
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", false
		}
	}
	return rel, true
}

// specPanelRoot finds the lexical worktree root for the known spec directory.
// The store can use another checkout of the same repository.
func specPanelRoot(s State) (string, bool) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", false
	}
	if workspace.TreeRootForSpecDir(s.SpecDir) == root {
		return root, true
	}
	data, err := actorGit(s.SpecDir, "rev-parse", "--show-toplevel", "--show-prefix")
	if err != nil {
		return "", false
	}
	lines := strings.SplitN(data, "\n", 2)
	prefix := ""
	if len(lines) == 2 {
		prefix = strings.TrimSuffix(lines[1], "/")
	}
	root = filepath.Clean(s.SpecDir)
	if prefix != "" {
		for range strings.Split(prefix, "/") {
			root = filepath.Dir(root)
		}
	}
	rel, err := filepath.Rel(root, s.SpecDir)
	if err != nil || (prefix != "" && filepath.ToSlash(rel) != prefix) {
		return "", false
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false
	}
	gitRoot, err := filepath.EvalSymlinks(lines[0])
	if err != nil || physical != gitRoot {
		return "", false
	}
	return root, true
}

// RegisteredPanelPaths returns the repository-relative directories recorded for this gate.
// Older records without directory paths use the canonical spec location.
func RegisteredPanelPaths(s State, gate string) []string {
	paths := []string{}
	wanted, err := kernelid.Parse(gate)
	if err != nil {
		return paths
	}
	for _, id := range s.gateAttempts() {
		t, err := kernelid.Parse(id)
		if err != nil || t.Gate != wanted.Gate {
			continue
		}
		reg, ok, err := Decode[RegistrationRecord](s.Records[id].Metadata, KeyRegistration)
		if err != nil || !ok {
			continue
		}
		paths = append(paths, registrationPaths(s, reg)...)
	}
	return paths
}

// registrationPaths returns the repository-relative directories of one registration.
func registrationPaths(s State, reg RegistrationRecord) []string {
	paths := []string{}
	if reg.Slug == "" || reg.Slug == "." || reg.Slug == ".." || filepath.Base(reg.Slug) != reg.Slug {
		return paths
	}
	dirs := reg.Directories
	if len(dirs) == 0 {
		if rel, ok := lexicalPanelPath(s, filepath.Join(s.SpecDir, "reviews", reg.Slug)); ok {
			paths = append(paths, filepath.ToSlash(rel)+"/")
		}
		return paths
	}
	for _, dir := range dirs {
		if dir == "." || filepath.IsAbs(dir) || filepath.ToSlash(filepath.Clean(dir)) != dir || dir == ".." || strings.HasPrefix(dir, "../") {
			continue
		}
		root := s.Root
		if specRoot, ok := specPanelRoot(s); ok {
			if specRel, err := filepath.Rel(specRoot, s.SpecDir); err == nil && strings.HasPrefix(dir, filepath.ToSlash(specRel)+"/") {
				root = specRoot
			}
		}
		if rel, ok := lexicalPanelPath(s, filepath.Join(root, filepath.FromSlash(dir))); ok {
			paths = append(paths, filepath.ToSlash(rel)+"/")
		}
	}
	return paths
}

func registeredPanelArtifact(s State, gate, path string) bool {
	for _, dir := range RegisteredPanelPaths(s, gate) {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}

// FinalizePanelPaths returns recorded panel directories for this spec's gates.
func FinalizePanelPaths(s State) []string {
	allowed := map[string]bool{s.Epic + ".final": true}
	for _, item := range s.Items {
		allowed[GateBase(item)] = true
	}
	paths := map[string]bool{}
	for _, gate := range s.gateAttempts() {
		t, err := kernelid.Parse(gate)
		if err != nil || !allowed[t.Gate] || !owned(s, gate) {
			continue
		}
		for _, dir := range RegisteredPanelPaths(s, gate) {
			paths[dir] = true
		}
	}
	result := make([]string, 0, len(paths))
	for dir := range paths {
		result = append(result, dir)
	}
	sort.Strings(result)
	return result
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
		if _, err := actorGit(s.Root, "merge-base", "--is-ancestor", parents[1], rawSubject(s, gate)); err == nil {
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
		frozen := passed(s, g)
		round, _, err := Decode[GateRound](g.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		if gate != activeGate(s, t.Gate) || (!frozen && !openTallyDue(s, gate, round) && !scopedPending(g, round)) {
			continue
		}
		ended, err := judgeEnded(s, gate)
		if err != nil {
			return c, err
		}
		if ended {
			continue
		}
		newest, err := effectiveSubject(s, gate)
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
			if sub, _, _ := Decode[Subject](r.Metadata, KeySubject); sub.Commit == newest && passed(s, r) {
				live = true
			}
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

func openTallyDue(s State, gate string, round GateRound) bool {
	t, _, _ := s.Records[gate].Tuple()
	if omitted := s.ExcludedStep; omitted != nil {
		d, _, _ := omitted.Tuple()
		if d.Kind == "decision" && d.Cause == "uncounted" && d.Gate == t.Gate && d.Round == round.Round {
			return false
		}
	}
	if s.Records[gate].Status == "closed" || round.Round == 0 || VerifyApplied(s, gate, round) || gateDecision(s, t.Gate, false) {
		return false
	}
	if _, _, ok := verifyReturns(s, round); ok {
		return true
	}
	for id, answer := range s.Answers() {
		d, _, _ := s.Records[id].Tuple()
		if d.Kind != "decision" || d.Cause != "uncounted" || d.Gate != t.Gate || d.Round != round.Round || answer.Option != "redispatch" {
			continue
		}
		decision, _, _ := Decode[DecisionRecord](s.Records[id].Metadata, KeyDecision)
		for _, slot := range decision.Slots {
			head, err := SlotLineageHead(s, fmt.Sprintf("%s.r%d.%s", t.Gate, round.Round, slot.Slot))
			if err != nil {
				return false
			}
			attempt, _, _ := head.Tuple()
			if attempt.Attempt <= slot.Attempt {
				return false
			}
		}
	}
	if len(round.Slots) == 0 {
		return false
	}
	for _, root := range round.Slots {
		r, err := SlotLineageHead(s, root)
		if err != nil || !passed(s, r) {
			return false
		}
	}
	return true
}

// TallyHeldForJudge holds gate outcomes until the subject change is judged.
func TallyHeldForJudge(s State, gate string) (bool, error) {
	round, _, err := Decode[GateRound](s.Records[gate].Metadata, KeyRound)
	if err != nil {
		return false, err
	}
	return tallyHeldForJudge(s, gate, round)
}

func tallyHeldForJudge(s State, gate string, round GateRound) (bool, error) {
	if VerifyApplied(s, gate, round) {
		return false, nil
	}
	t, _, _ := s.Records[gate].Tuple()
	for _, id := range s.Steps {
		r := s.Records[id]
		jt, _, _ := r.Tuple()
		if jt.Kind == "change-judge" && jt.Gate == t.Gate && r.Status != "closed" {
			return true, nil
		}
	}
	if !openTallyDue(s, gate, round) {
		return false, nil
	}
	newest, err := effectiveSubject(s, gate)
	if err != nil {
		return false, err
	}
	accepted, err := AcceptedSubject(s, gate)
	if err != nil {
		return false, err
	}
	for _, id := range s.Steps {
		r := s.Records[id]
		jt, _, _ := r.Tuple()
		sub, _, _ := Decode[Subject](r.Metadata, KeySubject)
		if jt.Kind == "change-judge" && jt.Gate == t.Gate && sub.Commit == newest && passed(s, r) {
			return false, nil
		}
	}
	return newest != "" && newest != accepted, nil
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
		newest, err := effectiveSubject(s, gate)
		if err != nil {
			return c, err
		}
		accepted, err := AcceptedSubject(s, gate)
		if err != nil {
			return c, err
		}
		sub, present, err := Decode[Subject](r.Metadata, KeySubject)
		if err != nil {
			return c, err
		}
		// A missing subject is not evidence that the judge is stale.
		if !present || sub.Commit == "" {
			continue
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

func readJudgement(r Record) (Judgement, error) {
	data, err := recordBytes(r)
	if err != nil {
		return Judgement{}, err
	}
	return strictRecord[Judgement](data)
}

func checkJudgement(s State, r Record, _ Receipt) error {
	return validateJudgement(s, r, true)
}

func validateJudgement(s State, r Record, current bool) error {
	t, _, _ := r.Tuple()
	gate := activeGate(s, t.Gate)
	g := s.Records[gate]
	source := judgementGate(s, g)
	if source.Status == "closed" && !passed(s, source) {
		return actorFailure("judgement needs a passing gate or an open review")
	}
	ended, err := judgeEnded(s, gate)
	if err != nil {
		return err
	}
	if current && ended {
		return actorFailure("review subject has already finished")
	}
	j, err := readJudgement(r)
	if err != nil {
		return err
	}
	accepted, err := AcceptedSubject(s, gate)
	if err != nil {
		return err
	}
	newest, err := effectiveSubject(s, gate)
	if err != nil {
		return err
	}
	sub, _, err := Decode[Subject](r.Metadata, KeySubject)
	if err != nil {
		return err
	}
	reg, registered, err := Decode[RegistrationRecord](source.Metadata, KeyRegistration)
	if err != nil {
		return err
	}
	round, _, err := Decode[GateRound](source.Metadata, KeyRound)
	if err != nil {
		return err
	}
	ceiling := round.SlotCount
	// Use the registered passing roster when no round count is stored.
	if ceiling == 0 {
		ceiling = len(reg.Slots)
	}
	expectedNew := sub.Commit
	if current {
		expectedNew = newest
	}
	if j.OldSubject != accepted || (current && sub.Commit != newest) || j.NewSubject != sub.Commit || j.OldSubject == j.NewSubject {
		return actorFailure(fmt.Sprintf("judgement subject is stale; expected old subject %s and new subject %s; slot ceiling %d", accepted, expectedNew, ceiling))
	}
	if !registered {
		return actorFailure("judgement needs a registered panel directory")
	}
	switch j.Value {
	case "carry-over":
		if strings.TrimSpace(j.Reason) == "" || j.SlotCount != 0 || len(j.Paths) != 0 {
			return actorFailure("carry-over needs a reason and no round scope")
		}
	case "re-panel":
		if j.SlotCount < 2 || j.SlotCount > ceiling || len(j.Paths) == 0 {
			return actorFailure(fmt.Sprintf("re-panel needs changed paths and 2 to %d slots; expected old subject %s and new subject %s", ceiling, accepted, sub.Commit))
		}
		for _, name := range j.Paths {
			if strings.TrimSpace(name) == "" || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
				return actorFailure(fmt.Sprintf("re-panel paths must be repository-relative files; expected old subject %s and new subject %s; slot ceiling %d", accepted, expectedNew, ceiling))
			}
		}
		changed, err := subjectPaths(s, gate, j.OldSubject, j.NewSubject)
		if err != nil {
			return err
		}
		covered := map[string]bool{}
		for _, name := range j.Paths {
			covered[name] = true
		}
		var missing []string
		for _, name := range changed {
			if !covered[name] {
				missing = append(missing, name)
			}
		}
		if len(missing) != 0 {
			return actorFailure("re-panel paths miss changed paths: " + strings.Join(missing, ", "))
		}
	default:
		return actorFailure("judgement must be carry-over or re-panel")
	}
	return nil
}

// scopedPending identifies a judged authorization that has not been registered.
func scopedPending(g Record, round GateRound) bool {
	scope, _, _ := Decode[RePanel](g.Metadata, KeyRePanel)
	reg, has, _ := Decode[RegistrationRecord](g.Metadata, KeyRegistration)
	return g.Status != "closed" && scope.Judge != "" && len(round.Paths) > 0 && (!has || reg.Round < round.Round)
}

func judgementGate(s State, g Record) Record {
	round, _, _ := Decode[GateRound](g.Metadata, KeyRound)
	if scopedPending(g, round) {
		if round.Prior != nil {
			g.Metadata = mergedMetadata(g.Metadata, Metadata(KeyRound, *round.Prior))
		} else if from, has, _ := Decode[From](g.Metadata, KeyFrom); has {
			return s.Records[string(from)]
		}
	}
	return g
}

func subjectPaths(s State, gate, old, newest string) ([]string, error) {
	data, err := actorGitBytes(s.Root, "diff", "--no-renames", "--name-only", "-z", old, newest, "--")
	if err != nil {
		return nil, pathFailure(err, "cannot read changed paths: ", "restore the subject commits, then run mindspec next-action")
	}
	var paths []string
	for _, name := range strings.Split(string(data), "\x00") {
		if name != "" && !registeredPanelArtifact(s, gate, name) {
			paths = append(paths, name)
		}
	}
	return paths, nil
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
		j, err := readJudgement(r)
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
		for _, candidate := range s.gateAttempts() {
			record := s.Records[candidate]
			prior, _, _ := Decode[Receipt](record.Metadata, KeyClose)
			for _, carry := range prior.CarryOvers {
				applied = applied || carry.Judge == id
			}
			round, _, _ := Decode[GateRound](record.Metadata, KeyRound)
			applied = applied || round.Judged != nil && round.Judged.Judge == id || round.Prior != nil && round.Prior.Judged != nil && round.Prior.Judged.Judge == id
			from, hasFrom, _ := Decode[From](record.Metadata, KeyFrom)
			scope, hasScope, _ := Decode[RePanel](record.Metadata, KeyRePanel)
			applied = applied || (hasFrom && hasScope && sameJudgeGate(string(from), t.Gate) && scope.Judge == id)
		}
		if applied {
			continue
		}
		// Recheck durable evidence when a crash left only the actor's receipt and close.
		if err := validateJudgement(s, r, false); err != nil {
			return c, err
		}
		round, _, err := Decode[GateRound](g.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		if scopedPending(g, round) {
			for _, w := range retireLens(s, round.Lens) {
				c.Writes = append(c.Writes, w)
				if err := overlay(&s, w); err != nil {
					return c, err
				}
			}
			if round.Prior != nil {
				round = *round.Prior
				w := Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyRound, round)}
				c.Writes = append(c.Writes, w)
				if err := overlay(&s, w); err != nil {
					return c, err
				}
			} else {
				w := Write{Kind: "close", ID: gate, Receipt: &Receipt{Kind: "superseded", Subject: j.NewSubject}}
				c.Writes = append(c.Writes, w)
				if err := overlay(&s, w); err != nil {
					return c, err
				}
				gate = activeGate(s, t.Gate)
			}
			g = s.Records[gate]
		}
		if g.Status != "closed" {
			round, _, err = Decode[GateRound](g.Metadata, KeyRound)
			if err != nil {
				return c, err
			}
			round.Judged = &Judged{Judge: id, Value: j.Value, NewSubject: j.NewSubject}
			metadata := Metadata(KeyRound, round)
			metadata[KeyRePanel] = nil
			if j.Value == "re-panel" {
				metadata[KeyRePanel] = RePanel{Judge: id, SlotCount: j.SlotCount, Paths: j.Paths}
			}
			w := Write{Kind: "metadata", ID: gate, Metadata: metadata}
			c.Writes = append(c.Writes, w)
			if err := overlay(&s, w); err != nil {
				return c, err
			}
			// The restored verify is no longer applied. Retire it once on the new subject.
			if round.Verify != "" {
				more, err := replaceJudgedVerify(s, gate, round)
				if err != nil {
					return c, err
				}
				c.Writes = append(c.Writes, more...)
				for _, w := range more {
					if err := overlay(&s, w); err != nil {
						return c, err
					}
				}
			}
			c.Writes = append(c.Writes, Write{Kind: "metadata", ID: id, Metadata: Metadata(KeyJudgement, j)})
			continue
		}
		frozen, _, err = Decode[Receipt](g.Metadata, KeyClose)
		if err != nil {
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
			attempt := gt.Attempt
			for _, candidate := range s.gateAttempts() {
				ct, _, _ := s.Records[candidate].Tuple()
				if ct.Gate == gt.Gate {
					attempt = max(attempt, ct.Attempt)
				}
			}
			next := fmt.Sprintf("%s.a%d", gt.Gate, attempt+1)
			count, err := RoundsForGate(s, gate)
			if err != nil {
				return c, err
			}
			round := GateRound{Round: count + 1, Subject: j.NewSubject, SlotCount: j.SlotCount, Paths: j.Paths, Slots: []string{}}
			for i := 1; i <= j.SlotCount; i++ {
				round.Slots = append(round.Slots, fmt.Sprintf("%s.r%d.R%d", gt.Gate, round.Round, i))
			}
			w := createStep(next, judgeParent(gt))
			w.Create.Metadata = map[string]any{KeyFrom: From(gate), KeyRePanel: RePanel{Judge: id, SlotCount: j.SlotCount, Paths: j.Paths}, KeyRound: round}
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
