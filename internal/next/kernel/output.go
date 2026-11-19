package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// OutputForPass finalizes a pass without writing state or evidence.
func OutputForPass(result PassResult, input State, apply bool) Output {
	out := result.Output
	out.Applied = apply
	for i := range out.Actions {
		_, out.Actions[i].Exists = input.Records[out.Actions[i].Step]
		if apply {
			out.Actions[i].Exists = true
		}
	}
	out.RetryAt = ""
	var earliest time.Time
	for _, w := range out.Waiting {
		if earliest.IsZero() || w.Lease.Expires.Before(earliest) {
			earliest = w.Lease.Expires
		}
	}
	if !earliest.IsZero() {
		out.RetryAt = earliest.UTC().Format(time.RFC3339Nano)
	}
	order := func(id string) [5]int {
		t, _ := kernelid.Parse(id)
		p := len(input.Items)
		for i, item := range input.Items {
			if t.WorkItem == item || id == item {
				p = i
				break
			}
		}
		kinds := map[string]int{"implement": 0, "review-slot": 1, "fix": 2, "change-judge": 3, "decision": 4}
		return [5]int{p, kinds[t.Kind], t.Round, t.Slot, t.Attempt}
	}
	less := func(a, b string) bool {
		x, y := order(a), order(b)
		for i := range x {
			if x[i] != y[i] {
				return x[i] < y[i]
			}
		}
		return a < b
	}
	sort.Slice(out.Actions, func(i, j int) bool { return less(out.Actions[i].Step, out.Actions[j].Step) })
	sort.Slice(out.Waiting, func(i, j int) bool { return less(out.Waiting[i].Step, out.Waiting[j].Step) })
	sort.Slice(out.Decisions, func(i, j int) bool { return less(out.Decisions[i].Step, out.Decisions[j].Step) })
	sort.SliceStable(out.Run, func(i, j int) bool { a, b := out.Run[i].Argv, out.Run[j].Argv; return less(a[len(a)-1], b[len(b)-1]) })
	sort.Slice(out.Flags, func(i, j int) bool {
		a, b := out.Flags[i], out.Flags[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Step < b.Step
	})
	out.Status = "indeterminate"
	if len(out.Waiting)+len(out.Flags) > 0 {
		out.Status = "wait"
	}
	if len(out.Actions)+len(out.Decisions)+len(out.Run) > 0 {
		out.Status = "work"
	}
	effective := result.State
	if effective.Records == nil {
		effective = input
	}
	done := effective.Records[effective.Epic].Status == "closed"
	for _, item := range input.Items {
		done = done && itemDone(effective, item)
	}
	for _, gate := range effective.Gates {
		done = done && passed(effective, effective.Records[gate])
	}
	if done {
		out.Status = "done"
	}
	if input.Halted() {
		out.Status = "halted"
	}
	out.Revision = revision(input)
	// Escape individual strings while retaining JSON structure and argv boundaries.
	escape := func(values []string) {
		for i := range values {
			values[i] = termsafe.Escape(values[i])
		}
	}
	for i := range out.Actions {
		a := &out.Actions[i]
		a.Step = termsafe.Escape(a.Step)
		a.Subject.Commit = termsafe.Escape(a.Subject.Commit)
		escape(a.Claim)
		escape(a.Brief)
		escape(a.Submit)
	}
	for i := range out.Decisions {
		d := &out.Decisions[i]
		d.Step = termsafe.Escape(d.Step)
		escape(d.Options)
		escape(d.Submit)
	}
	for i := range out.Run {
		escape(out.Run[i].Argv)
		escape(out.Run[i].Paths)
	}
	for i := range out.Flags {
		f := &out.Flags[i]
		f.Step = termsafe.Escape(f.Step)
		escape(f.Repair)
	}
	for i := range out.Waiting {
		out.Waiting[i].Step = termsafe.Escape(out.Waiting[i].Step)
	}
	return out
}

// OutputExitCode maps every public status, including an unknown status, to an exit.
func OutputExitCode(out Output) int {
	switch out.Status {
	case "work":
		return 0
	case "wait":
		return 3
	case "halted":
		return 4
	case "done":
		return 5
	default:
		return 1
	}
}
func revision(s State) string {
	records := map[string]any{}
	expired := map[string]bool{}
	all, edges := graph(s)
	relevant := map[string]bool{s.Epic: true}
	for changed := true; changed; {
		changed = false
		for _, e := range edges {
			if relevant[e.down] && !relevant[e.up] {
				relevant[e.up] = true
				changed = true
			}
			if relevant[e.up] && !relevant[e.down] {
				relevant[e.down] = true
				changed = true
			}
		}
	}
	for id, r := range all {
		if !relevant[id] {
			continue
		}
		// Titles and descriptions are display text. Wall-clock display fields are absent.
		records[id] = struct {
			ID, Parent, Type, Status string
			Labels                   []string
			Metadata                 map[string]any
			Dependencies             []Dependency
		}{r.ID, r.Parent, r.Type, r.Status, r.Labels, r.Metadata, r.Dependencies}
		if lease, ok, _ := Decode[LeaseRecord](r.Metadata, KeyLease); ok {
			expired[id] = !s.At.Before(lease.Expires)
		}
	}
	data, _ := json.Marshal(struct {
		Records  map[string]any
		Refs     map[string]string
		Ledgers  map[string][][]byte
		Verdicts map[string]json.RawMessage
		Policy   Policy
		Expired  map[string]bool
	}{records, s.Refs, s.Ledgers, s.Verdicts, s.Policy, expired})
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
