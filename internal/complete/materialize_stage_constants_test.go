package complete

// Spec 128 Bead 6, round-1 panel fix round (G7): materialize.go's own
// doc comment (lines ~34-36) cites this file as the pin proving
// stageMaterialization/stageArtifactSync (duplicated here per ADR-0030,
// since internal/complete may never import internal/gitutil) stay equal
// to gitutil.StageMaterialization/StageArtifactSync's own values. That
// file existed at 482bcd2f only in citation — never on disk. This closes
// it: a test file, unlike materialize.go itself, may import
// internal/gitutil for comparison purposes without violating ADR-0030
// (the boundary is a PRODUCTION-code import restriction, not a
// same-repo pinning-test restriction).

import (
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/gitutil"
)

func TestMaterializeStageConstants_MirrorGitutilExactly(t *testing.T) {
	if stageMaterialization != gitutil.StageMaterialization {
		t.Errorf("stageMaterialization %q != gitutil.StageMaterialization %q", stageMaterialization, gitutil.StageMaterialization)
	}
	if stageArtifactSync != gitutil.StageArtifactSync {
		t.Errorf("stageArtifactSync %q != gitutil.StageArtifactSync %q", stageArtifactSync, gitutil.StageArtifactSync)
	}
}
