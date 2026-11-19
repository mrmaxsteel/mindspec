package kernel_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func register(t *testing.T, r *kernel.Registry, name string, mode kernel.Mode, fn kernel.Rule) {
	t.Helper()
	if err := r.Register(name, mode, fn); err != nil {
		t.Fatal(err)
	}
}
func TestS134Dispatch_EmptyAndHalted(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	reg := kernel.NewRegistry()
	for _, halted := range []bool{false, true} {
		if halted {
			d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
			kerneltest.Answer(f, d, "halt")
		}
		for _, apply := range []bool{false, true} {
			before := kerneltest.Snapshot(t, f)
			result, err := kernel.Pass(reg, f.State(), apply)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Writes) != 0 || result.State.Halted() != halted || (result.Output.Status == "halted") != halted {
				t.Fatalf("empty pass %+v", result)
			}
			if kerneltest.Snapshot(t, f) != before {
				t.Fatal("empty registry wrote")
			}
		}
	}
}
func TestS134Dispatch_OrderVisibilityAndParity(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprint(native), func(t *testing.T) {
			dry := kerneltest.Enrolled(t, kerneltest.WithFake(native))
			live := kerneltest.Enrolled(t, kerneltest.WithFake(native))
			run := func(f *kerneltest.Fixture, apply bool) kernel.PassResult {
				reg := kernel.NewRegistry("before", "create", "after-create", "close", "after-close")
				id := f.Items[0] + ".impl.a1"
				gate := f.Items[0] + ".review"
				order := []string{}
				check := func(name string, exists, closed bool) kernel.Rule {
					return func(s kernel.State) (kernel.Changes, error) {
						order = append(order, name)
						r, ok := s.Records[id]
						if ok != exists {
							return kernel.Changes{}, fmt.Errorf("%s exists=%t", name, ok)
						}
						if exists && (r.Status == "closed") != closed {
							return kernel.Changes{}, fmt.Errorf("%s stale status", name)
						}
						if closed {
							receipt, ok, e := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
							if e != nil || !ok || receipt.Kind != "superseded" {
								return kernel.Changes{}, fmt.Errorf("receipt missing in overlay")
							}
							round, ok, e := kernel.Decode[kernel.GateRound](s.Records[gate].Metadata, kernel.KeyRound)
							if e != nil || !ok || round.Round != 2 || round.Subject != "new" {
								return kernel.Changes{}, fmt.Errorf("gate metadata missing in overlay")
							}
						}
						return kernel.Changes{}, nil
					}
				}
				// Registration order deliberately reverses the registry's fixed order.
				register(t, reg, "after-close", kernel.Ordinary, check("after-close", true, true))
				register(t, reg, "close", kernel.Ordinary, func(s kernel.State) (kernel.Changes, error) {
					order = append(order, "close")
					return kernel.Changes{Writes: []kernel.Write{{Kind: "close", ID: id, Receipt: &kernel.Receipt{Kind: "superseded", Subject: "new"}}, {Kind: "metadata", ID: gate, Metadata: kernel.Metadata(kernel.KeyRound, kernel.GateRound{Round: 2, Subject: "new", SlotCount: 2, Slots: []string{}})}}}, nil
				})
				register(t, reg, "after-create", kernel.Ordinary, check("after-create", true, false))
				register(t, reg, "create", kernel.Ordinary, func(s kernel.State) (kernel.Changes, error) {
					order = append(order, "create")
					return kernel.Changes{Writes: []kernel.Write{{Kind: "create", ID: id, Create: &kernel.CreateRecord{ID: id, Type: "step", Parent: f.Items[0], Title: "implement", Tuple: mustTuple(t, id)}}}}, nil
				})
				register(t, reg, "before", kernel.Ordinary, check("before", false, false))
				original := f.State()
				before := kerneltest.Snapshot(t, f)
				f.Fake.Calls = nil
				result, err := kernel.Pass(reg, original, apply)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(order, []string{"before", "create", "after-create", "close", "after-close"}) {
					t.Fatalf("wrong order: %v", order)
				}
				if _, exists := original.Records[id]; exists {
					t.Fatal("input state mutated")
				}
				if !apply {
					if kerneltest.Snapshot(t, f) != before {
						t.Fatal("dry pass wrote")
					}
				} else {
					actual := f.State()
					if actual.Records[id].Status != "closed" {
						t.Fatal("close missing")
					}
					for _, key := range []string{id, gate} {
						want := result.State.Records[key]
						got := actual.Records[key]
						want.ID = got.ID
						want.Parent = got.Parent
						if !jsonEqual(want, got) {
							t.Fatalf("snapshot diff disagrees with projected writes:\nwant %+v\ngot %+v", want, got)
						}
					}
					receiptAt, closeAt := -1, -1
					for i, args := range f.Fake.Calls {
						if args[0] != "update" {
							continue
						}
						joined := strings.Join(args, " ")
						if strings.Contains(joined, "mindspec_close") {
							receiptAt = i
						}
						if strings.Contains(joined, "--status closed") {
							closeAt = i
						}
					}
					if receiptAt < 0 || closeAt <= receiptAt {
						t.Fatalf("close before receipt: %v", f.Fake.Calls)
					}
				}
				return result
			}
			a := run(dry, false)
			b := run(live, true)
			if !jsonEqual(a.Writes, b.Writes) {
				t.Fatal("read-only write list differs from apply")
			}
		})
	}
}
func TestS134Dispatch_Errors(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	want := errors.New("rule failed")
	reg := kernel.NewRegistry("test")
	register(t, reg, "test", kernel.Ordinary, func(kernel.State) (kernel.Changes, error) { return kernel.Changes{}, want })
	if _, err := kernel.Pass(reg, f.State(), true); !errors.Is(err, want) {
		t.Fatalf("rule error lost: %v", err)
	}
	reg = kernel.NewRegistry("test")
	id := f.Items[0] + ".impl.a1"
	register(t, reg, "test", kernel.Ordinary, func(kernel.State) (kernel.Changes, error) {
		return kernel.Changes{Writes: []kernel.Write{{Kind: "create", ID: id, Create: &kernel.CreateRecord{ID: id, Parent: f.Items[0], Type: "step", Tuple: mustTuple(t, id)}}}}, nil
	})
	want = errors.New("bd write failed")
	f.Fake.Hook = func(args []string) error {
		if args[0] == "create" {
			return want
		}
		return nil
	}
	if _, err := kernel.Pass(reg, f.State(), true); !errors.Is(err, want) {
		t.Fatalf("bd error lost: %v", err)
	}
}
func TestS134Dispatch_HaltModes(t *testing.T) {
	for _, halted := range []bool{false, true} {
		for _, apply := range []bool{false, true} {
			t.Run(fmt.Sprintf("halted=%t/apply=%t", halted, apply), func(t *testing.T) {
				f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
				id := f.Items[0] + ".impl.a1"
				f.Create(id, f.Items[0])
				if halted {
					d := kerneltest.OpenDecision(f, f.Items[0]+".review", "reject", 1, "fix", "halt")
					kerneltest.Answer(f, d, "halt")
				}
				reg := kernel.NewRegistry("ordinary", "halt", "flag")
				calls := []string{}
				register(t, reg, "ordinary", kernel.Ordinary, func(kernel.State) (kernel.Changes, error) {
					calls = append(calls, "ordinary")
					created := f.Items[1] + ".impl.a1"
					return kernel.Changes{Writes: []kernel.Write{{Kind: "create", ID: created, Create: &kernel.CreateRecord{ID: created, Type: "step", Parent: f.Items[1], Tuple: mustTuple(t, created)}}}}, nil
				})
				register(t, reg, "halt", kernel.Halt, func(s kernel.State) (kernel.Changes, error) {
					calls = append(calls, "halt")
					if !s.Halted() {
						return kernel.Changes{}, nil
					}
					return kernel.Changes{Writes: []kernel.Write{{Kind: "close", ID: id, Receipt: &kernel.Receipt{Kind: "halted", HaltAnswer: "answer"}}}}, nil
				})
				register(t, reg, "flag", kernel.FlagOnly, func(kernel.State) (kernel.Changes, error) {
					calls = append(calls, "flag")
					return kernel.Changes{Flags: []kernel.Flag{{Code: "test-flag", Repair: []string{"inspect"}}}}, nil
				})
				before := kerneltest.Snapshot(t, f)
				result, err := kernel.Pass(reg, f.State(), apply)
				if err != nil {
					t.Fatal(err)
				}
				expected := []string{"ordinary", "halt", "flag"}
				if halted {
					expected = []string{"halt", "flag"}
				}
				if !reflect.DeepEqual(calls, expected) || len(result.Output.Flags) != 1 {
					t.Fatalf("modes: %v %+v", calls, result.Output)
				}
				if halted {
					if _, ok := result.State.Records[f.Items[1]+".impl.a1"]; ok {
						t.Fatal("ordinary create on halt")
					}
					if result.Output.Status != "halted" || result.State.Records[id].Status != "closed" {
						t.Fatal("halt close missing")
					}
				}
				if !apply && kerneltest.Snapshot(t, f) != before {
					t.Fatal("read-only halt wrote")
				}
				if apply && halted {
					r, e := f.Store.Lookup(id)
					if e != nil || r.Status != "closed" {
						t.Fatal("halt not written")
					}
				}
			})
		}
	}
}
func TestS134Dispatch_FlagWriteRefusesBeforeWrites(t *testing.T) {
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	reg := kernel.NewRegistry("create", "bad-flag")
	id := f.Items[0] + ".impl.a1"
	create := kernel.Write{Kind: "create", ID: id, Create: &kernel.CreateRecord{ID: id, Parent: f.Items[0], Type: "step", Tuple: mustTuple(t, id)}}
	register(t, reg, "create", kernel.Ordinary, func(kernel.State) (kernel.Changes, error) { return kernel.Changes{Writes: []kernel.Write{create}}, nil })
	register(t, reg, "bad-flag", kernel.FlagOnly, func(kernel.State) (kernel.Changes, error) { return kernel.Changes{Writes: []kernel.Write{create}}, nil })
	before := kerneltest.Snapshot(t, f)
	for _, apply := range []bool{false, true} {
		if _, err := kernel.Pass(reg, f.State(), apply); err == nil {
			t.Fatal("flag write accepted")
		}
		if kerneltest.Snapshot(t, f) != before {
			t.Fatal("invalid flag pass wrote")
		}
	}
}

func TestS134Dispatch_CASCloseRetry(t *testing.T) {
	for _, mismatches := range []int{1, 2} {
		t.Run(fmt.Sprint(mismatches), func(t *testing.T) {
			f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
			id := f.Items[0] + ".impl.a1"
			f.Create(id, f.Items[0])
			reg := kernel.NewRegistry("close")
			calls := 0
			register(t, reg, "close", kernel.Ordinary, func(s kernel.State) (kernel.Changes, error) {
				calls++
				if s.Records[id].Status == "closed" {
					return kernel.Changes{}, nil
				}
				return kernel.Changes{Writes: []kernel.Write{{Kind: "close", ID: id, Receipt: &kernel.Receipt{Kind: "superseded", Subject: "new"}}}}, nil
			})
			writes := 0
			f.Fake.Hook = func(args []string) error {
				for _, arg := range args {
					if arg == "--if-status" {
						writes++
						if writes <= mismatches {
							return kerneltest.ExitError(13)
						}
					}
				}
				return nil
			}
			_, err := kernel.Pass(reg, f.State(), true)
			if (err == nil) != (mismatches == 1) || calls != 2 || writes != 2 {
				t.Fatalf("retry did not rerun pass: calls=%d writes=%d error=%v", calls, writes, err)
			}
			r, e := f.Store.Lookup(id)
			if e != nil {
				t.Fatal(e)
			}
			if (r.Status == "closed") != (mismatches == 1) {
				t.Fatal("wrong durable status after retry")
			}
			receipt, ok, e := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
			if e != nil || !ok || receipt.Kind != "superseded" {
				t.Fatal("receipt lost during retry")
			}
		})
	}
}
