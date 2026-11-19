package kerneltest

import (
	"encoding/json"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"path/filepath"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
)

func PassGate(f *Fixture, gate, subject string) {
	f.Must(f.Store.WriteReceipt(gate, kernel.Receipt{Kind: "passed", Subject: subject, Retally: "passing-tally"}))
	closeRecord(f, gate)
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
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRound, round)))
}
func OpenDecision(f *Fixture, gate, cause string, round int, options ...string) string {
	id := fmt.Sprintf("%s.decide.%s.r%d", gate, cause, round)
	tuple, err := kernelid.Parse(id)
	f.Must(err)
	_, err = f.Store.Create(kernel.CreateRecord{ID: id, Parent: gate, Type: "decision", Title: "decision: halt stops the spec and every work item", Tuple: tuple, Metadata: kernel.Metadata(kernel.KeyDecision, kernel.DecisionRecord{Gate: tuple.Gate, Cause: cause, Options: options})})
	f.Must(err)
	return id
}
func Answer(f *Fixture, decision, option string) {
	answer := kernel.DecisionAnswer{Option: option, At: kernel.Now().UTC(), Actor: "operator"}
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
	roster := []panel.RosterSlot{}
	for _, slot := range registration.Slots {
		roster = append(roster, panel.RosterSlot{Slot: slot, Model: "test-model", Lens: "test-lens"})
	}
	snapshot := panel.Panel{VerdictSchema: 2, BeadID: beadID, Spec: f.SpecID, Target: "spec/" + f.SpecID, Round: registration.Round, ExpectedReviewers: len(roster), ReviewerCountSource: "explicit", ReviewedHeadSHA: registration.Subject, SubjectDigest: "subject-digest", Gate: gateKind, Roster: roster, Telemetry: "required"}
	event := panel.RoundRecord{VerdictSchema: 2, Record: panel.RecordRound, Spec: f.SpecID, Panel: registration.Slug, Round: snapshot.Round, ReviewedHeadSHA: snapshot.ReviewedHeadSHA, SubjectDigest: snapshot.SubjectDigest, Target: snapshot.Target, Gate: snapshot.Gate, BeadID: beadID, Roster: roster, ExpectedReviewers: len(roster), ReviewerCountSource: "explicit", Telemetry: "required", Carried: panel.CarriedEntries{}, CreatedAt: kernel.Now().UTC().Format(time.RFC3339)}
	data, err := json.Marshal(event)
	f.Must(err)
	dir := filepath.Join(f.SpecDir, "reviews", registration.Slug)
	f.Must(panel.AppendRecords(dir, data))
	data, err = json.Marshal(RegisteredPanel{Panel: snapshot, Orchestrated: true})
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
