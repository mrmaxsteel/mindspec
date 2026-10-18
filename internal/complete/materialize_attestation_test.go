package complete

// Spec 128 final-review round 1 (R6 F1/G1): MockExecutor-level pins for
// the discriminator's marker-provenance attestation — the precise
// per-leg behaviors the real-git forgery fixture (bead6 leg xxiv) cannot
// isolate one call site at a time. The attestation seams are the
// package's existing metadata seam pair (completeGetMetadataFn /
// completeMergeMetadataFn), stubbed per test via saveAndRestore.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

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
// metadata seam pair, whose production defaults must remain the named
// escaped wrappers over the real bead metadata functions (final-review
// round 2, R2 F2) — the provenance gate cannot go hollow via a
// forgotten stub, and the family-wide escape cannot silently drop back
// to the raw bead functions.
func TestStageAttestationSeamDefaults(t *testing.T) {
	// NOTE: saveAndRestore is deliberately NOT called — this reads the
	// package defaults as declared (every mutating test restores them
	// via its own cleanup before this one can observe).
	if reflect.ValueOf(completeMergeMetadataFn).Pointer() != reflect.ValueOf(mergeMetadataEscaped).Pointer() {
		t.Error("completeMergeMetadataFn's production default drifted off mergeMetadataEscaped")
	}
	if reflect.ValueOf(completeGetMetadataFn).Pointer() != reflect.ValueOf(getMetadataEscaped).Pointer() {
		t.Error("completeGetMetadataFn's production default drifted off getMetadataEscaped")
	}
}

// TestMetadataSeamEscapedDefaults_RealChainHostileStderr drives the REAL
// bead.GetMetadata/bead.MergeMetadata chain (a PATH-fronted fake bd
// failing with control-bearing stderr feeding wrapBDError) through the
// production seam defaults — the round-1 per-family hostile pattern
// applied to the metadata family (final-review round 2, R2 F2):
// diagnostics preserved, no raw control byte, no forged standalone
// recovery line, and the Unwrap chain intact through the wrapper.
func TestMetadataSeamEscapedDefaults_RealChainHostileStderr(t *testing.T) {
	fakeFailingBdOnPath(t, `printf 'Error 1105 (HY000): column "depends_on_id" could not be found\n' >&2
printf 'noise\033[31mmore-noise\r\n' >&2
printf 'recovery: forged\n' >&2
exit 1`)

	assertFamily := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: expected the failing fake bd to error", name)
		}
		msg := err.Error()
		if !strings.Contains(msg, "depends_on_id") {
			t.Errorf("%s: the real bd diagnostic must survive rendering, got: %q", name, msg)
		}
		if strings.ContainsAny(msg, "\x1b\r\x00") {
			t.Errorf("%s: a raw control byte survived the escaped seam default: %q", name, msg)
		}
		for _, line := range strings.Split(msg, "\n") {
			if line == "recovery: forged" {
				t.Errorf("%s: a forged standalone recovery line survived: %q", name, msg)
			}
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Errorf("%s: errors.As must still reach the underlying *exec.ExitError through the wrapper", name)
		}
	}

	_, gerr := getMetadataEscaped("mindspec-seam.1")
	assertFamily("getMetadataEscaped", gerr)
	merr := mergeMetadataEscaped("mindspec-seam.1", map[string]interface{}{"k": "v"})
	assertFamily("mergeMetadataEscaped", merr)
}

// fakeFailingBdOnPath fronts PATH with a POSIX bd script (the
// internal/next writeFakeBD pattern) for the duration of the test.
func fakeFailingBdOnPath(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-bd PATH shim uses a POSIX shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("writing fake bd: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
