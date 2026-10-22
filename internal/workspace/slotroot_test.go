package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindSlotRoot(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	slot := SlotRoot{Spec: "test", Slug: "p", Round: 1, Slot: "R1", MainRoot: root}
	dir := filepath.Join(root, "scratch", "test", "p", "r1", "R1")
	write := func(dir string, s SlotRoot) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, "tree", "sub"), 0o700); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "slot.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(dir, slot)
	resolve := func(main string) (string, error) {
		if main != root {
			t.Fatalf("wrong main root %s", main)
		}
		return "scratch", nil
	}
	found, ok, err := FindSlotRoot(filepath.Join(dir, "tree", "sub"), resolve)
	if err != nil || !ok || found.MainRoot != root || found.Dir != dir || found.Slot != "R1" {
		t.Fatalf("root: %+v %v %v", found, ok, err)
	}
	// A nested slot.json without a tree sibling is not a slot marker pair.
	if err := os.WriteFile(filepath.Join(dir, "tree", "sub", "slot.json"), []byte(`{"slot":"wrong"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	found, ok, err = FindSlotRoot(filepath.Join(dir, "tree", "sub"), resolve)
	if err != nil || !ok || found.Dir != dir {
		t.Fatalf("nested decoy: %+v %v %v", found, ok, err)
	}
	for _, name := range []string{"wrong slot", "outside isolation", "wrong round", "wrong slug", "wrong spec", "relative root"} {
		t.Run(name, func(t *testing.T) {
			bad := slot
			path := dir
			switch name {
			case "wrong slot":
				bad.Slot = "R2"
			case "outside isolation":
				path = filepath.Join(root, "copy")
			case "wrong round":
				bad.Round = 2
			case "wrong slug":
				bad.Slug = "q"
			case "wrong spec":
				bad.Spec = "other"
			case "relative root":
				bad.MainRoot = "relative"
			}
			write(path, bad)
			_, ok, err := FindSlotRoot(filepath.Join(path, "tree"), resolve)
			if err == nil || !ok || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("accepted bad slot: %v %v", ok, err)
			}
			write(dir, slot)
		})
	}
	if _, ok, err := FindSlotRoot(root, resolve); err != nil || ok {
		t.Fatalf("no slot: %v %v", ok, err)
	}
}
