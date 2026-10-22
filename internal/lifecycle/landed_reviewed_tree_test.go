package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

const reviewedTreeSpec = "spec/132-reviewed"
const reviewedTreeArtifact = ".mindspec/specs/132-reviewed/reviews/round-one/dispositions.jsonl"

type reviewedTreeFixture struct {
	dir, reviewed, second, merge string
	run                          func(...string)
}

func newReviewedTreeFixture(t *testing.T, change string) reviewedTreeFixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "/usr/bin/false")
	dir, run := initLandedRepo(t, "132-reviewed")
	writeReviewedTreeFile(t, dir, ".mindspec/specs/132-reviewed/spec.md", "# Fixture\n")
	run("add", "-A")
	run("commit", "-m", "spec document")
	run("checkout", "-b", "bead/"+fallbackBead)
	writeReviewedTreeFile(t, dir, "source.txt", "reviewed source\n")
	run("add", "-A")
	run("commit", "-m", "reviewed work")
	reviewed := revParseIn(t, dir, "HEAD")
	switch change {
	case "empty":
		run("commit", "--allow-empty", "-m", "empty wrapper")
	case "source":
		writeReviewedTreeFile(t, dir, reviewedTreeArtifact, "{}\n")
		writeReviewedTreeFile(t, dir, "source.txt", "unreviewed source\n")
	case "rename":
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, reviewedTreeArtifact)), 0755); err != nil {
			t.Fatal(err)
		}
		run("mv", "source.txt", reviewedTreeArtifact)
	case "other-spec":
		writeReviewedTreeFile(t, dir, ".mindspec/specs/999-foreign/reviews/round-one/dispositions.jsonl", "{}\n")
	default:
		writeReviewedTreeFile(t, dir, reviewedTreeArtifact, "{}\n")
		writeReviewedTreeFile(t, dir, ".beads/issues.jsonl", "{}\n")
	}
	if change != "empty" {
		run("add", "-A")
		run("commit", "-m", "wrapper")
	}
	second := revParseIn(t, dir, "HEAD")
	run("checkout", reviewedTreeSpec)
	run("merge", "--no-ff", "-m", "Merge bead/"+fallbackBead, "bead/"+fallbackBead)
	merge := revParseIn(t, dir, "HEAD")
	run("branch", "-D", "bead/"+fallbackBead)
	return reviewedTreeFixture{dir: dir, reviewed: reviewed, second: second, merge: merge, run: run}
}

func writeReviewedTreeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	file := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func requireReviewedTreeNoConfirmation(t *testing.T, f reviewedTreeFixture) error {
	t.Helper()
	got, err := FindLandedMerge(f.dir, reviewedTreeSpec, fallbackBead)
	var no *LandedMergeNoEvidence
	if got != nil || !errors.As(err, &no) || no.MergeSHA != f.merge || no.SecondParent != f.second {
		t.Fatalf("unreviewed tree must not confirm: %+v %v", got, err)
	}
	return err
}

func TestFindLandedMerge_ReviewedTreeEmptyWrapperConfirms(t *testing.T) {
	f := newReviewedTreeFixture(t, "empty")
	registerFallbackPanel(t, f.dir, "reviewed-tree", f.reviewed, 1)
	got, err := FindLandedMerge(f.dir, reviewedTreeSpec, fallbackBead)
	if err != nil || got == nil || got.SHA != f.merge || got.SecondParent != f.second || !strings.Contains(got.Provenance, "reviewed-tree") {
		t.Fatalf("empty wrapper must confirm with its rule: %+v %v", got, err)
	}
}

func TestFindLandedMerge_ReviewedTreeSourceDiffNoConfirm(t *testing.T) {
	f := newReviewedTreeFixture(t, "source")
	registerFallbackPanel(t, f.dir, "reviewed-tree", f.reviewed, 1)
	requireReviewedTreeNoConfirmation(t, f)
}

func TestFindLandedMerge_ReviewedTreeRenameNoConfirm(t *testing.T) {
	f := newReviewedTreeFixture(t, "rename")
	registerFallbackPanel(t, f.dir, "reviewed-tree", f.reviewed, 1)
	requireReviewedTreeNoConfirmation(t, f)
}

func TestFindLandedMerge_ReviewedTreeOtherSpecDirNoConfirm(t *testing.T) {
	f := newReviewedTreeFixture(t, "other-spec")
	registerFallbackPanel(t, f.dir, "reviewed-tree", f.reviewed, 1)
	requireReviewedTreeNoConfirmation(t, f)
}

func TestFindLandedMerge_UnregisteredKeyNoConfirm(t *testing.T) {
	for _, key := range []string{"reviewed", "second-parent", "malformed"} {
		t.Run(key, func(t *testing.T) {
			f := newReviewedTreeFixture(t, "artifacts")
			sha := f.reviewed
			if key == "second-parent" {
				sha = f.second
			} else if key == "malformed" {
				sha = "--hostile\x1b\n"
			}
			stubBindingRead(t, map[string]interface{}{"mindspec_landed_reviewed_sha": sha})
			if err := requireReviewedTreeNoConfirmation(t, f); !strings.Contains(err.Error(), "unverified") || strings.ContainsAny(err.Error(), "\x1b\n") {
				t.Fatalf("unregistered key provenance: %v", err)
			}
		})
	}
}

func TestReattestFallback_IgnoresPanelDatum(t *testing.T) {
	f := newFallbackFixture(t)
	registerFallbackPanel(t, f.dir, "exact", f.second, 1)
	registerFallbackPanel(t, f.dir, "ancestor", revParseIn(t, f.dir, f.second+"^1"), 2)
	// Automatic fallback must not read the panel datum at all.
	old := landedPanelScanFn
	t.Cleanup(func() { landedPanelScanFn = old })
	landedPanelScanFn = func(...string) []panel.Registration {
		t.Fatal("fallback evaluated panel datum")
		return nil
	}
	noDatumScan := landedPanelScanFn
	requireOffFirstParent(t, f, 1)
	// Reattest may scan registrations for advisory output.
	landedPanelScanFn = old
	writes := installReattestRecorder(t)
	result, err := ReattestLandedMerge(f.dir, fallbackSpec, fallbackBead, "fixture-actor")
	if err != nil || result == nil || !result.Wrote || len(*writes) != 1 || !strings.HasPrefix(result.Corroboration, "off-first-parent fallback;") || strings.Contains(result.Corroboration, "(c)") || strings.Contains(result.Corroboration, "reviewed-tree") {
		t.Fatalf("fallback must use audited git checks only: %+v %v writes=%v", result, err, *writes)
	}
	stubBindingRead(t, (*writes)[0].updates)
	landedPanelScanFn = noDatumScan
	requireReadBack(t, f)
}

func TestFindLandedMerge_ReviewedTreeKeepsSafetyChecks(t *testing.T) {
	for _, state := range []string{"tip-contradiction", "binding-contradiction", "reverted"} {
		t.Run(state, func(t *testing.T) {
			f := newReviewedTreeFixture(t, "artifacts")
			registerFallbackPanel(t, f.dir, "reviewed-tree", f.reviewed, 1)
			want := ErrLandedMergeContradiction
			switch state {
			case "tip-contradiction":
				f.run("branch", "bead/"+fallbackBead, f.reviewed)
			case "binding-contradiction":
				stubBindingRead(t, map[string]interface{}{"mindspec_landed_merge_sha": f.merge, "mindspec_landed_second_parent": f.reviewed})
			case "reverted":
				f.run("revert", "--no-edit", "-m", "1", f.merge)
				want = ErrLandedMergeReverted
			}
			got, err := FindLandedMerge(f.dir, reviewedTreeSpec, fallbackBead)
			if got != nil || !errors.Is(err, want) {
				t.Fatalf("review datum bypassed safety check: %+v %v want %v", got, err, want)
			}
		})
	}
}

func TestFindLandedMerge_ReviewedTreeExactAndRegisteredKey(t *testing.T) {
	f := newReviewedTreeFixture(t, "artifacts")
	registerFallbackPanel(t, f.dir, "exact", f.second, 1)
	stubBindingRead(t, map[string]interface{}{"mindspec_landed_reviewed_sha": f.second})
	got, err := FindLandedMerge(f.dir, reviewedTreeSpec, fallbackBead)
	if err != nil || got == nil || got.Provenance != "panel exact" {
		t.Fatalf("registered exact provenance: %+v %v", got, err)
	}
}

func TestFindLandedMerge_ReviewedTreeUnavailableReviewNeverVetoes(t *testing.T) {
	f := newReviewedTreeFixture(t, "artifacts")
	registerFallbackPanel(t, f.dir, "missing-object", strings.Repeat("f", 40), 1)
	stubBindingRead(t, map[string]interface{}{"mindspec_landed_reviewed_sha": f.second})
	f.run("branch", "bead/"+fallbackBead, f.second)
	got, err := FindLandedMerge(f.dir, reviewedTreeSpec, fallbackBead)
	if err != nil || got == nil || !strings.Contains(got.Provenance, "surviving tip exact") || !strings.Contains(got.Provenance, "unverified") || strings.Contains(got.Provenance, "reviewed-tree") {
		t.Fatalf("unavailable review must not veto the tip: %+v %v", got, err)
	}
	f.run("branch", "-D", "bead/"+fallbackBead)
	requireReviewedTreeNoConfirmation(t, f)
}
