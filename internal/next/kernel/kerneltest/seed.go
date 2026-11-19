package kerneltest

import (
	"encoding/json"
	"fmt"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"path/filepath"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
	f.Create(id, gate)
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyDecision, kernel.DecisionRecord{Gate: gate, Cause: cause, Options: options})))
	return id
}
func Answer(f *Fixture, decision, option string) {
	answer := kernel.DecisionAnswer{Option: option, At: kernel.Now().UTC(), Actor: "operator"}
	f.Must(f.Store.WriteMetadata(decision, kernel.Metadata(kernel.KeyAnswer, answer)))
	f.Must(f.Store.WriteReceipt(decision, kernel.Receipt{Kind: "passed", Answer: &answer}))
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
	selected, err := cfg.PanelGateReviewerSlots(gateKind, len(registration.Slots))
	f.Must(err)
	roster := []panel.RosterSlot{}
	for i, slot := range registration.Slots {
		r := selected[i]
		roster = append(roster, panel.RosterSlot{Slot: slot, Family: r.Family, Model: r.Model, Lens: r.Lens})
	}
	target := "spec/" + f.SpecID
	if beadID != nil {
		target, err = workspace.BeadBranch(*beadID)
		f.Must(err)
	}
	digest := "subject-digest"
	if len(registration.Subject) == 40 || len(registration.Subject) == 64 {
		listing, err := f.Command("git", "ls-tree", "-r", registration.Subject)
		f.Must(err)
		if len(listing) > 0 {
			digest, err = panel.SubjectDigest(string(listing), panel.Subject(gateKind, f.SpecDir))
			f.Must(err)
		}
	}
	round, _, err := kernel.Decode[kernel.GateRound](f.State().Records[gate].Metadata, kernel.KeyRound)
	f.Must(err)
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
