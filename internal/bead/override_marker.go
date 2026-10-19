package bead

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// MarkerValidity distinguishes absence from a present but unusable marker.
// Shape validation provides no authentication of agent-writable metadata.
type MarkerValidity string

const (
	MarkerAbsent    MarkerValidity = "absent"
	MarkerValid     MarkerValidity = "valid"
	MarkerLegacy    MarkerValidity = "legacy"
	MarkerMalformed MarkerValidity = "malformed"
)

type OverrideEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

// OverrideMarker is the schema-v2 audit datum shared by writers and readers.
type OverrideMarker struct {
	Schema       int                 `json:"schema"`
	Signals      []string            `json:"signals"`
	Fingerprints map[string][]string `json:"fingerprints"`
	Overridden   string              `json:"overridden"`
	Evidence     OverrideEvidence    `json:"evidence"`
}

// ParseOverrideMarker rejects unknown keys and incomplete or inconsistent
// records. A present null value is malformed, never an absent marker.
func ParseOverrideMarker(raw any) (OverrideMarker, MarkerValidity) {
	bad := func() (OverrideMarker, MarkerValidity) { return OverrideMarker{}, MarkerMalformed }
	data, err := json.Marshal(raw)
	if err != nil {
		return bad()
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return bad()
	}
	_, hasSchema := fields[OverrideFieldSchema]
	keys := []string{OverrideFieldSignals, OverrideFieldOverridden}
	if hasSchema {
		keys = append(keys, OverrideFieldSchema, OverrideFieldFingerprints, OverrideFieldEvidence)
	}
	if !exactOverrideKeys(fields, keys...) {
		return bad()
	}
	var marker OverrideMarker
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&marker); err != nil {
		return bad()
	}
	if len(marker.Signals) == 0 {
		return bad()
	}
	seen := map[string]bool{}
	for _, id := range marker.Signals {
		if (id != "MF-1" && id != "MF-2" && id != "MF-3" && id != "MF-4") || seen[id] {
			return bad()
		}
		seen[id] = true
	}
	if _, err := time.Parse(time.RFC3339, marker.Overridden); err != nil {
		return bad()
	}
	if !hasSchema {
		return marker, MarkerLegacy
	}
	if marker.Schema != 2 || len(marker.Fingerprints) != len(seen) {
		return bad()
	}
	for id := range seen {
		digests := marker.Fingerprints[id]
		if len(digests) == 0 {
			return bad()
		}
		for i, digest := range digests {
			if !overrideDigest(digest) || (i > 0 && digests[i-1] >= digest) {
				return bad()
			}
		}
	}
	var evidenceFields map[string]json.RawMessage
	if err := json.Unmarshal(fields[OverrideFieldEvidence], &evidenceFields); err != nil ||
		!exactOverrideKeys(evidenceFields, OverrideEvidenceFieldPath, OverrideEvidenceFieldSHA256, OverrideEvidenceFieldBytes) {
		return bad()
	}
	if marker.Evidence.Path == "" || !overrideDigest(marker.Evidence.SHA256) || marker.Evidence.Bytes < 1 || marker.Evidence.Bytes > 65536 {
		return bad()
	}
	return marker, MarkerValid
}

func exactOverrideKeys(fields map[string]json.RawMessage, keys ...string) bool {
	if len(fields) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return false
		}
	}
	return true
}

func overrideDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// NewOverrideMarkerV2 validates and copies writer input through the same
// contract as read-back. Callers supply FailureRecordDigests, not prose.
func NewOverrideMarkerV2(signals []string, fingerprints map[string][]string, evidence OverrideEvidence, now time.Time) (OverrideMarker, error) {
	marker, validity := ParseOverrideMarker(OverrideMarker{
		Schema: 2, Signals: signals, Fingerprints: fingerprints,
		Evidence: evidence, Overridden: now.UTC().Format(time.RFC3339),
	})
	if validity != MarkerValid {
		return OverrideMarker{}, fmt.Errorf("invalid schema-v2 readiness override")
	}
	return marker, nil
}

var overrideGetMetadataFn = GetMetadata

// ReadOverrideMarker propagates metadata read errors. Only a missing key
// counts as absent; a present null or other invalid datum is malformed.
func ReadOverrideMarker(beadID string) (OverrideMarker, MarkerValidity, error) {
	metadata, err := overrideGetMetadataFn(beadID)
	if err != nil {
		return OverrideMarker{}, MarkerAbsent, err
	}
	raw, ok := metadata[MetaKeyReadinessOverride]
	if !ok {
		return OverrideMarker{}, MarkerAbsent, nil
	}
	marker, validity := ParseOverrideMarker(raw)
	return marker, validity, nil
}
