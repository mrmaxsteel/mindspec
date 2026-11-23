package kernel

import (
	"reflect"
	"testing"
)

func TestS134Closes_RepairRunReason(t *testing.T) {
	options := SubmitOptions{Step: "fixture-work.review.r1.verify", Repair: true, Reason: "recover stored evidence"}
	run := repairRun(options)
	want := []string{"mindspec", "submit", options.Step, "--repair", "--reason", options.Reason}
	if run.Actor != "binary" || !reflect.DeepEqual(run.Argv, want) {
		t.Fatalf("repair run must use the evidence reason: %+v", run)
	}
}
