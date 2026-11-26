package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

// recordOnlyPanel registers a refused panel of two slots on an enrolled spec.
// Each slot holds its verdict as a stored record with a receipt. With remove,
// the verdict files are moved away, as in a fresh clone.
func recordOnlyPanel(t *testing.T, remove bool) *kerneltest.Fixture {
	t.Helper()
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	kerneltest.Register(f, gate, kernel.RegistrationRecord{Slug: "review", Round: 1, Subject: head, Slots: []string{"R1", "R2"}})
	for _, slot := range []string{"R1", "R2"} {
		kernelPanelVerdict(t, f, gate, head, "review", slot, "REQUEST_CHANGES", 1)
		file := filepath.Join(f.SpecDir, "reviews", "review", slot+"-round-1.json")
		data, err := os.ReadFile(file)
		f.Must(err)
		id := gate + ".r1." + slot
		f.Must(f.Store.WriteMetadata(id, kernel.Metadata(kernel.KeyRecord, string(data))))
		f.Must(f.Store.WriteReceipt(id, kernel.Receipt{Kind: "passed", Generation: 1, RecordSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}))
		if remove {
			f.Must(os.Rename(file, filepath.Join(t.TempDir(), slot+".json")))
		}
	}
	return f
}

// On an enrolled spec, panel refute and panel defer read the stored slot
// records. They work the same with the verdict files and without them.
func TestPanelRefute_StoredRecordsWithoutFiles(t *testing.T) {
	for _, verb := range []string{"refute", "defer"} {
		for _, remove := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/files-removed=%v", verb, remove), func(t *testing.T) {
				f := recordOnlyPanel(t, remove)
				if verb == "refute" {
					out, err := kernelPanelRun(t, panelRefuteCmd, "review", "--spec", f.SpecID, "--slot", "R1", "--round", "1", "--finding", "ccr[0]", "--by", "operator", "--ground", "disproven", "--reason", "checked", "--evidence", "proof.txt")
					if err != nil {
						t.Fatalf("refute of a stored verdict failed: %v\n%s", err, out)
					}
					return
				}
				fake := fakeDeferralBD(t)
				rebindWrite(t, fake, "epics.json", fmt.Sprintf(`[{"id":%q,"issue_type":"epic","metadata":{"spec_id":%q}}]`, f.Epic, f.SpecID))
				rebindWrite(t, fake, "follow-up.json", fmt.Sprintf(`[{"id":"follow-up","status":"open","description":"missing test","dependencies":[{"id":%q,"dependency_type":"discovered-from"}]}]`, f.Epic))
				out, err := kernelPanelRun(t, panelDeferCmd, "review", "--spec", f.SpecID, "--slot", "R2", "--round", "1", "--finding", "ccr[0]", "--bead", "follow-up", "--by", "operator")
				if err != nil {
					t.Fatalf("defer of a stored verdict failed: %v\n%s", err, out)
				}
			})
		}
	}
}

// On an enrolled spec, panel tally shows the findings of each stored slot
// record, and panel verify gives each slot the status of its record. Both
// print the same with the verdict files and without them.
func TestPanelReaders_StoredRecordsWithoutFiles(t *testing.T) {
	outputs := map[string][2]string{}
	for i, remove := range []bool{false, true} {
		f := recordOnlyPanel(t, remove)
		tally, _ := kernelPanelRun(t, panelTallyCmd, "review", "--spec", f.SpecID)
		verify, verifyErr := kernelPanelRun(t, panelVerifyCmd, "review", "--spec", f.SpecID)
		if strings.Contains(tally, "could not re-read") || !strings.Contains(tally, "concrete_changes_required (aggregated):\n  R1: ") {
			t.Fatalf("tally lost the stored findings (files removed=%v):\n%s", remove, tally)
		}
		if verifyErr != nil || strings.Contains(verify, "damaged") || !strings.Contains(verify, "R1 final ") || !strings.Contains(verify, "R2 final ") {
			t.Fatalf("verify did not read the stored records (files removed=%v): %v\n%s", remove, verifyErr, verify)
		}
		// The fixture commits differ, so only the record lines are compared.
		_, changes, _ := strings.Cut(tally, "concrete_changes_required (aggregated):")
		var slots []string
		for _, line := range strings.Split(verify, "\n") {
			if strings.HasPrefix(line, "R1 ") || strings.HasPrefix(line, "R2 ") {
				slots = append(slots, line)
			}
		}
		for name, out := range map[string]string{"tally": changes, "verify": strings.Join(slots, "\n")} {
			pair := outputs[name]
			pair[i] = out
			outputs[name] = pair
		}
	}
	for name, pair := range outputs {
		if pair[0] != pair[1] {
			t.Fatalf("%s differs without the verdict files:\nwith files:\n%s\nwithout files:\n%s", name, pair[0], pair[1])
		}
	}
}

// When the tally leaves out a present verdict file because it differs from
// its stored record, or because the record fails its receipt hash, panel
// tally names the file and the cause.
func TestPanelTally_NamesFileLeftOutForItsRecord(t *testing.T) {
	for _, mode := range []string{"edited-file", "changed-record"} {
		t.Run(mode, func(t *testing.T) {
			f := recordOnlyPanel(t, false)
			file := filepath.Join(f.SpecDir, "reviews", "review", "R1-round-1.json")
			data, err := os.ReadFile(file)
			f.Must(err)
			want := "R1-round-1.json not counted: file differs from the stored record"
			if mode == "edited-file" {
				f.Must(os.WriteFile(file, append(data, '\n'), 0644))
			} else {
				f.Must(f.Store.WriteMetadata(f.Items[0]+".review.r1.R1", kernel.Metadata(kernel.KeyRecord, string(data)+"\n")))
				want = "R1-round-1.json not counted: stored record does not match its receipt"
			}
			out, _ := kernelPanelRun(t, panelTallyCmd, "review", "--spec", f.SpecID)
			if !strings.Contains(out, "\n  "+want+"\n") {
				t.Fatalf("tally does not name the left-out file and its cause %q:\n%s", want, out)
			}
		})
	}
}
