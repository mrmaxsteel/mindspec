package complete

// Spec 128 final-review round 1 (R6 F1/G1): MockExecutor-level pins for
// the discriminator's marker-provenance attestation — the precise
// per-leg behaviors the real-git forgery fixture (bead6 leg xxiv) cannot
// isolate one call site at a time. The attestation seams are the
// package's existing metadata seam pair (completeGetMetadataFn /
// completeMergeMetadataFn), stubbed per test via saveAndRestore.

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/executor"
)

// TestDiscriminator_AttestedMarkerResumes: tip == marker, known stage,
// attestation names the marker OID → resume honored, nothing cleared.
func TestDiscriminator_AttestedMarkerResumes(t *testing.T) {
	saveAndRestore(t)
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-att.1", "bead/mindspec-att.1", "/tmp/root"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return "marker-att-1", stageArtifactSync, true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		return "marker-att-1", nil // tip == marker
	}
	var cleared bool
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		cleared = true
		return nil
	}
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		return map[string]interface{}{stageMarkerAttestationKey: "marker-att-1"}, nil
	}

	stage, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err != nil {
		t.Fatalf("expected the attested marker to resume, got: %v", err)
	}
	if stage != stageArtifactSync {
		t.Errorf("expected resumeStage %q, got %q", stageArtifactSync, stage)
	}
	if cleared {
		t.Error("an attested marker must never be cleared by the provenance gate")
	}
}

// TestDiscriminator_UnattestedMarkerForeignClears: a structurally
// perfect marker (tip == marker, known stage) with NO binary-side
// attestation — R6 F1's forged-ref shape — is FOREIGN: cleared (CAS
// against the observed OID), resumeStage=="" (a normal, fully-gated
// run).
func TestDiscriminator_UnattestedMarkerForeignClears(t *testing.T) {
	saveAndRestore(t)
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-att.2", "bead/mindspec-att.2", "/tmp/root"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return "marker-att-2", stageMaterialization, true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		return "marker-att-2", nil
	}
	var clearedWith string
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		clearedWith = expectedOID
		return nil
	}
	// saveAndRestore's default completeGetMetadataFn already returns an
	// empty map — the unattested shape — but pin it explicitly here.
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}

	stage, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err != nil {
		t.Fatalf("expected foreign-clear, got error: %v", err)
	}
	if stage != "" {
		t.Errorf("an unattested marker must NEVER resume, got resumeStage %q", stage)
	}
	if clearedWith != "marker-att-2" {
		t.Errorf("expected the forged marker cleared via CAS against the observed OID, got %q", clearedWith)
	}
}

// TestDiscriminator_MismatchedAttestationForeignClears: an attestation
// naming a DIFFERENT OID (a stale pre-crash value, or a forgery racing a
// real publication) is the same FOREIGN outcome — exact equality, no
// leniency.
func TestDiscriminator_MismatchedAttestationForeignClears(t *testing.T) {
	saveAndRestore(t)
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-att.3", "bead/mindspec-att.3", "/tmp/root"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return "marker-att-3", stageArtifactSync, true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		return "marker-att-3", nil
	}
	var cleared bool
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		cleared = true
		return nil
	}
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		return map[string]interface{}{stageMarkerAttestationKey: "some-other-oid"}, nil
	}

	stage, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err != nil {
		t.Fatalf("expected foreign-clear, got error: %v", err)
	}
	if stage != "" || !cleared {
		t.Errorf("mismatched attestation must clear and run normally, got stage=%q cleared=%v", stage, cleared)
	}
}

// TestDiscriminator_AttestationReadFailurePropagates: a bd read failure
// is NEITHER "unattested" (which would clear genuine write-ahead
// evidence on a blip) NOR "attested" — it propagates, fail-closed.
func TestDiscriminator_AttestationReadFailurePropagates(t *testing.T) {
	saveAndRestore(t)
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-att.4", "bead/mindspec-att.4", "/tmp/root"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return "marker-att-4", stageMaterialization, true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		return "marker-att-4", nil
	}
	var cleared bool
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		cleared = true
		return nil
	}
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		return nil, fmt.Errorf("transient: dolt gone")
	}

	_, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err == nil {
		t.Fatal("expected the attestation read failure to propagate")
	}
	if cleared {
		t.Error("a transient attestation-read failure must NEVER clear the marker (would destroy recovery evidence)")
	}
}

// TestDiscriminator_UnattestedParentLegNeverAdvancesBranch: the
// unfinished-advance leg (tip == marker's parent) verifies provenance
// BEFORE AdvanceBranchExpectedOld — a forged marker must never pull the
// bead branch's real ref forward onto it.
func TestDiscriminator_UnattestedParentLegNeverAdvancesBranch(t *testing.T) {
	saveAndRestore(t)
	mock := &executor.MockExecutor{}
	const beadID, beadBranch, root = "mindspec-att.5", "bead/mindspec-att.5", "/tmp/root"
	const markerOID = "marker-att-5"
	mock.ReadStageMarkerFn = func(id string) (string, string, bool, error) {
		return markerOID, stageMaterialization, true, nil
	}
	mock.RevParseRefFn = func(workdir, ref string) (string, error) {
		switch ref {
		case beadBranch:
			return "parent-tip", nil
		case markerOID + "^":
			return "parent-tip", nil // tip == marker's parent
		}
		return "", fmt.Errorf("unexpected RevParseRef(%s)", ref)
	}
	var cleared, advanced bool
	mock.ClearStageMarkerFn = func(id, expectedOID string) error {
		cleared = true
		return nil
	}
	mock.AdvanceBranchExpectedOldFn = func(branch, newSHA, expectedOld string) error {
		advanced = true
		return nil
	}
	completeGetMetadataFn = func(id string) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}

	stage, err := resolveMaterializationDiscriminator(mock, root, beadID, beadBranch)
	if err != nil {
		t.Fatalf("expected foreign-clear, got error: %v", err)
	}
	if stage != "" || !cleared {
		t.Errorf("unattested parent-leg marker must clear and run normally, got stage=%q cleared=%v", stage, cleared)
	}
	if advanced {
		t.Error("AdvanceBranchExpectedOld ran for an unattested marker — a forgery pulled the branch ref")
	}
}

// TestStageAttestationSeamDefaults is the anti-drift pointer pin (the
// spec-125 F3-2 house pattern): the attestation rides the package's
// metadata seam pair, whose production defaults must remain the real
// bead metadata functions — the provenance gate cannot go hollow via a
// forgotten stub.
func TestStageAttestationSeamDefaults(t *testing.T) {
	// NOTE: saveAndRestore is deliberately NOT called — this reads the
	// package defaults as declared (every mutating test restores them
	// via its own cleanup before this one can observe).
	if reflect.ValueOf(completeMergeMetadataFn).Pointer() != reflect.ValueOf(bead.MergeMetadata).Pointer() {
		t.Error("completeMergeMetadataFn's production default drifted off bead.MergeMetadata")
	}
	if reflect.ValueOf(completeGetMetadataFn).Pointer() != reflect.ValueOf(bead.GetMetadata).Pointer() {
		t.Error("completeGetMetadataFn's production default drifted off bead.GetMetadata")
	}
}
