package harness

import (
	"os"
	"strings"
	"testing"
)

func TestTestingGuide_SupportedBDFloor(t *testing.T) {
	for path, phrases := range map[string][]string{
		"TESTING.md":                             {"supported floor is bd >= 1.1.0", "harness requires >= 1.1.0 embedded mode"},
		"../../scripts/bd-jsonl-merge-driver.sh": {"recovery: install bd >= 1.1.0"},
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range phrases {
			if !strings.Contains(string(raw), phrase) {
				t.Errorf("%s must state %q", path, phrase)
			}
		}
	}
}
