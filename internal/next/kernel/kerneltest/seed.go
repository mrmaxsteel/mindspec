package kerneltest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func PassGate(f *Fixture, gate, subject string) {
	round, has, err := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
	f.Must(err)
	if !has {
		round = kernel.GateRound{Round: 1, Subject: subject, SlotCount: 3, Slots: []string{}}
	}
	round.Subject = subject
	reg, has, err := kernel.Decode[kernel.RegistrationRecord](f.State().Records[gate].Metadata, kernel.KeyRegistration)
	f.Must(err)
	if !has {
		reg = kernel.RegistrationRecord{Slug: gate, Round: round.Round, Subject: subject}
		for i := 1; i <= round.SlotCount; i++ {
			reg.Slots = append(reg.Slots, fmt.Sprintf("R%d", i))
		}
	}
	needsRegistration := !has || reg.Subject != subject
	if has && reg.Subject != subject {
		reg.Slug += "-" + subject
	}
	reg.Subject = subject
	round.Round = reg.Round
	round.SlotCount = len(reg.Slots)
	round.Slots = nil
	tuple, err := kernelid.Parse(gate)
	f.Must(err)
	for _, slot := range reg.Slots {
		round.Slots = append(round.Slots, fmt.Sprintf("%s.r%d.%s", tuple.Gate, round.Round, slot))
	}
	Authorise(f, gate, round) //nolint:misspell // Keep the shared fixture name.
	if needsRegistration {
		Register(f, gate, reg)
	}
	seedPassingSlots(f, gate, reg)
	f.Must(f.Store.WriteReceipt(gate, kernel.Receipt{Kind: "passed", Subject: subject, Retally: "passing-tally"}))
	closeRecord(f, gate)
}

func seedPassingSlots(f *Fixture, gate string, reg kernel.RegistrationRecord) {
	tuple, err := kernelid.Parse(gate)
	f.Must(err)
	dir := filepath.Join(f.SpecDir, "reviews", reg.Slug)
	for _, slot := range reg.Slots {
		v := paneltest.Verdict("APPROVE")
		v.Slot, v.Round, v.ReviewedSHA, v.Model = slot, reg.Round, reg.Subject, "test-model"
		_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: reg.Slug, Slot: slot, Round: reg.Round}, paneltest.Bytes(f.t, v), func(p panel.Panel, roster panel.RosterSlot) (panel.SubmitResolution, error) {
			return panel.SubmitResolution{Family: roster.Family, Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
		})
		f.Must(err)
		id := fmt.Sprintf("%s.r%d.%s", tuple.Gate, reg.Round, slot)
		if _, exists := f.State().Records[id]; !exists {
			f.Create(id, gate)
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: reg.Subject})))
		}
		f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Retally: "seeded-verdict"}))
		closeRecord(f, id)
	}
}

func closeRecord(f *Fixture, id string) {
	f.Must(kernel.WithCASRetry(func() error {
		r, err := f.Store.Lookup(id)
		if err != nil {
			return err
		}
		if r.Status == "closed" {
			return nil
		}
		return f.Store.SetStatusIf(id, r.Status, "closed")
	}))
}

//nolint:misspell // Keep the public fixture name used by downstream producers.
func Authorise(f *Fixture, gate string, round kernel.GateRound) {
	if round.Lens == "" {
		tuple, err := kernelid.Parse(gate)
		f.Must(err)
		round.Lens = fmt.Sprintf("%s.r%d.lens", tuple.Gate, round.Round)
	}
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
	cfg, err := config.Load(f.Root)
	f.Must(err)
	kind := "bead"
	tuple, err := kernelid.Parse(gate)
	f.Must(err)
	if tuple.WorkItem == "" {
		kind = "final_review"
	}
	roster, err := kernel.RoundReviewerSlots(cfg, kind, round)
	f.Must(err)
	lenses := []kernel.SlotLens{}
	for _, slot := range roster {
		if slot.Lens != "adversarial" {
			lenses = append(lenses, kernel.SlotLens{Slot: slot.Slot, Lens: "Review " + slot.Lens + "."})
		}
	}
	Lens(f, gate, kernel.LensRecord{Step: round.Lens, Lenses: lenses})
}
func OpenDecision(f *Fixture, gate, cause string, round int, options ...string) string {
	id := fmt.Sprintf("%s.decide.%s.r%d", gate, cause, round)
	tuple, err := kernelid.Parse(id)
	f.Must(err)
	metadata := kernel.Metadata(kernel.KeyDecision, kernel.DecisionRecord{Gate: tuple.Gate, Cause: cause, Options: options})
	roundRecord, _, err := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
	f.Must(err)
	if roundRecord.Verify != "" {
		metadata[kernel.KeyFrom] = kernel.From(roundRecord.Verify)
	}
	_, err = f.Store.Create(kernel.CreateRecord{ID: id, Parent: gate, Type: "decision", Title: "decision: halt stops the spec and every work item", Tuple: tuple, Metadata: metadata})
	f.Must(err)
	return id
}
func Answer(f *Fixture, decision, option string) {
	answer := kernel.DecisionAnswer{Option: option, At: kernel.Now().UTC()}
	f.Must(f.Store.WriteMetadata(decision, kernel.Metadata(kernel.KeyAnswer, answer)))
	receipt := kernel.Receipt{Kind: "passed", Answer: &answer}
	if option == "halt" {
		receipt.Kind = "halted"
		receipt.HaltAnswer = decision
	}
	f.Must(f.Store.WriteReceipt(decision, receipt))
	closeRecord(f, decision)
}

// RegisteredPanel carries the enrolment witness beside the panel snapshot.
type RegisteredPanel struct {
	panel.Panel
	Orchestrated bool `json:"orchestrated"`
}

func Register(f *Fixture, gate string, registration kernel.RegistrationRecord) {
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, registration)))

	tuple, err := kernelid.Parse(gate)
	f.Must(err)
	var beadID *string
	gateKind := "final_review"
	if tuple.WorkItem != "" {
		item := tuple.WorkItem
		beadID = &item
		gateKind = "bead"
	}
	cfg, err := config.Load(f.Root)
	f.Must(err)
	round, hasRound, err := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
	f.Must(err)
	if !hasRound {
		// Registration-only fixtures can deliberately lack authorization. Keep
		// that absence while supplying the closed lens prerequisite as history.
		round = kernel.GateRound{Round: registration.Round, Subject: registration.Subject, SlotCount: len(registration.Slots)}
		id := fmt.Sprintf("%s.r%d.lens", tuple.Gate, round.Round)
		f.Create(id, gate)
		record := kernel.LensRecord{Step: id, Lenses: []kernel.SlotLens{}}
		selected, err := kernel.RoundReviewerSlots(cfg, gateKind, round)
		f.Must(err)
		for _, slot := range selected {
			if slot.Lens != "adversarial" {
				record.Lenses = append(record.Lenses, kernel.SlotLens{Slot: slot.Slot, Lens: "Review " + slot.Lens + "."})
			}
		}
		data, err := json.Marshal(record)
		f.Must(err)
		f.Must(f.Store.WriteMetadata(id, map[string]any{
			kernel.KeySubject: kernel.Subject{Commit: round.Subject}, kernel.KeyRound: kernel.LensAuthorization(round),
			kernel.KeyRecord: string(data), kernel.KeyClose: kernel.Receipt{Kind: "passed", Generation: 1, RecordSHA256: fmt.Sprintf("%x", sha256.Sum256(data))},
		}))
		closeRecord(f, id)
	} else if round.Lens == "" || f.State().Records[round.Lens].Status != "closed" {
		Authorise(f, gate, round) //nolint:misspell // A registration fixture starts after lens close.
		round, _, err = kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
		f.Must(err)
	}
	rosterRound := round
	rosterRound.SlotCount = len(registration.Slots)
	selected, err := kernel.RoundReviewerSlots(cfg, gateKind, rosterRound)
	f.Must(err)
	roster := []panel.RosterSlot{}
	for i, slot := range registration.Slots {
		r := selected[i]
		entry := panel.RosterSlot{Slot: slot, Family: r.Family, Model: r.Model, Lens: r.Lens}
		if r.Lens != "adversarial" {
			for _, lens := range round.Lenses {
				if lens.Slot == slot {
					entry.Focus = lens.Lens
				}
			}
		}
		roster = append(roster, entry)
	}
	target := "spec/" + f.SpecID
	if beadID != nil {
		target, err = workspace.BeadBranch(*beadID)
		f.Must(err)
	}
	digest := "subject-digest"
	if len(registration.Subject) == 40 || len(registration.Subject) == 64 {
		listing, err := f.Command("git", "ls-tree", "-r", registration.Subject)
		if err == nil && len(listing) > 0 {
			digest, err = panel.SubjectDigest(string(listing), panel.Subject(gateKind, f.SpecDir))
			f.Must(err)
		}
	}
	snapshot := panel.Panel{Orchestrated: true, KernelGate: gate, Scope: round.Paths, VerdictSchema: 2, BeadID: beadID, Spec: f.SpecID, Target: target, Round: registration.Round, ExpectedReviewers: len(roster), ReviewerCountSource: "explicit", ReviewedHeadSHA: registration.Subject, SubjectDigest: digest, Gate: gateKind, Roster: roster, Telemetry: "required"}
	event := panel.RoundRecord{Scope: snapshot.Scope, VerdictSchema: 2, Record: panel.RecordRound, Spec: f.SpecID, Panel: registration.Slug, Round: snapshot.Round, ReviewedHeadSHA: snapshot.ReviewedHeadSHA, SubjectDigest: snapshot.SubjectDigest, Target: snapshot.Target, Gate: snapshot.Gate, BeadID: beadID, Roster: roster, ExpectedReviewers: len(roster), ReviewerCountSource: "explicit", Telemetry: "required", Carried: panel.CarriedEntries{}, CreatedAt: kernel.Now().UTC().Format(time.RFC3339)}
	data, err := json.Marshal(event)
	f.Must(err)
	dir := filepath.Join(f.SpecDir, "reviews", registration.Slug)
	f.Must(panel.AppendRecords(dir, data))
	data, err = json.Marshal(snapshot)
	f.Must(err)
	write(f.t, filepath.Join(dir, panel.FileName), data)
}
func Repaired(f *Fixture, step string, receipt kernel.Receipt) {
	receipt.Repaired = true
	f.Must(f.Store.WriteReceipt(step, receipt))
}
func Judge(f *Fixture, gate, subject string, attempt int) string {
	tuple, err := kernelid.Parse(gate)
	f.Must(err)
	id := fmt.Sprintf("%s.judge.a%d", tuple.Gate, attempt)
	parent := tuple.WorkItem
	if parent == "" {
		parent = f.Epic
	}
	f.Create(id, parent)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeySubject, kernel.Subject{Commit: subject})))
	return id
}

// Verify seeds a closed verify attempt with its exact record, returns and read set.
func Verify(f *Fixture, gate string, record kernel.VerifyRecord, findings []kernel.Finding, read map[string]string) string {
	f.t.Helper()
	id := record.Step
	f.Create(id, gate)
	data, err := json.Marshal(record)
	f.Must(err)
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	at := kernel.Now().UTC()
	f.Must(f.Store.WriteMetadata(id, map[string]any{
		kernel.KeySubject: kernel.Subject{Commit: record.Subject}, kernel.KeyFindings: findings,
		kernel.KeyRead: read, kernel.KeyReturns: record.Returns, kernel.KeyRecord: string(data),
		kernel.KeyClosing: kernel.Closing{Generation: 1, RecordSHA256: digest, At: at},
		kernel.KeyClose:   kernel.Receipt{Kind: "passed", Generation: 1, RecordSHA256: digest, At: at.Format(time.RFC3339Nano)},
	}))
	closeRecord(f, id)
	round, has, err := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
	f.Must(err)
	if has {
		round.Verify = id
		f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
	}
	return id
}

// Lens seeds a closed lens attempt with its exact record bytes.
func Lens(f *Fixture, gate string, record kernel.LensRecord) string {
	f.t.Helper()
	f.Create(record.Step, gate)
	round, _, err := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
	f.Must(err)
	data, err := json.Marshal(record)
	f.Must(err)
	metadata := map[string]any{
		kernel.KeySubject: kernel.Subject{Commit: round.Subject}, kernel.KeyRound: kernel.LensAuthorization(round), kernel.KeyRecord: string(data),
		kernel.KeyClose: kernel.Receipt{Kind: "passed", Generation: 1, RecordSHA256: fmt.Sprintf("%x", sha256.Sum256(data))},
	}
	if scope, has, err := kernel.Decode[kernel.RePanel](f.State().Records[gate].Metadata, kernel.KeyRePanel); err != nil {
		f.Must(err)
	} else if has {
		metadata[kernel.KeyRePanel] = scope
	}
	f.Must(f.Store.WriteMetadata(record.Step, metadata))
	round.Lens, round.Lenses = record.Step, record.Lenses
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
	closeRecord(f, record.Step)
	return record.Step
}
