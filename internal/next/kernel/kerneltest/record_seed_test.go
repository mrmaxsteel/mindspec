package kerneltest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
)

func TestS134Fixture_RecordSeeders(t *testing.T) {
	f := Enrolled(t, WithFake(false))
	gate := f.Items[0] + ".review"
	subject := strings.Repeat("a", 40)
	Authorise(f, gate, kernel.GateRound{Round: 1, Subject: subject, SlotCount: 2})
	finding := kernel.Finding{Slot: "R1", Round: 1, Finding: "ccr[0]", FindingSHA256: strings.Repeat("b", 64), Text: "check paths"}
	verify := kernel.VerifyRecord{Step: gate + ".r1.verify", Subject: subject, Returns: []kernel.VerifyReturn{{Slot: finding.Slot, Round: finding.Round, Finding: finding.Finding, FindingSHA256: finding.FindingSHA256, Return: "reproduced"}}}
	read := map[string]string{"verdict.json": "digest"}
	verifyID := Verify(f, gate, verify, []kernel.Finding{finding}, read)
	lens := kernel.LensRecord{Step: gate + ".r1.lens", Lenses: []kernel.SlotLens{{Slot: "R1", Lens: "inspect paths"}}}
	lensID := Lens(f, gate, lens)
	for _, tc := range []struct {
		id     string
		record any
	}{{verifyID, verify}, {lensID, lens}} {
		r, err := f.Store.Lookup(tc.id)
		f.Must(err)
		if r == nil || r.Status != "closed" {
			t.Fatalf("record seeder did not close %s", tc.id)
		}
		receipt, has, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
		f.Must(err)
		if !has || receipt.Kind != "passed" || receipt.Generation != 1 {
			t.Fatalf("record seeder receipt missing: %+v", receipt)
		}
		f.Must(kernel.ValidReceipt(f.Root, receipt, r.Metadata))
		data, err := json.Marshal(tc.record)
		f.Must(err)
		if r.Metadata[kernel.KeyRecord] != string(data) {
			t.Fatal("record seeder lost exact bytes")
		}
		seeded, has, err := kernel.Decode[kernel.Subject](r.Metadata, kernel.KeySubject)
		f.Must(err)
		if !has || seeded.Commit != subject {
			t.Fatal("record seeder lost subject")
		}
	}
	r, err := f.Store.Lookup(verifyID)
	f.Must(err)
	for key, want := range map[string]any{kernel.KeyFindings: []kernel.Finding{finding}, kernel.KeyReturns: verify.Returns, kernel.KeyRead: read} {
		data, err := json.Marshal(want)
		f.Must(err)
		var normalized any
		f.Must(json.Unmarshal(data, &normalized))
		if !reflect.DeepEqual(r.Metadata[key], normalized) {
			t.Fatalf("verify seeder lost %s", key)
		}
	}
	pin, has, err := kernel.Decode[kernel.Closing](r.Metadata, kernel.KeyClosing)
	f.Must(err)
	receipt, _, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
	f.Must(err)
	if !has || pin.Generation != receipt.Generation || pin.RecordSHA256 != receipt.RecordSHA256 || pin.At.Format("2006-01-02T15:04:05.999999999Z07:00") != receipt.At {
		t.Fatal("verify seeder lost closing pin")
	}
}

func TestS134Fixture_KernelTempCleanup(t *testing.T) {
	caller := t.TempDir()
	t.Setenv("TMPDIR", caller)
	var dirs []string
	t.Run("fixtures", func(t *testing.T) {
		if _, err := command(t, "", "sh", "-c", `test "$BD_DISABLE_METRICS" = 1 && test "$BD_DISABLE_EVENT_FLUSH" = 1`); err != nil {
			t.Fatalf("rootless fixture can start a late metrics or event writer: %v", err)
		}
		for _, f := range []*Fixture{Enrolled(t, WithFake(false)), EmptyRepo(t)} {
			out, err := f.BinaryCommand("sh", "-c", `printf %s "$TMPDIR"`)
			f.Must(err)
			childTemp := strings.TrimSpace(string(out))
			rel, err := filepath.Rel(filepath.Dir(f.Root), childTemp)
			f.Must(err)
			if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || childTemp == f.Root {
				t.Fatalf("child temporary directory escaped fixture lifetime: %s", childTemp)
			}
			toRepo, err := filepath.Rel(f.Root, childTemp)
			f.Must(err)
			if toRepo != ".." && !strings.HasPrefix(toRepo, ".."+string(filepath.Separator)) {
				t.Fatalf("child temporary directory is inside the repository: %s", childTemp)
			}
			dirs = append(dirs, childTemp)
			dir, err := kernel.KernelTempDir(f.Root)
			f.Must(err)
			dirs = append(dirs, dir)
			if err := os.WriteFile(filepath.Join(dir, "record.json"), []byte("record"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	})
	for _, dir := range dirs {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("fixture temporary directory leaked: %s %v", dir, err)
		}
	}
	entries, err := os.ReadDir(caller)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("fixture leaked into caller TMPDIR: %v", entries)
	}
}

func TestS134Fixture_ParallelTempCleanup(t *testing.T) {
	var dir string
	t.Run("group", func(t *testing.T) {
		t.Run("parallel", func(t *testing.T) {
			t.Parallel()
			f := Enrolled(t, WithFake(false))
			var err error
			dir, err = kernel.KernelTempDir(f.Root)
			f.Must(err)
		})
	})
	if dir == "" {
		t.Fatal("parallel fixture did not run")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("parallel fixture leaked its kernel directory: %v", err)
	}
}
