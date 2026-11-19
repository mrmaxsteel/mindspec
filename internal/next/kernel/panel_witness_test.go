package kernel_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

func TestS134Witness_BadPanelRefusesStateRead(t *testing.T) {
	f := kerneltest.PreKernelSpec(t, kerneltest.WithFake(kernel.NativeIDs))
	path := filepath.Join(f.SpecDir, "reviews", "old", "panel.json")
	f.Must(os.MkdirAll(filepath.Dir(path), 0755))
	f.Must(os.WriteFile(path, []byte(`{not json`), 0600))
	before := kerneltest.Snapshot(t, f)
	f.Fake.Calls = nil
	_, err := kernel.ReadState(f.Store, f.SpecID, f.SpecDir)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("want parse refusal naming %s, got %v", path, err)
	}
	if kerneltest.Snapshot(t, f) != before {
		t.Fatal("bad panel refusal changed snapshot")
	}
	for _, call := range f.Fake.Calls {
		if call[0] != "list" && !(len(call) > 1 && call[0] == "dep" && call[1] == "list") {
			t.Fatalf("bad panel state read wrote: %v", call)
		}
	}
}
