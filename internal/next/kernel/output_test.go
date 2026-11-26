package kernel_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

func TestS134Output_RevisionInputs(t *testing.T) {
	f := reducerFixture(t)
	reduce(t, f, true)
	id := f.Items[0] + ".impl.a1"
	f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyLease, kernel.LeaseRecord{Expires: fixedTime.Add(time.Hour), Generation: 1})))
	for _, input := range []string{"bd", "status", "metadata", "dependencies", "ref", "ledger", "verdict", "expiry", "display", "free-text"} {
		t.Run(input, func(t *testing.T) {
			state := f.State()
			base, err := kernel.NextAction(state, false)
			f.Must(err)
			switch input {
			case "bd":
				r := state.Records[id]
				r.Labels = append(r.Labels, "changed")
				state.Records[id] = r
			case "status":
				r := state.Records[id]
				r.Status = "in_progress"
				state.Records[id] = r
			case "metadata":
				r := state.Records[id]
				r.Metadata[kernel.KeyLease] = kernel.LeaseRecord{Expires: fixedTime.Add(time.Hour), Generation: 2}
				state.Records[id] = r
			case "dependencies":
				r := state.Records[id]
				// Both endpoints already belong to the digest; this edge adds no records.
				r.Dependencies = []kernel.Dependency{{ID: f.Items[0], Type: "related"}}
				state.Records[id] = r
			case "ref":
				state.Refs["refs/heads/bead/"+f.Items[0]] = "changed"
			case "ledger":
				state.Ledgers["directory"] = [][]byte{[]byte(`{"record":"round"}`)}
			case "verdict":
				state.Verdicts["path"] = json.RawMessage(`{"verdict":"APPROVE"}`)
			case "expiry":
				state.At = state.At.Add(2 * time.Hour)
			case "display":
				state.At = state.At.Add(time.Minute)
			case "free-text":
				r := state.Records[id]
				r.Title = "changed title"
				r.Description = "changed display text"
				state.Records[id] = r
			}
			out, err := kernel.NextAction(state, false)
			f.Must(err)
			if (base.Revision == out.Revision) != (input == "display" || input == "free-text") {
				t.Fatalf("revision ignored %s: %s %s", input, base.Revision, out.Revision)
			}
		})
	}
	// File inputs must reach the reader, not only a synthetic State field.
	registerRound(t, f, 1)
	a := reduce(t, f, false)
	path := filepath.Join(f.SpecDir, "reviews", "panel", "R1-round-1.json")
	put(t, path, `{"schema":2,"verdict":"APPROVE"}`)
	b := reduce(t, f, false)
	if a.Revision == b.Revision {
		t.Fatal("verdict file was not an input")
	}
}
func TestS134Output_StatusAndOrdering(t *testing.T) {
	f := reducerFixture(t)
	s := f.State()
	actual, err := kernel.NextAction(s, false)
	f.Must(err)
	if actual.Schema != "mindspec.next-action/v2" {
		t.Fatalf("next-action schema=%q", actual.Schema)
	}
	empty := kernel.PassResult{Output: kernel.Output{Schema: "mindspec.next-action/v2"}, State: s}
	out := kernel.OutputForPass(empty, s, false)
	if out.Status != "indeterminate" || kernel.OutputExitCode(out) != 1 {
		t.Fatalf("empty result=%+v", out)
	}
	for status, code := range map[string]int{"work": 0, "wait": 3, "halted": 4, "done": 5, "indeterminate": 1, "unknown": 1} {
		if got := kernel.OutputExitCode(kernel.Output{Status: status}); got != code {
			t.Errorf("%s exit %d", status, got)
		}
	}
	result := empty
	result.Output.Actions = []kernel.Action{{Step: f.Items[2] + ".impl.a1"}, {Step: f.Items[0] + ".review.judge.a1"}, {Step: f.Items[0] + ".review.r2.fix"}, {Step: f.Items[0] + ".review.r1.verify"}, {Step: f.Items[0] + ".review.r1.R10"}, {Step: f.Items[0] + ".review.r1.lens"}, {Step: f.Items[0] + ".review.r1.R2"}, {Step: f.Items[0] + ".impl.a1"}}
	out = kernel.OutputForPass(result, s, false)
	got := []string{}
	for _, a := range out.Actions {
		got = append(got, a.Step)
	}
	want := []string{f.Items[0] + ".impl.a1", f.Items[0] + ".review.r1.lens", f.Items[0] + ".review.r1.R2", f.Items[0] + ".review.r1.R10", f.Items[0] + ".review.r1.verify", f.Items[0] + ".review.r2.fix", f.Items[0] + ".review.judge.a1", f.Items[2] + ".impl.a1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan-kind-numeric-slot order: %v", got)
	}
}

func TestS134Output_AnswerRetrySchema(t *testing.T) {
	kerneltest.WithRules(t, kernel.NewRegistry())
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	id := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
	_, err := kernel.Answer(f.State(), id, "fix")
	f.Must(err)
	out, err := kernel.Answer(f.State(), id, "fix")
	f.Must(err)
	if out.Schema != "mindspec.next-action/v2" {
		t.Fatalf("answer retry schema=%q", out.Schema)
	}
}
func TestS134Output_Done(t *testing.T) {
	f := reducerFixture(t)
	for _, item := range f.Items {
		head := reducerGit(t, f, "rev-parse", "bead/"+item)
		kerneltest.PassGate(f, item+".review", head)
		f.Must(f.Store.WriteMetadata(item, map[string]any{kernel.KeyCompletion: head}))
		f.Must(f.Store.SetStatusIf(item, "open", "closed"))
	}
	f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
	t.Run("final-open", func(t *testing.T) {
		out := reduce(t, f, false)
		if out.Status == "done" || kernel.OutputExitCode(out) == 5 {
			t.Fatalf("open final gate reported done: %+v", out)
		}
	})
	f.Must(f.Store.SetStatusIf(f.Epic, "closed", "open"))
	kerneltest.PassGate(f, f.Epic+".final", reducerGit(t, f, "rev-parse", "spec/"+f.SpecID))
	t.Run("epic-open", func(t *testing.T) {
		out := reduce(t, f, false)
		if out.Status == "done" || kernel.OutputExitCode(out) == 5 {
			t.Fatalf("open epic reported done: %+v", out)
		}
	})
	f.Must(f.Store.SetStatusIf(f.Epic, "open", "closed"))
	out := reduce(t, f, false)
	if out.Status != "done" || kernel.OutputExitCode(out) != 5 {
		t.Fatalf("all done exit: %+v", out)
	}
}

func TestS134Output_EarliestExpiryAndSafeText(t *testing.T) {
	f := reducerFixture(t)
	s := f.State()
	earlier := fixedTime.Add(time.Hour)
	later := earlier.Add(time.Millisecond)
	result := kernel.PassResult{State: s, Output: kernel.Output{Waiting: []kernel.Waiting{
		{Step: f.Items[0] + ".impl.a1", Lease: kernel.LeaseView{Generation: 1, Expires: later}},
		{Step: f.Items[1] + ".impl.a1", Lease: kernel.LeaseView{Generation: 2, Expires: earlier}},
	}, Flags: []kernel.Flag{{Code: "foreign-child-open", Step: "bad\n\x1bchild", Repair: []string{"bd", "close", "bad\n\x1bchild"}}}}}
	out := kernel.OutputForPass(result, s, false)
	if out.RetryAt != earlier.Format(time.RFC3339Nano) {
		t.Fatalf("earliest expiry = %s", out.RetryAt)
	}
	for _, r := range out.Flags[0].Step {
		if r < 32 {
			t.Fatalf("unsafe terminal text: %q", out.Flags[0].Step)
		}
	}
}

func TestS134Output_RevisionAcrossRoots(t *testing.T) {
	a, b := reducerFixture(t), reducerFixture(t)
	if a.Root == b.Root {
		t.Fatal("fixtures must use different roots")
	}
	for _, f := range []*kerneltest.Fixture{a, b} {
		registerRound(t, f, 1)
		put(t, filepath.Join(f.SpecDir, "reviews", "panel", "R1-round-1.json"), `{"schema":2,"verdict":"APPROVE"}`)
		state := f.State()
		if len(state.Ledgers) != 1 || len(state.Verdicts) != 1 {
			t.Fatal("fixture must read a registered round and a verdict")
		}
	}
	left, right := reduce(t, a, false), reduce(t, b, false)
	if left.Revision != right.Revision {
		t.Fatalf("checkout root changed revision: %s != %s", left.Revision, right.Revision)
	}
}

// The same panel files read from the spec worktree, at any checkout path, give
// the revision that they give from the main checkout.
func TestS134Output_RevisionIgnoresSpecWorktreePath(t *testing.T) {
	f := reducerFixture(t)
	registerRound(t, f, 1)
	put(t, filepath.Join(f.SpecDir, "reviews", "panel", "R1-round-1.json"), `{"schema":2,"verdict":"APPROVE"}`)
	base := f.State()
	if len(base.Ledgers) != 1 || len(base.Verdicts) != 1 {
		t.Fatal("fixture must read a registered round and a verdict")
	}
	inMain, err := kernel.NextAction(base, false)
	f.Must(err)
	tree, err := workspace.SpecWorktreePath(base.Root, nil, base.SpecID)
	f.Must(err)
	moved := func(path string) string {
		rel, err := filepath.Rel(base.Root, path)
		f.Must(err)
		return filepath.Join(tree, rel)
	}
	state := f.State()
	state.Ledgers = map[string][][]byte{}
	for path, lines := range base.Ledgers {
		state.Ledgers[moved(path)] = lines
	}
	state.Verdicts = map[string]json.RawMessage{}
	for path, data := range base.Verdicts {
		state.Verdicts[moved(path)] = data
	}
	spec, err := kernel.NextAction(state, false)
	f.Must(err)
	if spec.Revision != inMain.Revision {
		t.Fatalf("spec worktree checkout path changed revision: %s != %s", spec.Revision, inMain.Revision)
	}
	// The same relative path in both checkouts must keep two keys. All four
	// combinations of two verdicts must give four revisions; one shared key can
	// encode only two, in any map order.
	values := []string{`{"schema":2,"verdict":"APPROVE"}`, `{"schema":2,"verdict":"REQUEST_CHANGES"}`}
	revisions := map[string]bool{}
	for _, pair := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		both := f.State()
		both.Verdicts = map[string]json.RawMessage{}
		for path := range base.Verdicts {
			both.Verdicts[path] = json.RawMessage(values[pair[0]])
			both.Verdicts[moved(path)] = json.RawMessage(values[pair[1]])
		}
		out, err := kernel.NextAction(both, false)
		f.Must(err)
		revisions[out.Revision] = true
	}
	if len(revisions) != 4 {
		t.Fatal("verdicts at one relative path in two checkouts share a revision key")
	}
}

func TestS134Output_RevisionSameBasenameEvidence(t *testing.T) {
	f := reducerFixture(t)
	dirs := []string{
		filepath.Join(f.Root, "review", "panel"),
		filepath.Join(f.Root, ".mindspec", "reviews", "panel"),
	}
	for _, dir := range dirs {
		registerRound(t, f, 1)
		f.Must(os.MkdirAll(filepath.Dir(dir), 0755))
		f.Must(os.Rename(filepath.Join(f.SpecDir, "reviews", "panel"), dir))
		put(t, filepath.Join(dir, "R1-round-1.json"), `{"schema":2,"verdict":"APPROVE"}`)
	}
	ledger, err := os.ReadFile(filepath.Join(dirs[0], "dispositions.jsonl"))
	f.Must(err)
	ledgers := []string{string(ledger), strings.Replace(string(ledger), fixedTime.Format(time.RFC3339), fixedTime.Add(time.Second).Format(time.RFC3339), 1)}
	if ledgers[0] == ledgers[1] {
		t.Fatal("ledger variants must have different created_at values")
	}
	verdicts := []string{`{"schema":2,"verdict":"APPROVE"}`, `{"schema":2,"verdict":"REQUEST_CHANGES"}`}
	for _, input := range []string{"ledger", "verdict"} {
		t.Run(input, func(t *testing.T) {
			seen := map[string][2]int{}
			// All four combinations must have distinct revisions. A collapsed key can
			// encode only two values, so this catches collisions in any map order.
			for _, variant := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				for i, dir := range dirs {
					ledgerIndex, verdictIndex := 0, 0
					if input == "ledger" {
						ledgerIndex = variant[i]
					} else {
						verdictIndex = variant[i]
					}
					put(t, filepath.Join(dir, "dispositions.jsonl"), ledgers[ledgerIndex])
					put(t, filepath.Join(dir, "R1-round-1.json"), verdicts[verdictIndex])
				}
				state := f.State()
				if len(state.Ledgers) != 2 || len(state.Verdicts) != 2 {
					t.Fatalf("reader must retain both ledgers and verdicts: %d ledgers, %d verdicts", len(state.Ledgers), len(state.Verdicts))
				}
				for _, dir := range dirs {
					if _, ok := state.Ledgers[dir]; !ok {
						t.Fatalf("reader missed ledger in %s", dir)
					}
					if _, ok := state.Verdicts[filepath.Join(dir, "R1-round-1.json")]; !ok {
						t.Fatalf("reader missed verdict in %s", dir)
					}
				}
				out := kernel.OutputForPass(kernel.PassResult{State: state}, state, false)
				if previous, exists := seen[out.Revision]; exists {
					t.Fatalf("revision lost same-basename %s evidence: variants %v and %v collide", input, previous, variant)
				}
				seen[out.Revision] = variant
			}
		})
	}
}
