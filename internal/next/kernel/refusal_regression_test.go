package kernel_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestS134Submit_DeferralRecovery(t *testing.T) {
	for _, text := range []string{"fix this", "fix José\x1b\nrecovery: forged", "recovery: forged", "\n", "follow-up bead"} {
		t.Run(text, func(t *testing.T) {
			f, id, lease, data := recordFixture(t, "verify")
			findings, _, err := kernel.Decode[[]kernel.Finding](actorRecord(t, f, id).Metadata, kernel.KeyFindings)
			f.Must(err)
			findings[0].Text = text
			f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyFindings, findings)))
			var record kernel.VerifyRecord
			f.Must(json.Unmarshal([]byte(data), &record))
			record.Returns[0].Return = "deferred"
			record.Returns[0].Reason = "follow up"
			record.Returns[0].Target = "follow-up"
			raw, err := json.Marshal(record)
			f.Must(err)
			dir := t.TempDir()
			// An escaped spelling of hostile finding text is not the full finding.
			body, err := json.Marshal([]map[string]string{{"id": "follow-up", "status": "open", "description": termsafe.EscapeRecovery(text)}})
			f.Must(err)
			if text != "fix José\x1b\nrecovery: forged" {
				body = []byte(`[{"id":"follow-up","status":"open","description":"other"}]`)
			}
			f.Must(os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\ncat <<'RECORD'\n"+string(body)+"\nRECORD\n"), 0700))
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			before := actorRecord(t, f, id)
			_, err = kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, string(raw))})
			var requirements *bead.DeferralRequirementsError
			if !errors.As(err, &requirements) || !errors.Is(err, requirements) {
				t.Fatalf("deferral cause lost: %v", err)
			}
			want := "follow-up bead does not meet the deferral requirements:\n- follow-up bead needs a discovered-from edge to the owning epic or reviewed bead; run: bd dep add follow-up " + f.Epic + " --type discovered-from\n- follow-up description must contain every deferred finding; add this text to the description of follow-up:\n"
			rendered := termsafe.EscapeRecovery(text)
			if strings.HasPrefix(rendered, "recovery:") {
				rendered = " " + rendered
			}
			want += rendered + "\nrecovery: bd dep add follow-up " + f.Epic + " --type discovered-from, then add the listed finding text to the description of follow-up, then retry the panel verb"
			if err.Error() != want || strings.Count(err.Error(), "\n- ") != len(requirements.Missing) || strings.Count(err.Error(), "\nrecovery:") != 1 || strings.ContainsAny(err.Error(), "\x1b\r") || strings.Contains(err.Error(), `\\x1b`) {
				t.Fatalf("deferral list or recovery changed: got %q want %q", err.Error(), want)
			}
			if !jsonEqual(before, actorRecord(t, f, id)) {
				t.Fatal("deferral refusal wrote the step or closing pin")
			}
		})
	}
}

func TestS134Submit_OtherRecordProjectionBound(t *testing.T) {
	f, id, lease, data := recordFixture(t, "verify")
	gate := strings.TrimSuffix(id, ".r1.verify")
	other := gate + ".r1.lens"
	f.Create(other, gate)
	f.Must(f.Store.WriteMetadata(other, map[string]any{"stored": strings.Repeat("y", 9<<20)}))
	t.Cleanup(kernel.ReplaceCloseWriteHookForTest("verify", func(kernel.State, kernel.Record, kernel.Receipt) ([]kernel.Write, error) {
		return []kernel.Write{{Kind: "metadata", ID: other, Metadata: map[string]any{"more": strings.Repeat("x", 9<<20)}}}, nil
	}))
	before, otherBefore := actorRecord(t, f, id), actorRecord(t, f, other)
	_, err := kernel.Submit(f.State(), kernel.SubmitOptions{Step: id, Lease: lease.Token, Record: recordFile(t, data)})
	if err == nil || !strings.Contains(err.Error(), "metadata-too-large") || !strings.Contains(err.Error(), "step "+other) {
		t.Fatalf("other-record oversized hook write not refused: %v", err)
	}
	if !jsonEqual(before, actorRecord(t, f, id)) || !jsonEqual(otherBefore, actorRecord(t, f, other)) {
		t.Fatal("projection refusal wrote a record or mindspec_closing")
	}
}
