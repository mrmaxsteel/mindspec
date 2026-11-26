package kernel_test

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS134Verify_Schema1SlotRedaction(t *testing.T) {
	f := reducerFixture(t)
	item := f.Items[0]
	gate := item + ".review"
	head := gateHead(t, f, gate)
	dir := filepath.Join(f.SpecDir, "reviews", "legacy")
	kerneltest.Authorise(f, gate, gateRound(t, f, gate, 1, head))
	f.Must(f.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, kernel.RegistrationRecord{Slug: "legacy", Round: 1, Subject: head, Slots: []string{"R1", "R2", "R3"}})))
	listing := reducerGit(t, f, "ls-tree", "-r", head)
	digest := "subject-digest"
	if listing != "" {
		var err error
		digest, err = panel.SubjectDigest(listing, panel.Subject("bead", f.SpecDir))
		f.Must(err)
	}
	p := panel.Panel{Orchestrated: true, KernelGate: gate, VerdictSchema: 1, Spec: f.SpecID, BeadID: &item, Gate: "bead", Target: "bead/" + item, Round: 1, ExpectedReviewers: 3, ReviewedHeadSHA: head, SubjectDigest: digest, Telemetry: "required"}
	for i := 1; i <= 3; i++ {
		p.Roster = append(p.Roster, panel.RosterSlot{Slot: fmt.Sprintf("R%d", i), Family: "harness", Model: "test-model", Lens: "correctness"})
	}
	// Seed the round through the same ledger and snapshot APIs as paneltest.
	paneltest.Fixture{Dir: dir, Panel: p}.Round(t, 1)
	f.Must(panel.WriteSnapshot(dir, p))
	raw := "check /Users/example/private/source.go and /tmp/private/proof.txt"
	want := "check <path> and <path>"
	for i := 1; i <= 3; i++ {
		slot := fmt.Sprintf("R%d", i)
		payload := panel.FindingVerdict{Verdict: panel.VerdictApprove}
		if i == 1 {
			payload.Verdict, payload.Rationale = panel.VerdictRequestChanges, raw
		}
		data, err := json.Marshal(payload)
		f.Must(err)
		put(t, filepath.Join(dir, slot+"-round-1.json"), string(data))
		f.Create(gate+".r1."+slot, gate)
		closeSlotPass(t, f, gate+".r1."+slot, data)
	}
	res, err := panel.Tally(dir)
	f.Must(err)
	f.Must(res.Intact())
	if res.VerdictSchema != 1 || !res.Complete() || res.PassRule(true) {
		t.Fatalf("fixture must be a complete refused schema-1 round: %+v", res)
	}
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 0)
	findings, has, err := kernel.Decode[[]kernel.Finding](record(t, f, id).Metadata, kernel.KeyFindings)
	f.Must(err)
	if !has || len(findings) != 1 || findings[0].Finding != "slot" || findings[0].Slot != "R1" || findings[0].Text != want || strings.Contains(findings[0].Text, "/Users/") || strings.Contains(findings[0].Text, "/tmp/") {
		t.Fatalf("schema-1 slot text is not redacted: %+v", findings)
	}
	if findings[0].FindingSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) {
		t.Fatal("schema-1 slot digest must pin the raw rationale")
	}
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, v, lease, nil)
	f.Must(err)
	res, err = panel.Tally(dir)
	f.Must(err)
	if len(res.Panel.Refutations) != 1 || res.Panel.Refutations[0].Finding != "slot" || res.Panel.Refutations[0].FindingText != want {
		t.Fatalf("schema-1 close lost redacted finding text: %+v", res.Panel.Refutations)
	}
	lines, err := panel.ReadLedger(dir)
	f.Must(err)
	rows := 0
	for _, line := range lines {
		var row panel.DispositionRow
		f.Must(json.Unmarshal(line, &row))
		if row.Record == panel.RecordDisposition {
			rows++
			if row.Summary != termsafe.Escape(want) {
				t.Fatalf("schema-1 audit summary is not redacted: %+v", row)
			}
		}
	}
	if rows != 1 {
		t.Fatalf("want one schema-1 audit row, got %d", rows)
	}
}

func TestS134Verify_AbsoluteEvidenceStoredRelative(t *testing.T) {
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 0)
	abs := filepath.Join(f.SpecDir, "spec.md")
	relative, err := filepath.Rel(f.Root, abs)
	f.Must(err)
	want := filepath.ToSlash(relative)
	for i := range v.Returns {
		v.Returns[i].Evidence = abs
	}
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, v, lease, nil)
	f.Must(err)
	dir := filepath.Join(f.SpecDir, "reviews", "review")
	res, err := panel.Tally(dir)
	f.Must(err)
	if len(res.Panel.Refutations) != len(v.Returns) {
		t.Fatalf("missing refutations: %+v", res.Panel.Refutations)
	}
	for _, ref := range res.Panel.Refutations {
		if ref.Evidence != want || filepath.IsAbs(ref.Evidence) {
			t.Fatalf("refutation evidence is not repository-relative: got %q want %q", ref.Evidence, want)
		}
	}
	lines, err := panel.ReadLedger(dir)
	f.Must(err)
	rows := 0
	for _, line := range lines {
		var row panel.DispositionRow
		f.Must(json.Unmarshal(line, &row))
		if row.Record == panel.RecordDisposition {
			rows++
			if row.EvidenceRef != want || filepath.IsAbs(row.EvidenceRef) {
				t.Fatalf("audit evidence is not repository-relative: got %q want %q", row.EvidenceRef, want)
			}
		}
	}
	if rows != len(v.Returns) {
		t.Fatalf("missing audit rows: got %d want %d", rows, len(v.Returns))
	}
}

func TestS134Verify_DeferralAttemptHistory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("follow-up fixture uses a POSIX shell script")
	}
	f := reducerFixture(t)
	gate := f.Items[0] + ".review"
	refusedPanel(t, f, gate, "review", 1, panel.VerdictRequestChanges)
	reduce(t, f, true)
	id, v := verifyRecord(t, f, gate, 2)
	ret := &v.Returns[2]
	ret.Return, ret.Target, ret.Reason = "deferred", "follow-up", "track the follow-up"
	findings, _, err := kernel.Decode[[]kernel.Finding](record(t, f, id).Metadata, kernel.KeyFindings)
	f.Must(err)
	fake := t.TempDir()
	data, err := json.Marshal([]any{map[string]any{"id": "follow-up", "status": "open", "description": findings[2].Text, "dependencies": []map[string]string{{"id": f.Epic, "dependency_type": "discovered-from"}}}})
	f.Must(err)
	path := filepath.Join(fake, "follow-up.json")
	f.Must(os.WriteFile(path, data, 0600))
	t.Setenv("S134_VERIFY_FOLLOW_UP", path)
	f.Must(os.WriteFile(filepath.Join(fake, "bd"), []byte("#!/bin/sh\nif [ \"$1\" != show ] || [ \"$2\" != follow-up ]; then exit 1; fi\ncat \"$S134_VERIFY_FOLLOW_UP\"\n"), 0700))
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	killed := errors.New("stopped after deferral close writes")
	_, err = submitVerify(t, f, v, lease, func(kernel.State, kernel.Receipt) error { return killed })
	if !errors.Is(err, killed) {
		t.Fatalf("crash seam did not run: %v", err)
	}
	before, has, err := kernel.Decode[[]kernel.DeferRecord](record(t, f, gate).Metadata, kernel.KeyDeferrals)
	f.Must(err)
	if !has || len(before) != 1 || before[0].VerifyStep != id {
		t.Fatalf("crashed attempt lost its deferral: %+v", before)
	}
	kerneltest.SetClock(t, fixedTime.Add(time.Second))
	lease, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: id, Take: true, Reason: "retry after crash"}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, v, lease, nil)
	assertSubmitRefusal(t, err, "mindspec_closing differs from this submission", "mindspec next-action")
	reduce(t, f, true)
	receipt, _, err := kernel.Decode[kernel.Receipt](record(t, f, id).Metadata, kernel.KeyClose)
	f.Must(err)
	if record(t, f, id).Status != "closed" || receipt.Kind != "superseded" {
		t.Fatalf("crashed attempt did not retire: %+v", receipt)
	}
	next, again := verifyRecord(t, f, gate, 2)
	if next != id+".a2" {
		t.Fatalf("want second attempt, got %s", next)
	}
	again.Returns[2] = v.Returns[2]
	lease, err = kernel.Lease(f.State(), kernel.LeaseOptions{Step: next}, kernel.LeaseDeps{})
	f.Must(err)
	_, err = submitVerify(t, f, again, lease, nil)
	f.Must(err)
	history, has, err := kernel.Decode[[]kernel.DeferRecord](record(t, f, gate).Metadata, kernel.KeyDeferrals)
	f.Must(err)
	if !has || len(history) != 2 || !reflect.DeepEqual(history[:1], before) || history[1].VerifyStep != next {
		t.Fatalf("deferral history must keep both verify attempts: before=%+v after=%+v", before, history)
	}
	for _, d := range history {
		if d.Panel != "review" || d.Round != 1 || d.Slot != ret.Slot || d.Finding != ret.Finding || d.FindingSHA256 != ret.FindingSHA256 || d.To != ret.Target {
			t.Fatalf("deferral identity changed between attempts: %+v", d)
		}
	}
}
