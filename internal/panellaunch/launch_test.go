package panellaunch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
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
			raw := []byte("MINDSPEC-VERDICT-BEGIN n\n" + strings.Repeat("x", maxVerdictBytes+1) + "\nMINDSPEC-VERDICT-END n\n")
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

func TestVerdictLargeDiagnostics(t *testing.T) {
	for _, placement := range []string{"before", "after", "both"} {
		t.Run(placement, func(t *testing.T) {
			dir := t.TempDir()
			f, err := os.Create(filepath.Join(dir, "stdout.log"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			diagnostics := func() {
				chunk := strings.Repeat("x", 65536)
				for i := 0; i < 273; i++ {
					if _, err := f.WriteString(chunk); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := f.WriteString("\n"); err != nil {
					t.Fatal(err)
				}
			}
			if placement != "after" {
				diagnostics()
			}
			if _, err := f.WriteString("MINDSPEC-VERDICT-BEGIN n\r\n{}\r\nMINDSPEC-VERDICT-END n\r\n"); err != nil {
				t.Fatal(err)
			}
			if placement != "before" {
				diagnostics()
			}
			got, err := Verdict(dir, "stdout", Hash([]byte("n")))
			if err != nil || string(got) != "{}" {
				t.Fatalf("large transcript: %q %v", got, err)
			}
		})
	}
}

func TestExtractBlockRules(t *testing.T) {
	const block = "MINDSPEC-VERDICT-BEGIN n\n{}\nMINDSPEC-VERDICT-END n\n"
	for _, tc := range []struct{ name, transcript, want string }{
		{"one", block, ""},
		{"two", block + block, "multiple verdict blocks"},
		{"wrong_nonce", strings.ReplaceAll(block, " n", " other"), "verdict nonce differs"},
		{"wrong_end", strings.Replace(block, "END n", "END other", 1), "invalid verdict end marker"},
		{"no_end", "MINDSPEC-VERDICT-BEGIN n\n{}", "one complete verdict block is required"},
		{"oversize", "MINDSPEC-VERDICT-BEGIN n\n" + strings.Repeat("x", maxVerdictBytes+1) + "\nMINDSPEC-VERDICT-END n\n", "verdict block exceeds size limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Extract([]byte(tc.transcript), Hash([]byte("n")))
			if tc.want == "" {
				if err != nil || string(got) != "{}" {
					t.Fatalf("valid block: %q %v", got, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s, got %q %v", tc.want, got, err)
			}
		})
	}
}

func TestExtractMarkerBoundary(t *testing.T) {
	for _, endBytes := range []int{4095, 4096, 4097, 65536} {
		t.Run(fmt.Sprint(endBytes), func(t *testing.T) {
			nonce := strings.Repeat("n", endBytes-len("MINDSPEC-VERDICT-END "))
			raw := "MINDSPEC-VERDICT-BEGIN " + nonce + "\n{}\nMINDSPEC-VERDICT-END " + nonce
			got, err := Extract([]byte(raw), Hash([]byte(nonce)))
			if err != nil || string(got) != "{}" {
				t.Fatalf("marker at read boundary: %q %v", got, err)
			}
		})
	}
}

func TestVerdictPayloadLimit(t *testing.T) {
	v := panel.VerdictV2{Schema: 2, Slot: "R1", Round: 1, ReviewedSHA: strings.Repeat("a", 40), Status: "final", Verdict: "APPROVE", Model: "m", Rationale: "x", ConcreteChangesRequired: []panel.ChangeRequired{}, CommandsRun: []panel.CommandRun{}}
	base, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{maxVerdictBytes, maxVerdictBytes + 1} {
		v.Rationale = strings.Repeat("x", size-len(base)+1)
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) != size {
			t.Fatalf("payload size: %d, want %d", len(raw), size)
		}
		if err := panel.ValidateVerdictV2(raw); (err == nil) != (size == maxVerdictBytes) {
			t.Fatalf("validator boundary: %v", err)
		}
		for _, output := range []string{"file", "stdout"} {
			for _, newline := range []string{"\n", "\r\n"} {
				t.Run(fmt.Sprintf("%s/%d/%q", output, size, newline), func(t *testing.T) {
					dir := t.TempDir()
					name, content := "verdict.json", raw
					if output == "stdout" {
						name = "stdout.log"
						content = []byte("MINDSPEC-VERDICT-BEGIN n" + newline + string(raw) + newline + "MINDSPEC-VERDICT-END n" + newline)
					}
					if err := os.WriteFile(filepath.Join(dir, name), content, 0600); err != nil {
						t.Fatal(err)
					}
					got, err := Verdict(dir, output, Hash([]byte("n")))
					if size > maxVerdictBytes {
						if err == nil || !strings.Contains(err.Error(), "size limit") {
							t.Fatalf("oversize payload accepted: %v", err)
						}
					} else if err != nil || !bytes.Equal(got, raw) {
						t.Fatalf("valid boundary payload refused or changed: %v (got %d bytes)", err, len(got))
					}
				})
			}
		}
	}
}

func TestExtractMarkerLimit(t *testing.T) {
	for _, marker := range []string{"BEGIN", "END"} {
		t.Run(marker, func(t *testing.T) {
			line := "MINDSPEC-VERDICT-" + marker + " " + strings.Repeat("n", maxVerdictBytes) + "\n"
			if marker == "END" {
				line = "MINDSPEC-VERDICT-BEGIN n\n{}\n" + line
			}
			if _, err := Extract([]byte(line), Hash([]byte("n"))); err == nil || err.Error() != "verdict marker exceeds size limit" {
				t.Fatalf("marker cap did not stop the line: %v", err)
			}
		})
	}
}
