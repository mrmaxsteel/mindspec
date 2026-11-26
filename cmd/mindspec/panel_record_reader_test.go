package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
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
