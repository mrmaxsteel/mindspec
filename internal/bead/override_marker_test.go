package bead

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func validOverrideRaw(t *testing.T) map[string]any {
	t.Helper()
	marker, err := NewOverrideMarkerV2([]string{"MF-3"}, map[string][]string{"MF-3": {strings.Repeat("a", 64), strings.Repeat("b", 64)}},
		OverrideEvidence{Path: "/tmp/evidence", SHA256: strings.Repeat("c", 64), Bytes: 1}, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOverrideMarker_Validity(t *testing.T) {
	tests := []struct {
		name string
		edit func(map[string]any)
		want MarkerValidity
	}{
		{"valid", func(m map[string]any) {}, MarkerValid},
		{"upper-hex-and-max-bytes", func(m map[string]any) {
			m["evidence"].(map[string]any)["bytes"] = 65536
			m["evidence"].(map[string]any)["sha256"] = strings.Repeat("F", 64)
		}, MarkerValid},
		{"legacy", func(m map[string]any) { delete(m, "schema"); delete(m, "fingerprints"); delete(m, "evidence") }, MarkerLegacy},
		{"schema-missing-v2", func(m map[string]any) { delete(m, "schema") }, MarkerMalformed},
		{"schema-wrong", func(m map[string]any) { m["schema"] = 1 }, MarkerMalformed},
		{"schema-string", func(m map[string]any) { m["schema"] = "2" }, MarkerMalformed},
		{"schema-fraction", func(m map[string]any) { m["schema"] = 2.5 }, MarkerMalformed},
		{"signals-missing", func(m map[string]any) { delete(m, "signals") }, MarkerMalformed},
		{"signals-empty", func(m map[string]any) { m["signals"] = []string{} }, MarkerMalformed},
		{"signals-null", func(m map[string]any) { m["signals"] = nil }, MarkerMalformed},
		{"signals-unknown", func(m map[string]any) { m["signals"] = []string{"MF-5"} }, MarkerMalformed},
		{"signals-duplicate", func(m map[string]any) { m["signals"] = []string{"MF-3", "MF-3"} }, MarkerMalformed},
		{"signals-wrong-type", func(m map[string]any) { m["signals"] = "MF-3" }, MarkerMalformed},
		{"fingerprints-missing", func(m map[string]any) { delete(m, "fingerprints") }, MarkerMalformed},
		{"fingerprints-missing-signal", func(m map[string]any) { m["fingerprints"] = map[string][]string{} }, MarkerMalformed},
		{"fingerprints-extra-signal", func(m map[string]any) { m["fingerprints"].(map[string]any)["MF-2"] = []string{strings.Repeat("a", 64)} }, MarkerMalformed},
		{"fingerprints-wrong-signal", func(m map[string]any) { m["fingerprints"] = map[string][]string{"MF-2": {strings.Repeat("a", 64)}} }, MarkerMalformed},
		{"fingerprints-empty", func(m map[string]any) { m["fingerprints"].(map[string]any)["MF-3"] = []string{} }, MarkerMalformed},
		{"fingerprints-short", func(m map[string]any) { m["fingerprints"].(map[string]any)["MF-3"] = []string{"abc"} }, MarkerMalformed},
		{"fingerprints-nonhex", func(m map[string]any) { m["fingerprints"].(map[string]any)["MF-3"] = []string{strings.Repeat("z", 64)} }, MarkerMalformed},
		{"fingerprints-duplicate", func(m map[string]any) {
			m["fingerprints"].(map[string]any)["MF-3"] = []string{strings.Repeat("a", 64), strings.Repeat("a", 64)}
		}, MarkerMalformed},
		{"fingerprints-unsorted", func(m map[string]any) {
			m["fingerprints"].(map[string]any)["MF-3"] = []string{strings.Repeat("b", 64), strings.Repeat("a", 64)}
		}, MarkerMalformed},
		{"overridden-missing", func(m map[string]any) { delete(m, "overridden") }, MarkerMalformed},
		{"overridden-invalid", func(m map[string]any) { m["overridden"] = "yesterday" }, MarkerMalformed},
		{"evidence-missing", func(m map[string]any) { delete(m, "evidence") }, MarkerMalformed},
		{"evidence-null", func(m map[string]any) { m["evidence"] = nil }, MarkerMalformed},
		{"evidence-path-empty", func(m map[string]any) { m["evidence"].(map[string]any)["path"] = "" }, MarkerMalformed},
		{"evidence-sha-invalid", func(m map[string]any) { m["evidence"].(map[string]any)["sha256"] = strings.Repeat("g", 64) }, MarkerMalformed},
		{"evidence-bytes-zero", func(m map[string]any) { m["evidence"].(map[string]any)["bytes"] = 0 }, MarkerMalformed},
		{"evidence-bytes-too-large", func(m map[string]any) { m["evidence"].(map[string]any)["bytes"] = 65537 }, MarkerMalformed},
		{"evidence-bytes-fraction", func(m map[string]any) { m["evidence"].(map[string]any)["bytes"] = 1.5 }, MarkerMalformed},
		{"evidence-unknown-key", func(m map[string]any) { m["evidence"].(map[string]any)["extra"] = true }, MarkerMalformed},
		{"unknown-extra-key", func(m map[string]any) { m["extra"] = true }, MarkerMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := validOverrideRaw(t)
			tt.edit(raw)
			_, got := ParseOverrideMarker(raw)
			if got != tt.want {
				t.Fatalf("validity = %s, want %s", got, tt.want)
			}
		})
	}
	for _, raw := range []any{nil, "marker", []string{"MF-3"}, 2, make(chan int)} {
		if _, got := ParseOverrideMarker(raw); got != MarkerMalformed {
			t.Errorf("%T: validity = %s, want malformed", raw, got)
		}
	}
}

func TestOverrideMarker_Builder(t *testing.T) {
	raw := validOverrideRaw(t)
	marker, validity := ParseOverrideMarker(raw)
	if validity != MarkerValid {
		t.Fatal(validity)
	}
	now, err := time.Parse(time.RFC3339, marker.Overridden)
	if err != nil {
		t.Fatal(err)
	}
	built, err := NewOverrideMarkerV2(marker.Signals, marker.Fingerprints, marker.Evidence, now)
	if err != nil || !reflect.DeepEqual(built, marker) {
		t.Fatalf("round-trip: %#v, %v", built, err)
	}
	marker.Fingerprints["MF-3"][0] = "mutated"
	if built.Fingerprints["MF-3"][0] == "mutated" {
		t.Fatal("builder retained caller's mutable input")
	}
	if _, err := NewOverrideMarkerV2(marker.Signals, marker.Fingerprints, marker.Evidence, now); err == nil {
		t.Fatal("builder accepted invalid fingerprint")
	}
}

func TestOverrideMarker_Read(t *testing.T) {
	old := overrideGetMetadataFn
	t.Cleanup(func() { overrideGetMetadataFn = old })
	wantErr := errors.New("metadata unavailable")
	overrideGetMetadataFn = func(string) (map[string]any, error) { return nil, wantErr }
	if _, _, err := ReadOverrideMarker("mindspec-fixture.1"); !errors.Is(err, wantErr) {
		t.Fatalf("read error lost: %v", err)
	}
	for _, tt := range []struct {
		metadata map[string]any
		want     MarkerValidity
	}{
		{map[string]any{}, MarkerAbsent},
		{map[string]any{MetaKeyReadinessOverride: nil}, MarkerMalformed},
		{map[string]any{MetaKeyReadinessOverride: validOverrideRaw(t)}, MarkerValid},
	} {
		overrideGetMetadataFn = func(string) (map[string]any, error) { return tt.metadata, nil }
		if _, got, err := ReadOverrideMarker("mindspec-fixture.1"); err != nil || got != tt.want {
			t.Fatalf("read = %s, %v; want %s", got, err, tt.want)
		}
	}
}
