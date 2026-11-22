package kernel_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func jsonEqual(a, b any) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	var x, y any
	_ = json.Unmarshal(aa, &x)
	_ = json.Unmarshal(bb, &y)
	return reflect.DeepEqual(x, y)
}
func golden(t *testing.T, name string, value any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err = json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(actual, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s golden differs:\n%s\nwant %s", name, actual, data)
	}
}
func TestS134Records_RoundTrips(t *testing.T) {
	cases := map[string]any{
		"approval": kernel.PlanApproval(true),
		"tuple":    mustTuple(t, "test-abc.1.review.r2.R3.a2"),
		"key":      kernel.ComputedKey("test-abc.1.impl.a1"), "from": kernel.From("test-abc.1.review"),
		"links":        kernel.Links{{ID: "test-abc.1", Upstream: []string{}}, {ID: "test-abc.2", Upstream: []string{"test-abc.1"}}},
		"run":          kernel.DefaultPolicy,
		"lease":        kernel.LeaseRecord{Generation: 2, Expires: fixedTime, TokenHash: "sha256:token", ClaimBase: "base"},
		"round":        kernel.GateRound{Round: 2, Subject: "subject", Slots: []string{"slot"}, SlotCount: 2, Paths: []string{"app.go"}},
		"raw-claims":   []kernel.RawClaim{{At: fixedTime}},
		"take":         kernel.TakeReason{Reason: "operator", At: fixedTime, Generation: 2},
		"decision":     kernel.DecisionRecord{Options: []string{"fix", "halt"}, Cause: "reject", Gate: "test-abc.1.review"},
		"answer":       kernel.DecisionAnswer{Option: "fix", At: fixedTime},
		"defer":        kernel.DeferRecord{Finding: "finding", To: "follow-up", Reason: "bounded", At: fixedTime},
		"judgement":    kernel.Judgement{Value: "re-panel", OldSubject: "old", NewSubject: "new", Reason: "changed", SlotCount: 2, Paths: []string{"app.go"}},
		"repanel":      kernel.RePanel{Judge: "judge", SlotCount: 2, Paths: []string{"app.go"}},
		"carry":        kernel.CarryOver{Rebind: "accepted-rebind", OldSubject: "old", NewSubject: "new", Judge: "judge", Receipt: &kernel.Receipt{Kind: "passed", RecordSHA256: "digest", Generation: 1}},
		"close":        kernel.Receipt{Kind: "passed", Path: "proof", Commit: "commit", Blob: "blob", Repaired: true, Rebind: "rebind", CarryOvers: []kernel.CarryOver{{OldSubject: "old", NewSubject: "new", Judge: "judge"}}},
		"registration": kernel.RegistrationRecord{Slug: "panel", Round: 2, Subject: "subject", Slots: []string{"R1", "R2"}},
		"subject":      kernel.Subject{Commit: "subject"},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			golden(t, "record-"+name, value)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			copy := reflect.New(reflect.TypeOf(value))
			if err = json.Unmarshal(data, copy.Interface()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(value, copy.Elem().Interface()) {
				t.Fatal("record did not round trip")
			}
		})
	}
	keys := []string{kernel.KeyPlanApproval, kernel.KeyTuple, kernel.KeyID, kernel.KeyFrom, kernel.KeyLinks, kernel.KeyRun, kernel.KeyLease, kernel.KeyRound, kernel.KeyTake, kernel.KeyTakes, kernel.KeyDeferrals, kernel.KeyDecision, kernel.KeyAnswer, kernel.KeyDefer, kernel.KeyJudgement, kernel.KeyRePanel, kernel.KeyClose, kernel.KeySubject, kernel.KeyRegistration, kernel.KeyRawClaims, kernel.KeyCompletion, kernel.KeyRecord, kernel.KeyClosing, kernel.KeyFindings, kernel.KeyRead, kernel.KeyReturns, kernel.KeyReproduced}
	golden(t, "record-keys", keys)
}

func TestS134Records_AmendmentRoundTrips(t *testing.T) {
	finding := kernel.Finding{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: "digest", Text: "full finding"}
	ret := kernel.VerifyReturn{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: "digest", Return: "refuted", Ground: "out_of_acceptance_scope", AC: "AC-1", Reason: "outside scope", Evidence: "https://example.com/proof", Target: "test-target"}
	minimal := kernel.GateRound{Round: 1, Subject: "old", Slots: []string{"slot"}, SlotCount: 1}
	prior := minimal
	prior.Verify = "verify"
	prior.Read = map[string]string{"verdict.json": "digest"}
	prior.Asks = []kernel.PriorAsk{{Finding: finding, Returns: []kernel.VerifyReturn{ret}}}
	cases := map[string]any{
		"lens":     kernel.LensRecord{Step: "lens", Lenses: []kernel.SlotLens{{Slot: "R1", Lens: "inspect paths"}}},
		"verify":   kernel.VerifyRecord{Step: "verify", Subject: "subject", Returns: []kernel.VerifyReturn{ret}},
		"findings": []kernel.Finding{finding}, "returns": []kernel.VerifyReturn{ret},
		"reproduced":       []kernel.Finding{{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: "digest"}},
		"read":             map[string]string{"verdict.json": "digest"},
		"record":           " {\n\"step\": \"lens\", \"lenses\": [] }\n",
		"closing":          kernel.Closing{Generation: 1, RecordSHA256: "digest", At: fixedTime},
		"closing-conflict": kernel.Closing{Generation: 1, RecordSHA256: "digest", At: fixedTime, Conflict: &kernel.ClosingIdentity{Generation: 2, RecordSHA256: "different"}},
		"lease-ended":      kernel.LeaseRecord{Generation: 2, Expires: fixedTime, TokenHash: "hash", ClaimBase: "base", Ended: "superseded"},
		"take-via":         kernel.TakeReason{Reason: "take", At: fixedTime, Generation: 2, Via: "lease", RevokedTokenHash: "hash", Subject: "subject"},
		"decision-slots":   kernel.DecisionRecord{Options: []string{"redispatch", "halt"}, Cause: "uncounted", Gate: "gate", Slots: []kernel.DecisionSlot{{Slot: "R2", Attempt: 3}}},
		"prior-minimal":    minimal, "prior": prior,
		"round-expanded": kernel.GateRound{Round: 2, Subject: "subject", Slots: []string{"slot"}, SlotCount: 1, Lenses: []kernel.SlotLens{{Slot: "R1", Lens: "inspect paths"}}, Lens: "lens", Verify: "verify", Prior: &prior, Judged: &kernel.Judged{Judge: "judge", Value: "carry-over", NewSubject: "subject"}},
		"record-receipt": kernel.Receipt{Kind: "passed", RecordSHA256: "digest", Generation: 1, SupersededGeneration: 2, Via: "repair", At: fixedTime.Format("2006-01-02T15:04:05Z07:00"), Retally: "verify"},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			golden(t, "record-"+name, value)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			copy := reflect.New(reflect.TypeOf(value))
			if err = json.Unmarshal(data, copy.Interface()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(value, copy.Elem().Interface()) {
				t.Fatal("amended record did not round trip")
			}
		})
	}
}
func TestS134Records_PassReceipts(t *testing.T) {
	cases := map[string]kernel.Receipt{
		"slot": {Kind: "superseded", Subject: "new"}, "fix": {Kind: "superseded", Retally: "passing-tally"}, "judge": {Kind: "superseded", Rebind: "accepted-rebind"}, "halt": {Kind: "halted", HaltAnswer: "halt-decision"}, "gate": {Kind: "passed", Subject: "subject", Retally: "passing-tally"},
	}
	for name, receipt := range cases {
		t.Run(name, func(t *testing.T) {
			f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
			gate := f.Items[0] + ".review"
			id := gate
			switch name {
			case "slot":
				id = gate + ".r1.R1"
				f.Create(id, gate)
			case "fix":
				id = gate + ".r2.fix"
				f.Create(id, gate)
			case "judge":
				id = kerneltest.Judge(f, gate, "new", 1)
			case "halt":
				id = f.Items[0] + ".impl.a1"
				f.Create(id, f.Items[0])
			}
			reg := kernel.NewRegistry("close")
			register(t, reg, "close", kernel.Ordinary, func(kernel.State) (kernel.Changes, error) {
				return kernel.Changes{Writes: []kernel.Write{{Kind: "close", ID: id, Receipt: &receipt}}}, nil
			})
			if _, err := kernel.Pass(reg, f.State(), true); err != nil {
				t.Fatal(err)
			}
			record, err := f.Store.Lookup(id)
			if err != nil {
				t.Fatal(err)
			}
			actual, ok, err := kernel.Decode[kernel.Receipt](record.Metadata, kernel.KeyClose)
			if err != nil || !ok {
				t.Fatal("receipt missing")
			}
			golden(t, "receipt-"+name, actual)
			if err = kernel.ValidReceipt(f.Root, actual, record.Metadata); err != nil {
				t.Fatal(err)
			}
			if actual.Path != "" || actual.Commit != "" || actual.Blob != "" {
				t.Fatal("pass receipt has artifact fields")
			}
		})
	}
}
func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func TestS134Records_ArtifactReceipt(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	path := "proof.json"
	if err := os.WriteFile(filepath.Join(f.Root, path), []byte(`{"verdict":"APPROVE"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.Root, "other.json"), []byte(`{"verdict":"REJECT"}`), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, f.Root, "add", path, "other.json")
	git(t, f.Root, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "evidence")
	commit := git(t, f.Root, "rev-parse", "HEAD")
	blob := git(t, f.Root, "rev-parse", "HEAD:"+path)
	otherBlob := git(t, f.Root, "rev-parse", "HEAD:other.json")
	receipt := kernel.Receipt{Kind: "passed", Path: path, Commit: commit, Blob: blob, Generation: 1}
	if err := kernel.ValidReceipt(f.Root, receipt, nil); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*kernel.Receipt){"mutable-commit": func(r *kernel.Receipt) { r.Commit = "HEAD" },
		"missing-blob": func(r *kernel.Receipt) { r.Blob = "" }, "missing-commit": func(r *kernel.Receipt) { r.Commit = strings.Repeat("0", 40) }, "wrong-blob": func(r *kernel.Receipt) { r.Blob = otherBlob }} {
		t.Run(name, func(t *testing.T) {
			bad := receipt
			mutate(&bad)
			if kernel.ValidReceipt(f.Root, bad, nil) == nil {
				t.Fatal("bad artifact receipt accepted")
			}
		})
	}
	// The implement submit writer consumes this same named receipt shape.
	normalized := receipt
	normalized.Commit = "commit"
	normalized.Blob = "blob"
	golden(t, "receipt-implement", normalized)
	id := f.Items[0] + ".impl.a1"
	f.Create(id, f.Items[0])
	if err := f.Store.WriteReceipt(id, receipt); err != nil {
		t.Fatal(err)
	}
	r, err := f.Store.Lookup(id)
	if err != nil {
		t.Fatal(err)
	}
	stored, _, _ := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	if !reflect.DeepEqual(stored, receipt) {
		t.Fatal("artifact identity lost at store boundary")
	}
}

func TestS134Records_ReceiptNeedsEvidence(t *testing.T) {
	if err := kernel.ValidReceipt(t.TempDir(), kernel.Receipt{Kind: "passed"}, nil); err == nil {
		t.Fatal("receipt without artifact or reason accepted")
	}
}

func TestS134Contract_OutputGolden(t *testing.T) {
	out := kernel.Output{
		Schema: "mindspec.next-action/v2", Status: "work", Applied: true,
		Revision: "sha256:revision", RetryAt: "2026-01-02T03:04:05Z", Policy: kernel.PolicyView{RoundLimit: kernel.DefaultPolicy.RoundLimit, LeaseTTL: kernel.DefaultPolicy.LeaseTTL},
		Actions: []kernel.Action{{Step: "test-abc.1.review.r2.R3", Kind: "review-slot", Exists: true, Round: 2,
			Subject: kernel.Subject{Commit: "subject"}, Claim: []string{"mindspec", "lease", "step"}, Brief: []string{"brief"}, Submit: []string{"submit"}}},
		Waiting:   []kernel.Waiting{{Step: "test-abc.1.impl.a1", Lease: kernel.LeaseView{Generation: 1, Expires: fixedTime}}},
		Decisions: []kernel.Decision{{Step: "decision", Options: []string{"fix", "halt"}, Submit: []string{"submit"}}},
		Run:       []kernel.Run{{Actor: "binary", Argv: []string{"mindspec", "complete", "work"}, Paths: []string{"app.go"}}},
		Flags:     []kernel.Flag{{Code: "unreceipted-close", Step: "step", Repair: []string{"repair"}}},
	}
	golden(t, "output", out)
}
