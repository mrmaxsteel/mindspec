package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func init() {
	if err := RegisterCloseWriteHook("verify", verifyCloseWrites); err != nil {
		panic(err)
	}
}

func reviewSubject(round GateRound) string {
	if round.Judged != nil {
		return round.Judged.NewSubject
	}
	return round.Subject
}

func findingKey(slot string, round int, finding, sha string) string {
	return fmt.Sprintf("%s/%d/%s/%s", slot, round, finding, sha)
}

func returnsCover(findings []Finding, returns []VerifyReturn) bool {
	if len(findings) == 0 || len(findings) != len(returns) {
		return false
	}
	expected := map[string]bool{}
	for _, f := range findings {
		expected[findingKey(f.Slot, f.Round, f.Finding, f.FindingSHA256)] = true
	}
	for _, r := range returns {
		key := findingKey(r.Slot, r.Round, r.Finding, r.FindingSHA256)
		if !expected[key] || (r.Return != "reproduced" && r.Return != "refuted" && r.Return != "deferred") {
			return false
		}
		delete(expected, key)
	}
	return len(expected) == 0
}

// roundFindings uses the counted tally for findings and the actual file bytes
// for veto identity and the read set. It never stores a shortened finding.
func roundFindings(s State, res *panel.Result) ([]Finding, map[string]string, error) {
	names, err := panel.RoundVerdictFiles(res.Dir, res.LatestRound)
	if err != nil {
		return nil, nil, pathFailure(err, "cannot read round verdict files: ", "restore the panel directory and retry")
	}
	root := workspace.TreeRootForSpecDir(s.SpecDir)
	if root == "" {
		root = s.Root
	}
	read := map[string]string{}
	bytesByName := map[string][]byte{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(res.Dir, name))
		if err != nil {
			return nil, nil, pathFailure(err, "cannot read round verdict: ", "restore the panel verdict file and retry")
		}
		rel, err := filepath.Rel(root, filepath.Join(res.Dir, name))
		if err != nil {
			return nil, nil, pathFailure(err, "cannot locate round verdict: ", "restore the panel directory and retry")
		}
		read[filepath.ToSlash(rel)] = recordDigest(data)
		bytesByName[name] = data
	}
	var findings []Finding
	addVeto := func(v panel.Verdict) {
		data := bytesByName[v.File]
		var payload panel.FindingVerdict
		if parsed, err := panel.ParseVerdictV2(data); err == nil {
			payload = parsed.FindingVerdict()
		} else {
			_ = json.Unmarshal(data, &payload)
		}
		texts := append([]string{payload.Rationale}, payload.ConcreteChangesRequired...)
		hasText := false
		for i, text := range texts {
			hasText = hasText || text != ""
			texts[i] = panel.RedactFindingPaths(text)
		}
		text := strings.Join(texts, "\n")
		if !hasText {
			text = "the verdict file gives no reason"
		}
		findings = append(findings, Finding{Slot: v.Slot, Round: v.Round, Finding: "veto", FindingSHA256: recordDigest(data), Text: text})
	}
	for _, v := range res.Verdicts {
		if v.Verdict == panel.VerdictReject || v.HardBlock {
			addVeto(v)
			continue
		}
		if v.Verdict != panel.VerdictRequestChanges {
			continue
		}
		var payload panel.FindingVerdict
		_ = json.Unmarshal(bytesByName[v.File], &payload)
		if res.VerdictSchema != 2 && len(payload.ConcreteChangesRequired) == 0 {
			findings = append(findings, Finding{Slot: v.Slot, Round: v.Round, Finding: "slot", FindingSHA256: v.SlotSHA256, Text: panel.RedactFindingPaths(payload.Rationale)})
			continue
		}
		for _, f := range v.Findings {
			findings = append(findings, Finding{Slot: v.Slot, Round: v.Round, Finding: f.ID, FindingSHA256: f.SHA256, Text: f.Text})
		}
	}
	for _, v := range res.Unexpected {
		if v.Verdict == panel.VerdictReject || v.HardBlock || v.Invalid || (v.Verdict != panel.VerdictApprove && v.Verdict != panel.VerdictRequestChanges) {
			addVeto(v)
		}
	}
	return findings, read, nil
}

func currentVerify(s State, gate string, n int) string {
	round, _, _ := Decode[GateRound](s.Records[activeGate(s, gate)].Metadata, KeyRound)
	if round.Round == n {
		return round.Verify
	}
	if round.Prior != nil && round.Prior.Round == n {
		return round.Prior.Verify
	}
	id, attempt := "", 0
	for _, step := range s.Steps {
		t, _, _ := s.Records[step].Tuple()
		if t.Gate == gate && t.Kind == "verify" && t.Round == n && t.Attempt > attempt {
			id, attempt = step, t.Attempt
		}
	}
	return id
}

// KeepDisposition determines whether a disposition can discharge a finding.
// Retired and unfinished verify attempts remain in the ledger as history.
func KeepDisposition(s State, gate string, round int, verifyStep, createdAt string) bool {
	if verifyStep != "" {
		r := s.Records[verifyStep]
		t, has, err := r.Tuple()
		currentRound, _, _ := Decode[GateRound](s.Records[activeGate(s, gate)].Metadata, KeyRound)
		if currentRound.Round != round && currentRound.Prior != nil {
			currentRound = *currentRound.Prior
		}
		if currentRound.Round == round {
			subject, _, _ := Decode[Subject](r.Metadata, KeySubject)
			if subject.Commit != reviewSubject(currentRound) {
				return false
			}
		}
		return err == nil && has && t.Kind == "verify" && t.Gate == gate && t.Round == round && currentVerify(s, gate, round) == verifyStep && passed(s, r)
	}
	var latest time.Time
	attempt := 0
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if t.Kind != "verify" || t.Gate != gate || t.Round != round || t.Attempt <= attempt || !passed(s, r) {
			continue
		}
		receipt, _, _ := Decode[Receipt](r.Metadata, KeyClose)
		at, err := time.Parse(time.RFC3339Nano, receipt.At)
		if err == nil {
			latest, attempt = at, t.Attempt
		}
	}
	if attempt == 0 {
		return true
	}
	at, err := time.Parse(time.RFC3339Nano, createdAt)
	return err == nil && at.After(latest)
}

// FilterDispositions copies the panel and both disposition slices. Callers may
// write their original tally back without deleting audit history.
func FilterDispositions(s State, gate string, res *panel.Result) *panel.Result {
	copyResult := *res
	if res.Panel == nil {
		return &copyResult
	}
	p := *res.Panel
	p.Refutations = []panel.Refutation{}
	p.Deferrals = []panel.Deferral{}
	for _, ref := range res.Panel.Refutations {
		if KeepDisposition(s, gate, ref.Round, ref.VerifyStep, ref.CreatedAt) {
			p.Refutations = append(p.Refutations, ref)
		}
	}
	for _, d := range res.Panel.Deferrals {
		if KeepDisposition(s, gate, d.Round, d.VerifyStep, d.CreatedAt) {
			p.Deferrals = append(p.Deferrals, d)
		}
	}
	copyResult.Panel = &p
	return &copyResult
}

func verifyReturns(s State, round GateRound) ([]Finding, []VerifyReturn, bool) {
	r := s.Records[round.Verify]
	if round.Verify == "" || !passed(s, r) {
		return nil, nil, false
	}
	findings, _, err := Decode[[]Finding](r.Metadata, KeyFindings)
	if err != nil {
		return nil, nil, false
	}
	returns, _, err := Decode[[]VerifyReturn](r.Metadata, KeyReturns)
	if err != nil || !returnsCover(findings, returns) {
		return nil, nil, false
	}
	data, err := recordBytes(r)
	if err != nil {
		return nil, nil, false
	}
	v, err := strictRecord[VerifyRecord](data)
	if err != nil || v.Step != r.Key() || v.Subject != reviewSubject(round) || !reflect.DeepEqual(v.Returns, returns) {
		return nil, nil, false
	}
	return findings, returns, true
}

// VerifyApplied reports whether the current verify has produced an outcome.
func VerifyApplied(s State, gate string, round GateRound) bool {
	if _, _, ok := verifyReturns(s, round); !ok {
		return false
	}
	g := s.Records[gate]
	current, _, _ := Decode[GateRound](g.Metadata, KeyRound)
	if current.Round > round.Round && current.Prior != nil && current.Prior.Verify == round.Verify {
		return true
	}
	if receipt, _, _ := Decode[Receipt](g.Metadata, KeyClose); g.Status == "closed" && receipt.Kind == "passed" && receipt.Retally == "verify" {
		return true
	}
	steps := make([]Record, 0, len(s.Steps)+1)
	for _, id := range s.Steps {
		steps = append(steps, s.Records[id])
	}
	// The first half of a verb's pass omits its target from rule writes.
	// An existing outcome still freezes the round while that fix is submitted
	// or that decision is answered.
	if s.ExcludedStep != nil {
		steps = append(steps, *s.ExcludedStep)
	}
	for _, r := range steps {
		from, _, _ := Decode[From](r.Metadata, KeyFrom)
		t, _, _ := r.Tuple()
		if string(from) == round.Verify && t.Gate != "" && (t.Kind == "fix" || t.Kind == "decision") {
			return true
		}
	}
	return false
}

// VerifyRetallyFits checks the frozen round against the files on disk, including
// added and removed off-roster files. The panel snapshot is not in this set.
func VerifyRetallyFits(s State, gate string, res *panel.Result) error {
	round, _, err := Decode[GateRound](s.Records[gate].Metadata, KeyRound)
	if err != nil || !VerifyApplied(s, gate, round) {
		return err
	}
	findings, read, err := roundFindings(s, res)
	if err != nil {
		return err
	}
	_, returns, ok := verifyReturns(s, round)
	wanted, _, err := Decode[map[string]string](s.Records[round.Verify].Metadata, KeyRead)
	if err != nil {
		return err
	}
	var changed []string
	for path, sha := range wanted {
		if read[path] != sha {
			changed = append(changed, path)
		}
	}
	for path := range read {
		if _, exists := wanted[path]; !exists {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	if len(changed) != 0 {
		return actorFailure("verify read set differs at: " + strings.Join(changed, ", "))
	}
	if !ok || !returnsContain(findings, returns) {
		return actorFailure("verify returns do not cover the round findings")
	}
	return nil
}

// returnsContain covers the findings on disk; the frozen read-set check also
// requires the original file set, so an extra pinned return cannot hide removal.
func returnsContain(findings []Finding, returns []VerifyReturn) bool {
	for _, f := range findings {
		found := false
		for _, r := range returns {
			found = found || findingKey(f.Slot, f.Round, f.Finding, f.FindingSHA256) == findingKey(r.Slot, r.Round, r.Finding, r.FindingSHA256)
		}
		if !found {
			return false
		}
	}
	return true
}

func verifyPassed(s State, round GateRound) bool {
	_, returns, ok := verifyReturns(s, round)
	if !ok {
		return false
	}
	for _, ret := range returns {
		if ret.Return == "reproduced" {
			return false
		}
	}
	return true
}

func verifyPrior(s State, round GateRound) GateRound {
	findings, returns, ok := verifyReturns(s, round)
	if !ok {
		round.Verify = ""
		return round
	}
	round.Read, _, _ = Decode[map[string]string](s.Records[round.Verify].Metadata, KeyRead)
	round.Asks = []PriorAsk{}
	for _, f := range findings {
		ask := PriorAsk{Finding: f}
		for _, ret := range returns {
			if findingKey(f.Slot, f.Round, f.Finding, f.FindingSHA256) == findingKey(ret.Slot, ret.Round, ret.Finding, ret.FindingSHA256) {
				ask.Returns = append(ask.Returns, ret)
			}
		}
		round.Asks = append(round.Asks, ask)
	}
	return round
}

func verifyCloseWrites(s State, r Record, receipt Receipt) ([]Write, error) {
	data, err := recordBytes(r)
	if err != nil {
		return nil, err
	}
	v, err := strictRecord[VerifyRecord](data)
	if err != nil {
		return nil, err
	}
	t, _, _ := r.Tuple()
	gate := activeGate(s, t.Gate)
	registration, _, err := Decode[RegistrationRecord](s.Records[gate].Metadata, KeyRegistration)
	if err != nil {
		return nil, err
	}
	writes := []Write{{Kind: "metadata", ID: r.Key(), Metadata: Metadata(KeyReturns, v.Returns)}}
	// A veto needs no panel disposition. A record with only vetoes can also be
	// checked without reading a registration snapshot.
	needPanel := false
	for _, ret := range v.Returns {
		needPanel = needPanel || (ret.Finding != "veto" && ret.Return != "reproduced")
	}
	if !needPanel {
		return writes, nil
	}
	var res *panel.Result
	for _, p := range s.Panels {
		if p.Slug() == registration.Slug {
			res, err = TallyPanel(s, p)
			break
		}
	}
	if err != nil {
		return nil, pathFailure(err, "cannot read verify panel: ", "restore the panel directory and retry submit")
	}
	if res == nil || res.Panel == nil || res.Intact() != nil {
		return nil, actorFailure("verify close needs an intact registered panel")
	}
	p := *res.Panel
	p.Refutations = append([]panel.Refutation{}, p.Refutations...)
	p.Deferrals = append([]panel.Deferral{}, p.Deferrals...)
	findings, _, err := Decode[[]Finding](r.Metadata, KeyFindings)
	if err != nil {
		return nil, err
	}
	read, _, err := Decode[map[string]string](r.Metadata, KeyRead)
	if err != nil {
		return nil, err
	}
	sources, _, err := Decode[VerifyVerdictSources](r.Metadata, KeyVerifyVerdicts)
	if err != nil {
		return nil, err
	}
	history, _, err := Decode[[]DeferRecord](s.Records[gate].Metadata, KeyDeferrals)
	if err != nil {
		return nil, err
	}
	var records [][]byte
	for _, ret := range v.Returns {
		if ret.Finding == "veto" || ret.Return == "reproduced" {
			continue
		}
		text := ""
		for _, f := range findings {
			if findingKey(f.Slot, f.Round, f.Finding, f.FindingSHA256) == findingKey(ret.Slot, ret.Round, ret.Finding, ret.FindingSHA256) {
				text = f.Text
			}
		}
		row := panel.DispositionRow{Record: panel.RecordDisposition, Spec: panel.RecordSpec(p), Gate: p.Gate, Panel: registration.Slug, Reviewer: ret.Slot, Model: res.ReviewerModel(ret.Slot), Severity: "blocking", Summary: termsafe.Escape(text), ConvergentWith: []string{}, CreatedAt: receipt.At, Round: &ret.Round}
		var event any
		if ret.Return == "refuted" {
			evidence := ret.Evidence
			if filepath.IsAbs(evidence) {
				resolved, err := filepath.EvalSymlinks(evidence)
				if err != nil {
					return nil, pathFailure(err, "cannot resolve verify evidence: ", "give a resolving repository path, commit or HTTPS URL")
				}
				root, err := filepath.EvalSymlinks(s.Root)
				if err != nil {
					return nil, pathFailure(err, "cannot resolve verify repository: ", "restore access to the repository and retry submit")
				}
				relative, err := filepath.Rel(root, resolved)
				if err != nil {
					return nil, pathFailure(err, "cannot locate verify evidence: ", "give a resolving repository path, commit or HTTPS URL")
				}
				evidence = filepath.ToSlash(relative)
			}
			ref := panel.Refutation{Slot: ret.Slot, Round: ret.Round, Finding: ret.Finding, FindingSHA256: ret.FindingSHA256, FindingText: text, Ground: ret.Ground, AC: ret.AC, Reason: ret.Reason, Evidence: evidence, By: r.Key(), CreatedAt: receipt.At, VerifyStep: r.Key(), Enrolled: true}
			event = panel.RefutationRecord{Record: panel.RecordRefutation, Spec: panel.RecordSpec(p), Panel: registration.Slug, Refutation: ref}
			row.ID, row.Disposition, row.EvidenceRef = panel.RefutationRowID(registration.Slug, ref), "audited-refuted", evidence
			found := false
			for _, old := range p.Refutations {
				found = found || old == ref
			}
			if !found {
				p.Refutations = append(p.Refutations, ref)
			}
		} else {
			source := sources[ret.Slot].FileSHA256
			name := fmt.Sprintf("%s-round-%d.json", ret.Slot, ret.Round)
			if source == "" {
				for path, hash := range read {
					if filepath.Base(path) == name {
						source = hash
					}
				}
			}
			if source == "" {
				return nil, actorFailure("deferred finding has no pinned source verdict")
			}
			d := panel.Deferral{Slot: ret.Slot, Round: ret.Round, Finding: ret.Finding, FindingSHA256: ret.FindingSHA256, FindingText: text, Bead: ret.Target, By: r.Key(), CreatedAt: receipt.At, VerifyStep: r.Key(), Enrolled: true, SourceVerdict: panel.VerdictRequestChanges, SourceFileSHA256: source}
			event = panel.DeferralRecord{Record: panel.RecordDeferral, Spec: panel.RecordSpec(p), Panel: registration.Slug, Deferral: d}
			row.ID, row.Disposition, row.EvidenceRef = panel.DeferralRowID(registration.Slug, d), "confirmed-deferred", ret.Target
			found := false
			for _, old := range p.Deferrals {
				found = found || old == d
			}
			if !found {
				p.Deferrals = append(p.Deferrals, d)
			}
			at, err := time.Parse(time.RFC3339Nano, receipt.At)
			if err != nil {
				return nil, actorFailure("verify receipt needs a valid at")
			}
			dr := DeferRecord{Panel: registration.Slug, Round: ret.Round, Slot: ret.Slot, Finding: ret.Finding, FindingSHA256: ret.FindingSHA256, To: ret.Target, Reason: ret.Reason, At: at.UTC(), SourceVerdict: d.SourceVerdict, SourceFileSHA256: source, VerifyStep: r.Key()}
			found = false
			for i, old := range history {
				if old.VerifyStep == dr.VerifyStep && old.Panel == dr.Panel && old.Round == dr.Round && old.Slot == dr.Slot && old.Finding == dr.Finding && old.FindingSHA256 == dr.FindingSHA256 {
					history[i], found = dr, true
				}
			}
			if !found {
				history = append(history, dr)
			}
		}
		for _, value := range []any{event, row} {
			line, err := json.Marshal(value)
			if err != nil {
				return nil, pathFailure(err, "cannot encode verify disposition: ", "correct the verify record and retry submit")
			}
			records = append(records, line)
		}
	}
	writes = append(writes, Write{Kind: "metadata", ID: gate, Metadata: Metadata(KeyDeferrals, history)}, Write{Kind: "panel", ID: r.Key(), Panel: &PanelWrite{Dir: res.Dir, Snapshot: p, Records: records}})
	return writes, nil
}

func verifyApply(s State) (Changes, error) {
	c := Changes{}
	for _, gate := range s.Gates {
		g := s.Records[gate]
		t, _, _ := g.Tuple()
		if gate != activeGate(s, t.Gate) || g.Status == "closed" {
			continue
		}
		round, _, err := Decode[GateRound](g.Metadata, KeyRound)
		if err != nil {
			return c, err
		}
		if held, err := tallyHeldForJudge(s, gate, round); err != nil {
			return c, err
		} else if held {
			continue
		}
		findings, returns, ok := verifyReturns(s, round)
		if !ok {
			continue
		}
		var reproduced []Finding
		reject := false
		for _, ret := range returns {
			if ret.Return != "reproduced" {
				continue
			}
			for _, f := range findings {
				if findingKey(f.Slot, f.Round, f.Finding, f.FindingSHA256) == findingKey(ret.Slot, ret.Round, ret.Finding, ret.FindingSHA256) {
					reproduced = append(reproduced, f)
				}
			}
		}
		if len(reproduced) == 0 {
			if !hasOpenChild(s, gate) && round.Judged != nil && round.Judged.Value == "re-panel" {
				writes, err := scopedRoundWrites(s, gate, round)
				if err != nil {
					return c, err
				}
				c.Writes = append(c.Writes, writes...)
				continue
			}
			if !hasOpenChild(s, gate) {
				c.Writes = append(c.Writes, Write{Kind: "close", ID: gate, Receipt: &Receipt{Kind: "passed", Retally: "verify", Subject: reviewSubject(round)}})
			}
			continue
		}
		count, err := RoundsForGate(s, gate)
		if err != nil {
			return c, err
		}
		if VerifyApplied(s, gate, round) {
			// An answered limit may release the fix; other applied outcomes are final.
			release := false
			for id, answer := range s.Answers() {
				d, _, _ := s.Records[id].Tuple()
				release = release || d.Gate == t.Gate && d.Cause == "limit" && d.Round == round.Round && strings.HasPrefix(answer.Option, "continue:")
			}
			if !release {
				// A crash can persist the fix before its paired reject decision.
				// Finish that outcome only while the original fix is still open.
				fix := s.Records[fmt.Sprintf("%s.r%d.fix", t.Gate, round.Round+1)]
				from, _, _ := Decode[From](fix.Metadata, KeyFrom)
				if fix.Status != "closed" && string(from) == round.Verify {
					sources, _, _ := Decode[VerifyVerdictSources](s.Records[round.Verify].Metadata, KeyVerifyVerdicts)
					for _, f := range reproduced {
						if f.Finding == "veto" && sources[f.Slot].Verdict == panel.VerdictReject {
							more := decisionWrite(s, gate, "reject", round.Round)
							for i := range more {
								more[i].Create.Metadata[KeyFrom] = From(round.Verify)
							}
							c.Writes = append(c.Writes, more...)
							break
						}
					}
				}
				continue
			}
		}
		var writes []Write
		if count >= Allowance(s, gate) {
			writes = decisionWrite(s, gate, "limit", round.Round)
		} else {
			writes = fixWrite(s, gate, round.Round+1, reviewSubject(round))
			for i := range writes {
				writes[i].Create.Metadata[KeyReproduced] = reproduced
			}
			// The pinned source distinguishes a reproduced REJECT without
			// another read of a mutable verdict file.
			sources, _, _ := Decode[VerifyVerdictSources](s.Records[round.Verify].Metadata, KeyVerifyVerdicts)
			for _, f := range reproduced {
				reject = reject || f.Finding == "veto" && sources[f.Slot].Verdict == panel.VerdictReject
			}
			for id, answer := range s.Answers() {
				d, _, _ := s.Records[id].Tuple()
				if d.Gate == t.Gate && d.Round == round.Round && d.Cause == "limit" && strings.HasPrefix(answer.Option, "continue:") {
					reject = false
				}
			}
			if reject {
				writes = append(writes, decisionWrite(s, gate, "reject", round.Round)...)
			}
		}
		for i := range writes {
			writes[i].Create.Metadata[KeyFrom] = From(round.Verify)
		}
		c.Writes = append(c.Writes, writes...)
	}
	return c, nil
}
