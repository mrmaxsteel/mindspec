package executor

import (
	"os"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
)

var finalizeReadDefault = finalizeMetadataReadFn
var finalizeWriteDefault = finalizeMetadataWriteFn

// Keep existing executor fixtures tracker-free. Approval integration tests run
// in their own package and exercise the real metadata functions with fake bd.
func TestMain(m *testing.M) {
	finalizeMetadataReadFn = func(string) (map[string]interface{}, error) { return map[string]interface{}{}, nil }
	finalizeMetadataWriteFn = func(string, map[string]interface{}) error { return nil }
	os.Exit(m.Run())
}

func TestFinalizeMetadataSeams_DefaultsToTracker(t *testing.T) {
	if reflect.ValueOf(finalizeReadDefault).Pointer() != reflect.ValueOf(bead.GetMetadata).Pointer() {
		t.Fatal("finalize reads must use tracker metadata")
	}
	if reflect.ValueOf(finalizeWriteDefault).Pointer() != reflect.ValueOf(bead.MergeMetadata).Pointer() {
		t.Fatal("finalize writes must merge tracker metadata")
	}
}
