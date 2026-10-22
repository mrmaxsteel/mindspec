package panellaunch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerdictFileBytes(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(" \n{\"schema\": 2}\r\n\t ")
	if err := os.WriteFile(filepath.Join(dir, "verdict.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Verdict(dir, "file", "")
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("file bytes changed: got=%q want=%q error=%v", got, raw, err)
	}
}
func TestVerdictChannelOverflow(t *testing.T) {
	for _, output := range []string{"stdout", "file"} {
		t.Run(output, func(t *testing.T) {
			dir := t.TempDir()
			name := "stdout.log"
			if output == "file" {
				name = "verdict.json"
			}
			raw := []byte("MINDSPEC-VERDICT-BEGIN n\n{}\nMINDSPEC-VERDICT-END n\n" + strings.Repeat(" ", 16<<20))
			if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Verdict(dir, output, Hash([]byte("n"))); err == nil || !strings.Contains(err.Error(), "size limit") {
				t.Fatalf("overflow accepted: %v", err)
			}
		})
	}
}
func TestQuotaStderrTail(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      bool
	}{
		{"outside", "LIMIT" + strings.Repeat("x", 65536), false},
		{"at_boundary", "xLIMIT" + strings.Repeat("x", 65531), true},
		{"split_boundary", "LIMIT" + strings.Repeat("x", 65532), false},
		{"end", strings.Repeat("x", 65536) + "LIMIT", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "stderr.log"), []byte(tc.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := Quota(dir, []string{"LIMIT"}); got != tc.want {
				t.Fatalf("last 64 KiB quota=%v want %v", got, tc.want)
			}
		})
	}
}
