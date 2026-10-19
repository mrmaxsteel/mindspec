package readiness

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func TestFailureRecordDigests_Deterministic(t *testing.T) {
	root := t.TempDir()
	specID, beadID, epicID := writeSimpleWorkspace(t, root, "digests", "    key_file_paths: []", "AC-1 exists.", "R9(a) AC9\nTBD\nTBD")
	store := NewFakeBDStore()
	store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
	store.Records[beadID] = FakeBeadRecord{Description: "TBD\x1b[31m"}
	t.Cleanup(store.Install())
	first, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatal(err)
	}
	wantCounts := []int{1, 2, 0, 3} // MF-3 emission belongs to Bead 4.
	for i, a := range first.Signals {
		b := second.Signals[i]
		if len(a.Failures) != wantCounts[i] {
			t.Fatalf("%s records=%v, want %d", a.ID, a.Failures, wantCounts[i])
		}
		if !sort.StringsAreSorted(a.Failures) || !reflect.DeepEqual(a.Failures, b.Failures) {
			t.Fatalf("%s records are not stable sorted identities: %v / %v", a.ID, a.Failures, b.Failures)
		}
		var expected []string
		for _, rec := range a.Failures {
			expected = append(expected, fmt.Sprintf("%x", sha256.Sum256([]byte(rec))))
		}
		sort.Strings(expected)
		digests := FailureRecordDigests(a)
		if len(digests) >= 3 && !sort.StringsAreSorted(digests) {
			t.Fatalf("%s digests are not sorted: %v", a.ID, digests)
		}
		if len(digests) != len(expected) {
			t.Fatalf("%s digest count=%d, want %d", a.ID, len(digests), len(expected))
		}
		for j := range expected {
			if digests[j] != expected[j] {
				t.Fatalf("%s digest=%s, want %s", a.ID, digests[j], expected[j])
			}
		}
		encoded1, _ := json.Marshal(digests)
		encoded2, _ := json.Marshal(FailureRecordDigests(b))
		if string(encoded1) != string(encoded2) {
			t.Fatalf("%s digests differ: %s / %s", a.ID, encoded1, encoded2)
		}
	}
	// Digest sorting is independent of the incoming record order and Detail.
	aDigest := fmt.Sprintf("%x", sha256.Sum256([]byte("a")))
	zDigest := fmt.Sprintf("%x", sha256.Sum256([]byte("z")))
	if !("a" < "z" && aDigest > zDigest) {
		t.Fatalf("fixture must reverse record/digest order: a=%s z=%s", aDigest, zDigest)
	}
	input := Signal{Failures: []string{"a", "z"}, Detail: "human detail"}
	before := append([]string(nil), input.Failures...)
	want := []string{zDigest, aDigest}
	if got := FailureRecordDigests(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("digest sort=%v, want %v", got, want)
	}
	if !reflect.DeepEqual(input.Failures, before) {
		t.Fatal("digesting mutated raw records")
	}
	three := FailureRecordDigests(Signal{Failures: []string{"a", "b", "z"}})
	if len(three) != 3 || !sort.StringsAreSorted(three) {
		t.Fatalf("three-record digests must be sorted: %v", three)
	}
}

func TestFailureRecordDigests_WriterReaderAgree(t *testing.T) {
	const line = "TBD\x1b[31m decide"
	root := t.TempDir()
	specID, beadID, epicID := writeSimpleWorkspace(t, root, "digestio", "    key_file_paths: [real.go]", "AC-1 exists.", "")
	store := NewFakeBDStore()
	store.Lineage[beadID] = FakeLineage{EpicID: epicID, SpecID: specID}
	store.Records[beadID] = FakeBeadRecord{Description: line}
	t.Cleanup(store.Install())
	writerReport, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatal(err)
	}
	writerSignal := signalByID(writerReport, SignalBlocking)
	rawRecord := fmt.Sprintf("token:description:-:1:%x", sha256.Sum256([]byte(line)))
	if !reflect.DeepEqual(writerSignal.Failures, []string{rawRecord}) {
		t.Fatalf("raw records=%q, want %q", writerSignal.Failures, rawRecord)
	}
	// Exercise the serialized fingerprint contract consumed by the later
	// marker writer/reader beads, without introducing marker reads here.
	fingerprints := map[string][]string{SignalBlocking: FailureRecordDigests(writerSignal)}
	encoded, err := json.Marshal(fingerprints)
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string][]string
	if err := json.Unmarshal(encoded, &persisted); err != nil {
		t.Fatal(err)
	}
	rendered := Render(writerReport)
	if strings.ContainsRune(rendered, '\x1b') || !strings.Contains(rendered, `\x1b`) {
		t.Fatalf("control byte was not escaped by renderer: %q", rendered)
	}
	readerReport, err := EvaluateReadiness(root, beadID)
	if err != nil {
		t.Fatal(err)
	}
	readerDigests := FailureRecordDigests(signalByID(readerReport, SignalBlocking))
	want := []string{fmt.Sprintf("%x", sha256.Sum256([]byte(rawRecord)))}
	if !reflect.DeepEqual(persisted[SignalBlocking], want) || !reflect.DeepEqual(readerDigests, want) {
		t.Fatalf("writer=%v reader=%v, want raw digest %v", persisted, readerDigests, want)
	}
	if !reflect.DeepEqual(writerSignal.Failures, []string{rawRecord}) {
		t.Fatal("rendering mutated failure identity")
	}
	t.Run("raw-control-record", func(t *testing.T) {
		raw := DependencyFailureRecord("mindspec-dep\x1b.1", FailureNotClosed, "")
		if raw != "mindspec-dep\x1b.1:not-closed" {
			t.Fatalf("constructor changed raw record: %q", raw)
		}
		writer := Signal{ID: SignalDependencies, Failures: []string{raw}}
		encoded, err := json.Marshal(writer)
		if err != nil {
			t.Fatal(err)
		}
		var reader Signal
		if err := json.Unmarshal(encoded, &reader); err != nil {
			t.Fatal(err)
		}
		want := []string{fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))}
		escapedDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(termsafe.Escape(raw))))
		if want[0] == escapedDigest {
			t.Fatal("raw and escaped digest fixture must differ")
		}
		writerDigests, readerDigests := FailureRecordDigests(writer), FailureRecordDigests(reader)
		if !reflect.DeepEqual(writerDigests, want) || !reflect.DeepEqual(readerDigests, want) {
			t.Fatalf("control-byte digests writer=%v reader=%v, want raw=%v (escaped=%s)", writerDigests, readerDigests, want, escapedDigest)
		}
	})
}

func TestMF3FailureRecords(t *testing.T) {
	for _, class := range []string{FailureIndeterminate, FailureNotClosed, FailureNoCandidate, FailureOffFirstParent, FailureNoEvidence, FailureContradiction, FailureReverted} {
		for _, sha := range []string{"", strings.Repeat("a", 40)} {
			want := "mindspec-dep.1:" + class
			if sha != "" {
				want += ":" + sha
			}
			got := DependencyFailureRecord("mindspec-dep.1", class, sha)
			if got != want {
				t.Fatalf("record=%q, want %q", got, want)
			}
			if IsIndeterminateRecord(got) != (class == "indeterminate") {
				t.Fatalf("wrong read-failure classification: %q", got)
			}
		}
	}
	for _, rec := range []string{"", "indeterminate", ":indeterminate", "dep:no-evidence:indeterminate", "dep:indeterminate:sha:extra"} {
		if IsIndeterminateRecord(rec) {
			t.Fatalf("malformed/non-indeterminate record classified: %q", rec)
		}
	}
}
