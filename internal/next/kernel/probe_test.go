package kernel_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Probe_CustomIDs(t *testing.T) {
	root := kerneltest.NewRepo(t)
	run := kerneltest.Runner(t, root)
	create := func(args ...string) kernel.Record {
		out, err := run(args...)
		if err != nil {
			t.Fatalf("bd %v: %v %s", args, err, out)
		}
		var r kernel.Record
		if err = json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	epic := create("create", "--title", "probe", "--type", "epic", "--json")
	work := create("create", "--title", "work", "--type", "task", "--parent", epic.ID, "--json")
	for _, id := range []string{work.ID + ".impl.a1", work.ID + ".review", epic.ID + ".final", work.ID + ".review.decide.reject.r1", work.ID + ".review.judge.a1"} {
		t.Run(strings.TrimPrefix(id, epic.ID), func(t *testing.T) {
			out, err := run("create", "--id", id, "--parent", work.ID, "--type", "step", "--title", "probe", "--json")
			t.Logf("bd 1.3.4 create --id %s: %s %v", id, out, err)
			accepted := err == nil
			if accepted != kernel.NativeIDs {
				t.Fatalf("NativeIDs=%t differs from probe accepted=%t: %s", kernel.NativeIDs, accepted, fmt.Sprint(err))
			}
			if !accepted && !strings.Contains(fmt.Sprint(err), "cannot specify both --id and --parent flags") {
				t.Fatalf("probe failed for another reason: %v", err)
			}
		})
	}
}
