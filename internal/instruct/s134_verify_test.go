package instruct

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func TestS134Instruct_VerifiedReject(t *testing.T) {
	instructVerifiedReject(t, true)
}

func TestS134Instruct_RealStoreVerifiedReject(t *testing.T) {
	instructVerifiedReject(t, false)
}

func instructVerifiedReject(t *testing.T, fake bool) {
	t.Helper()
	var options []kerneltest.Option
	if fake {
		options = append(options, kerneltest.WithFake(true))
	}
	f := kerneltest.Enrolled(t, options...)
	if fake {
		f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
		f.Must(os.WriteFile(filepath.Join(f.Root, ".beads", "config.yaml"), []byte("types.custom: gate,step\n"), 0600))
	}
	gate := f.Items[0] + ".review"
	raw, err := f.Command("git", "rev-parse", "HEAD")
	f.Must(err)
	head := strings.TrimSpace(string(raw))
	_, err = f.Command("git", "branch", "bead/"+f.Items[0], head)
	f.Must(err)
	slots := []string{gate + ".r1.R1", gate + ".r1.R2"}
	kerneltest.Authorise(f, gate, kernel.GateRound{Round: 1, Subject: head, SlotCount: 2, Slots: slots})
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	dir := filepath.Join(f.SpecDir, "reviews", "review")
	for i, id := range slots {
		v := paneltest.Verdict(panel.VerdictReject)
		v.Slot, v.ReviewedSHA, v.Model = fmt.Sprintf("R%d", i+1), head, "test-model"
		data := paneltest.Bytes(t, v)
		_, err := panel.Submit(panel.SubmitTarget{Dir: dir, Spec: f.SpecID, Panel: "review", Slot: v.Slot, Round: 1}, data, func(panel.Panel, panel.RosterSlot) (panel.SubmitResolution, error) {
			return panel.SubmitResolution{Family: "harness", Model: "test-model", RunnerKind: "harness", RunnerSource: "default"}, nil
		})
		f.Must(err)
		f.Create(id, gate)
		kerneltest.SlotReceipt(f, id, data)
		f.Must(f.Store.SetStatusIf(id, "open", "closed"))
	}
	_, err = kernel.NextAction(f.State(), true)
	f.Must(err)
	id := gate + ".r1.verify"
	r := f.State().Records[id]
	findings, _, err := kernel.Decode[[]kernel.Finding](r.Metadata, kernel.KeyFindings)
	f.Must(err)
	v := kernel.VerifyRecord{Step: id, Subject: head, Returns: []kernel.VerifyReturn{}}
	for _, finding := range findings {
		v.Returns = append(v.Returns, kernel.VerifyReturn{Slot: finding.Slot, Round: finding.Round, Finding: finding.Finding, FindingSHA256: finding.FindingSHA256, Return: "refuted", Ground: "disproven", Reason: "checked", Evidence: "https://example.invalid/proof"})
	}
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	data, err := json.Marshal(v)
	f.Must(err)
	recordPath := filepath.Join(t.TempDir(), "verify.json")
	f.Must(os.WriteFile(recordPath, data, 0600))
	_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordPath})
	f.Must(err)
	receipt, _, err := kernel.Decode[kernel.Receipt](f.State().Records[gate].Metadata, kernel.KeyClose)
	f.Must(err)
	if receipt.Retally != "verify" {
		t.Fatalf("fixture did not close through verify: %+v", receipt)
	}
	if fake {
		old := instructKernelStateFn
		instructKernelStateFn = func(string, string) (*kernel.State, error) { s := f.State(); return &s, nil }
		t.Cleanup(func() { instructKernelStateFn = old })
	}
	text := renderPanelState(gatherPanelState(func(string) (string, bool) { return head, true }, f.Root, f.SpecDir))
	if !strings.Contains(text, "- **review** — gate would PASS\n") || !strings.Contains(text, "kernel gate passed; completion can proceed") || strings.Contains(text, "gate would BLOCK") || strings.Contains(strings.ToLower(text), "halt") {
		t.Fatalf("instruct retained the pre-kernel REJECT halt: %s", text)
	}
}

func TestS134Instruct_StoreFailureNotice(t *testing.T) {
	f := paneltest.New(t)
	old := instructKernelStateFn
	instructKernelStateFn = func(string, string) (*kernel.State, error) {
		return nil, errors.New("tracker read failed\nrecovery: forged")
	}
	t.Cleanup(func() { instructKernelStateFn = old })
	p := f.Panel
	p.Gate, p.Orchestrated = "final_review", true
	// A fresh registered panel keeps its ledger and snapshot consistent.
	dir := filepath.Join(t.TempDir(), "reviews", "review")
	p.Round = 1
	data, err := json.Marshal(panel.RoundRecord{Record: panel.RecordRound, Spec: p.Spec, Panel: "review", Round: 1, Gate: p.Gate, Target: p.Target, BeadID: nil, Roster: p.Roster, ExpectedReviewers: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Telemetry: p.Telemetry, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	if err := panel.AppendRecords(dir, data); err != nil {
		t.Fatal(err)
	}
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	text := renderPanelState(gatherPanelState(nil, filepath.Dir(filepath.Dir(dir))))
	if strings.Contains(text, "would PASS") || !strings.Contains(text, "- **review** — gate state is unknown\n") || !strings.Contains(text, "notice:") || !strings.Contains(text, "tracker read failed") || strings.Contains(text, "gate would BLOCK") || strings.Contains(strings.ToLower(text), "halt") || strings.Contains(text, "\nrecovery: forged") {
		t.Fatalf("store read failure gave a halt or hid its notice: %s", text)
	}
}

func TestS134Instruct_LegacyReadFailure(t *testing.T) {
	f := paneltest.New(t)
	p := f.Panel
	p.Gate, p.Orchestrated, p.Telemetry, p.SubjectDigest = "bead", false, "", ""
	bead := "test-1.1"
	p.BeadID = &bead
	p.Target = "bead/" + bead
	dir := filepath.Join(t.TempDir(), "reviews", "review")
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	old := instructKernelStateFn
	t.Cleanup(func() { instructKernelStateFn = old })
	resolve := func(string) (string, bool) { return strings.Repeat("b", 40), true }
	instructKernelStateFn = func(string, string) (*kernel.State, error) { return nil, nil }
	before := gatherPanelState(resolve, filepath.Dir(filepath.Dir(dir)))
	if len(before) != 1 {
		t.Fatalf("missing legacy panel: %+v", before)
	}
	verdict, reason := before[0].verdict()
	if verdict != GateBlock || !strings.Contains(reason, "commits landed after review") {
		t.Fatalf("stale legacy panel must block: %s", renderPanelState(before))
	}
	p.Orchestrated = false
	if err := panel.WriteSnapshot(dir, p); err != nil {
		t.Fatal(err)
	}
	instructKernelStateFn = func(string, string) (*kernel.State, error) {
		return nil, errors.New("tracker read failed\nrecovery: forged")
	}
	after := gatherPanelState(resolve, filepath.Dir(filepath.Dir(dir)))
	if len(after) != 1 {
		t.Fatalf("missing legacy panel after read failure: %+v", after)
	}
	got, why := after[0].verdict()
	text := renderPanelState(after)
	if got != verdict || why != reason || !strings.Contains(text, "notice: kernel panel state is unavailable:") || !strings.Contains(text, "tracker read failed") || strings.Contains(text, "\nrecovery: forged") {
		t.Fatalf("legacy verdict changed or notice lost: %s", text)
	}
	// A nil resolver uses the plain tally entry, which must keep the notice.
	plain := gatherPanelState(nil, filepath.Dir(filepath.Dir(dir)))
	if len(plain) != 1 || plain[0].Facts != nil || plain[0].KernelVerdict != nil {
		t.Fatalf("missing tally-only fallback: %+v", plain)
	}
	text = renderPanelState(plain)
	if !strings.Contains(text, "notice: kernel panel state is unavailable:") || !strings.Contains(text, "tracker read failed") || strings.Contains(text, "\nrecovery: forged") {
		t.Fatalf("tally-only fallback lost its notice: %s", text)
	}
	for _, orchestrated := range []bool{false, true} {
		p.Orchestrated = orchestrated
		if err := panel.WriteSnapshot(dir, p); err != nil {
			t.Fatal(err)
		}
		instructKernelStateFn = func(string, string) (*kernel.State, error) { return nil, nil }
		text := renderPanelState(gatherPanelState(resolve, filepath.Dir(filepath.Dir(dir))))
		if strings.Contains(text, "unavailable") || !strings.Contains(text, "gate would BLOCK\n") {
			t.Fatalf("unenrolled read changed legacy verdict: %s", text)
		}
	}
}

func TestS134Instruct_LegacyEmptySpec(t *testing.T) {
	for _, orchestrated := range []bool{false, true} {
		t.Run(fmt.Sprint(orchestrated), func(t *testing.T) {
			f := paneltest.New(t)
			p := f.Panel
			p.Gate, p.Spec, p.Orchestrated, p.Telemetry, p.SubjectDigest = "bead", "", orchestrated, "", ""
			bead := "test-1.1"
			p.BeadID, p.Target = &bead, "bead/"+bead
			dir := filepath.Join(t.TempDir(), "reviews", "review")
			if err := panel.WriteSnapshot(dir, p); err != nil {
				t.Fatal(err)
			}
			old := instructKernelStateFn
			t.Cleanup(func() { instructKernelStateFn = old })
			resolve := func(string) (string, bool) { return strings.Repeat("b", 40), true }
			instructKernelStateFn = func(string, string) (*kernel.State, error) { return nil, nil }
			before := gatherPanelState(resolve, filepath.Dir(filepath.Dir(dir)))
			if len(before) != 1 {
				t.Fatalf("missing legacy panel: %+v", before)
			}
			verdict, reason := before[0].verdict()
			if verdict != GateBlock || !strings.Contains(reason, "commits landed after review") {
				t.Fatalf("stale legacy panel must block: %s", renderPanelState(before))
			}
			instructKernelStateFn = func(string, string) (*kernel.State, error) {
				t.Error("empty spec must not read kernel state")
				return nil, errors.New("invalid spec ID: empty")
			}
			after := gatherPanelState(resolve, filepath.Dir(filepath.Dir(dir)))
			if len(after) != 1 {
				t.Fatalf("missing legacy panel after kernel stub: %+v", after)
			}
			got, why := after[0].verdict()
			text := renderPanelState(after)
			if got != verdict || why != reason || after[0].KernelVerdict != nil || after[0].Notice != "" || text != renderPanelState(before) || strings.Contains(text, "notice:") {
				t.Fatalf("empty spec changed the legacy verdict or added a notice: %s", text)
			}
		})
	}
}

// A bead panel of a pre-kernel spec keeps the panel route when the real
// kernel state reader runs against a real tracker that holds the spec's epic
// without enrolment: no kernel verdict, and the facts of the panel decide.
func TestS134Instruct_PreKernelBeadPanelRealReader(t *testing.T) {
	f := kerneltest.PreKernelSpec(t)
	raw, err := f.Command("git", "rev-parse", "HEAD")
	f.Must(err)
	head := strings.TrimSpace(string(raw))
	p := paneltest.New(t).Panel
	p.Gate, p.Spec, p.Orchestrated, p.Telemetry, p.SubjectDigest = "bead", f.SpecID, false, "", ""
	bead := f.Items[0]
	p.BeadID, p.Target = &bead, "bead/"+bead
	dir := filepath.Join(f.SpecDir, "reviews", "review")
	f.Must(panel.WriteSnapshot(dir, p))
	entries := gatherPanelState(func(string) (string, bool) { return head, true }, f.Root, f.SpecDir)
	if len(entries) != 1 {
		t.Fatalf("want one panel entry, got %+v", entries)
	}
	if entries[0].KernelVerdict != nil || entries[0].Facts == nil {
		t.Fatalf("a pre-kernel bead panel took the kernel route: %s", renderPanelState(entries))
	}
	state, err := instructKernelStateFn(f.Root, f.SpecID)
	f.Must(err)
	if state != nil {
		t.Fatalf("the kernel state reader selected a pre-kernel epic: %+v", state)
	}
}
